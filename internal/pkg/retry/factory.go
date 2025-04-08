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

package retry

import (
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"github.com/ecodeclub/ekit/retry"
	"time"
)



func NewRetry(cfg domain.RetryConfig) (retry.Strategy, error) {
	// 根据 config 中的字段来检测
	switch cfg.Type {
	case "fixed":
		return retry.NewFixedIntervalRetryStrategy(msToDuration(cfg.FixedInterval.Interval), cfg.FixedInterval.MaxRetries)
	case "exponential":
		return retry.NewExponentialBackoffRetryStrategy(msToDuration(cfg.ExponentialBackoff.InitialInterval), msToDuration(cfg.ExponentialBackoff.MaxInterval), cfg.ExponentialBackoff.MaxRetries)
	default:
		return nil, fmt.Errorf("unknown retry type: %s", cfg.Type)
	}

}

func msToDuration(ms int) time.Duration {
	return time.Duration(ms * 1e6) // 3ms = 3,000,000ns
}
