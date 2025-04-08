package provider

import (
	"context"
	"errors"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/adapter/sms"
	"gitee.com/flycash/notification-platform/internal/service/template"
)

var ErrSendFailed = errors.New("发送失败")

// Provider 供应商接口
type Provider interface {
	// Send 发送消息
	Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error)
}

// Dispatcher 供应商分发器，对外伪装成Provider，作为统一入口。负责创建和调用Selector
type Dispatcher struct {
	selector Selector
}

// NewDispatcher 创建供应商分发器
func NewDispatcher(
	providerSvc ManageService,
	templateRepo repository.ChannelTemplateRepository,
	smsClients map[string]sms.Client,
) Provider {
	d := &Dispatcher{
		//selector: newSelector(providerSvc, templateSvc, smsClients),
	}
	return d
}

func (d *Dispatcher) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	d.selector.Reset()

	var retryCount int8
	for {
		// 获取供应商
		provider, err1 := d.selector.Next(ctx, notification)
		if err1 != nil {
			// 没有可用的供应商
			return domain.SendResponse{RetryCount: retryCount}, err1
		}

		// 使用当前供应商发送
		resp, err2 := provider.Send(ctx, notification)
		if err2 == nil {
			// 发送成功，填写重试次数
			resp.RetryCount += retryCount
			return resp, nil
		}

		retryCount += resp.RetryCount
	}
}

// smsProvider SMS供应商
type smsProvider struct {
	name        string
	templateSvc template.ChannelTemplateService
	client      sms.Client
}

// NewSMSProvider SMS供应商
func NewSMSProvider(name string, templateSvc template.ChannelTemplateService, client sms.Client) Provider {
	return &smsProvider{
		name:        name,
		templateSvc: templateSvc,
		client:      client,
	}
}

// Send 发送短信
func (p *smsProvider) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	template, err := p.templateSvc.GetTemplate(ctx, notification.Template.ID, notification.Template.VersionID, p.name, template.ChannelSMS)
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	version := template.Versions[0]
	provider := version.Providers[0]

	resp, err := p.client.Send(sms.SendReq{
		PhoneNumbers:  []string{notification.Receiver},
		SignName:      version.Signature,
		TemplateID:    provider.ProviderTemplateID,
		TemplateParam: notification.Template.Params,
	})
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("%w: %w", ErrSendFailed, err)
	}

	respStatus := resp.PhoneNumbers[notification.Receiver]
	if respStatus.Code != "OK" {
		return domain.SendResponse{}, fmt.Errorf("%w: Code = %s, Message = %s", ErrSendFailed, respStatus.Code, respStatus.Message)
	}

	return domain.SendResponse{
		NotificationID: notification.ID,
		//Status:         domain.StatusSucceeded,
		Status: domain.StatusSucceeded,
	}, nil
}
