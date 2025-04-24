package sharding

import (
	"fmt"
	"gitee.com/flycash/notification-platform/internal/pkg/hash"
	idgen "gitee.com/flycash/notification-platform/internal/pkg/id_generator"
)

type ShardingSvc struct {
	dbPrefix      string
	tablePrefix   string
	tableSharding int64
	dbSharding    int64
}

type Dst struct {
	Table string
	DB    string
}

func (s *ShardingSvc) Shard(bizID int64, key string) Dst {
	hashValue := hash.Hash(bizID, key)
	dbHash := hashValue % s.dbSharding
	tabHash := (hashValue / s.dbSharding) % s.tableSharding
	return Dst{
		Table: fmt.Sprintf("%s_%d", s.tablePrefix, tabHash),
		DB:    fmt.Sprintf("%s_%d", s.dbPrefix, dbHash),
	}
}

func (s *ShardingSvc) ShardWithID(id int64) Dst {
	hashValue := idgen.ExtractHashValue(id)
	dbHash := hashValue % s.dbSharding
	tabHash := (hashValue / s.dbSharding) % s.tableSharding
	return Dst{
		Table: fmt.Sprintf("%s_%d", s.tablePrefix, tabHash),
		DB:    fmt.Sprintf("%s_%d", s.dbPrefix, dbHash),
	}
}
