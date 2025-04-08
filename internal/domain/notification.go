package domain

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
	ID             uint64     // 通知唯一标识
	BizID          int64      // 业务唯一标识
	Key            string     // 业务内唯一标识
	Receiver       string     // 接收者(手机/邮箱/用户ID)
	Channel        Channel    // 发送渠道
	Template       Template   // 关联的模版
	Status         SendStatus // 发送状态
	RetryCount     int8       // 当前重试次数
	ScheduledSTime int64      // 计划发送开始时间
	ScheduledETime int64      // 计划发送结束时间
	Version        int        // 版本号

	SendStrategyConfig SendStrategyConfig
}
