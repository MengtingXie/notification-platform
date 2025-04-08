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

package errs

import (
	"errors"
	"gitee.com/flycash/notification-platform/internal/repository"
)

// 定义统一的错误类型
var (
	// ErrInvalidParameter 表示参数错误
	ErrInvalidParameter             = errors.New("参数错误")
	ErrSendNotificationFailed       = errors.New("发送通知失败")
	ErrNotificationIDGenerateFailed = errors.New("通知ID生成失败")
	ErrNotificationNotFound         = repository.ErrNotificationNotFound
	ErrCreateNotificationFailed     = errors.New("创建通知失败")
	ErrNotificationDuplicate        = repository.ErrNotificationDuplicate
)
