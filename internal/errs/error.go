package errs

import (
	"errors"
)

// 定义统一的错误类型
var (
	ErrInvalidParameter             = errors.New("参数错误")
	ErrSendNotificationFailed       = errors.New("发送通知失败")
	ErrNotificationIDGenerateFailed = errors.New("通知ID生成失败")
	ErrNotificationNotFound         = errors.New("通知记录不存在")
	ErrCreateNotificationFailed     = errors.New("创建通知失败")
	ErrNotificationDuplicate        = errors.New("通知记录主键冲突")
	ErrNotificationVersionMismatch  = errors.New("通知记录版本不匹配")
	ErrCreateCallbackLogFailed      = errors.New("创建回调记录失败")

	ErrNoAvailableProvider = errors.New("无可用供应商")
	ErrNoAvailableChannel  = errors.New("无可用渠道")

	ErrConfigNotFound   = errors.New("业务配置不存在")
	ErrNoQuota          = errors.New("额度已经用完")
	ErrProviderNotFound = errors.New("供应商记录不存在")
)
