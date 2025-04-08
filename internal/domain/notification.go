package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/errs"
)

// Channel 通知渠道

// SendStatus 通知状态
type SendStatus string

const (
	SendStatusPrepare   SendStatus = "PREPARE"   // 准备中
	SendStatusCanceled  SendStatus = "CANCELED"  // 已取消
	SendStatusPending   SendStatus = "PENDING"   // 待发送
	SendStatusSucceeded SendStatus = "SUCCEEDED" // 发送成功
	SendStatusFailed    SendStatus = "FAILED"    // 发送失败
)

type Template struct {
	ID        int64             // 模板ID
	VersionID int64             // 版本ID
	Params    map[string]string // 渲染模版时使用的参数
}

// Notification 通知领域模型
type Notification struct {
	ID                 uint64     // 通知唯一标识
	BizID              int64      // 业务唯一标识
	Key                string     // 业务内唯一标识
	Receivers          []string   // 接收者(手机/邮箱/用户ID)
	Channel            Channel    // 发送渠道
	Template           Template   // 关联的模版
	Status             SendStatus // 发送状态
	RetryCount         int8       // 当前重试次数
	ScheduledSTime     time.Time  // 计划发送开始时间
	ScheduledETime     time.Time  // 计划发送结束时间
	Version            int        // 版本号
	SendStrategyConfig SendStrategyConfig
}

func (n *Notification) SetSendTime() {
	stime, etime := n.SendStrategyConfig.SendTimeWindow()
	n.ScheduledSTime = stime
	n.ScheduledETime = etime
}

func (n *Notification) Validate() error {
	if n.BizID <= 0 {
		return fmt.Errorf("%w: BizID = %d", errs.ErrInvalidParameter, n.BizID)
	}

	if n.Key == "" {
		return fmt.Errorf("%w: Key = %q", errs.ErrInvalidParameter, n.Key)
	}

	if len(n.Receivers) == 0 {
		return fmt.Errorf("%w: Receivers= %v", errs.ErrInvalidParameter, n.Receivers)
	}

	if n.Channel != ChannelSMS && n.Channel != ChannelEmail && n.Channel != ChannelInApp {
		return fmt.Errorf("%w: Channel = %q", errs.ErrInvalidParameter, n.Channel)
	}

	if n.Template.ID <= 0 {
		return fmt.Errorf("%w: Template.ID = %d", errs.ErrInvalidParameter, n.Template.ID)
	}

	if n.Template.VersionID <= 0 {
		return fmt.Errorf("%w: Template.VersionID = %d", errs.ErrInvalidParameter, n.Template.VersionID)
	}

	if len(n.Template.Params) == 0 {
		return fmt.Errorf("%w: Template.Params = %q", errs.ErrInvalidParameter, n.Template.Params)
	}

	if err := n.SendStrategyConfig.Validate(); err != nil {
		return err
	}

	return nil
}

func (n *Notification) IsValidBizID() error {
	if n.BizID <= 0 {
		return fmt.Errorf("%w: BizID = %d", errs.ErrInvalidParameter, n.BizID)
	}
	return nil
}

func (n *Notification) MarshalReceivers() (string, error) {
	return n.marshal(n.Receivers)
}

func (n *Notification) MarshalTemplateParams() (string, error) {
	return n.marshal(n.Template.Params)
}

func (n *Notification) marshal(v any) (string, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(jsonBytes), nil
}
