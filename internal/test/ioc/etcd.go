package ioc

import (
	"github.com/ego-component/eetcd"
	"github.com/gotomicro/ego/core/econf"
)

// 初始化 etcd 客户端
func InitEtcdClient() *eetcd.Component {
	econf.Set("etcd", map[string]any{
		"addrs":          []string{"127.0.0.1:2379"},
		"secure":         false,
		"connectTimeout": "1s",
	})
	return eetcd.Load("etcd").Build()
}
