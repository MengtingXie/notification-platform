package main

func main() {
	//if err := ego.New().Serve(func() server.Server {
	//	return ioc.InitGrpcServer().GrpcServer
	//}()).Run(); err != nil {
	//	elog.Panic("startup", elog.Any("err", err))
	//}
	println("hello, world")
}

//func newSelectorBuilder() *provider.SelectorBuilder {
//	return provider.NewSelectorBuilder(initSMSProviders(nil))
//}
//
//func initSMSProviders(psvc manage.Service) []provider.Provider {
//	// 发起数据库查询
//	ali, _ := sms.NewAliyunSMS("", "", "")
//	tencent, _ := sms.NewTencentCloudSMS("", "", "", "")
//	return []provider.Provider{
//		provider.NewSMSProvider("ali", nil, ali),
//		provider.NewSMSProvider("tencent", nil, tencent),
//	}
//}
