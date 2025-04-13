package ioc

import "github.com/google/wire"

var BaseSet = wire.NewSet(InitDBAndTables, InitCache, InitMQ, InitIDGenerator, InitRedis, InitRedisClient)
