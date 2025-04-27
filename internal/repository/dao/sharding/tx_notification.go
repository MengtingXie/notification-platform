package sharding

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	idgen "gitee.com/flycash/notification-platform/internal/pkg/id_generator"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/sharding"
	"github.com/ecodeclub/ekit/syncx"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TxNShardingDAO struct {
	dbs                 *syncx.Map[string, *gorm.DB]
	nShardingStrategy   sharding.ShardingStrategy
	txnShardingStrategy sharding.ShardingStrategy
	idGen               idgen.Generator
}

// NewTxNShardingDAO creates a new TxNShardingDAO with the provided dependencies
func NewTxNShardingDAO(
	dbs *syncx.Map[string, *gorm.DB],
	nStrategy sharding.ShardingStrategy,
	txnStrategy sharding.ShardingStrategy,
) *TxNShardingDAO {
	return &TxNShardingDAO{
		dbs:                 dbs,
		nShardingStrategy:   nStrategy,
		txnShardingStrategy: txnStrategy,
	}
}

func (t *TxNShardingDAO) Create(ctx context.Context, notification dao.TxNotification) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TxNShardingDAO) FindCheckBack(ctx context.Context, offset, limit int) ([]dao.TxNotification, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TxNShardingDAO) UpdateCheckStatus(ctx context.Context, txNotifications []dao.TxNotification, status domain.SendStatus) error {
	//TODO implement me
	panic("implement me")
}

func (t *TxNShardingDAO) First(ctx context.Context, txID int64) (dao.TxNotification, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TxNShardingDAO) BatchGetTxNotification(ctx context.Context, txIDs []int64) (map[int64]dao.TxNotification, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TxNShardingDAO) GetByBizIDKey(ctx context.Context, bizID int64, key string) (dao.TxNotification, error) {
	//TODO implement me
	panic("implement me")
}

func (t *TxNShardingDAO) UpdateNotificationID(ctx context.Context, bizID int64, key string, notificationID uint64) error {
	//TODO implement me
	panic("implement me")
}

func (t *TxNShardingDAO) Prepare(ctx context.Context, txn dao.TxNotification, notification dao.Notification) (uint64, error) {
	nowTime := time.Now()
	now := nowTime.UnixMilli()
	txn.Ctime = now
	txn.Utime = now
	notification.Ctime = now
	notification.Utime = now
	// 获取db
	txndst := t.txnShardingStrategy.Shard(txn.BizID, txn.Key)
	notificationDst := t.nShardingStrategy.Shard(notification.BizID, notification.Key)
	gormDB, ok := t.dbs.Load(txndst.DB)
	if !ok {
		return 0, fmt.Errorf("未知库名 %s", notificationDst.DB)
	}

	err := gormDB.Transaction(func(tx *gorm.DB) error {
		for {
			notification.ID = uint64(t.idGen.GenerateID(notification.BizID, notification.Key, nowTime))
			res := tx.WithContext(ctx).
				Table(notificationDst.Table).
				Create(&notification)
			if res.Error != nil {
				if errors.Is(res.Error, gorm.ErrDuplicatedKey) {
					// 唯一键冲突直接返回
					if !dao.CheckErrIsIDDuplicate(notification.ID, res.Error) {
						return nil
					}
					// 主键冲突 重试再找个主键
					continue
				}
				return res.Error
			}
			if res.RowsAffected == 0 {
				return nil
			}
			txn.NotificationID = notification.ID
			return tx.WithContext(ctx).
				Table(txndst.Table).
				Clauses(clause.OnConflict{
					DoNothing: true,
				}).Create(&txn).Error
		}
	})
	return notification.ID, err

}

func (t *TxNShardingDAO) UpdateStatus(ctx context.Context, bizID int64, key string, status domain.TxNotificationStatus, notificationStatus domain.SendStatus) error {
	// 获取db
	now := time.Now().UnixMilli()
	txndst := t.txnShardingStrategy.Shard(bizID, key)
	notificationDst := t.nShardingStrategy.Shard(bizID, key)
	gormDB, ok := t.dbs.Load(txndst.DB)
	if !ok {
		return fmt.Errorf("未知库名 %s", txndst.DB)
	}

	return gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.WithContext(ctx).
			Table(txndst.Table).
			Model(&dao.TxNotification{}).
			Where("biz_id = ? AND `key` = ? AND status = 'PREPARE'", bizID, key).
			Updates(map[string]any{
				"status": status,
				"utime":  now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return dao.ErrUpdateStatusFailed
		}
		return tx.WithContext(ctx).
			Table(notificationDst.Table).
			Model(&dao.Notification{}).
			Where("biz_id = ? AND `key` = ? ", bizID, key).
			Updates(map[string]any{
				"status": notificationStatus,
				"utime":  now,
			}).Error
	})
}
