package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/notification/domain"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification/service"
	"github.com/sony/sonyflake"
)

// executor 执行器实现
type executor struct {
	notificationSvc notificationsvc.NotificationService
	idGenerator     *sonyflake.Sonyflake
}

// NewExecutorService 创建执行器实例
func NewExecutorService(notificationSvc notificationsvc.NotificationService, idGenerator *sonyflake.Sonyflake) ExecutorService {
	return &executor{
		notificationSvc: notificationSvc,
		idGenerator:     idGenerator,
	}
}

// SendNotification 同步单条发送
func (e *executor) SendNotification(ctx context.Context, n Notification) (SendResponse, error) {
	resp := SendResponse{
		Status: SendStatusPending,
	}

	if ok := e.isValidate(n, &resp); !ok {
		return resp, nil
	}

	// 生成通知ID
	id, err := e.idGenerator.NextID()
	if err != nil {
		return resp, fmt.Errorf("通知ID生成失败: %w", err)
	}

	// 构建领域模型并发送通知
	domainNotification := n.ToDomainNotification(id)

	// 调用服务发送通知
	sentNotification, err := e.notificationSvc.SendNotification(ctx, domainNotification)
	if err != nil {
		// 处理业务错误
		resp.Status = SendStatusFailed
		resp.ErrorMessage = err.Error()

		// 映射错误代码
		switch {
		case errors.Is(err, notificationsvc.ErrChannelDisabled):
			resp.ErrorCode = ErrorCodeChannelDisabled
		case errors.Is(err, notificationsvc.ErrInvalidParameter):
			resp.ErrorCode = ErrorCodeInvalidParameter
		case errors.Is(err, notificationsvc.ErrNotificationNotFound):
			resp.ErrorCode = ErrorCodeUnspecified
		default:
			resp.ErrorCode = ErrorCodeUnspecified
		}
		return resp, nil
	}

	// 设置响应
	resp.NotificationID = sentNotification.ID
	resp.Status = mapDomainStatusToSendStatus(sentNotification.Status)
	resp.SendTime = time.Unix(sentNotification.SendTime, 0)

	return resp, nil
}

// isValidate 检查通知参数是否有效
func (e *executor) isValidate(n Notification, resp *SendResponse) bool {
	// 参数校验
	if n.BizID <= 0 {
		resp.Status = SendStatusFailed
		resp.ErrorCode = ErrorCodeInvalidParameter
		resp.ErrorMessage = "业务ID不能为空"
		return false
	}

	if n.Key == "" {
		resp.Status = SendStatusFailed
		resp.ErrorCode = ErrorCodeInvalidParameter
		resp.ErrorMessage = "业务唯一标识不能为空"
		return false
	}

	// 接受者
	if n.Receiver == "" {
		resp.Status = SendStatusFailed
		resp.ErrorCode = ErrorCodeInvalidParameter
		resp.ErrorMessage = "接收者不能为空"
		return false
	}

	// 校验渠道
	if n.Channel == "" || n.Channel == ChannelUnspecified {
		resp.Status = SendStatusFailed
		resp.ErrorCode = ErrorCodeInvalidParameter
		resp.ErrorMessage = "不支持的通知渠道"
		return false
	}

	// 校验模板ID
	if n.TemplateID <= 0 {
		resp.Status = SendStatusFailed
		resp.ErrorCode = ErrorCodeInvalidParameter
		resp.ErrorMessage = "无效的模板ID"
		return false
	}

	return e.isValidSendStrategy(n, resp)
}

