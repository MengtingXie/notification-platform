package main

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/service/provider/sms"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	providersvc "gitee.com/flycash/notification-platform/internal/service/provider/manage"
	"gitee.com/flycash/notification-platform/internal/service/provider/sequential"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template/manage"
)

func main() {
	// if err := ego.New().Serve(func() server.Server {
	//	return ioc.InitGrpcServer().GrpcServer
	// }()).Run(); err != nil {
	//	elog.Panic("startup", elog.Any("err", err))
	// }
	println("hello, world")
}

func newSMSSelectorBuilder(
	ctx context.Context,
	providerSvc providersvc.Service,
	templateSvc templatesvc.ChannelTemplateService,
) (*sequential.SelectorBuilder, error) {
	providers, err := initSMSProviders(ctx, providerSvc, templateSvc)
	if err != nil {
		return nil, err
	}
	return sequential.NewSelectorBuilder(providers), nil
}

func initSMSProviders(
	ctx context.Context,
	providerSvc providersvc.Service,
	templateSvc templatesvc.ChannelTemplateService,
) ([]provider.Provider, error) {
	entities, err := providerSvc.GetProvidersByChannel(ctx, domain.ChannelSMS)
	if err != nil {
		return nil, err
	}

	providers := make([]provider.Provider, 0, len(entities))

	for i := range entities {
		var c client.Client
		var err1 error
		if entities[i].Name == "ali" {
			c, err1 = client.NewAliyunSMS(entities[i].RegionID, entities[i].APIKey, entities[i].APISecret)
			if err1 != nil {
				return nil, err
			}
		} else if entities[i].Name == "tencent" {
			c, err1 = client.NewTencentCloudSMS(entities[i].RegionID, entities[i].APIKey, entities[i].APISecret, entities[i].APPID)
			if err1 != nil {
				return nil, err
			}
		}
		providers = append(providers, sms.NewSMSProvider(
			entities[i].Name,
			templateSvc,
			c,
		))
	}
	return providers, nil
}
