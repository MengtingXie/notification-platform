package txnotification

import (
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/service"
)

type (
	Module struct {
		ignoredInitTablesErr error // 必须放在第一个
		Svc                  Service
	}
	Service              = service.TxNotificationService
	TxNotification       = domain.TxNotification
	TxNotificationStatus = domain.TxNotificationStatus
)

const (
	TxNotificationStatusPrepare TxNotificationStatus = "PREPARE"
	TxNotificationStatusCommit  TxNotificationStatus = "COMMIT"
	TxNotificationStatusCancel  TxNotificationStatus = "CANCEL"
)
