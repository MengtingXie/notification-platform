package domain

// Channel 通知渠道
type Channel string

const (
	ChannelSMS   Channel = "SMS"    // 短信
	ChannelEmail Channel = "EMAIL"  // 邮件
	ChannelInApp Channel = "IN_APP" // 站内信
)

// Status 供应商状态
type ProviderStatus string

const (
	ProviderStatusActive   Status = "ACTIVE"   // 激活
	ProviderStatusInactive Status = "INACTIVE" // 未激活
)

// Provider 供应商领域模型
type Provider struct {
	ID      int64   // 供应商ID
	Name    string  // 供应商名称
	Code    string  // 供应商编码
	Channel Channel // 支持的渠道

	// 基本信息
	Endpoint  string // API入口地址
	RegionID  string
	APIKey    string // API密钥
	APISecret string // API密钥
	APPID     string

	Weight     int // 权重
	QPSLimit   int // 每秒请求数限制
	DailyLimit int // 每日请求数限制

	AuditCallbackURL string // 审核请求回调地址
	Status           Status
}
