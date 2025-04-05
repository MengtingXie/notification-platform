package provider

import (
	"gitee.com/flycash/notification-platform/internal/service/provider/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider/internal/service"
)

// Service 供应商服务接口
type (
	Module struct {
		ignoredInitTablesErr error // 必须放在第一个
		Svc                  Service
	}

	Service  service.ProviderService
	Provider = domain.Provider
	Channel  = domain.Channel
)

const (
	ChannelSMS   = domain.ChannelSMS
	ChannelEmail = domain.ChannelEmail
	ChannelInApp = domain.ChannelInApp
)
