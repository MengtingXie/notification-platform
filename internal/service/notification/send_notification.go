package notification

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gitee.com/flycash/notification-platform/internal/errs"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/send_strategy"
	"gitee.com/flycash/notification-platform/internal/service/template"

	"github.com/gotomicro/ego/core/elog"
	"github.com/sony/sonyflake"
)

// SendService 负责处理发送
//
//go:generate mockgen -source=./executor.go -destination=./mocks/executor.mock.go -package=notificationmocks -typed SendService
type SendService interface {
	// SendNotification 同步单条发送
	SendNotification(ctx context.Context, n domain.Notification) (domain.SendResponse, error)
	// SendNotificationAsync 异步单条发送
	SendNotificationAsync(ctx context.Context, n domain.Notification) (domain.SendResponse, error)
	// BatchSendNotifications 同步批量发送
	BatchSendNotifications(ctx context.Context, ns ...domain.Notification) (domain.BatchSendResponse, error)
	// BatchSendNotificationsAsync 异步批量发送
	BatchSendNotificationsAsync(ctx context.Context, ns ...domain.Notification) (domain.BatchSendAsyncResponse, error)
}

// sendService 执行器实现
type sendService struct {
	notificationSvc Service
	templateSvc     template.ChannelTemplateService
	idGenerator     *sonyflake.Sonyflake
	sendStrategy    send_strategy.SendStrategy
	logger          *elog.Component
}

// NewSendService 创建执行器实例
func NewSendService(templateSvc template.ChannelTemplateService, notificationSvc Service, idGenerator *sonyflake.Sonyflake, sendStrategy send_strategy.SendStrategy) SendService {
	return &sendService{
		notificationSvc: notificationSvc,
		templateSvc:     templateSvc,
		idGenerator:     idGenerator,
		sendStrategy:    sendStrategy,
	}
}

// SendNotification 同步单条发送
func (e *sendService) SendNotification(ctx context.Context, n domain.Notification) (domain.SendResponse, error) {
	resp := domain.SendResponse{
		Status: domain.SendStatusFailed,
	}

	// 参数校验
	if err := n.Validate(); err != nil {
		return resp, err
	}

	// 生成通知ID，后续考虑分库分表
	id, err := e.idGenerator.NextID()
	if err != nil {
		return resp, fmt.Errorf("%w 生成 ID 失败，原因: %w", errs.ErrSendNotificationFailed, err)
	}
	n.ID = id
	// 发送通知
	response, err := e.sendStrategy.Send(ctx, n)
	// 处理策略错误
	if err != nil {
		// 通用的发送失败错误
		return resp, fmt.Errorf("%w, 发送通知失败，原因：%w", errs.ErrSendNotificationFailed, err)
	}
	return response, nil
}

// SendNotificationAsync 异步单条发送
func (e *sendService) SendNotificationAsync(ctx context.Context, n domain.Notification) (domain.SendResponse, error) {
	resp := domain.SendResponse{
		Status: domain.SendStatusFailed,
	}

	// 参数校验
	if err := n.Validate(); err != nil {
		return resp, err
	}

	tmpl, err := e.templateSvc.GetTemplateByID(ctx, n.Template.ID)
	if err != nil {
		e.logger.Warn("异步单条发送通知失败", elog.Any("获取模版失败", err))
		return resp, fmt.Errorf("%w", errs.ErrSendNotificationFailed)
	}
	if !tmpl.HasPublished() {
		return resp, fmt.Errorf("%w: 模板ID=%d未发布", errs.ErrInvalidParameter, n.Template.ID)
	}

	// 生成通知ID
	id, err := e.idGenerator.NextID()
	if err != nil {
		e.logger.Warn("异步单条发送通知失败", elog.Any("通知ID生成失败", err))
		return resp, fmt.Errorf("%w", errs.ErrSendNotificationFailed)
	}
	n.ID = id

	// 使用异步接口但要立即发送，修改为延时发送
	if n.SendStrategyConfig.Type == domain.SendStrategyImmediate {
		n.SendStrategyConfig.DeadlineTime = time.Now().Add(time.Minute)
	}

	// 发送通知
	response, err := e.sendStrategy.Send(ctx, n)
	// 处理策略错误
	if err != nil {
		e.logger.Warn("异步单条发送通知失败", elog.Any("Error", err))
		if errors.Is(err, errs.ErrInvalidParameter) {
			return resp, err
		}
		return resp, fmt.Errorf("%w", errs.ErrSendNotificationFailed)
	}

	return response, nil
}

