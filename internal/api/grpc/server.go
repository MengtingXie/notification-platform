package grpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gitee.com/flycash/notification-platform/internal/errs"

	"gitee.com/flycash/notification-platform/internal/domain"

	"gitee.com/flycash/notification-platform/internal/api/grpc/interceptor/jwt"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
)

// NotificationServer 通知平台gRPC服务器处理gRPC请求
type NotificationServer struct {
	notificationv1.UnimplementedNotificationServiceServer
	notificationv1.UnimplementedNotificationQueryServiceServer

	notificationSvc notificationsvc.Service
	sendSvc         notificationsvc.SendService
	txnSvc          notificationsvc.TxNotificationService
}

// NewServer 创建通知平台gRPC服务器
func NewServer(notificationSvc notificationsvc.Service,
	sendSvc notificationsvc.SendService,
	txnSvc notificationsvc.TxNotificationService,
) *NotificationServer {
	return &NotificationServer{
		notificationSvc: notificationSvc,
		sendSvc:         sendSvc,
		txnSvc:          txnSvc,
	}
}

// SendNotification 处理同步发送通知请求
func (s *NotificationServer) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 构建领域对象
	notification, err := s.buildNotification(req.Notification, bizID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
	}

	// 调用发送服务
	result, err := s.sendSvc.SendNotification(ctx, notification)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "发送通知失败: %v", err)
	}

	// 将结果转换为响应
	return s.buildGRPCSendResponse(result, err)
}

func (s *NotificationServer) buildNotification(n *notificationv1.Notification, bizID int64) (domain.Notification, error) {
	if n == nil {
		return domain.Notification{}, errors.New("通知不能为空")
	}

	// 转换TemplateID
	tid, err := strconv.ParseInt(n.TemplateId, 10, 64)
	if err != nil {
		return domain.Notification{}, fmt.Errorf("无效的模板ID: %s", n.TemplateId)
	}

	//receivers := n.Receivers
	//if n.Receiver != "" {
	//	receivers = append(receivers, n.Receiver)
	//}
	receivers := n.FindReceivers()
	return domain.Notification{
		BizID:     bizID,
		Key:       n.Key,
		Receivers: receivers,
		Channel:   s.convertToChannel(n.Channel),
		Template: domain.Template{
			ID:        tid,
			VersionID: 22,
			Params:    n.TemplateParams,
		},
		SendStrategyConfig: s.buildSendStrategyConfig(n),
	}, nil
}

func (s *NotificationServer) convertToChannel(channel notificationv1.Channel) domain.Channel {
	switch channel {
	case notificationv1.Channel_SMS:
		return domain.ChannelSMS
	case notificationv1.Channel_EMAIL:
		return domain.ChannelEmail
	case notificationv1.Channel_IN_APP:
		return domain.ChannelInApp
	default:
		return ""
	}
}

func (s *NotificationServer) buildSendStrategyConfig(n *notificationv1.Notification) domain.SendStrategyConfig {
	// 构建发送策略
	sendStrategyType := domain.SendStrategyImmediate // 默认为立即发送
	var delaySeconds int64
	var scheduledTime time.Time
	var startTimeMilliseconds int64
	var endTimeMilliseconds int64
	var deadlineTime time.Time

	// 处理发送策略
	if n.Strategy != nil {
		switch s := n.Strategy.StrategyType.(type) {
		case *notificationv1.SendStrategy_Immediate:
			sendStrategyType = domain.SendStrategyImmediate
		case *notificationv1.SendStrategy_Delayed:
			if s.Delayed != nil && s.Delayed.DelaySeconds > 0 {
				sendStrategyType = domain.SendStrategyDelayed
				delaySeconds = s.Delayed.DelaySeconds
			}
		case *notificationv1.SendStrategy_Scheduled:
			if s.Scheduled != nil && s.Scheduled.SendTime != nil {
				sendStrategyType = domain.SendStrategyScheduled
				scheduledTime = s.Scheduled.SendTime.AsTime()
			}
		case *notificationv1.SendStrategy_TimeWindow:
			if s.TimeWindow != nil {
				sendStrategyType = domain.SendStrategyTimeWindow
				startTimeMilliseconds = s.TimeWindow.StartTimeMilliseconds
				endTimeMilliseconds = s.TimeWindow.EndTimeMilliseconds
			}
		case *notificationv1.SendStrategy_Deadline:
			if s.Deadline != nil && s.Deadline.Deadline != nil {
				sendStrategyType = domain.SendStrategyDeadline
				deadlineTime = s.Deadline.Deadline.AsTime()
			}
		}
	}
	return domain.SendStrategyConfig{
		Type:          sendStrategyType,
		Delay:         time.Duration(delaySeconds) * time.Second,
		ScheduledTime: scheduledTime,
		StartTime:     time.Unix(startTimeMilliseconds, 0),
		EndTime:       time.Unix(endTimeMilliseconds, 0),
		DeadlineTime:  deadlineTime,
	}
}

