package sender

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/channel"
	"sync"

	configsvc "gitee.com/flycash/notification-platform/internal/service/config"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
	"github.com/gotomicro/ego/core/elog"
)

var (
	ErrRateLimited   = errors.New("已达到速率限制")
	ErrQuotaExceeded = errors.New("已超过配额限制")
)

// NotificationSender 通知发送接口
type NotificationSender interface {
	// Send 发送一批通知，返回发送结果
	Send(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error)
}

// sender 通知发送器实现
type sender struct {
	notificationSvc   notificationsvc.NotificationService
	repo              repository.NotificationRepository
	configSvc         configsvc.BusinessConfigService
	channelDispatcher channel.Channel
	logger            *elog.Component
}

// NewSender 创建通知发送器
func NewSender(
	repo repository.NotificationRepository,
	configSvc configsvc.BusinessConfigService,
	channelDispatcher channel.Channel,
) NotificationSender {

	return &sender{
		configSvc:         configSvc,
		repo:              repo,
		channelDispatcher: channelDispatcher,
		logger:            elog.DefaultLogger,
	}
}

// Send 批量发送通知
func (d *sender) Send(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error) {
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
	var succeedMu, failedMu sync.Mutex
	var succeed, failed []domain.SendResponse

	var wg sync.WaitGroup
	for i := range notifications {
		wg.Add(1)

		n := notifications[i]

		go func() {
			defer wg.Done()

			response, err1 := d.channelDispatcher.Send(ctx, n)
			if err1 != nil {
				resp := domain.SendResponse{
					NotificationID: n.ID,
					Status:         domain.StatusFailed,
					RetryCount:     response.RetryCount,
				}
				failedMu.Lock()
				failed = append(failed, resp)
				failedMu.Unlock()
			} else {
				resp := domain.SendResponse{
					NotificationID: n.ID,
					Status:         domain.StatusSucceeded,
					RetryCount:     response.RetryCount,
				}
				succeedMu.Lock()
				succeed = append(succeed, resp)
				succeedMu.Unlock()
			}
		}()
	}
	wg.Wait()

	// 获取通知信息，以便获取版本号
	allNotificationIDs := make([]uint64, 0, len(succeed)+len(failed))
	for _, s := range succeed {
		allNotificationIDs = append(allNotificationIDs, s.NotificationID)
	}
	for _, f := range failed {
		allNotificationIDs = append(allNotificationIDs, f.NotificationID)
	}

	// 获取所有通知的详细信息，包括版本号
	notificationsMap, err := d.repo.BatchGetByIDs(ctx, allNotificationIDs)
	if err != nil {
		d.logger.Warn("批量获取通知失败",
			elog.Any("Error", err),
			elog.Any("allNotificationIDs", allNotificationIDs),
		)
		return nil, fmt.Errorf("获取通知失败: %w", err)
	}

	succeedNotifications := d.getUpdatedNotifications(succeed, notificationsMap)
	failedNotifications := d.getUpdatedNotifications(failed, notificationsMap)

	// 更新发送状态
	err = d.batchUpdateStatus(ctx, succeedNotifications, failedNotifications)
	if err != nil {
		return nil, err
	}

	// 合并结果并返回
	return append(succeed, failed...), nil
}

// getUpdatedNotifications 获取更新字段后的实体
func (d *sender) getUpdatedNotifications(responses []domain.SendResponse, notificationsMap map[uint64]domain.Notification) []domain.Notification {
	notifications := make([]domain.Notification, 0, len(responses))
	for i := range responses {
		if notification, ok := notificationsMap[responses[i].NotificationID]; ok {
			notification.Status = responses[i].Status
			notification.RetryCount = responses[i].RetryCount
			notifications = append(notifications, notification)
		}
	}
	return notifications
}

// batchUpdateStatus 更新发送状态
func (d *sender) batchUpdateStatus(ctx context.Context, succeedNotifications, failedNotifications []domain.Notification) error {
	if len(succeedNotifications) > 0 || len(failedNotifications) > 0 {
		err := d.repo.BatchUpdateStatusSucceededOrFailed(ctx, succeedNotifications, failedNotifications)
		if err != nil {
			d.logger.Warn("批量更新通知状态失败",
				elog.Any("Error", err),
				elog.Any("succeedNotifications", succeedNotifications),
				elog.Any("failedNotifications", failedNotifications),
			)
			return fmt.Errorf("批量更新通知状态失败: %w", err)
		}
	}
	return nil
}

// isRateLimited 检查是否达到速率限制
func (d *sender) isRateLimited(config domain.BusinessConfig, count int) bool {
	return config.RateLimit > 0 && count > config.RateLimit
}

// isQuotaExceeded 检查是否超过配额
func (d *sender) isQuotaExceeded(_ domain.BusinessConfig, _ []domain.Notification) bool {
	// 实现配额检查逻辑
	// 实际代码中需根据 config.Quota 检查各渠道的配额
	return false
}
