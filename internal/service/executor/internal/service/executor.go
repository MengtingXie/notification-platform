package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/strategy"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template"
	"github.com/gotomicro/ego/core/elog"
	"github.com/sony/sonyflake"
)

// 定义通用错误
var (
	ErrInvalidParameter        = errors.New("参数错误")
	ErrNotificationNotFound    = errors.New("通知不存在")
	ErrSendNotificationFailed  = errors.New("发送通知失败")
	ErrQueryNotificationFailed = errors.New("查询通知失败")
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
	// QueryNotification 同步单条查询
	QueryNotification(ctx context.Context, bizID int64, key string) (domain.SendResponse, error)
	// BatchQueryNotifications 同步批量查询
	BatchQueryNotifications(ctx context.Context, bizID int64, keys ...string) ([]domain.SendResponse, error)
}

// executor 执行器实现
type executor struct {
	notificationSvc notificationsvc.Service
	templateSvc     templatesvc.Service
	idGenerator     *sonyflake.Sonyflake
	sendStrategy    strategy.SendStrategy
	logger          *elog.Component
}

// NewExecutorService 创建执行器实例
func NewExecutorService(templateSvc templatesvc.Service, notificationSvc notificationsvc.Service, idGenerator *sonyflake.Sonyflake, sendStrategy strategy.SendStrategy) ExecutorService {
	return &executor{
		notificationSvc: notificationSvc,
		templateSvc:     templateSvc,
		idGenerator:     idGenerator,
		sendStrategy:    sendStrategy,
	}
}

// SendNotification 同步单条发送
func (e *executor) SendNotification(ctx context.Context, n domain.Notification) (domain.SendResponse, error) {
	resp := domain.SendResponse{
		Status: notificationsvc.SendStatusFailed,
	}

	// 参数校验
	if err := e.validateNotification(n); err != nil {
		return resp, err
	}

	template, err := e.templateSvc.GetTemplateByID(ctx, n.Notification.Template.ID)
	if err != nil {
		e.logger.Warn("同步单条发送通知失败", elog.Any("获取模版失败", err))
		return resp, fmt.Errorf("%w", ErrSendNotificationFailed)
	}
	if !template.HasPublished() {
		return resp, fmt.Errorf("%w: 模板ID=%d未发布", ErrInvalidParameter, n.Notification.Template.ID)
	}

	// 生成通知ID
	id, err := e.idGenerator.NextID()
	if err != nil {
		e.logger.Warn("同步单条发送通知失败", elog.Any("通知ID生成失败", err))
		return resp, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 调用服务发送通知
	n.Notification.ID = id

	// 发送通知
	notifications := []domain.Notification{n}
	responses, err := e.sendStrategy.Send(ctx, notifications)
	// 处理策略错误
	if err != nil {

		e.logger.Warn("同步单条发送通知失败", elog.Any("Error", err))

		// 对不同类型的错误进行通用包装
		if errors.Is(err, strategy.ErrInvalidParameter) || errors.Is(err, notificationsvc.ErrInvalidParameter) {
			return resp, fmt.Errorf("%w: %s", ErrInvalidParameter, err.Error())
		}
		// 通用的发送失败错误
		return resp, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 从响应获取结果
	const zero = 0
	return responses[zero], nil
}

// validateNotification 检查通知参数是否有效
func (e *executor) validateNotification(n domain.Notification) error {
	// 参数校验
	if err := e.validateBizID(n.Notification.BizID); err != nil {
		return err
	}

	if err := e.validateKey(n.Notification.Key); err != nil {
		return err
	}

	// 接受者
	if n.Notification.Receiver == "" {
		return fmt.Errorf("%w: key = %s, 接收者不能为空", ErrInvalidParameter, n.Notification.Key)
	}

	// 校验渠道
	if n.Notification.Channel != notificationsvc.ChannelSMS &&
		n.Notification.Channel != notificationsvc.ChannelEmail &&
		n.Notification.Channel != notificationsvc.ChannelInApp {
		return fmt.Errorf("%w: key = %s, 不支持的通知渠道", ErrInvalidParameter, n.Notification.Key)
	}

	// 校验模板ID
	if n.Notification.Template.ID <= 0 {
		return fmt.Errorf("%w: key = %s, 无效的模板ID", ErrInvalidParameter, n.Notification.Key)
	}

	return e.validateSendStrategy(n)
}

func (e *executor) validateBizID(bizID int64) error {
	if bizID <= 0 {
		return fmt.Errorf("%w: 业务ID不能为空", ErrInvalidParameter)
	}
	return nil
}

func (e *executor) validateKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: 业务唯一标识不能为空", ErrInvalidParameter)
	}
	return nil
}

