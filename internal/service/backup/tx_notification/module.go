package txnotification

import (
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification"
)

type (
	Module struct {
		ignoredInitTablesErr error // 必须放在第一个
		Svc                  Service
	}
	Service        = notification.TxNotificationService
	TxNotification = domain.TxNotification
	Status         = domain.TxNotificationStatus
)

const (
	TxNotificationStatusPrepare Status = "PREPARE"
	TxNotificationStatusCommit  Status = "COMMIT"
	TxNotificationStatusCancel  Status = "CANCEL"
)
