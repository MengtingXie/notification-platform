package sharding

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/pkg/loopjob"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/syncx"
	"github.com/ego-component/egorm"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type NotificationTask struct {
	dbs *syncx.Map[string, *egorm.Component]
}

func (n *NotificationTask) Create(_ context.Context, _ dao.Notification) (dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) CreateWithCallbackLog(_ context.Context, _ dao.Notification) (dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) BatchCreate(_ context.Context, _ []dao.Notification) ([]dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) BatchCreateWithCallbackLog(_ context.Context, _ []dao.Notification) ([]dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) GetByID(_ context.Context, _ uint64) (dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) BatchGetByIDs(_ context.Context, _ []uint64) (map[uint64]dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) GetByKey(_ context.Context, _ int64, _ string) (dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) GetByKeys(_ context.Context, _ int64, _ ...string) ([]dao.Notification, error) {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) CASStatus(_ context.Context, _ dao.Notification) error {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) UpdateStatus(_ context.Context, _ dao.Notification) error {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) BatchUpdateStatusSucceededOrFailed(_ context.Context, _, _ []dao.Notification) error {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) FindReadyNotifications(ctx context.Context, offset, limit int) ([]dao.Notification, error) {
	dbName, tableName, err := n.getDBTabFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	gormDB, ok := n.dbs.Load(dbName)
	if !ok {
		return nil, fmt.Errorf("未知库名 %s", dbName)
	}
	var res []dao.Notification
	now := time.Now().UnixMilli()
	err = gormDB.WithContext(ctx).
		Table(tableName).
		Where("scheduled_stime <=? AND scheduled_etime >= ? AND status=?", now, now, domain.SendStatusPending.String()).
		Limit(limit).Offset(offset).
		Find(&res).Error
	return res, err
}

func (n *NotificationTask) MarkSuccess(_ context.Context, _ dao.Notification) error {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) MarkFailed(_ context.Context, _ dao.Notification) error {
	// TODO implement me
	panic("implement me")
}

func (n *NotificationTask) MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error) {
	now := time.Now()
	ddl := now.Add(-time.Minute).UnixMilli()
	var rowsAffected int64
	db, tab, err := n.getDBTabFromCtx(ctx)
	if err != nil {
		return rowsAffected, err
	}
	gormDB, ok := n.dbs.Load(db)
	if !ok {
		return rowsAffected, fmt.Errorf("未知库名 %s", db)
	}

	err = gormDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var idsToUpdate []uint64

		// 查询需要更新的 ID
		err := tx.Model(&dao.Notification{}).
			Table(tab).
			Select("id").
			Where("status = ? AND utime <= ?", domain.SendStatusSending.String(), ddl).
			Limit(batchSize).
			Find(&idsToUpdate).Error

		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// 没有找到需要更新的记录，直接成功返回 (事务将提交)
		if len(idsToUpdate) == 0 {
			rowsAffected = 0
			return nil
		}

		// 根据查询到的 ID 集合更新记录
		res := tx.Model(&dao.Notification{}).
			Where("id IN ?", idsToUpdate).
			Updates(map[string]any{
				"status":  domain.SendStatusFailed.String(),
				"version": gorm.Expr("version + 1"),
				"utime":   now.UnixMilli(),
			})

		rowsAffected = res.RowsAffected
		return res.Error
	})

	return rowsAffected, err
}

func (n *NotificationTask) getDBTabFromCtx(ctx context.Context) (db, ntab string, err error) {
	dbName, ok := loopjob.DBFromCtx(ctx)
	if !ok {
		return "", "", errors.New("db在ctx中没找到")
	}

	nVal := ctx.Value(ntabName)
	if nVal == nil {
		return "", "", errors.New("nTab表在ctx中没找到")
	}
	ntab, ok = nVal.(string)
	if !ok {
		return "", "", errors.New("nTab表不是字符串")
	}
	return dbName, ntab, nil
}