func (e *executor) validateSendStrategy(n domain.Notification) error {
	// 校验策略相关字段
	switch n.SendStrategyConfig.Type {
	case domain.SendStrategyImmediate:
		return nil
	case domain.SendStrategyDelayed:
		if n.SendStrategyConfig.DelaySeconds <= 0 {
			return fmt.Errorf("%w: key = %s, 延迟发送策略需要指定正数的延迟秒数", ErrInvalidParameter, n.Notification.Key)
		}
	case domain.SendStrategyScheduled:
		if n.SendStrategyConfig.ScheduledTime.IsZero() || n.SendStrategyConfig.ScheduledTime.Before(time.Now()) {
			return fmt.Errorf("%w: key = %s, 定时发送策略需要指定未来的发送时间", ErrInvalidParameter, n.Notification.Key)
		}
	case domain.SendStrategyTimeWindow:
		if n.SendStrategyConfig.StartTimeMilliseconds <= 0 || n.SendStrategyConfig.EndTimeMilliseconds <= n.SendStrategyConfig.StartTimeMilliseconds {
			return fmt.Errorf("%w: key = %s, 时间窗口发送策略需要指定有效的开始和结束时间", ErrInvalidParameter, n.Notification.Key)
		}
	case domain.SendStrategyDeadline:
		if n.SendStrategyConfig.DeadlineTime.IsZero() || n.SendStrategyConfig.DeadlineTime.Before(time.Now()) {
			return fmt.Errorf("%w: key = %s, 截止日期发送策略需要指定未来的发送时间", ErrInvalidParameter, n.Notification.Key)
		}
	}
	return nil
}

