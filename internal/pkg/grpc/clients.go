// Copyright 2023 ecodeclub
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package grpc

import (
	"fmt"
	"github.com/ecodeclub/ekit/syncx"
	"github.com/gotomicro/ego/client/egrpc"
)

type Clients[T any] struct {
	clientMap syncx.Map[string, T]
	creator   func(conn *egrpc.Component) T
}

func NewClients[T any](creator func(conn *egrpc.Component) T) *Clients[T] {
	return &Clients[T]{creator: creator}
}

func (c *Clients[T]) Get(serviceName string) T {
	client, ok := c.clientMap.Load(serviceName)
	if !ok {
		// 我要初始化 client
		// ego 如果服务发现失败，会 panic
		grpcConn := egrpc.Load("").Build(egrpc.WithAddr(fmt.Sprintf("etcd:///%s", serviceName)))
		client = c.creator(grpcConn)
		c.clientMap.Store(serviceName, client)
	}
	return client
}