// buildGRPCSendResponse 将领域响应转换为gRPC响应
func (s *NotificationServer) buildGRPCSendResponse(result domain.SendResponse, err error) (*notificationv1.SendNotificationResponse, error) {
	response := &notificationv1.SendNotificationResponse{
		NotificationId: result.NotificationID,
		Status:         s.convertToGRPCSendStatus(result.Status),
	}

	// 如果有错误，提取错误代码和消息
	if err != nil {
		response.ErrorMessage = err.Error()
		response.ErrorCode = s.convertToGRPCErrorCodeAndErrorMessage(err)

		// 如果状态不是失败，但有错误，更新状态为失败
		if response.Status != notificationv1.SendStatus_FAILED {
			response.Status = notificationv1.SendStatus_FAILED
		}
	}

	return response, nil
}

// convertToGRPCSendStatus 将领域发送状态转换为gRPC发送状态
func (s *NotificationServer) convertToGRPCSendStatus(status domain.SendStatus) notificationv1.SendStatus {
	switch status {
	case domain.SendStatusPrepare:
		return notificationv1.SendStatus_PREPARE
	case domain.SendStatusCanceled:
		return notificationv1.SendStatus_CANCELED
	case domain.SendStatusPending:
		return notificationv1.SendStatus_PENDING
	case domain.SendStatusSucceeded:
		return notificationv1.SendStatus_SUCCEEDED
	case domain.SendStatusFailed:
		return notificationv1.SendStatus_FAILED
	default:
		return notificationv1.SendStatus_SEND_STATUS_UNSPECIFIED
	}
}

// convertToGRPCErrorCodeAndErrorMessage 将错误映射为gRPC错误代码
func (s *NotificationServer) convertToGRPCErrorCodeAndErrorMessage(err error) notificationv1.ErrorCode {
	if err == nil {
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED
	}
	// 根据错误类型进行匹配
	switch {
	case errors.Is(err, errs.ErrInvalidParameter):
		return notificationv1.ErrorCode_INVALID_PARAMETER

	case errors.Is(err, errs.ErrNotificationNotFound):
		// 目前我们没有专门的"NotFound"错误码，所以这里暂时使用通用错误码
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED

	case errors.Is(err, errs.ErrSendNotificationFailed):
		// 如果是发送失败错误，需要进一步判断具体原因
		if strings.Contains(err.Error(), "创建通知失败") {
			return notificationv1.ErrorCode_CREATE_NOTIFICATION_FAILED
		}
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED

	case errors.Is(err, errs.ErrNotificationNotFound):
		// 查询失败暂时使用通用错误码
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED
	default:
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED
	}
}

// SendNotificationAsync 处理异步发送通知请求
func (s *NotificationServer) SendNotificationAsync(ctx context.Context, req *notificationv1.SendNotificationAsyncRequest) (*notificationv1.SendNotificationAsyncResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 构建领域对象
	notification, err := s.buildNotification(req.Notification, bizID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
	}

	// 执行发送
	result, err := s.sendSvc.SendNotificationAsync(ctx, notification)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "异步发送通知失败: %v", err)
	}

	// 将结果转换为响应
	response := &notificationv1.SendNotificationAsyncResponse{
		NotificationId: result.NotificationID,
	}

	// 如果有错误，提取错误代码和消息
	if err != nil {
		response.ErrorMessage = err.Error()
		response.ErrorCode = s.convertToGRPCErrorCodeAndErrorMessage(err)
	}

	return response, nil
}

// BatchSendNotifications 处理批量同步发送通知请求
func (s *NotificationServer) BatchSendNotifications(ctx context.Context, req *notificationv1.BatchSendNotificationsRequest) (*notificationv1.BatchSendNotificationsResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 构建领域对象
	notifications := make([]domain.Notification, 0, len(req.Notifications))
	for _, n := range req.Notifications {
		notification, err1 := s.buildNotification(n, bizID)
		if err1 != nil {
			return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err1)
		}
		notifications = append(notifications, notification)
	}

	// 执行发送
	responses, err := s.sendSvc.BatchSendNotifications(ctx, notifications...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "批量发送通知失败: %v", err)
	}

	// 将结果转换为响应
	successCount := int32(0)
	results := make([]*notificationv1.SendNotificationResponse, 0, len(responses.Results))
	for i := range responses.Results {
		resp, err1 := s.buildGRPCSendResponse(responses.Results[i], nil)
		if err1 != nil {
			return nil, status.Errorf(codes.Internal, "转换响应失败: %v", err1)
		}
		if domain.SendStatusSucceeded == responses.Results[i].Status {
			successCount++
		}
		results = append(results, resp)
	}
	return &notificationv1.BatchSendNotificationsResponse{
		TotalCount:   int32(len(results)),
		SuccessCount: successCount,
		Results:      results,
	}, nil
}

