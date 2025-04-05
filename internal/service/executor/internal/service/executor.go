package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/strategy"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"github.com/sony/sonyflake"
)

// ExecutorService 执行器
//
//go:generate mockgen -source=./executor.go -destination=../../mocks/executor.mock.go -package=executormocks -typed ExecutorService
type ExecutorService interface {
	// SendNotification 同步单条发送
	SendNotification(ctx context.Context, n domain.Notification) (domain.SendResponse, error)
	// SendNotificationAsync 异步单条发送
	SendNotificationAsync(ctx context.Context, n domain.Notification) (domain.SendResponse, error)
	// BatchSendNotifications 同步批量发送
	BatchSendNotifications(ctx context.Context, ns ...domain.Notification) (domain.BatchSendResponse, error)
	// BatchSendNotificationsAsync 异步批量发送
	BatchSendNotificationsAsync(ctx context.Context, ns ...domain.Notification) (domain.BatchSendAsyncResponse, error)
	// BatchQueryNotifications 同步批量查询
	BatchQueryNotifications(ctx context.Context, keys ...string) ([]domain.SendResponse, error)
}

// executor 执行器实现
type executor struct {
	notificationSvc notificationsvc.Service
	idGenerator     *sonyflake.Sonyflake
	sendStrategy    strategy.SendStrategy
}

// NewExecutorService 创建执行器实例
func NewExecutorService(notificationSvc notificationsvc.Service, idGenerator *sonyflake.Sonyflake, sendStrategy strategy.SendStrategy) ExecutorService {
	return &executor{
		notificationSvc: notificationSvc,
		idGenerator:     idGenerator,
		sendStrategy:    sendStrategy,
	}
}

// SendNotification 同步单条发送
func (e *executor) SendNotification(ctx context.Context, n domain.Notification) (domain.SendResponse, error) {
	resp := domain.SendResponse{
		Status: notificationsvc.SendStatusPending,
	}

	if ok := e.isValidateRequest(n, &resp); !ok {
		return resp, nil
	}

	// 生成通知ID
	id, err := e.idGenerator.NextID()
	if err != nil {
		return resp, fmt.Errorf("通知ID生成失败: %w", err)
	}

	// 调用服务发送通知
	n.Notification.ID = id
	sentNotification, err := e.notificationSvc.CreateNotification(ctx, n.Notification)
	if err != nil {
		// 直接处理通知服务错误
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode, resp.ErrorMessage = e.convertToErrorCodeAndErrorMessage(err)
		return resp, nil
	}

	// 发送通知
	notifications := []domain.Notification{n}
	responses, err := e.sendStrategy.Send(ctx, notifications)
	// 处理策略错误
	if err != nil {
		// 直接使用mapErrorToResponse处理错误
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode, resp.ErrorMessage = e.convertToErrorCodeAndErrorMessage(err)

		// 仅检查是否为已知业务错误
		if errors.Is(err, strategy.ErrInvalidParameter) ||
			errors.Is(err, notificationsvc.ErrInvalidParameter) ||
			errors.Is(err, notificationsvc.ErrCreateNotificationFailed) {
			return resp, nil
		}

		// 未知的系统错误，返回原始错误
		return resp, fmt.Errorf("发送通知系统错误: %w", err)
	}

	// 从响应中提取结果
	if len(responses) > 0 {
		return responses[0], nil
	}

	// 如果没有获得响应，返回基本响应
	resp.NotificationID = sentNotification.ID
	resp.Status = sentNotification.Status
	resp.SendTime = time.Unix(sentNotification.SendTime, 0)
	return resp, nil
}

