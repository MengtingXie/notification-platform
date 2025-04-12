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

package provider

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
)

// Provider 供应商接口
type Provider interface {
	// Send 发送消息
	Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error)
}

// Selector 供应商选择器接口
type Selector interface {
	// Next 获取下一个供应商
	Next(ctx context.Context, notification domain.Notification) (Provider, error)
}

// SelectorBuilder 供应商选择器的构造器
type SelectorBuilder interface {
	// Build 构造选择器，可以在Build方法上添加参数来构建更复杂选择器
	Build() (Selector, error)
}
