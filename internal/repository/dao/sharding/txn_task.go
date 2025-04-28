package sharding

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/pkg/loopjob"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/syncx"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

const (
	txnTabName loopjob.CtxKey = "txnTab"
	ntabName   loopjob.CtxKey = "nTab"
)

// 专门为task
type TxnTaskDAO struct {
	dbs *syncx.Map[string, *gorm.DB]
}

func NewTxnTaskDAO(dbs *syncx.Map[string, *gorm.DB]) *TxnTaskDAO {
	return &TxnTaskDAO{
		dbs: dbs,
	}
}

func (t *TxnTaskDAO) FindCheckBack(ctx context.Context, offset, limit int) ([]dao.TxNotification, error) {
	db, txnTab, _, err := t.getDBTabFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	gormDB, ok := t.dbs.Load(db)
	if !ok {
		return nil, fmt.Errorf("未知库名 %s", db)
	}
	var txns []dao.TxNotification
	currentTime := time.Now().UnixMilli()
	err = gormDB.WithContext(ctx).
		Table(txnTab).
		Where("status = ? AND next_check_time <= ? AND next_check_time > 0", domain.TxNotificationStatusPrepare, currentTime).
		Offset(offset).
		Limit(limit).
		Order("next_check_time").
		Find(&txns).Error
	return txns, err
}

func (t *TxnTaskDAO) UpdateCheckStatus(ctx context.Context, txNotifications []dao.TxNotification, status domain.SendStatus) error {
	db, txnTab, ntab, err := t.getDBTabFromCtx(ctx)
	if err != nil {
		return err
	}
	gormDB, ok := t.dbs.Load(db)
	if !ok {
		return fmt.Errorf("未知库名 %s", db)
	}
	sqls := make([]string, 0, len(txNotifications))
	now := time.Now().UnixMilli()
	notificationIDs := make([]uint64, 0, len(txNotifications))
	for _, txNotification := range txNotifications {
		updateSQL := fmt.Sprintf("UPDATE `%s` set `status` = '%s',`utime` = %d ,`next_check_time` = %d,`check_count` = %d WHERE `key` = '%s' AND `biz_id` = %d AND `status` = 'PREPARE'", txnTab, txNotification.Status, now, txNotification.NextCheckTime, txNotification.CheckCount, txNotification.Key, txNotification.BizID)
		sqls = append(sqls, updateSQL)
		notificationIDs = append(notificationIDs, txNotification.NotificationID)
	}
	// 拼接所有SQL并执行
	// UPDATE xxx; UPDATE xxx;UPDATE xxx;
	if len(sqls) > 0 {
		return gormDB.Transaction(func(tx *gorm.DB) error {
			combinedSQL := strings.Join(sqls, "; ")
			err := tx.WithContext(ctx).Exec(combinedSQL).Error
			if err != nil {
				return err
			}
			if status != domain.SendStatusPrepare {
				return tx.
					Table(ntab).
					WithContext(ctx).Model(&dao.Notification{}).
					Where("id in ?", notificationIDs).
					Update("status", status).Error
			}
			return nil
		})
	}
	return nil
}

func (t *TxnTaskDAO) First(_ context.Context, _ int64) (dao.TxNotification, error) {
	// TODO implement me
	panic("implement me")
}

func (t *TxnTaskDAO) BatchGetTxNotification(_ context.Context, _ []int64) (map[int64]dao.TxNotification, error) {
	// TODO implement me
	panic("implement me")
}

func (t *TxnTaskDAO) GetByBizIDKey(_ context.Context, _ int64, _ string) (dao.TxNotification, error) {
	// TODO implement me
	panic("implement me")
}

func (t *TxnTaskDAO) UpdateNotificationID(_ context.Context, _ int64, _ string, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (t *TxnTaskDAO) Prepare(_ context.Context, _ dao.TxNotification, _ dao.Notification) (uint64, error) {
	// TODO implement me
	panic("implement me")
}

func (t *TxnTaskDAO) UpdateStatus(_ context.Context, _ int64, _ string, _ domain.TxNotificationStatus, _ domain.SendStatus) error {
	// TODO implement me
	panic("implement me")
}

func (t *TxnTaskDAO) getDBTabFromCtx(ctx context.Context) (db, txnTab, ntab string, err error) {
	dbName, ok := loopjob.DBFromCtx(ctx)
	if !ok {
		return "", "", "", errors.New("db在ctx中没找到")
	}
	txnVal := ctx.Value(txnTabName)
	if txnVal == nil {
		return "", "", "", errors.New("txnTab表在ctx中没找到")
	}
	txnTab, ok = txnVal.(string)
	if !ok {
		return "", "", "", errors.New("txnTab不是字符串")
	}

	nVal := ctx.Value(ntabName)
	if nVal == nil {
		return "", "", "", errors.New("nTab表在ctx中没找到")
	}
	ntab, ok = nVal.(string)
	if !ok {
		return "", "", "", errors.New("nTab表不是字符串")
	}
	return dbName, txnTab, ntab, nil
}
