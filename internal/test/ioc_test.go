package test

import (
	"log"
	"testing"

	"gitee.com/flycash/notification-platform/internal/test/ioc"
)

func TestIoc(*testing.T) {
	ioc.InitDB()
	log.Println("启动db成功")
	ioc.InitCache()
	log.Println("启动redis成功")
	ioc.InitMQ()
	log.Println("启动mq成功")
	ioc.InitEtcdClient()
	log.Println("启动etcd成功")
}
