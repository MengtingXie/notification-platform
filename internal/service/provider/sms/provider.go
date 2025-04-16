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

	// baseProvider
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
	tmpl, err := p.templateSvc.GetTemplate(ctx, notification.Template.ID, notification.Template.VersionID, p.name, domain.ChannelSMS)
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("%w: %w", errs.ErrSendNotificationFailed, err)
	}

	activeVersion := tmpl.ActiveVersion()
	if activeVersion == nil {
		return domain.SendResponse{}, fmt.Errorf("%w: 无已发布模版", errs.ErrSendNotificationFailed)
	}

	const first = 0
	resp, err := p.client.Send(client.SendReq{
		PhoneNumbers:  notification.Receivers,
		SignName:      activeVersion.Signature,
		TemplateID:    activeVersion.Providers[first].ProviderTemplateID,
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
