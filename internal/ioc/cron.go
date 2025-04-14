package ioc

import (
	"gitee.com/flycash/notification-platform/internal/service/quota"
	"github.com/gotomicro/ego/task/ecron"
)

func Crons(q *quota.QuotaMonthlyResetCron) []ecron.Ecron {
	q1 := ecron.Load("cron").Build(ecron.WithJob(q.Do))
	return []ecron.Ecron{q1}
}
