package template

import (
	"gitee.com/flycash/notification-platform/internal/domain"
	template2 "gitee.com/flycash/notification-platform/internal/service/template"
)

type (
	Module struct {
		ignoredInitTablesErr error // 必须放在第一个
		Svc                  Service
	}

	Service                 template2.ChannelTemplateService
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