// SendNotificationAsync 异步单条发送
func (e *executor) SendNotificationAsync(ctx context.Context, n domain.Notification) (domain.SendResponse, error) {
	resp := domain.SendResponse{
		Status: notificationsvc.SendStatusFailed,
	}

	// 参数校验
	if err := e.validateNotification(n); err != nil {
		return resp, err
	}

	template, err := e.templateSvc.GetTemplateByID(ctx, n.Notification.Template.ID)
	if err != nil {
		e.logger.Warn("异步单条发送通知失败", elog.Any("获取模版失败", err))
		return resp, fmt.Errorf("%w", ErrSendNotificationFailed)
	}
	if !template.HasPublished() {
		return resp, fmt.Errorf("%w: 模板ID=%d未发布", ErrInvalidParameter, n.Notification.Template.ID)
	}

	// 生成通知ID
	id, err := e.idGenerator.NextID()
	if err != nil {
		e.logger.Warn("异步单条发送通知失败", elog.Any("通知ID生成失败", err))
		return resp, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 创建通知记录
	n.Notification.ID = id

	// 使用异步接口但要立即发送，修改为延时发送
	if n.SendStrategyConfig.Type == domain.SendStrategyImmediate {
		n.SendStrategyConfig.DeadlineTime = time.Now().Add(time.Minute)
	}

	// 发送通知
	notifications := []domain.Notification{n}
	responses, err := e.sendStrategy.Send(ctx, notifications)
	// 处理策略错误
	if err != nil {

		e.logger.Warn("异步单条发送通知失败", elog.Any("Error", err))

		// 对不同类型的错误进行通用包装
		if errors.Is(err, strategy.ErrInvalidParameter) || errors.Is(err, notificationsvc.ErrInvalidParameter) {
			return resp, fmt.Errorf("%w: %s", ErrInvalidParameter, err.Error())
		}
		// 通用的发送失败错误
		return resp, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 从响应获取结果
	const zero = 0
	return responses[zero], nil
}

// BatchSendNotifications 同步批量发送
func (e *executor) BatchSendNotifications(ctx context.Context, notifications ...domain.Notification) (domain.BatchSendResponse, error) {
	response := domain.BatchSendResponse{
		TotalCount: len(notifications),
		Results:    make([]domain.SendResponse, 0, len(notifications)),
	}

	// 参数校验
	if len(notifications) == 0 {
		return response, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	resp := domain.SendResponse{Status: notificationsvc.SendStatusFailed}
	errMessages := make([]string, 0, len(notifications))
	for i := range notifications {

		if err := e.validateNotification(notifications[i]); err != nil {
			response.Results = append(response.Results, resp)
			errMessages = append(errMessages, err.Error())
		}

		template, err := e.templateSvc.GetTemplateByID(ctx, notifications[i].Notification.Template.ID)
		if err != nil {
			e.logger.Warn("同步批量发送通知失败", elog.Any("获取模版失败", err))
			response.Results = append(response.Results, resp)
		}

		if !template.HasPublished() {
			response.Results = append(response.Results, resp)
			errMessages = append(errMessages, fmt.Errorf("%w: key = %s, 模板ID=%d未发布", ErrInvalidParameter, notifications[i].Notification.Key, template.ID).Error())
		}

		// 生成通知ID
		id, err := e.idGenerator.NextID()
		if err != nil {
			e.logger.Warn("同步批量发送通知失败", elog.Any("通知ID生成失败", err))
			response.Results = append(response.Results, resp)
		}
		notifications[i].Notification.ID = id
	}

	if len(response.Results) != 0 {
		// 参数错误
		if len(errMessages) != 0 {
			return response, fmt.Errorf("%w: 通知列表中有非法通知: %s", ErrInvalidParameter, strings.Join(errMessages, "; "))
		}
		// 其他错误
		return response, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 发送通知
	results, err := e.sendStrategy.Send(ctx, notifications)
	if err != nil {

		e.logger.Warn("同步批量发送通知失败", elog.Any("Error", err))

		// 部分失败
		response.Results = results
		for i := range results {
			if results[i].Status == notificationsvc.SendStatusSucceeded {
				response.SuccessCount++
			}
		}

		// 对不同类型的错误进行通用包装
		if errors.Is(err, strategy.ErrInvalidParameter) || errors.Is(err, notificationsvc.ErrInvalidParameter) {
			return response, fmt.Errorf("%w: %s", ErrInvalidParameter, err.Error())
		}

		// 通用的发送失败错误
		return response, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 从响应获取结果
	response.Results = results
	response.SuccessCount = len(response.Results)
	return response, nil
}

// BatchSendNotificationsAsync 异步批量发送
func (e *executor) BatchSendNotificationsAsync(ctx context.Context, notifications ...domain.Notification) (domain.BatchSendAsyncResponse, error) {
	response := domain.BatchSendAsyncResponse{
		NotificationIDs: make([]uint64, 0, len(notifications)),
	}

	// 参数校验
	if len(notifications) == 0 {
		return response, fmt.Errorf("%w: 通知列表不能为空", ErrInvalidParameter)
	}

	systemErrCounter := 0
	errMessages := make([]string, 0, len(notifications))
	for i := range notifications {

		if err := e.validateNotification(notifications[i]); err != nil {
			errMessages = append(errMessages, err.Error())
		}

		template, err := e.templateSvc.GetTemplateByID(ctx, notifications[i].Notification.Template.ID)
		if err != nil {
			e.logger.Warn("异步批量发送通知失败", elog.Any("获取模版失败", err))
			systemErrCounter++
		}

		if !template.HasPublished() {
			errMessages = append(errMessages, fmt.Errorf("%w: key = %s, 模板ID=%d未发布",
				ErrInvalidParameter, notifications[i].Notification.Key, template.ID).Error())
		}

		// 生成通知ID
		id, err := e.idGenerator.NextID()
		if err != nil {
			e.logger.Warn("异步批量发送通知失败", elog.Any("通知ID生成失败", err))
			systemErrCounter++
		}

		notifications[i].Notification.ID = id
	}

	// 参数错误
	if len(errMessages) != 0 {
		return response, fmt.Errorf("%w: 通知列表中有非法通知: %s", ErrInvalidParameter, strings.Join(errMessages, "; "))
	}

	if systemErrCounter != 0 {
		return response, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 发送通知
	results, err := e.sendStrategy.Send(ctx, notifications)
	if err != nil {

		e.logger.Warn("异步批量发送通知失败", elog.Any("Error", err))

		// 对不同类型的错误进行通用包装
		if errors.Is(err, strategy.ErrInvalidParameter) || errors.Is(err, notificationsvc.ErrInvalidParameter) {
			return response, fmt.Errorf("%w: %s", ErrInvalidParameter, err.Error())
		}

		// 通用的发送失败错误
		return response, fmt.Errorf("%w", ErrSendNotificationFailed)
	}

	// 从响应获取结果
	for i := range results {
		response.NotificationIDs[i] = results[i].NotificationID
	}
	return response, nil
}

// QueryNotification 同步单条查询
func (e *executor) QueryNotification(ctx context.Context, bizID int64, key string) (domain.SendResponse, error) {
	// 参数校验
	resp := domain.SendResponse{
		Status: notificationsvc.SendStatusFailed,
	}

	if err := e.validateBizID(bizID); err != nil {
		return resp, err
	}

	if err := e.validateKey(key); err != nil {
		return resp, err
	}

	// 查询通知
	notifications, err := e.notificationSvc.GetByKeys(ctx, bizID, key)
	if err != nil {
		e.logger.Warn("同步单条查询通知失败", elog.Any("Error", err))
		return resp, fmt.Errorf("%w", ErrQueryNotificationFailed)
	}

	// 未找到通知
	if len(notifications) == 0 {
		return resp, fmt.Errorf("%w: 未找到通知", ErrNotificationNotFound)
	}

	// 构建响应
	response := domain.SendResponse{
		NotificationID: notifications[0].ID,
		Status:         notifications[0].Status,
	}
	return response, nil
}

// BatchQueryNotifications 同步批量查询
func (e *executor) BatchQueryNotifications(ctx context.Context, bizID int64, keys ...string) ([]domain.SendResponse, error) {
	// 参数校验
	if err := e.validateBizID(bizID); err != nil {
		return nil, err
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("%w: 业务唯一标识列表不能为空", ErrInvalidParameter)
	}
	for i := range keys {
		if err := e.validateKey(keys[i]); err != nil {
			return nil, fmt.Errorf("%w: 业务唯一标识列表中不能有空key", ErrInvalidParameter)
		}
	}

	// 查询通知
	notifications, err := e.notificationSvc.GetByKeys(ctx, bizID, keys...)
	if err != nil {
		e.logger.Warn("同步批量查询通知失败", elog.Any("Error", err))
		return nil, fmt.Errorf("%w", ErrQueryNotificationFailed)
	}

	// 构建响应
	responses := make([]domain.SendResponse, 0, len(notifications))
	for i := range notifications {
		resp := domain.SendResponse{
			NotificationID: notifications[i].ID,
			Status:         notifications[i].Status,
		}
		responses = append(responses, resp)
	}
	return responses, nil
}
