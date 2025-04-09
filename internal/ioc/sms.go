package ioc

import (
	"gitee.com/flycash/notification-platform/internal/service/provider/sms/client"
	"github.com/gotomicro/ego/core/econf"
)

func InitAliyunSms() client.Client {
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
	cli, err := client.NewAliyunSMS(cfg.RegionID, cfg.AccessKeyID, cfg.AccessKeySecret)
	if err != nil {
		panic(err)
	}
	return cli
}

func InitTxSms() client.Client {
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
	cli, err := client.NewTencentCloudSMS(cfg.RegionID, cfg.AccessKeyID, cfg.AccessKeySecret, cfg.AppID)
	if err != nil {
		panic(err)
	}
	return cli
}

func InitSmsClients() map[string]client.Client {
	return map[string]client.Client{
		"aliyun":       InitAliyunSms(),
		"tencentcloud": InitTxSms(),
	}
}