// BatchSendNotifications 同步批量发送
func (e *sendService) BatchSendNotifications(ctx context.Context, notifications ...domain.Notification) (domain.BatchSendResponse, error) {
	response := domain.BatchSendResponse{
		TotalCount: len(notifications),
		Results:    make([]domain.SendResponse, 0, len(notifications)),
	}

	// 参数校验
	if len(notifications) == 0 {
		return response, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
	}

	resp := domain.SendResponse{Status: domain.SendStatusFailed}
	errMessages := make([]string, 0, len(notifications))
	for i := range notifications {

		if err := notifications[i].Validate(); err != nil {
			response.Results = append(response.Results, resp)
			errMessages = append(errMessages, err.Error())
		}

		tmpl, err := e.templateSvc.GetTemplateByID(ctx, notifications[i].Template.ID)
		if err != nil {
			e.logger.Warn("同步批量发送通知失败", elog.Any("获取模版失败", err))
			response.Results = append(response.Results, resp)
		}

		if !tmpl.HasPublished() {
			response.Results = append(response.Results, resp)
			errMessages = append(errMessages, fmt.Errorf("%w: key = %s, 模板ID=%d未发布", errs.ErrInvalidParameter, notifications[i].Key, tmpl.ID).Error())
		}

		// 生成通知ID
		id, err := e.idGenerator.NextID()
		if err != nil {
			e.logger.Warn("同步批量发送通知失败", elog.Any("通知ID生成失败", err))
			response.Results = append(response.Results, resp)
		}
		notifications[i].ID = id

		// todo: 检查发送策略类型，必须相同
	}

	if len(response.Results) != 0 {
		// 参数错误
		if len(errMessages) != 0 {
			return response, fmt.Errorf("%w: 通知列表中有非法通知: %s", errs.ErrInvalidParameter, strings.Join(errMessages, "; "))
		}
		// 其他错误
		return response, fmt.Errorf("%w", errs.ErrSendNotificationFailed)
	}

	// 发送通知
	results, err := e.sendStrategy.BatchSend(ctx, notifications)
	if err != nil {

		e.logger.Warn("同步批量发送通知失败", elog.Any("Error", err))

		// 部分失败
		response.Results = results
		for i := range results {
			if results[i].Status == domain.SendStatusSucceeded {
				response.SuccessCount++
			}
		}

		if errors.Is(err, errs.ErrInvalidParameter) {
			return response, err
		}
		return response, fmt.Errorf("%w", errs.ErrSendNotificationFailed)
	}

	// 从响应获取结果
	response.Results = results
	response.SuccessCount = len(response.Results)
	return response, nil
}

// BatchSendNotificationsAsync 异步批量发送
func (e *sendService) BatchSendNotificationsAsync(ctx context.Context, notifications ...domain.Notification) (domain.BatchSendAsyncResponse, error) {
	response := domain.BatchSendAsyncResponse{
		NotificationIDs: make([]uint64, 0, len(notifications)),
	}

	// 参数校验
	if len(notifications) == 0 {
		return response, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
	}

	systemErrCounter := 0
	errMessages := make([]string, 0, len(notifications))
	for i := range notifications {

		if err := notifications[i].Validate(); err != nil {
			errMessages = append(errMessages, err.Error())
		}

		tmpl, err := e.templateSvc.GetTemplateByID(ctx, notifications[i].Template.ID)
		if err != nil {
			e.logger.Warn("异步批量发送通知失败", elog.Any("获取模版失败", err))
			systemErrCounter++
		}

		if !tmpl.HasPublished() {
			errMessages = append(errMessages, fmt.Errorf("%w: key = %s, 模板ID=%d未发布",
				errs.ErrInvalidParameter, notifications[i].Key, tmpl.ID).Error())
		}

		// 生成通知ID
		id, err := e.idGenerator.NextID()
		if err != nil {
			e.logger.Warn("异步批量发送通知失败", elog.Any("通知ID生成失败", err))
			systemErrCounter++
		}

		notifications[i].ID = id

		// todo: 检查发送策略类型，必须相同
	}

	// 参数错误
	if len(errMessages) != 0 {
		return response, fmt.Errorf("%w: 通知列表中有非法通知: %s", errs.ErrInvalidParameter, strings.Join(errMessages, "; "))
	}

	if systemErrCounter != 0 {
		return response, fmt.Errorf("%w", errs.ErrSendNotificationFailed)
	}

	// 发送通知
	results, err := e.sendStrategy.BatchSend(ctx, notifications)
	if err != nil {
		e.logger.Warn("异步批量发送通知失败", elog.Any("Error", err))
		if errors.Is(err, errs.ErrInvalidParameter) {
			return response, err
		}
		return response, fmt.Errorf("%w", errs.ErrSendNotificationFailed)
	}

	// 从响应获取结果
	for i := range results {
		response.NotificationIDs[i] = results[i].NotificationID
	}
	return response, nil
}
