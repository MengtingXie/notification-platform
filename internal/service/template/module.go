package template

import (
	"gitee.com/flycash/notification-platform/internal/domain"
)

type (
	Module struct {
		ignoredInitTablesErr error // 必须放在第一个
		Svc                  Service
	}

	Service                 ChannelTemplateService
	ChannelTemplateVersion  = domain.ChannelTemplateVersion
	ChannelTemplateProvider = domain.ChannelTemplateProvider
	ChannelTemplate         = domain.ChannelTemplate
	AuditStatus             = domain.AuditStatus
	OwnerType               = domain.OwnerType
	Channel                 = domain.Channel
)

const (
	ChannelSMS   = domain.ChannelSMS
	ChannelEmail = domain.ChannelEmail
	ChannelInApp = domain.ChannelInApp
)
