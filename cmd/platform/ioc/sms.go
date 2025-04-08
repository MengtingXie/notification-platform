package ioc

import (
	sms2 "gitee.com/flycash/notification-platform/internal/service/provider/sms"
	"github.com/gotomicro/ego/core/econf"
)

func InitAliyunSms() sms2.Client {
	type Config struct {
		RegionID        string `yaml:"regionId"`
		AccessKeyID     string `yaml:"accessKeyId"`
		AccessKeySecret string `yaml:"accessKeySecret"`
	}
	var cfg Config
	err := econf.UnmarshalKey("sms.aliyun", &cfg)
	if err != nil {
		panic(err)
	}
	cli, err := sms2.NewAliyunSMS(cfg.RegionID, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		panic(err)
	}
	return cli
}

func InitTxSms() sms2.Client {
	type Config struct {
		RegionID        string `yaml:"regionId"`
		AccessKeyID     string `yaml:"accessKeyId"`
		AccessKeySecret string `yaml:"accessKeySecret"`
		AppID           string `yaml:"appId"`
	}
	var cfg Config
	err := econf.UnmarshalKey("sms.tx", &cfg)
	if err != nil {
		panic(err)
	}
	cli, err := sms2.NewTencentCloudSMS(cfg.RegionID, cfg.AccessKeyID, cfg.AccessKeySecret, cfg.AppID)
	if err != nil {
		panic(err)
	}
	return cli
}

func InitSmsClients() map[string]sms2.Client {
	return map[string]sms2.Client{
		"aliyun":       InitAliyunSms(),
		"tencentcloud": InitTxSms(),
	}
}