// isValidateRequest 检查通知参数是否有效
func (e *executor) isValidateRequest(n domain.Notification, resp *domain.SendResponse) bool {
	// 参数校验
	if n.Notification.BizID <= 0 {
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode = domain.ErrorCodeInvalidParameter
		resp.ErrorMessage = "业务ID不能为空"
		return false
	}

	if n.Notification.Key == "" {
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode = domain.ErrorCodeInvalidParameter
		resp.ErrorMessage = "业务唯一标识不能为空"
		return false
	}

	// 接受者
	if n.Notification.Receiver == "" {
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode = domain.ErrorCodeInvalidParameter
		resp.ErrorMessage = "接收者不能为空"
		return false
	}

	// 校验渠道
	if n.Notification.Channel != notificationsvc.ChannelSMS &&
		n.Notification.Channel != notificationsvc.ChannelEmail &&
		n.Notification.Channel != notificationsvc.ChannelInApp {
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode = domain.ErrorCodeInvalidParameter
		resp.ErrorMessage = "不支持的通知渠道"
		return false
	}

	// 校验模板ID
	if n.Notification.Template.ID <= 0 {
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode = domain.ErrorCodeInvalidParameter
		resp.ErrorMessage = "无效的模板ID"
		return false
	}

	return e.isValidSendStrategy(n, resp)
}

func (e *executor) isValidSendStrategy(n domain.Notification, resp *domain.SendResponse) bool {
	// 校验策略相关字段
	switch n.SendStrategyConfig.Type {
	case domain.SendStrategyImmediate:
		return true
	case domain.SendStrategyDelayed:
		if n.SendStrategyConfig.DelaySeconds <= 0 {
			resp.Status = notificationsvc.SendStatusFailed
			resp.ErrorCode = domain.ErrorCodeInvalidParameter
			resp.ErrorMessage = "延迟发送策略需要指定正数的延迟秒数"
			return false
		}
	case domain.SendStrategyScheduled:
		if n.SendStrategyConfig.ScheduledTime.IsZero() || n.SendStrategyConfig.ScheduledTime.Before(time.Now()) {
			resp.Status = notificationsvc.SendStatusFailed
			resp.ErrorCode = domain.ErrorCodeInvalidParameter
			resp.ErrorMessage = "定时发送策略需要指定未来的发送时间"
			return false
		}
	case domain.SendStrategyTimeWindow:
		if n.SendStrategyConfig.StartTimeMilliseconds <= 0 || n.SendStrategyConfig.EndTimeMilliseconds <= n.SendStrategyConfig.StartTimeMilliseconds {
			resp.Status = notificationsvc.SendStatusFailed
			resp.ErrorCode = domain.ErrorCodeInvalidParameter
			resp.ErrorMessage = "时间窗口策略需要指定有效的开始和结束时间"
			return false
		}
	}
	return true
}

// convertToErrorCodeAndErrorMessage 将错误映射为错误代码和错误消息
func (e *executor) convertToErrorCodeAndErrorMessage(err error) (code domain.ErrorCode, message string) {
	if err == nil {
		return domain.ErrorCodeUnspecified, ""
	}

	// 错误消息直接使用错误字符串
	errorMessage := err.Error()

	// 映射错误代码
	switch {
	// 参数错误
	case errors.Is(err, strategy.ErrInvalidParameter):
		return domain.ErrorCodeInvalidParameter, errorMessage
	case errors.Is(err, notificationsvc.ErrInvalidParameter):
		return domain.ErrorCodeInvalidParameter, errorMessage

	// 通知服务错误
	case errors.Is(err, notificationsvc.ErrCreateNotificationFailed):
		return domain.ErrorCodeCreateNotificationFailed, errorMessage
	case errors.Is(err, notificationsvc.ErrNotificationNotFound):
		return domain.ErrorCodeUnspecified, errorMessage

	// 其他错误
	default:
		return domain.ErrorCodeUnspecified, errorMessage
	}
}

