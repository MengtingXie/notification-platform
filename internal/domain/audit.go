package domain

type ResourceType string

const (
	ResourceTypeTemplate ResourceType = "Template"
)

type Audit struct {
	ResourceID   int64        // 模版版本ID
	ResourceType ResourceType // TEMPLATE
	Content      string       // 完整JSON串，模版信息-基本+版本+渠道名（多个）
}
