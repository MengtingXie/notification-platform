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

package notification

import (
	"context"
	"gitee.com/flycash/notification-platform/internal/pkg/loopjob"
	"gitee.com/flycash/notification-platform/internal/repository"
	"github.com/meoying/dlock-go"
	"time"
)

type SendingTimeoutTask struct {
	dclient dlock.Client
	repo    repository.NotificationRepository
}

func (s *SendingTimeoutTask) Start(ctx context.Context) {
	const key = "notification_handling_sending_timeout"
	lj := loopjob.NewInfiniteLoop(s.dclient, s.HandleSendingTimeout, key)
	lj.Run(ctx)
}

func (s *SendingTimeoutTask) HandleSendingTimeout(ctx context.Context) error {
	const batchSize = 10
	cnt, err := s.repo.MarkTimeoutSendingAsFailed(ctx, batchSize)
	if err != nil {
		return err
	}
	// 说明 SENDING 的不多，可以休息一下
	if cnt < batchSize {
		// 这里可以随便设置，在分钟以内都可以
		time.Sleep(time.Second * 10)
	}
	return nil
}
