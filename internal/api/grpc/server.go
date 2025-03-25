package grpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	v1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification/service"
	"github.com/sony/sonyflake"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NotificationServer 处理通知平台的gRPC请求
type NotificationServer struct {
	v1.UnimplementedNotificationServiceServer
	notificationSvc notificationsvc.NotificationService
	idGenerator     *sonyflake.Sonyflake
}

// NewServer 创建通知平台gRPC服务器
func NewServer(notificationSvc notificationsvc.NotificationService, idGenerator *sonyflake.Sonyflake) *NotificationServer {
	return &NotificationServer{
		notificationSvc: notificationSvc,
		idGenerator:     idGenerator,
	}
}

// SendNotification 处理同步发送通知请求
func (s *NotificationServer) SendNotification(ctx context.Context, req *v1.SendNotificationRequest) (*v1.SendNotificationResponse, error) {
	resp := &v1.SendNotificationResponse{
		RequestKey: req.Key,
		Status:     v1.SendStatus_PENDING,
	}

	// 验证参数
	if req.BizId == "" {
		resp.Status = v1.SendStatus_FAILED
		resp.ErrorCode = v1.ErrorCode_INVALID_PARAMETER
		resp.ErrorMessage = "业务ID不能为空"
		return resp, nil
	}

	if req.Receiver == "" {
		resp.Status = v1.SendStatus_FAILED
		resp.ErrorCode = v1.ErrorCode_INVALID_PARAMETER
		resp.ErrorMessage = "接收者不能为空"
		return resp, nil
	}

	// 转换Channel
	var channel domain.Channel
	switch req.Channel {
	case v1.Channel_SMS:
		channel = domain.ChannelSMS
	case v1.Channel_EMAIL:
		channel = domain.ChannelEmail
	case v1.Channel_IN_APP:
		channel = domain.ChannelInApp
	default:
		resp.Status = v1.SendStatus_FAILED
		resp.ErrorCode = v1.ErrorCode_INVALID_PARAMETER
		resp.ErrorMessage = "不支持的通知渠道"
		return resp, nil
	}

	// 转换TemplateID
	templateID, err := strconv.ParseInt(req.TemplateId, 10, 64)
	if err != nil {
		resp.Status = v1.SendStatus_FAILED
		resp.ErrorCode = v1.ErrorCode_INVALID_PARAMETER
		resp.ErrorMessage = "无效的模板ID: " + err.Error()
		return resp, nil
	}

	// 构建模板参数的内容字符串 (简化处理)
	var content string
	if len(req.TemplateParams) > 0 {
		var parts []string
		for k, v := range req.TemplateParams {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		content = strings.Join(parts, ";")
	}

	// 处理发送策略
	now := time.Now()
	scheduledSTime := now.Unix()
	scheduledETime := now.Add(24 * time.Hour).Unix()

	if req.Strategy != nil {
		switch strategyType := req.Strategy.StrategyType.(type) {
		case *v1.SendStrategy_Delayed:
			if strategyType.Delayed != nil && strategyType.Delayed.DelaySeconds > 0 {
				delayDuration := time.Duration(strategyType.Delayed.DelaySeconds) * time.Second
				scheduledTime := now.Add(delayDuration)
				scheduledSTime = scheduledTime.Unix()
				scheduledETime = scheduledTime.Add(24 * time.Hour).Unix()
			}
		case *v1.SendStrategy_Scheduled:
			if strategyType.Scheduled != nil && strategyType.Scheduled.SendTime != nil {
				scheduledTime := strategyType.Scheduled.SendTime.AsTime()
				if scheduledTime.After(now) {
					scheduledSTime = scheduledTime.Unix()
					scheduledETime = scheduledTime.Add(24 * time.Hour).Unix()
				}
			}
		case *v1.SendStrategy_TimeWindow:
			if strategyType.TimeWindow != nil {
				startMs := strategyType.TimeWindow.StartTimeMilliseconds
				endMs := strategyType.TimeWindow.EndTimeMilliseconds
				if startMs > 0 && endMs > startMs {
					scheduledSTime = startMs / 1000 // 毫秒转秒
					scheduledETime = endMs / 1000   // 毫秒转秒
				}
			}
		}
	}

	// 构建notification对象
	id, err := s.idGenerator.NextID()
	if err != nil {
		return nil, fmt.Errorf("通知ID生成失败: %w", err)
	}
	notification := domain.Notification{
		ID:             id,
		BizID:          req.BizId,
		Receiver:       req.Receiver,
		Channel:        channel,
		TemplateID:     templateID,
		Content:        content,
		ScheduledSTime: scheduledSTime,
		ScheduledETime: scheduledETime,
	}

	// 调用服务发送通知
	sentNotification, err := s.notificationSvc.SendNotification(ctx, notification)
	if err != nil {
		// 处理业务错误
		resp.Status = v1.SendStatus_FAILED
		resp.ErrorMessage = err.Error()

		// 映射错误代码
		switch {
		case errors.Is(err, notificationsvc.ErrChannelDisabled):
			resp.ErrorCode = v1.ErrorCode_CHANNEL_DISABLED
		case errors.Is(err, notificationsvc.ErrInvalidParameter):
			resp.ErrorCode = v1.ErrorCode_INVALID_PARAMETER
		case errors.Is(err, notificationsvc.ErrNotificationNotFound):
			resp.ErrorCode = v1.ErrorCode_ERROR_CODE_UNSPECIFIED // 如果proto中没有NOT_FOUND，使用未指定错误码
		default:
			resp.ErrorCode = v1.ErrorCode_ERROR_CODE_UNSPECIFIED
		}
		return resp, nil
	}

	// 设置响应
	resp.NotificationId = fmt.Sprintf("%d", sentNotification.ID)
	log.Printf("%#v\n", sentNotification)
	resp.Status = mapStatusToProtoStatus(sentNotification.Status)
	resp.SendTime = timestamppb.New(time.Unix(sentNotification.Utime, 0))

	return resp, nil
}

// 将domain.Status映射到v1.SendStatus
func mapStatusToProtoStatus(status domain.Status) v1.SendStatus {
	switch status {
	case domain.StatusPending:
		return v1.SendStatus_PENDING
	case domain.StatusSucceeded:
		return v1.SendStatus_SUCCEEDED
	case domain.StatusFailed:
		return v1.SendStatus_FAILED
	case domain.StatusPrepare:
		return v1.SendStatus_PREPARE
	case domain.StatusCanceled:
		return v1.SendStatus_CANCELED
	default:
		return v1.SendStatus_SEND_STATUS_UNSPECIFIED
	}
}

// SendNotificationAsync 处理异步发送通知请求
func (s *NotificationServer) SendNotificationAsync(ctx context.Context, req *v1.SendNotificationAsyncRequest) (*v1.SendNotificationAsyncResponse, error) {
	// TODO implement me
	panic("implement me")
}

// BatchSendNotifications 处理批量同步发送通知请求
func (s *NotificationServer) BatchSendNotifications(ctx context.Context, req *v1.BatchSendNotificationsRequest) (*v1.BatchSendNotificationsResponse, error) {
	// TODO implement me
	panic("implement me")
}

// BatchSendNotificationsAsync 处理批量异步发送通知请求
func (s *NotificationServer) BatchSendNotificationsAsync(ctx context.Context, req *v1.BatchSendNotificationsAsyncRequest) (*v1.BatchSendNotificationsAsyncResponse, error) {
	// TODO implement me
	panic("implement me")
}

func (s *NotificationServer) QueryNotification(ctx context.Context, request *v1.QueryNotificationRequest) (*v1.QueryNotificationResponse, error) {
	// TODO implement me
	panic("implement me")
}
