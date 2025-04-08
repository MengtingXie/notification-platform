package grpc

import (
	"context"
	"errors"
	"fmt"
	executorsvc "gitee.com/flycash/notification-platform/internal/service/backup/executor"
	"gitee.com/flycash/notification-platform/internal/service/backup/tx_notification"
	"strconv"
	"strings"
	"time"

	"gitee.com/flycash/notification-platform/internal/api/grpc/interceptor/jwt"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
)

// NotificationServer 处理通知平台的gRPC请求
type NotificationServer struct {
	notificationv1.UnimplementedNotificationServiceServer
	notificationv1.UnimplementedNotificationQueryServiceServer
	executor executorsvc.Service
	// TODO: 配置服务 configService config.ConfigService
	txnSvc txnotification.Service
}

// NewServer 创建通知平台gRPC服务器
func NewServer(executor executorsvc.Service, txnSvc txnotification.Service) *NotificationServer {
	return &NotificationServer{
		executor: executor,
		txnSvc:   txnSvc,
	}
}

func (s *NotificationServer) TxPrepare(ctx context.Context, request *notificationv1.TxPrepareRequest) (*notificationv1.TxPrepareResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 将请求转换为领域对象
	txn, err := s.convertToTxNotification(request.Notification, bizID)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "无效的请求参数: %v", err)
	}
	_, err = s.txnSvc.Prepare(ctx, txn)
	return &notificationv1.TxPrepareResponse{}, err
}

func (s *NotificationServer) TxCommit(ctx context.Context, request *notificationv1.TxCommitRequest) (*notificationv1.TxCommitResponse, error) {
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = s.txnSvc.Commit(ctx, bizID, request.GetKey())
	return &notificationv1.TxCommitResponse{}, err
}

func (s *NotificationServer) TxCancel(ctx context.Context, request *notificationv1.TxCancelRequest) (*notificationv1.TxCancelResponse, error) {
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	err = s.txnSvc.Cancel(ctx, bizID, request.GetKey())
	return &notificationv1.TxCancelResponse{}, err
}

// SendNotification 处理同步发送通知请求
func (s *NotificationServer) SendNotification(ctx context.Context, req *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
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
	return s.convertToSendResponse(result, err)
}

// SendNotificationAsync 处理异步发送通知请求
func (s *NotificationServer) SendNotificationAsync(ctx context.Context, req *notificationv1.SendNotificationAsyncRequest) (*notificationv1.SendNotificationAsyncResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
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
	response := &notificationv1.SendNotificationAsyncResponse{
		NotificationId: result.NotificationID,
	}

	// 如果有错误，提取错误代码和消息
	if err != nil {
		response.ErrorMessage = err.Error()
		response.ErrorCode = s.mapErrorToErrorCode(err)
	}

	return response, nil
}

// BatchSendNotifications 处理批量同步发送通知请求
func (s *NotificationServer) BatchSendNotifications(ctx context.Context, req *notificationv1.BatchSendNotificationsRequest) (*notificationv1.BatchSendNotificationsResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 将请求转换为领域对象
	notifications := make([]executorsvc.Notification, 0, len(req.Notifications))
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
		resp, err := s.convertToSendResponse(r, nil)
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
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 将请求转换为领域对象
	notifications := make([]executorsvc.Notification, 0, len(req.Notifications))
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
	response := &notificationv1.BatchSendNotificationsAsyncResponse{
		NotificationIds: result.NotificationIDs,
	}

	return response, nil
}

// BatchQueryNotifications 处理批量查询通知请求
func (s *NotificationServer) BatchQueryNotifications(ctx context.Context, req *notificationv1.BatchQueryNotificationsRequest) (*notificationv1.BatchQueryNotificationsResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 调用执行器
	results, err := s.executor.BatchQueryNotifications(ctx, bizID, req.Keys...)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "批量查询通知失败: %v", err)
	}

	// 3. 将结果转换为响应
	response := &notificationv1.BatchQueryNotificationsResponse{
		Results: make([]*notificationv1.SendNotificationResponse, 0, len(results)),
	}

	for _, r := range results {
		sendResp, err := s.convertToSendResponse(r, nil)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "转换响应失败: %v", err)
		}
		response.Results = append(response.Results, sendResp)
	}

	return response, nil
}

