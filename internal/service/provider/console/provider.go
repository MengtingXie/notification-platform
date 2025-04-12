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

package console

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
	"github.com/gotomicro/ego/core/elog"
)

// 你可以在这里提供一个 provider 输出到控制台的实现

type Provider struct {
	logger *elog.Component
}

func NewProvider() *Provider {
	return &Provider{
		logger: elog.DefaultLogger,
	}
}

func (p *Provider) Send(_ context.Context, notification domain.Notification) (domain.SendResponse, error) {
	p.logger.Info("发送通知", elog.Any("notification", notification))
	return domain.SendResponse{Status: domain.SendStatusSucceeded}, nil
}
