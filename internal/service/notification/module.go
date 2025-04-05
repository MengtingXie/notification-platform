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
	Template     = domain.Template
	Service      service.NotificationService
	Channel      = domain.Channel
	SendStatus   = domain.Status
)

var (
	ErrInvalidParameter         = service.ErrInvalidParameter
	ErrNotificationNotFound     = service.ErrNotificationNotFound
	ErrCreateNotificationFailed = service.ErrCreateNotificationFailed
)

const (
	SendStatusPrepare   = domain.StatusPrepare   // 准备中
	SendStatusCanceled  = domain.StatusCanceled  // 已取消
	SendStatusPending   = domain.StatusPending   // 待发送
	SendStatusSucceeded = domain.StatusSucceeded // 发送成功
	SendStatusFailed    = domain.StatusFailed    // 发送失败

	ChannelSMS   = domain.ChannelSMS
	ChannelEmail = domain.ChannelEmail
	ChannelInApp = domain.ChannelInApp
)
