package domain

// Channel 通知渠道
type Channel string

const (
	ChannelSMS   Channel = "SMS"    // 短信
	ChannelEmail Channel = "EMAIL"  // 邮件
	ChannelInApp Channel = "IN_APP" // 站内信
)

// Status 通知状态
type Status string

const (
	StatusPrepare   Status = "PREPARE"   // 准备中
	StatusCanceled  Status = "CANCELED"  // 已取消
	StatusPending   Status = "PENDING"   // 待发送
	StatusSucceeded Status = "SUCCEEDED" // 发送成功
	StatusFailed    Status = "FAILED"    // 发送失败
)

type Template struct {
	ID        int64             // 模板ID
	VersionID int64             // 版本ID
	Params    map[string]string // 渲染模版时使用的参数
}

// Notification 通知领域模型
type Notification struct {
	ID             uint64   // 通知唯一标识
	BizID          int64    // 业务唯一标识
	Key            string   // 业务内唯一标识
	Receiver       string   // 接收者(手机/邮箱/用户ID)
	Channel        Channel  // 发送渠道
	Template       Template // 关联的模版
	Status         Status   // 发送状态
	RetryCount     int8     // 当前重试次数
	ScheduledSTime int64    // 计划发送开始时间
	ScheduledETime int64    // 计划发送结束时间
	SendTime       int64    // 实际发送时间
}
