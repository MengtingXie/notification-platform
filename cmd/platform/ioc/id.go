package ioc

import (
	"github.com/sony/sonyflake"
	"time"
)

func InitIDGenerator() *sonyflake.Sonyflake {
	// 使用固定设置的ID生成器
	return sonyflake.NewSonyflake(sonyflake.Settings{
		StartTime: time.Now(),
		MachineID: func() (uint16, error) {
			return 1, nil
		},
	})
}
