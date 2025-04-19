package hotreload

import (
	"errors"

	"gitee.com/flycash/notification-platform/internal/domain"
	"github.com/ecodeclub/ekit/spi"
	"github.com/ecodeclub/ekit/syncx"
)

// 核心接口
type MyChannel interface {
	// 按照名字索引
	Name() string
	Send(n domain.Notification) error
}

// MyServiceFacade 主管分发
type MyServiceFacade struct {
	services syncx.Map[string, MyChannel]
	dir      string
}

// Reload 可以通过 HTTP 接口来触发，并且在 HTTP 里面传入一些必要的参数
// 多实例部署的时候，要记住 HTTP 得把所有的实例都调用一遍
func (s *MyServiceFacade) Reload() error {
	svcs, err := spi.LoadService[MyChannel](s.dir, "MyChannel")
	if err != nil {
		return err
	}
	for _, svc := range svcs {
		s.services.Store(svc.Name(), svc)
	}
	return nil
}

func (s *MyServiceFacade) Send(n domain.Notification) error {
	channel := string(n.Channel)
	svc, ok := s.services.Load(channel)
	if !ok {
		return errors.New("找不到对应的渠道")
	}
	return svc.Send(n)
}
