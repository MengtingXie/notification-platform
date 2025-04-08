package domain

// BusinessConfig 业务配置领域对象
type BusinessConfig struct {
	ID            int64         // 业务标识
	OwnerID       int64         // 业务方ID
	OwnerType     string        // 业务方类型：person-个人,organization-组织
	ChannelConfig *ChannelConfig // 渠道配置，JSON格式
	TxnConfig     *TxnConfig     // 事务配置，JSON格式
	RateLimit     int           // 每秒最大请求数
	Quota         *QuotaConfig   // 配额设置，JSON格式
	RetryPolicy   *RetryConfig   // 重试策略，JSON格式
	Ctime         int64         // 创建时间
	Utime         int64         // 更新时间
}
type QuotaConfig struct {
	Monthly MonthlyConfig `json:"monthly"`
}
type MonthlyConfig struct {
	SMS   int `json:"SMS"`
	EMAIL int `json:"EMAIL"`
}
type ChannelConfig struct {
	Channels []ChannelItem `json:"channels"`
}
type ChannelItem struct {
	Channel  string `json:"channel"`
	Priority int `json:"priority"`
	Enabled  bool `json:"enabled"`
}

type TxnConfig struct {
	// 方法名
	ServiceName string `json:"serviceName"`
	// 期望事务在 initialDelay秒后完成
	InitialDelay int `json:"initialDelay"`
}

type RetryConfig struct {
	Type               string                    `json:"type"` // 重试策略
	FixedInterval      *FixedIntervalConfig      `json:"fixedInterval"`
	ExponentialBackoff *ExponentialBackoffConfig `json:"exponentialBackoff"`
}

type ExponentialBackoffConfig struct {
	// 初始重试间隔 单位ms
	InitialInterval int `json:"initialInterval"`
	// 最大重试间隔 单位ms
	MaxInterval int `json:"maxInterval"`
	// 最大重试次数
	MaxRetries int32 `json:"maxRetries"`
}

type FixedIntervalConfig struct {
	MaxRetries int32 `json:"maxRetries"`
	Interval   int   `json:"interval"`
}
