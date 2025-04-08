package provider

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/domain"
)

// Selector 供应商选择器接口
type Selector interface {
	// Next 获取下一个供应商
	Next(ctx context.Context, notification domain.Notification) (Provider, error)
	// Reset 重置选择器状态
	Reset()
}

// Builder 供应商选择器的构造器
type Builder interface {
	// Build 构造选择器，可以在Build方法上添加参数来构建更复杂选择器
	Build() (Selector, error)
}