// SendNotificationAsync 异步单条发送
func (e *executor) SendNotificationAsync(ctx context.Context, n domain.Notification) (domain.SendResponse, error) {
	resp := domain.SendResponse{}

	if ok := e.isValidateRequest(n, &resp); !ok {
		return resp, nil
	}

	// 生成通知ID
	id, err := e.idGenerator.NextID()
	if err != nil {
		return resp, fmt.Errorf("通知ID生成失败: %w", err)
	}

	// 创建通知记录
	n.Notification.ID = id
	createdNotification, err := e.notificationSvc.CreateNotification(ctx, n.Notification)
	if err != nil {
		// 直接处理错误
		resp.Status = notificationsvc.SendStatusFailed
		resp.ErrorCode, resp.ErrorMessage = e.convertToErrorCodeAndErrorMessage(err)
		return resp, nil
	}

	// 设置响应
	resp.NotificationID = createdNotification.ID
	resp.Status = notificationsvc.SendStatusPending

	// 异步发送通知，这里只返回已创建状态
	// TODO: 实际实现可能需要将通知加入队列

	return resp, nil
}

// BatchSendNotifications 同步批量发送
func (e *executor) BatchSendNotifications(ctx context.Context, ns ...domain.Notification) (domain.BatchSendResponse, error) {
	response := domain.BatchSendResponse{
		TotalCount: len(ns),
		Results:    make([]domain.SendResponse, 0, len(ns)),
	}

	// 参数校验
	if len(ns) == 0 {
		singleResponse := domain.SendResponse{
			Status:       notificationsvc.SendStatusFailed,
			ErrorCode:    domain.ErrorCodeInvalidParameter,
			ErrorMessage: "通知列表不能为空",
		}
		response.Results = append(response.Results, singleResponse)
		return response, nil
	}

	// 进行批量处理
	responses, err := e.sendStrategy.Send(ctx, ns)
	// 处理错误
	if err != nil {
		errorResp := domain.SendResponse{
			Status: notificationsvc.SendStatusFailed,
		}

		errorResp.ErrorCode, errorResp.ErrorMessage = e.convertToErrorCodeAndErrorMessage(err)

		// 如果是已知业务错误，为每个通知添加相同的错误响应
		if errorResp.ErrorCode != domain.ErrorCodeUnspecified || errors.Is(err, strategy.ErrInvalidParameter) {
			for i := 0; i < len(ns); i++ {
				response.Results = append(response.Results, errorResp)
			}
			return response, nil
		}

		// 如不是已知业务错误，则为系统错误
		return response, fmt.Errorf("批量发送通知系统错误: %w", err)
	}

	// 处理成功响应
	response.Results = responses
	for _, result := range responses {
		if result.Status == notificationsvc.SendStatusSucceeded {
			response.SuccessCount++
		}
	}

	return response, nil
}

// BatchSendNotificationsAsync 批量异步发送
func (e *executor) BatchSendNotificationsAsync(ctx context.Context, ns ...domain.Notification) (domain.BatchSendAsyncResponse, error) {
	response := domain.BatchSendAsyncResponse{
		NotificationIDs: make([]uint64, 0, len(ns)),
	}

	// 批量处理每一条通知
	for i := range ns {
		result, err := e.SendNotificationAsync(ctx, ns[i])
		if err != nil {
			// 如果有严重错误，直接返回
			return response, err
		}

		if result.Status != notificationsvc.SendStatusFailed {
			response.NotificationIDs = append(response.NotificationIDs, result.NotificationID)
		} else if response.ErrorMessage == "" {
			// 记录第一个错误信息
			response.ErrorCode = result.ErrorCode
			response.ErrorMessage = result.ErrorMessage
		}
	}

	return response, nil
}

// BatchQueryNotifications 批量查询通知
func (e *executor) BatchQueryNotifications(_ context.Context, keys ...string) ([]domain.SendResponse, error) {
	results := make([]domain.SendResponse, 0, len(keys))

	// 实际实现应该通过 Key 查询通知记录
	// e.notificationSvc.GetNotificationsByKeys(ctx, keys)

	// 将查询到的Notification转换为SendResponse

	return results, nil
}
