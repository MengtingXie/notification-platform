package txnotification

import "gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/service/retry"

func InitRetryServiceBuilder() retry.Builder {
	return retry.NewFacadeBuilder(map[string]retry.Builder{
		"normal": &retry.NormalBuilder{},
	})
}
