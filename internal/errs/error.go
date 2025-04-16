package errs

import (
	"errors"
)

// 定义统一的错误类型
var (
	// 业务错误
	ErrInvalidParameter         = errors.New("参数错误")
	ErrSendNotificationFailed   = errors.New("发送通知失败")
	ErrNotificationNotFound     = errors.New("通知记录不存在")
	ErrCreateNotificationFailed = errors.New("创建通知失败")
	ErrBizIDNotFound            = errors.New("BizID不存在")
	ErrTemplateNotFound         = errors.New("模板不存在")
	ErrChannelDisabled          = errors.New("渠道已禁用")
	ErrRateLimited              = errors.New("频率受限")
	ErrNoAvailableProvider      = errors.New("无可用供应商")
	ErrNoAvailableChannel       = errors.New("无可用渠道")
	ErrConfigNotFound           = errors.New("业务配置不存在")
	ErrNoQuotaConfig            = errors.New("没有提供 Quota 有关的配置")
	ErrNoQuota                  = errors.New("额度已经用完")
	ErrQuotaNotFound            = errors.New("额度记录不存在")
	ErrProviderNotFound         = errors.New("供应商记录不存在")
	ErrUnknownChannel           = errors.New("未知渠道类型")

	// 系统错误
	ErrNotificationDuplicate       = errors.New("通知记录主键冲突")
	ErrNotificationVersionMismatch = errors.New("通知记录版本不匹配")
	ErrCreateCallbackLogFailed     = errors.New("创建回调记录失败")
	ErrDatabaseError               = errors.New("数据库错误")
	ErrExternalServiceError        = errors.New("外部服务调用错误")
)
