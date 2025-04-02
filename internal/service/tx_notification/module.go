package txnotification

import "gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/service"

type (
	Module struct {
		ignoredInitTablesErr error // 必须放在第一个
		Svc                  Service
	}
	Service = service.TxNotificationService
)
