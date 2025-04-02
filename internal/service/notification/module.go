package notification

import (
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification/internal/service"
)

type (
	Module struct {
		ignoredInitTablesErr error // 必须放在第一个
		Svc                  Service
	}

	Notification = domain.Notification
	Service      = service.NotificationService
	Channel      = domain.Channel
	Status       = domain.Status
	Template     = domain.Template
)

var (
	ErrInvalidParameter     = service.ErrInvalidParameter
	ErrNotificationNotFound = service.ErrNotificationNotFound
	ErrChannelDisabled      = service.ErrChannelDisabled

	StatusPrepare   = domain.StatusPrepare   // 准备中
	StatusCanceled  = domain.StatusCanceled  // 已取消
	StatusPending   = domain.StatusPending   // 待发送
	StatusSucceeded = domain.StatusSucceeded // 发送成功
	StatusFailed    = domain.StatusFailed    // 发送失败
)
