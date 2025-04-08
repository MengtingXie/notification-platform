package send_strategy

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/sender"
	"github.com/gotomicro/ego/core/elog"
)

// ImmediateSendStrategy 立即发送策略
type ImmediateSendStrategy struct {
	repo   repository.NotificationRepository
	sender sender.NotificationSender
	logger *elog.Component
}

// NewImmediateStrategy 创建立即发送策略
func NewImmediateStrategy(repo repository.NotificationRepository, sender sender.NotificationSender) *ImmediateSendStrategy {
	return &ImmediateSendStrategy{
		repo:   repo,
		sender: sender,
	}
}

// Send 单条发送通知
func (s *ImmediateSendStrategy) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {

	notification.SetSendTime()
	created, err := s.repo.Create(ctx, notification)
	if err == nil {
		// 立即发送
		return s.sender.Send(ctx, created)
	}

	// 非唯一索引冲突直接返回错误
	if !errors.Is(err, repository.ErrNotificationDuplicate) {
		return domain.SendResponse{}, fmt.Errorf("创建通知失败: %w", err)
	}

	// 唯一索引冲突表示业务方重试
	notifications, err := s.repo.GetByKeys(ctx, created.BizID, created.Key)
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("获取通知失败: %w", err)
	}

	// 已经发送成功了
	const first = 0
	found := notifications[first]
	if found.Status == domain.SendStatusSucceeded {
		return domain.SendResponse{
			NotificationID: found.ID,
			Status:         found.Status,
		}, nil
	}

	// 更新通知状态为PENDING同时获取乐观锁（版本号）
	oldStatus := found.Status
	found.Status = domain.SendStatusPending
	err = s.repo.UpdateStatus(ctx, found.ID, found.Status, found.Version)
	if err != nil {
		s.logger.Warn("更新通知状态失败", elog.FieldErr(err))
		return domain.SendResponse{
			NotificationID: found.ID,
			Status:         oldStatus,
		}, nil

	}
	found.Version++

	// 再次立即发送
	return s.sender.Send(ctx, found)
}

// BatchSend 批量发送通知，其中每个通知的发送策略必须相同
func (s *ImmediateSendStrategy) BatchSend(ctx context.Context, ns []domain.Notification) ([]domain.SendResponse, error) {

	if len(ns) == 0 {
		return nil, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
	}

	for _, not := range ns {
		not.SetSendTime()
	}

	// 创建通知记录
	createdNotifications, err := s.repo.BatchCreate(ctx, ns)
	if err != nil {
		// 只要有一个唯一索引冲突整批失败
		return nil, fmt.Errorf("创建通知失败: %w", err)
	}
	// 立即发送
	return s.sender.BatchSend(ctx, createdNotifications)
}