// QueryNotification 处理单条查询通知请求
func (s *NotificationServer) QueryNotification(ctx context.Context, req *notificationv1.QueryNotificationRequest) (*notificationv1.QueryNotificationResponse, error) {
	// 1. 从metadata中解析Authorization JWT Token
	bizID, err := jwt.GetBizIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	// 2. 调用执行器
	result, err := s.executor.QueryNotification(ctx, bizID, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "查询通知失败: %v", err)
	}

	// 3. 将结果转换为响应
	sendResp, err := s.convertToSendResponse(result, err)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "转换响应失败: %v", err)
	}

	return &notificationv1.QueryNotificationResponse{
		Result: sendResp,
	}, nil
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
func (s *NotificationServer) convertToNotification(n *notificationv1.Notification, bizID int64) (executorsvc.Notification, error) {
	if n == nil {
		return executorsvc.Notification{}, errors.New("通知不能为空")
	}

	// 转换TemplateID
	tid, err := strconv.ParseInt(n.TemplateId, 10, 64)
	if err != nil {
		return executorsvc.Notification{}, fmt.Errorf("无效的模板ID: %s", n.TemplateId)
	}

	// 构建基本Notification
	notification := notificationsvc.Notification{
		BizID:    bizID,
		Key:      n.Key,
		Receiver: n.Receiver,
		Channel:  convertChannel(n.Channel),
		Template: notificationsvc.Template{
			ID:     tid,
			Params: n.TemplateParams,
		},
	}

	// 构建发送策略
	sendStrategyType := executorsvc.SendStrategyImmediate // 默认为立即发送
	var delaySeconds int64
	var scheduledTime time.Time
	var startTimeMilliseconds int64
	var endTimeMilliseconds int64
	var deadlineTime time.Time

	// 处理发送策略
	if n.Strategy != nil {
		switch s := n.Strategy.StrategyType.(type) {
		case *notificationv1.SendStrategy_Immediate:
			sendStrategyType = executorsvc.SendStrategyImmediate
		case *notificationv1.SendStrategy_Delayed:
			if s.Delayed != nil && s.Delayed.DelaySeconds > 0 {
				sendStrategyType = executorsvc.SendStrategyDelayed
				delaySeconds = s.Delayed.DelaySeconds
			}
		case *notificationv1.SendStrategy_Scheduled:
			if s.Scheduled != nil && s.Scheduled.SendTime != nil {
				sendStrategyType = executorsvc.SendStrategyScheduled
				scheduledTime = s.Scheduled.SendTime.AsTime()
			}
		case *notificationv1.SendStrategy_TimeWindow:
			if s.TimeWindow != nil {
				sendStrategyType = executorsvc.SendStrategyTimeWindow
				startTimeMilliseconds = s.TimeWindow.StartTimeMilliseconds
				endTimeMilliseconds = s.TimeWindow.EndTimeMilliseconds
			}
		case *notificationv1.SendStrategy_Deadline:
			if s.Deadline != nil && s.Deadline.Deadline != nil {
				sendStrategyType = executorsvc.SendStrategyDeadline
				deadlineTime = s.Deadline.Deadline.AsTime()
			}
		}
	}

	// 构建最终的Notification
	return executorsvc.Notification{
		Notification: notification,
		SendStrategyConfig: executorsvc.SendStrategyConfig{
			Type:                  sendStrategyType,
			DelaySeconds:          delaySeconds,
			ScheduledTime:         scheduledTime,
			StartTimeMilliseconds: startTimeMilliseconds,
			EndTimeMilliseconds:   endTimeMilliseconds,
			DeadlineTime:          deadlineTime,
		},
	}, nil
}

// convertChannel 将gRPC通道转换为领域通道
func convertChannel(channel notificationv1.Channel) notificationsvc.Channel {
	switch channel {
	case notificationv1.Channel_SMS:
		return notificationsvc.ChannelSMS
	case notificationv1.Channel_EMAIL:
		return notificationsvc.ChannelEmail
	case notificationv1.Channel_IN_APP:
		return notificationsvc.ChannelInApp
	default:
		return ""
	}
}

// convertToSendResponse 将领域响应转换为gRPC响应
func (s *NotificationServer) convertToSendResponse(result executorsvc.SendResponse, err error) (*notificationv1.SendNotificationResponse, error) {
	response := &notificationv1.SendNotificationResponse{
		NotificationId: result.NotificationID,
		Status:         convertSendStatus(result.Status),
	}

	// 如果有错误，提取错误代码和消息
	if err != nil {
		response.ErrorMessage = err.Error()
		response.ErrorCode = s.mapErrorToErrorCode(err)

		// 如果状态不是失败，但有错误，更新状态为失败
		if response.Status != notificationv1.SendStatus_FAILED {
			response.Status = notificationv1.SendStatus_FAILED
		}
	}

	return response, nil
}

