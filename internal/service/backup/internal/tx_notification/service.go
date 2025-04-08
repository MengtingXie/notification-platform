package txnotification

import (
	"context"

	"gitee.com/flycash/notification-platform/internal/repository"
	"gitee.com/flycash/notification-platform/internal/service/notification/retry"

	"gitee.com/flycash/notification-platform/internal/service/config"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	"github.com/meoying/dlock-go"
	dlockRedis "github.com/meoying/dlock-go/redis"
	"github.com/redis/go-redis/v9"
)

func InitService(repo repository.TxNotificationRepository,
	notificationSvc notification.Service,
	configSvc config.Service,
	retryStrategyBuilder retry.Builder,
	lock dlock.Client,
) Service {
	txSvc := notification.NewTxNotificationService(repo, notificationSvc, configSvc, retryStrategyBuilder, lock)
	txSvc.StartTask(context.Background())
	return txSvc
}

func InitDlickClient(redisClient redis.Cmdable) dlock.Client {
	return dlockRedis.NewClient(redisClient)
}
