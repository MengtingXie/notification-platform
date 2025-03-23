package dao

import (
	"database/sql/driver"
	"fmt"
)

type (
	// NotificationChannel 定义通知渠道的类型
	NotificationChannel string

	// NotificationStatus 定义通知状态的类型
	NotificationStatus string
)

// 定义枚举常量
const (
	NotificationChannelSMS   NotificationChannel = "SMS"
	NotificationChannelEmail NotificationChannel = "EMAIL"
	NotificationChannelInApp NotificationChannel = "IN_APP"

	NotificationStatusPrepare   NotificationStatus = "PREPARE"
	NotificationStatusCanceled  NotificationStatus = "CANCELED"
	NotificationStatusPending   NotificationStatus = "PENDING"
	NotificationStatusSucceeded NotificationStatus = "SUCCEEDED"
	NotificationStatusFailed    NotificationStatus = "FAILED"
)

var (
	// ValidNotificationChannels 定义有效的通知渠道的集合
	ValidNotificationChannels = map[NotificationChannel]struct{}{
		NotificationChannelSMS:   {},
		NotificationChannelEmail: {},
		NotificationChannelInApp: {},
	}
	// ValidNotificationStatuses 定义有效的通知状态的集合
	ValidNotificationStatuses = map[NotificationStatus]struct{}{
		NotificationStatusPrepare:   {},
		NotificationStatusCanceled:  {},
		NotificationStatusPending:   {},
		NotificationStatusSucceeded: {},
		NotificationStatusFailed:    {},
	}

	ErrInvalidType  = fmt.Errorf("无效的类型")
	ErrInvalidValue = fmt.Errorf("无效的值")
	ErrNilValue     = fmt.Errorf("值不能为null")
)

// String 实现 Stringer 接口
func (nc *NotificationChannel) String() string {
	return string(*nc)
}

// Value 实现 driver.Valuer 接口
func (nc *NotificationChannel) Value() (driver.Value, error) {
	return string(*nc), nil
}

// Scan 实现 sql.Scanner 接口
func (nc *NotificationChannel) Scan(value any) error {
	if value == nil {
		return fmt.Errorf("%w: 通知渠道", ErrNilValue)
	}

	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case []byte:
		strValue = string(v)
	default:
		return fmt.Errorf("%w: 通知渠道 %T", ErrInvalidType, value)
	}

	_, valid := ValidNotificationChannels[NotificationChannel(strValue)]
	if !valid {
		return fmt.Errorf("%w: 通知渠道 %s", ErrInvalidValue, strValue)
	}

	*nc = NotificationChannel(strValue)
	return nil
}

// String 实现 Stringer 接口
func (ns *NotificationStatus) String() string {
	return string(*ns)
}

// Value 实现 driver.Valuer 接口
func (ns *NotificationStatus) Value() (driver.Value, error) {
	return string(*ns), nil
}

// Scan 实现 sql.Scanner 接口
func (ns *NotificationStatus) Scan(value any) error {
	if value == nil {
		return fmt.Errorf("%w: 通知状态", ErrNilValue)
	}

	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case []byte:
		strValue = string(v)
	default:
		return fmt.Errorf("%w: 通知状态 %T", ErrInvalidType, value)
	}

	_, valid := ValidNotificationStatuses[NotificationStatus(strValue)]
	if !valid {
		return fmt.Errorf("%w: 通知状态 %s", ErrInvalidValue, strValue)
	}

	*ns = NotificationStatus(strValue)
	return nil
}
