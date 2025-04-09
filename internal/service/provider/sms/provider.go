package sms

import (
	"context"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/service/template/manage"
)

// smsProvider SMS供应商
type smsProvider struct {
	name        string
	templateSvc manage.ChannelTemplateService
	client      client.Client
}

// NewSMSProvider SMS供应商
func NewSMSProvider(name string, templateSvc manage.ChannelTemplateService, client client.Client) provider.Provider {
	return &smsProvider{
		name:        name,
		templateSvc: templateSvc,
		client:      client,
	}
}

// Send 发送短信
func (p *smsProvider) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	// SMS 有多个供应商 aliyun，腾讯云
	tmpl, err := p.templateSvc.GetTemplate(ctx, notification.Template.ID, notification.Template.VersionID, p.name, domain.ChannelSMS)
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("%w: %w", errs.ErrSendNotificationFailed, err)
	}

	version := tmpl.Versions[0]
	provider := version.Providers[0]

	resp, err := p.client.Send(client.SendReq{
		PhoneNumbers:  notification.Receivers,
		SignName:      version.Signature,
		TemplateID:    provider.ProviderTemplateID,
		TemplateParam: notification.Template.Params,
	})
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("%w: %w", errs.ErrSendNotificationFailed, err)
	}

	for _, status := range resp.PhoneNumbers {
		if status.Code != "OK" {
			return domain.SendResponse{}, fmt.Errorf("%w: Code = %s, Message = %s", errs.ErrSendNotificationFailed, status.Code, status.Message)
		}
	}

	return domain.SendResponse{
		NotificationID: notification.ID,
		Status:         domain.SendStatusSucceeded,
	}, nil
}
