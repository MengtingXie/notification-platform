package sharding

import (
	"fmt"

	"gitee.com/flycash/notification-platform/internal/pkg/hash"
	idgen "gitee.com/flycash/notification-platform/internal/pkg/id_generator"
)

type ShardingStrategy struct {
	dbPrefix      string
	tablePrefix   string
	tableSharding int64
	dbSharding    int64
}

type Dst struct {
	Table string
	DB    string
}

func NewShardingStrategy(dbPrefix, tablePrefix string, tableSharding, dbSharding int64) ShardingStrategy {
	return ShardingStrategy{
		dbSharding:    dbSharding,
		tableSharding: tableSharding,
		dbPrefix:      dbPrefix,
		tablePrefix:   tablePrefix,
	}
}

func (s ShardingStrategy) Shard(bizID int64, key string) Dst {
	hashValue := hash.Hash(bizID, key)
	dbHash := hashValue % s.dbSharding
	tabHash := (hashValue / s.dbSharding) % s.tableSharding
	return Dst{
		Table: fmt.Sprintf("%s_%d", s.tablePrefix, tabHash),
		DB:    fmt.Sprintf("%s_%d", s.dbPrefix, dbHash),
	}
}

func (s ShardingStrategy) ShardWithID(id int64) Dst {
	hashValue := idgen.ExtractHashValue(id)
	dbHash := hashValue % s.dbSharding
	tabHash := (hashValue / s.dbSharding) % s.tableSharding
	return Dst{
		Table: fmt.Sprintf("%s_%d", s.tablePrefix, tabHash),
		DB:    fmt.Sprintf("%s_%d", s.dbPrefix, dbHash),
	}
}

func (s ShardingStrategy) Broadcast() []Dst {
	ans := make([]Dst, 0, s.tableSharding*s.dbSharding)
	for i := 0; i < int(s.dbSharding); i++ {
		for j := 0; j < int(s.tableSharding); j++ {
			ans = append(ans, Dst{
				Table: fmt.Sprintf("%s_%d", s.tablePrefix, j),
				DB:    fmt.Sprintf("%s_%d", s.dbPrefix, i),
			})
		}
	}
	return ans
}

// 获取所有库名
func (s ShardingStrategy) DBs() []string {
	ans := make([]string, 0, s.dbSharding)
	for i := 0; i < int(s.dbSharding); i++ {
		ans = append(ans, fmt.Sprintf("%s_%d", s.dbPrefix, i))
	}
	return ans
}

// 获取一个库中所有的表名
func (s ShardingStrategy) Tables() []string {
	ans := make([]string, 0, s.tableSharding)
	for i := 0; i < int(s.tableSharding); i++ {
		ans = append(ans, fmt.Sprintf("%s_%d", s.tablePrefix, i))
	}
	return ans
}

func (s ShardingStrategy) TablePrefix() string {
	return s.tablePrefix
}

func (s ShardingStrategy) TableSuffix() []string {
	ans := make([]string, 0, s.tableSharding)
	for i := 0; i < int(s.tableSharding); i++ {
		ans = append(ans, fmt.Sprintf("%d", i))
	}
	return ans
}
