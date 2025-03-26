package domain

// BusinessConfig 业务配置领域对象
type BusinessConfig struct {
	ID            int64  // 业务标识
	OwnerID       int64  // 业务方ID
	OwnerType     string // 业务方类型：person-个人,organization-组织
	ChannelConfig string // 渠道配置，JSON格式
	TxnConfig     string // 事务配置，JSON格式
	RateLimit     int    // 每秒最大请求数
	Quota         string // 配额设置，JSON格式
	RetryPolicy   string // 重试策略，JSON格式
	Ctime         int64  // 创建时间
	Utime         int64  // 更新时间
}
