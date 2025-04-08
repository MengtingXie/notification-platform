package main

import (
	"gitee.com/flycash/notification-platform/cmd/platform/ioc"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	"gitee.com/flycash/notification-platform/internal/service/provider/manage"
	"gitee.com/flycash/notification-platform/internal/service/provider/sms"
	"github.com/gotomicro/ego"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server"
)

func main() {
	if err := ego.New().Serve(func() server.Server {
		return ioc.InitGrpcServer().GrpcServer
	}()).Run(); err != nil {
		elog.Panic("startup", elog.Any("err", err))
	}
}

func newSelectorBuilder() *provider.SelectorBuilder {
	return provider.NewSelectorBuilder(initSMSProviders(nil))
}

func initSMSProviders(psvc manage.ManageService) []provider.Provider {
	// 发起数据库查询
	ali, _ := sms.NewAliyunSMS("", "", "")
	tencent, _ := sms.NewTencentCloudSMS("", "", "", "")
	return []provider.Provider{
		provider.NewSMSProvider("ali", nil, ali),
		provider.NewSMSProvider("tencent", nil, tencent),
	}
}