func (e *executor) isValidSendStrategy(n Notification, resp *SendResponse) bool {
	// 校验策略相关字段
	switch n.Strategy {
	case SendStrategyImmediate:
		return true
	case SendStrategyDelayed:
		if n.DelaySeconds <= 0 {
			resp.Status = SendStatusFailed
			resp.ErrorCode = ErrorCodeInvalidParameter
			resp.ErrorMessage = "延迟发送策略需要指定正数的延迟秒数"
			return false
		}
	case SendStrategyScheduled:
		if n.ScheduledTime.IsZero() || n.ScheduledTime.Before(time.Now()) {
			resp.Status = SendStatusFailed
			resp.ErrorCode = ErrorCodeInvalidParameter
			resp.ErrorMessage = "定时发送策略需要指定未来的发送时间"
			return false
		}
	case SendStrategyTimeWindow:
		if n.StartTimeMilliseconds <= 0 || n.EndTimeMilliseconds <= n.StartTimeMilliseconds {
			resp.Status = SendStatusFailed
			resp.ErrorCode = ErrorCodeInvalidParameter
			resp.ErrorMessage = "时间窗口策略需要指定有效的开始和结束时间"
			return false
		}
	}
	return true
}

// SendNotificationAsync 异步单条发送
func (e *executor) SendNotificationAsync(ctx context.Context, n Notification) (SendResponse, error) {
	resp := SendResponse{}

	if ok := e.isValidate(n, &resp); !ok {
		return resp, nil
	}

	// 生成通知ID
	id, err := e.idGenerator.NextID()
	if err != nil {
		return resp, fmt.Errorf("通知ID生成失败: %w", err)
	}

	// 构建领域模型
	domainNotification := n.ToDomainNotification(id)

	// 创建通知记录
	createdNotification, err := e.notificationSvc.CreateNotification(ctx, domainNotification)
	if err != nil {
		resp.Status = SendStatusFailed
		resp.ErrorCode = ErrorCodeCreateNotificationFailed
		resp.ErrorMessage = fmt.Sprintf("创建通知失败: %s", err.Error())
		return resp, nil
	}

	// 设置响应
	resp.NotificationID = createdNotification.ID
	resp.Status = SendStatusPending

	// 异步发送通知，这里只返回已创建状态
	// TODO: 实际实现可能需要将通知加入队列

	return resp, nil
}

// BatchSendNotifications 同步批量发送
func (e *executor) BatchSendNotifications(ctx context.Context, ns ...Notification) (BatchSendResponse, error) {
	response := BatchSendResponse{
		TotalCount: len(ns),
		Results:    make([]SendResponse, 0, len(ns)),
	}

	// 批量处理每一条通知
	for i := range ns {
		result, err := e.SendNotification(ctx, ns[i])
		if err != nil {
			// 如果有严重错误，直接返回
			return response, err
		}

		response.Results = append(response.Results, result)
		if result.Status == SendStatusSucceeded {
			response.SuccessCount++
		}
	}

	return response, nil
}

// BatchSendNotificationsAsync 批量异步发送
func (e *executor) BatchSendNotificationsAsync(ctx context.Context, ns ...Notification) (BatchSendAsyncResponse, error) {
	response := BatchSendAsyncResponse{
		NotificationIDs: make([]uint64, 0, len(ns)),
	}

	// 批量处理每一条通知
	for i := range ns {
		result, err := e.SendNotificationAsync(ctx, ns[i])
		if err != nil {
			// 如果有严重错误，直接返回
			return response, err
		}

		if result.Status != SendStatusFailed {
			response.NotificationIDs = append(response.NotificationIDs, result.NotificationID)
		}
	}

	return response, nil
}

// BatchQueryNotifications 批量查询通知
func (e *executor) BatchQueryNotifications(_ context.Context, keys ...string) ([]SendResponse, error) {
	results := make([]SendResponse, 0, len(keys))

	// 实际实现应该通过 Key 查询通知记录
	// e.notificationSvc.GetNotificationsByKeys(ctx, keys)

	// 将查询到的Notification转换为SendResponse

	return results, nil
}

// mapDomainStatusToSendStatus 将领域状态映射到发送状态
func mapDomainStatusToSendStatus(status domain.Status) SendStatus {
	switch status {
	case domain.StatusPrepare:
		return SendStatusPrepare
	case domain.StatusCanceled:
		return SendStatusCanceled
	case domain.StatusPending:
		return SendStatusPending
	case domain.StatusSucceeded:
		return SendStatusSucceeded
	case domain.StatusFailed:
		return SendStatusFailed
	default:
		return SendStatusUnspecified
	}
}
