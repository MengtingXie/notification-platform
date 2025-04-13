package ioc

import "github.com/google/wire"

var BaseSet = wire.NewSet(InitDBAndTables, InitProviderEncryptKey, InitCache, InitMQ, InitIDGenerator, InitRedis, InitRedisClient)
