package sender

import (
	"context"
	"errors"
	"fmt"
	"sync"

	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/channel"
	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/provider"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"gitee.com/flycash/notification-platform/internal/service/pkg/client/sms"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template"
)

var (
	ErrRateLimited            = errors.New("已达到速率限制")
	ErrQuotaExceeded          = errors.New("已超过配额限制")
	ErrBusinessConfigNotFound = errors.New("业务配置未找到")
)

// NotificationSender 通知发送接口
type NotificationSender interface {
	// Send 发送一批通知，返回发送结果
	Send(ctx context.Context, notifications []notificationsvc.Notification) ([]domain.SendResponse, error)
}

// sender 通知发送器实现
type sender struct {
	notificationSvc   notificationsvc.Service
	configSvc         configsvc.Service
	channelDispatcher channel.Channel
}

// NewSender 创建通知发送器
func NewSender(
	notificationSvc notificationsvc.Service,
	configSvc configsvc.Service,
	providerSvc providersvc.Service,
	templateSvc templatesvc.Service,
	smsClients map[string]sms.Client,
) NotificationSender {
	// 创建 provider.Dispatcher
	providerDispatcher := provider.NewDispatcher(
		providerSvc,
		templateSvc,
		smsClients,
	)

	// 创建 channel.Dispatcher
	channelDispatcher := channel.NewDispatcher(
		providerDispatcher,
		configSvc,
	)

	return &sender{
		notificationSvc:   notificationSvc,
		configSvc:         configSvc,
		channelDispatcher: channelDispatcher,
	}
}

// Send 批量发送通知
func (d *sender) Send(ctx context.Context, notifications []notificationsvc.Notification) ([]domain.SendResponse, error) {
	if len(notifications) == 0 {
		return nil, nil
	}

	// 获取业务配置
	bizID := notifications[0].BizID
	bizConfig, err := d.configSvc.GetByID(ctx, bizID)
	if err != nil {
		return nil, fmt.Errorf("获取业务配置失败: %w", err)
	}

	// 检查速率限制
	if d.isRateLimited(bizConfig, len(notifications)) {
		return nil, fmt.Errorf("%w", ErrRateLimited)
	}

	// 检查配额
	if d.isQuotaExceeded(bizConfig, notifications) {
		return nil, fmt.Errorf("%w", ErrQuotaExceeded)
	}

	// 并发发送通知
	var succeedMu sync.Mutex
	var failedMu sync.Mutex
	var succeed []domain.SendResponse
	var failed []domain.SendResponse

	var wg sync.WaitGroup
	for i := range notifications {
		wg.Add(1)

		// 创建本地变量避免闭包问题
		n := notifications[i]

		go func() {
			defer wg.Done()

			response, err1 := d.channelDispatcher.Send(ctx, n)

			if err1 != nil {

				resp := domain.SendResponse{
					NotificationID: n.ID,
					Status:         notificationsvc.SendStatusFailed,
					RetryCount:     response.RetryCount,
				}

				failedMu.Lock()
				failed = append(failed, resp)
				failedMu.Unlock()

			} else {

				resp := domain.SendResponse{
					NotificationID: n.ID,
					Status:         notificationsvc.SendStatusSucceeded,
					RetryCount:     response.RetryCount,
				}
				succeedMu.Lock()
				succeed = append(succeed, resp)
				succeedMu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 转换结果数据结构
	succeedNotifications := make([]notificationsvc.Notification, len(succeed))
	failedNotifications := make([]notificationsvc.Notification, len(failed))
	for i := range succeed {
		succeedNotifications = append(succeedNotifications, notificationsvc.Notification{
			ID: succeed[i].NotificationID,
		})
	}

	for i := range failed {
		failedNotifications = append(failedNotifications, notificationsvc.Notification{
			ID:         failed[i].NotificationID,
			RetryCount: failed[i].RetryCount,
		})
	}

	// 更新发送状态
	if len(succeedNotifications) > 0 || len(failedNotifications) > 0 {
		err = d.notificationSvc.BatchUpdateNotificationStatusSucceededOrFailed(ctx, succeedNotifications, failedNotifications)
		if err != nil {
			// 仅记录错误，不影响返回结果
			fmt.Printf("更新通知状态失败: %v\n", err)
		}
	}

	// 合并结果并返回
	return append(succeed, failed...), nil
}

// isRateLimited 检查是否达到速率限制
func (d *sender) isRateLimited(config configsvc.BusinessConfig, count int) bool {
	return config.RateLimit > 0 && count > config.RateLimit
}

// isQuotaExceeded 检查是否超过配额
func (d *sender) isQuotaExceeded(_ configsvc.BusinessConfig, _ []notificationsvc.Notification) bool {
	// 实现配额检查逻辑
	// 实际代码中需根据 config.Quota 检查各渠道的配额
	return false
}
