package ioc

import (
	"gitee.com/flycash/notification-platform/internal/service/provider/sms"
	"github.com/gotomicro/ego/core/econf"
)

func InitAliyunSms() sms.Client {
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
	cli, err := sms.NewAliyunSMS(cfg.RegionID, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		panic(err)
	}
	return cli
}

func InitTxSms() sms.Client {
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
	cli, err := sms.NewTencentCloudSMS(cfg.RegionID, cfg.AccessKeyID, cfg.AccessKeySecret, cfg.AppID)
	if err != nil {
		panic(err)
	}
	return cli
}

func InitSmsClients() map[string]sms.Client {
	return map[string]sms.Client{
		"aliyun":       InitAliyunSms(),
		"tencentcloud": InitTxSms(),
	}
}
