package grpc

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
	"gitee.com/flycash/notification-platform/internal/service/executor/service"
)

// NotificationServer 处理通知平台的gRPC请求
type NotificationServer struct {
	notificationv1.UnimplementedNotificationServiceServer
	executor service.ExecutorService
	// TODO: 配置服务 configService config.ConfigService
}

// NewServer 创建通知平台gRPC服务器
func NewServer(executor service.ExecutorService) *NotificationServer {
	return &NotificationServer{
		executor: executor,
	}
}

// SendNotification 处理同步发送通知请求
func (s *NotificationServer) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := s.extractAndValidateBizID(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 将请求转换为领域对象
	notification, err := s.convertToNotification(req.Notification, bizID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
	}

	// 3. 调用执行器
	result, err := s.executor.SendNotification(ctx, notification)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "发送通知失败: %v", err)
	}

	// 4. 将结果转换为响应
	return s.convertToSendResponse(result)
}

// SendNotificationAsync 处理异步发送通知请求
func (s *NotificationServer) SendNotificationAsync(ctx context.Context, req *notificationv1.SendNotificationAsyncRequest) (*notificationv1.SendNotificationAsyncResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := s.extractAndValidateBizID(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 将请求转换为领域对象
	notification, err := s.convertToNotification(req.Notification, bizID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
	}

	// 3. 调用执行器
	result, err := s.executor.SendNotificationAsync(ctx, notification)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "异步发送通知失败: %v", err)
	}

	// 4. 将结果转换为响应
	return &notificationv1.SendNotificationAsyncResponse{
		NotificationId: result.NotificationID,
		ErrorCode:      convertErrorCode(result.ErrorCode),
		ErrorMessage:   result.ErrorMessage,
	}, nil
}

// BatchSendNotifications 处理批量同步发送通知请求
func (s *NotificationServer) BatchSendNotifications(ctx context.Context, req *notificationv1.BatchSendNotificationsRequest) (*notificationv1.BatchSendNotificationsResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := s.extractAndValidateBizID(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 将请求转换为领域对象
	notifications := make([]service.Notification, 0, len(req.Notifications))
	for _, n := range req.Notifications {
		notification, err := s.convertToNotification(n, bizID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
		}
		notifications = append(notifications, notification)
	}

	// 3. 调用执行器
	result, err := s.executor.BatchSendNotifications(ctx, notifications...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "批量发送通知失败: %v", err)
	}

	// 4. 将结果转换为响应
	response := &notificationv1.BatchSendNotificationsResponse{
		TotalCount:   int32(result.TotalCount),
		SuccessCount: int32(result.SuccessCount),
		Results:      make([]*notificationv1.SendNotificationResponse, 0, len(result.Results)),
	}

	for _, r := range result.Results {
		resp, err := s.convertToSendResponse(r)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "转换响应失败: %v", err)
		}
		response.Results = append(response.Results, resp)
	}

	return response, nil
}

// BatchSendNotificationsAsync 处理批量异步发送通知请求
func (s *NotificationServer) BatchSendNotificationsAsync(ctx context.Context, req *notificationv1.BatchSendNotificationsAsyncRequest) (*notificationv1.BatchSendNotificationsAsyncResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := s.extractAndValidateBizID(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 将请求转换为领域对象
	notifications := make([]service.Notification, 0, len(req.Notifications))
	for _, n := range req.Notifications {
		notification, err := s.convertToNotification(n, bizID)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
		}
		notifications = append(notifications, notification)
	}

	// 3. 调用执行器
	result, err := s.executor.BatchSendNotificationsAsync(ctx, notifications...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "批量异步发送通知失败: %v", err)
	}

	// 4. 将结果转换为响应
	return &notificationv1.BatchSendNotificationsAsyncResponse{
		NotificationIds: result.NotificationIDs,
	}, nil
}

// BatchQueryNotifications 处理批量查询通知请求
func (s *NotificationServer) BatchQueryNotifications(ctx context.Context, req *notificationv1.BatchQueryNotificationsRequest) (*notificationv1.BatchQueryNotificationsResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	_, err := s.extractAndValidateBizID(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 调用执行器
	results, err := s.executor.BatchQueryNotifications(ctx, req.Keys...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "批量查询通知失败: %v", err)
	}

	// 3. 将结果转换为响应
	response := &notificationv1.BatchQueryNotificationsResponse{
		Results: make([]*notificationv1.SendNotificationResponse, 0, len(results)),
	}

	for _, r := range results {
		resp, err := s.convertToSendResponse(r)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "转换响应失败: %v", err)
		}
		response.Results = append(response.Results, resp)
	}

	return response, nil
}

