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

package channel

import (
	"context"
	"fmt"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/service/provider"
)

type baseChannel struct {
	builder provider.Builder
}

func (s *baseChannel) Send(ctx context.Context, notification domain.Notification) (domain.SendResponse, error) {
	selector, err := s.builder.Build()
	if err != nil {
		return domain.SendResponse{}, fmt.Errorf("%w: %w", errs.ErrSendNotificationFailed, err)
	}

	var retryCount int8
	for {
		// 获取供应商
		p, err1 := selector.Next(ctx, notification)
		if err1 != nil {
			// 没有可用的供应商
			return domain.SendResponse{RetryCount: retryCount}, err1
		}

		// 使用当前供应商发送
		resp, err2 := p.Send(ctx, notification)
		if err2 == nil {
			// 发送成功，填写重试次数
			resp.RetryCount += retryCount
			return resp, nil
		}

		retryCount += resp.RetryCount
	}
}

type smsChannel struct {
	baseChannel
}

func NewSMSChannel(builder provider.Builder) Channel {
	return &smsChannel{
		baseChannel{
			builder: builder,
		},
	}
}

type emailChannel struct {
	baseChannel
}

func NewEmailChannel(builder provider.Builder) Channel {
	return &smsChannel{
		baseChannel{
			builder: builder,
		},
	}
}