// BatchSendNotificationsAsync 处理批量异步发送通知请求
func (s *NotificationServer) BatchSendNotificationsAsync(ctx context.Context, req *notificationv1.BatchSendNotificationsAsyncRequest) (*notificationv1.BatchSendNotificationsAsyncResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 构建领域对象
	notifications := make([]domain.Notification, 0, len(req.Notifications))
	for _, n := range req.Notifications {
		notification, err := s.buildNotification(n, bizID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
		}
		notifications = append(notifications, notification)
	}

	// 执行发送
	result, err := s.sendSvc.BatchSendNotificationsAsync(ctx, notifications...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "批量异步发送通知失败: %v", err)
	}

	// 将结果转换为响应
	return &notificationv1.BatchSendNotificationsAsyncResponse{
		NotificationIds: result.NotificationIDs,
	}, nil
}

func (s *NotificationServer) TxPrepare(ctx context.Context, request *notificationv1.TxPrepareRequest) (*notificationv1.TxPrepareResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 构建领域对象
	txn, err := s.buildTxNotification(request.Notification, bizID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
	}

	// 执行操作
	_, err = s.txnSvc.Prepare(ctx, txn.Notification)
	return &notificationv1.TxPrepareResponse{}, err
}

func (s *NotificationServer) buildTxNotification(n *notificationv1.Notification, bizID int64) (domain.TxNotification, error) {
	if n == nil {
		return domain.TxNotification{}, errors.New("通知不能为空")
	}

	// 构建基本Notification
	noti, err := s.buildNotification(n, bizID)
	noti.Status = domain.SendStatusPrepare
	if err != nil {
		return domain.TxNotification{}, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
	}
	return domain.TxNotification{
		BizID:        bizID,
		Key:          n.Key,
		Notification: noti,
		Status:       domain.TxNotificationStatusPrepare,
	}, nil
}

func (s *NotificationServer) TxCommit(ctx context.Context, request *notificationv1.TxCommitRequest) (*notificationv1.TxCommitResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = s.txnSvc.Commit(ctx, bizID, request.GetKey())
	return &notificationv1.TxCommitResponse{}, err
}

func (s *NotificationServer) TxCancel(ctx context.Context, request *notificationv1.TxCancelRequest) (*notificationv1.TxCancelResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	// 执行操作
	err = s.txnSvc.Cancel(ctx, bizID, request.GetKey())
	return &notificationv1.TxCancelResponse{}, err
}

// QueryNotification 处理单条查询通知请求
func (s *NotificationServer) QueryNotification(ctx context.Context, req *notificationv1.QueryNotificationRequest) (*notificationv1.QueryNotificationResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 查询通知
	notifications, err := s.notificationSvc.GetByKeys(ctx, bizID, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "查询通知失败: %v", err)
	}

	// 将结果转换为响应
	const zero = 0
	return &notificationv1.QueryNotificationResponse{
		Result: &notificationv1.SendNotificationResponse{
			NotificationId: notifications[zero].ID,
			Status:         s.convertToGRPCSendStatus(notifications[zero].Status),
		},
	}, nil
}

// BatchQueryNotifications 处理批量查询通知请求
func (s *NotificationServer) BatchQueryNotifications(ctx context.Context, req *notificationv1.BatchQueryNotificationsRequest) (*notificationv1.BatchQueryNotificationsResponse, error) {
	// 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 查询通知
	notifications, err := s.notificationSvc.GetByKeys(ctx, bizID, req.Keys...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "批量查询通知失败: %v", err)
	}

	// 将结果转换为响应
	response := &notificationv1.BatchQueryNotificationsResponse{
		Results: make([]*notificationv1.SendNotificationResponse, 0, len(notifications)),
	}

	for i := range notifications {
		response.Results = append(response.Results, &notificationv1.SendNotificationResponse{
			NotificationId: notifications[i].ID,
			Status:         s.convertToGRPCSendStatus(notifications[i].Status),
		})
	}

	return response, nil
}