// mapErrorToErrorCode 将错误映射为gRPC错误代码
func (s *NotificationServer) mapErrorToErrorCode(err error) notificationv1.ErrorCode {
	if err == nil {
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED
	}

	// 根据错误类型进行匹配
	switch {
	case errors.Is(err, executorsvc.ErrInvalidParameter):
		return notificationv1.ErrorCode_INVALID_PARAMETER

	case errors.Is(err, executorsvc.ErrNotificationNotFound):
		// 目前我们没有专门的"NotFound"错误码，所以这里暂时使用通用错误码
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED

	case errors.Is(err, executorsvc.ErrSendNotificationFailed):
		// 如果是发送失败错误，需要进一步判断具体原因
		if strings.Contains(err.Error(), "创建通知失败") {
			return notificationv1.ErrorCode_CREATE_NOTIFICATION_FAILED
		}
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED

	case errors.Is(err, executorsvc.ErrQueryNotificationFailed):
		// 查询失败暂时使用通用错误码
		return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED

	default:
		// 如果无法通过类型匹配，回退到消息内容匹配
		errorMsg := err.Error()

		switch {
		case strings.Contains(errorMsg, "频率限制") ||
			strings.Contains(errorMsg, "rate limit"):
			return notificationv1.ErrorCode_RATE_LIMITED

		case strings.Contains(errorMsg, "模板未找到") ||
			strings.Contains(errorMsg, "template not found"):
			return notificationv1.ErrorCode_TEMPLATE_NOT_FOUND

		case strings.Contains(errorMsg, "渠道被禁用") ||
			strings.Contains(errorMsg, "channel disabled"):
			return notificationv1.ErrorCode_CHANNEL_DISABLED

		default:
			return notificationv1.ErrorCode_ERROR_CODE_UNSPECIFIED
		}
	}
}

// convertSendStatus 将领域发送状态转换为gRPC发送状态
func convertSendStatus(status notificationsvc.SendStatus) notificationv1.SendStatus {
	switch status {
	case notificationsvc.SendStatusPrepare:
		return notificationv1.SendStatus_PREPARE
	case notificationsvc.SendStatusCanceled:
		return notificationv1.SendStatus_CANCELED
	case notificationsvc.SendStatusPending:
		return notificationv1.SendStatus_PENDING
	case notificationsvc.SendStatusSucceeded:
		return notificationv1.SendStatus_SUCCEEDED
	case notificationsvc.SendStatusFailed:
		return notificationv1.SendStatus_FAILED
	default:
		return notificationv1.SendStatus_SEND_STATUS_UNSPECIFIED
	}
}

func (s *NotificationServer) convertToTxNotification(n *notificationv1.Notification, bizID int64) (txnotification.TxNotification, error) {
	if n == nil {
		return txnotification.TxNotification{}, errors.New("通知不能为空")
	}

	// 转换TemplateID
	tid, err := strconv.ParseInt(n.TemplateId, 10, 64)
	if err != nil {
		return txnotification.TxNotification{}, fmt.Errorf("无效的模板ID: %s", n.TemplateId)
	}
	// 构建基本Notification
	noti := notificationsvc.Notification{
		BizID:    bizID,
		Key:      n.Key,
		Receiver: n.Receiver,
		Channel:  notificationsvc.Channel(n.Channel.String()),
		Template: notificationsvc.Template{
			ID:     tid,
			Params: n.TemplateParams,
		},
		Status: notificationsvc.SendStatusPrepare,
	}
	const (
		d      = 24
		second = 1000
	)
	now := time.Now()
	if n.Strategy != nil {
		switch s := n.Strategy.StrategyType.(type) {
		case *notificationv1.SendStrategy_Immediate:
			noti.ScheduledSTime = now.UnixMilli()
			noti.ScheduledETime = now.Add(d * time.Hour).UnixMilli()
		case *notificationv1.SendStrategy_Delayed:
			if s.Delayed != nil && s.Delayed.DelaySeconds > 0 {
				noti.ScheduledSTime = now.UnixMilli() + s.Delayed.DelaySeconds*second
				noti.ScheduledETime = now.UnixMilli() + (s.Delayed.DelaySeconds+10)*second
			}

		case *notificationv1.SendStrategy_Scheduled:
			if s.Scheduled != nil && s.Scheduled.SendTime != nil {
				noti.ScheduledSTime = s.Scheduled.SendTime.AsTime().UnixMilli()
				noti.ScheduledETime = s.Scheduled.SendTime.AsTime().UnixMilli() + 10*second
			}
		case *notificationv1.SendStrategy_TimeWindow:
			if s.TimeWindow != nil {
				noti.ScheduledSTime = s.TimeWindow.StartTimeMilliseconds
				noti.ScheduledETime = s.TimeWindow.EndTimeMilliseconds
			}
		}
	}
	return txnotification.TxNotification{
		BizID:        bizID,
		Key:          n.Key,
		Notification: noti,
		Status:       txnotification.TxNotificationStatusPrepare,
	}, nil
}