// extractAndValidateBizID 从请求中提取并验证BizID
func (s *NotificationServer) extractAndValidateBizID(ctx context.Context) (int64, error) {
	// 从metadata中获取Authorization header
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0, status.Errorf(codes.Unauthenticated, "缺少认证信息")
	}

	// 获取Authorization token
	authHeaders := md.Get("Authorization")
	if len(authHeaders) == 0 {
		return 0, status.Errorf(codes.Unauthenticated, "缺少认证Token")
	}

	token := authHeaders[0]
	if token == "" {
		return 0, status.Errorf(codes.Unauthenticated, "无效的认证Token")
	}

	// TODO: 解析JWT Token获取BizID
	// 这里仅作为示例，实际应该解析JWT并验证
	// 临时使用固定的BizID
	bizID := int64(101)

	// TODO: 调用配置服务验证BizID是否有效
	// 实际应该调用配置服务来验证
	// if !s.configService.IsValidBizID(ctx, bizID) {
	//     return 0, status.Errorf(codes.PermissionDenied, "无效的业务ID: %d", bizID)
	// }

	return bizID, nil
}

// convertToNotification 将请求转换为通知领域对象
func (s *NotificationServer) convertToNotification(n *notificationv1.Notification, bizID int64) (service.Notification, error) {
	if n == nil {
		return service.Notification{}, errors.New("通知不能为空")
	}

	// 转换TemplateID
	tid, err := strconv.ParseInt(n.TemplateId, 10, 64)
	if err != nil {
		return service.Notification{}, fmt.Errorf("无效的模板ID: %s", n.TemplateId)
	}

	// 构建基本Notification
	notification := service.Notification{
		BizID:          bizID,
		Key:            n.Key,
		Receiver:       n.Receiver,
		Channel:        convertChannel(n.Channel),
		TemplateID:     tid,
		TemplateParams: n.TemplateParams,
		Strategy:       service.SendStrategyImmediate, // 默认为立即发送
	}

	// 添加发送策略
	if n.Strategy != nil {
		switch s := n.Strategy.StrategyType.(type) {
		case *notificationv1.SendStrategy_Immediate:
			notification.Strategy = service.SendStrategyImmediate
		case *notificationv1.SendStrategy_Delayed:
			if s.Delayed != nil && s.Delayed.DelaySeconds > 0 {
				notification.Strategy = service.SendStrategyDelayed
				notification.DelaySeconds = s.Delayed.DelaySeconds
			}
		case *notificationv1.SendStrategy_Scheduled:
			if s.Scheduled != nil && s.Scheduled.SendTime != nil {
				notification.Strategy = service.SendStrategyScheduled
				notification.ScheduledTime = s.Scheduled.SendTime.AsTime()
			}
		case *notificationv1.SendStrategy_TimeWindow:
			if s.TimeWindow != nil {
				notification.Strategy = service.SendStrategyTimeWindow
				notification.StartTimeMilliseconds = s.TimeWindow.StartTimeMilliseconds
				notification.EndTimeMilliseconds = s.TimeWindow.EndTimeMilliseconds
			}
		}
	}

	return notification, nil
}

// convertChannel 将gRPC通道转换为领域通道
func convertChannel(channel notificationv1.Channel) service.Channel {
	switch channel {
	case notificationv1.Channel_SMS:
		return service.ChannelSMS
	case notificationv1.Channel_EMAIL:
		return service.ChannelEmail
	case notificationv1.Channel_IN_APP:
		return service.ChannelInApp
	default:
		return service.ChannelUnspecified
	}
}

// convertToSendResponse 将领域响应转换为gRPC响应
func (s *NotificationServer) convertToSendResponse(result service.SendResponse) (*notificationv1.SendNotificationResponse, error) {
	response := &notificationv1.SendNotificationResponse{
		NotificationId: result.NotificationID,
		Status:         convertSendStatus(result.Status),
		ErrorCode:      convertErrorCode(result.ErrorCode),
		ErrorMessage:   result.ErrorMessage,
	}

	if !result.SendTime.IsZero() {
		response.SendTime = timestamppb.New(result.SendTime)
	}

	return response, nil
}

// convertSendStatus 将领域发送状态转换为gRPC发送状态
func convertSendStatus(status service.SendStatus) notificationv1.SendStatus {
	switch status {
	case service.SendStatusPrepare:
		return notificationv1.SendStatus_PREPARE
	case service.SendStatusCanceled:
		return notificationv1.SendStatus_CANCELED
	case service.SendStatusPending:
		return notificationv1.SendStatus_PENDING
	case service.SendStatusSucceeded:
		return notificationv1.SendStatus_SUCCEEDED
	case service.SendStatusFailed:
		return notificationv1.SendStatus_FAILED
	default:
		return notificationv1.SendStatus_SEND_STATUS_UNSPECIFIED
	}
}

// convertErrorCode 将领域错误码转换为gRPC错误码
func convertErrorCode(code service.ErrorCode) notificationv1.ErrorCode {
	switch code {
	case service.ErrorCodeInvalidParameter:
		return notificationv1.ErrorCode_INVALID_PARAMETER
	case service.ErrorCodeRateLimited:
		return notificationv1.ErrorCode_RATE_LIMITED
	case service.ErrorCodeTemplateNotFound:
		return notificationv1.ErrorCode_TEMPLATE_NOT_FOUND
	case service.ErrorCodeChannelDisabled:
		return notificationv1.ErrorCode_CHANNEL_DISABLED
	case service.ErrorCodeCreateNotificationFailed:
		return notificationv1.ErrorCode_CREATE_NOTIFICATION_FAILED
	default:
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED
	}
}
