package dao

type Quota struct {
	ID uint64 `gorm:"primaryKey;comment:'雪花算法ID'"`
	// 构成一个唯一索引
	BizID   int64  `gorm:"type:BIGINT;NOT NULL;uniqueIndex:biz_id_channel,priority:1;comment:'业务配表ID，业务方可能有多个业务每个业务配置不同'"`
	Channel string `gorm:"type:ENUM('SMS','EMAIL','IN_APP');uniqueIndex:biz_id_channel;NOT NULL;comment:'发送渠道'"`
	// 每个月的 quota
	// 如果你要分开控制不同渠道的 Quota，那么就加一个 Channel 列
	// 确保不同 Channel 使用不同的 Quota 来规避更新的锁竞争（CAS 等）
	Quota int32

	// 版本号，用于 CAS，你可以考虑使用 CAS 来更新
	// Version int `gorm:"type:INT;NOT NULL;DEFAULT:1;comment:'版本号，用于CAS操作'"`
	// 时间戳，毫秒数
	Utime int64
	Ctime int64
}
