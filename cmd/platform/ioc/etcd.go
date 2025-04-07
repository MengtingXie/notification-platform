package ioc

import (
	"github.com/ego-component/eetcd"
)

// 初始化 etcd 客户端
func InitEtcdClient() *eetcd.Component {
	return eetcd.Load("etcd").Build()
}


