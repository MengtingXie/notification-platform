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

package quota

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository"
)

type Service interface {
	ResetQuota(ctx context.Context, biz domain.BusinessConfig) error
}

type service struct {
	repo repository.QuotaRepository
}

func NewService(repo repository.QuotaRepository) Service {
	return &service{repo: repo}
}

func (s *service) ResetQuota(ctx context.Context, biz domain.BusinessConfig) error {
	if biz.Quota == nil {
		return errs.ErrNoQuotaConfig
	}
	sms := domain.Quota{
		BizID:   biz.ID,
		Quota:   int32(biz.Quota.Monthly.SMS),
		Channel: domain.ChannelSMS,
	}
	email := domain.Quota{
		BizID:   biz.ID,
		Quota:   int32(biz.Quota.Monthly.EMAIL),
		Channel: domain.ChannelEmail,
	}
	return s.repo.CreateOrUpdate(ctx, sms, email)
}
