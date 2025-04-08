package txnotification

import (
	retry2 "gitee.com/flycash/notification-platform/internal/service/notification/retry"
)

func InitRetryServiceBuilder() retry2.Builder {
	return retry2.NewFacadeBuilder(map[string]retry2.Builder{
		"normal": &retry2.NormalBuilder{},
	})
}
