package sharding

import (
	"context"
	"fmt"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"gitee.com/flycash/notification-platform/internal/sharding"
	"github.com/ecodeclub/ekit/list"
	"github.com/ecodeclub/ekit/syncx"
	"github.com/go-sql-driver/mysql"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
	"sync"
	"time"
)

const (
	maxBatchSize = 20
)

type NotificationSvc struct {
	dbs                     syncx.Map[string, *gorm.DB]
	db                      *gorm.DB
	notificationShardingSvc sharding.ShardingSvc
	callbackLogShardingSvc  sharding.ShardingSvc
}

// isUniqueConstraintError 检查是否是唯一索引冲突错误
func (d *NotificationSvc) isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	me := new(mysql.MySQLError)
	if ok := errors.As(err, &me); ok {
		const uniqueIndexErrNo uint16 = 1062
		return me.Number == uniqueIndexErrNo
	}
	return false
}
func (s *NotificationSvc) Create(ctx context.Context, data dao.Notification) (dao.Notification, error) {
	return s.create(ctx, data, false)
}

func (s *NotificationSvc) create(ctx context.Context, data dao.Notification, createCallbackLog bool) (dao.Notification, error) {
	now := time.Now().UnixMilli()
	data.Ctime, data.Utime = now, now
	data.Version = 1
	// 获取分库分表规则
	notificationDst := s.notificationShardingSvc.Shard(data.BizID, data.Key)
	callBackLogDst := s.callbackLogShardingSvc.Shard(data.BizID, data.Key)
	notiDB, ok := s.dbs.Load(notificationDst.DB)
	if !ok {
		return dao.Notification{}, fmt.Errorf("未知库名 %s", notificationDst.DB)
	}
	// notification 和callbacklog 是在同一个库的但是 表不同
	err := notiDB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table(notificationDst.Table).Create(&data).Error; err != nil {
			if s.isUniqueConstraintError(err) {
				return fmt.Errorf("%w", errs.ErrNotificationDuplicate)
			}
			return err
		}
		if createCallbackLog {
			if err := tx.Table(callBackLogDst.Table).Create(&dao.CallbackLog{
				NotificationID: data.ID,
				Status:         domain.CallbackLogStatusInit.String(),
				NextRetryTime:  now,
			}).Error; err != nil {
				return fmt.Errorf("%w", errs.ErrCreateCallbackLogFailed)
			}
		}
		return nil
	})
	return data, err
}

func (s *NotificationSvc) CreateWithCallbackLog(ctx context.Context, data dao.Notification) (dao.Notification, error) {
	return s.create(ctx, data, true)
}

func (s *NotificationSvc) BatchCreate(ctx context.Context, dataList []dao.Notification) ([]dao.Notification, error) {
	return s.batchCreate(ctx, dataList, false)
}

func (s *NotificationSvc) BatchCreateWithCallbackLog(ctx context.Context, datas []dao.Notification) ([]dao.Notification, error) {
	return s.batchCreate(ctx, datas, true)
}

func (s *NotificationSvc) GetByID(ctx context.Context, id uint64) (dao.Notification, error) {
	dst := s.notificationShardingSvc.ShardWithID(int64(id))
	gormdb, ok := s.dbs.Load(dst.DB)
	if !ok {
		return dao.Notification{}, fmt.Errorf("未知库名 %s", dst.DB)
	}
	var data dao.Notification
	err := gormdb.WithContext(ctx).Table(dst.Table).
		Where("id = ?", id).
		First(&data).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return dao.Notification{}, fmt.Errorf("%w: id=%d", errs.ErrNotificationNotFound, id)
		}
		return dao.Notification{}, err
	}
	return data, nil
}

func (s *NotificationSvc) BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]dao.Notification, error) {
	idsMap := make(map[[2]string][]uint64, len(ids))
	for _, id := range ids {
		dst := s.notificationShardingSvc.ShardWithID(int64(id))
		v, ok := idsMap[[2]string{
			dst.DB,
			dst.Table,
		}]
		if ok {
			v = append(v, id)
		} else {
			v = []uint64{id}
		}
		idsMap[[2]string{
			dst.DB,
			dst.Table,
		}] = v
	}
	notifiMap := make(map[uint64]dao.Notification, len(idsMap))
	mu := new(sync.RWMutex)
	var eg errgroup.Group
	for key := range idsMap {
		dbTab := key
		dbIds := idsMap[dbTab]
		eg.Go(func() error {
			var notifications []dao.Notification
			dbName := dbTab[0]
			tableName := dbTab[1]
			gormdb, ok := s.dbs.Load(dbName)
			if !ok {
				return fmt.Errorf("未知库名 %s", dbName)
			}
			err := gormdb.WithContext(ctx).
				Table(tableName).
				Where("id in (?)", dbIds).
				Find(&notifications).Error

			for idx := range notifications {
				notification := notifications[idx]
				mu.Lock()
				notifiMap[notification.ID] = notification
				mu.Unlock()
			}
			return err
		})
	}
	return notifiMap, eg.Wait()
}

func (s *NotificationSvc) GetByKey(ctx context.Context, bizID int64, key string) (dao.Notification, error) {
	dst := s.notificationShardingSvc.Shard(bizID, key)
	gormdb, ok := s.dbs.Load(dst.DB)
	if !ok {
		return dao.Notification{}, fmt.Errorf("未知库名 %s", dst.DB)
	}
	var data dao.Notification
	err := gormdb.WithContext(ctx).Table(dst.Table).
		Where("`key` = ? AND `biz_id` = ?", key, bizID).
		First(&data).Error
	if err != nil {
		return dao.Notification{}, fmt.Errorf("查询通知列表失败:bizID: %d, key %s %w", bizID, key, err)
	}
	return data, nil
}

func (s *NotificationSvc) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]dao.Notification, error) {
	notiMap := make(map[[2]string][]string, len(keys))
	for idx := range keys {
		key := keys[idx]
		dst := s.notificationShardingSvc.Shard(bizID, key)
		v, ok := notiMap[[2]string{
			dst.DB,
			dst.Table,
		}]
		if !ok {
			v = []string{
				key,
			}
		} else {
			v = append(v, key)
		}
		notiMap[[2]string{
			dst.DB,
			dst.Table,
		}] = v
	}

	var eg errgroup.Group
	notificationList := list.NewArrayList[dao.Notification](len(keys))
	for tabDBKey, ks := range notiMap {
		tabDBKey := tabDBKey
		ks := ks
		eg.Go(func() error {
			dbName := tabDBKey[0]
			tabName := tabDBKey[1]
			gromDb, ok := s.dbs.Load(dbName)
			if !ok {
				return fmt.Errorf("未知库名 %s", dbName)
			}
			var data []dao.Notification
			err := gromDb.WithContext(ctx).
				Table(tabName).Where("`key` in ? AND `biz_id` = ?", ks, bizID).Find(&data).Error
			if err != nil {
				return err
			}
			return notificationList.Append(data...)
		})
	}
	return notificationList.AsSlice(), eg.Wait()
}

func (s *NotificationSvc) CASStatus(ctx context.Context, notification dao.Notification) error {
	updates := map[string]any{
		"status":  notification.Status,
		"version": gorm.Expr("version + 1"),
		"utime":   time.Now().Unix(),
	}
	dst := s.notificationShardingSvc.ShardWithID(int64(notification.ID))
	gormDb, ok := s.dbs.Load(dst.DB)
	if !ok {
		return fmt.Errorf("未知库名 %s", dst.DB)
	}
	result := gormDb.WithContext(ctx).
		Table(dst.Table).
		Where("id = ? AND version = ?", notification.ID, notification.Version).
		Updates(updates)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected < 1 {
		return fmt.Errorf("并发竞争失败 %w, id %d", errs.ErrNotificationVersionMismatch, notification.ID)
	}
	return nil
}

func (s *NotificationSvc) UpdateStatus(ctx context.Context, notification dao.Notification) error {
	dst := s.notificationShardingSvc.ShardWithID(int64(notification.ID))
	gormDb, ok := s.dbs.Load(dst.DB)
	if !ok {
		return fmt.Errorf("未知库名 %s", dst.DB)
	}
	return gormDb.WithContext(ctx).
		Table(dst.Table).
		Model(&dao.Notification{}).
		Where("id = ?", notification.ID).
		Updates(map[string]any{
			"status":  notification.Status,
			"version": gorm.Expr("version + 1"),
			"utime":   time.Now().Unix(),
		}).Error
}

func (s *NotificationSvc) BatchUpdateStatusSucceededOrFailed(ctx context.Context, successNotifications, failedNotifications []dao.Notification) error {
	if len(successNotifications) == 0 && len(failedNotifications) == 0 {
		return nil
	}
	type modifyIds struct {
		callBackTab string
		successIds  []uint64
		failedIds   []uint64
	}

	tabdbMap := make(map[[2]string]modifyIds)

	for idx := range successNotifications {
		no := successNotifications[idx]
		dst := s.notificationShardingSvc.ShardWithID(int64(no.ID))
		callbackLogDst := s.callbackLogShardingSvc.ShardWithID(int64(no.ID))
		v, ok := tabdbMap[[2]string{dst.DB, dst.Table}]
		if !ok {
			v = modifyIds{
				callBackTab: callbackLogDst.Table,
				successIds:  []uint64{no.ID},
			}
		} else {
			v.successIds = append(v.successIds, no.ID)
		}
	}

	for idx := range failedNotifications {
		no := successNotifications[idx]
		dst := s.notificationShardingSvc.ShardWithID(int64(no.ID))
		v, ok := tabdbMap[[2]string{dst.DB, dst.Table}]
		if !ok {
			v = modifyIds{
				failedIds: []uint64{no.ID},
			}
		} else {
			v.failedIds = append(v.failedIds, no.ID)
		}
	}

	var eg errgroup.Group
	for tabDBKey := range tabdbMap {
		tabDBKey := tabDBKey
		idItem := tabdbMap[tabDBKey]
		eg.Go(func() error {
			dbName := tabDBKey[0]
			tabName := tabDBKey[1]
			gormDb, ok := s.dbs.Load(dbName)
			if !ok {
				return fmt.Errorf("未知库名 %s", dbName)
			}
			return gormDb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				if len(idItem.successIds) != 0 {
					err := s.batchMarkSuccess(tx, tabName, idItem.callBackTab, idItem.successIds)
					if err != nil {
						return err
					}
				}

				if len(idItem.failedIds) != 0 {
					now := time.Now().Unix()
					return tx.Model(&dao.Notification{}).
						Table(tabName).
						Where("id IN ?", idItem.failedIds).
						Updates(map[string]any{
							"version": gorm.Expr("version + 1"),
							"utime":   now,
							"status":  domain.SendStatusFailed.String(),
						}).Error
				}
				return nil
			})
		})

	}
	return eg.Wait()
}

// FindReadyNotifications 这个是循环任务用的不在这个dao中实现
func (s *NotificationSvc) FindReadyNotifications(ctx context.Context, offset, limit int) ([]dao.Notification, error) {
	//TODO implement me
	panic("implement me")
}

func (s *NotificationSvc) MarkSuccess(ctx context.Context, entity dao.Notification) error {
	now := time.Now().UnixMilli()
	dst := s.notificationShardingSvc.ShardWithID(int64(entity.ID))
	callbackLogDst := s.callbackLogShardingSvc.ShardWithID(int64(entity.ID))
	gormDb, ok := s.dbs.Load(dst.DB)
	if !ok {
		return fmt.Errorf("未知库名 %s", dst.DB)
	}
	return gormDb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.
			Table(dst.Table).
			Model(&dao.Notification{}).
			Where("id = ?", entity.ID).
			Updates(map[string]any{
				"status":  entity.Status,
				"utime":   now,
				"version": gorm.Expr("version + 1"),
			}).Error
		if err != nil {
			return err
		}
		// 要把 callback log 标记为可以发送了
		return tx.Model(&dao.CallbackLog{}).
			Table(callbackLogDst.Table).
			Where("notification_id = ?", entity.ID).
			Updates(map[string]any{
				// 标记为可以发送回调了
				"status": domain.CallbackLogStatusPending,
				"utime":  now,
			}).Error
	})
}

func (s *NotificationSvc) MarkFailed(ctx context.Context, entity dao.Notification) error {
	now := time.Now().UnixMilli()
	dst := s.notificationShardingSvc.ShardWithID(int64(entity.ID))
	gormDb, ok := s.dbs.Load(dst.DB)
	if !ok {
		return fmt.Errorf("未知库名 %s", dst.DB)
	}
	return gormDb.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.
			Model(&dao.Notification{}).
			Table(dst.Table).
			Where("id = ?", entity.ID).
			Updates(map[string]any{
				"status":  entity.Status,
				"utime":   now,
				"version": gorm.Expr("version + 1"),
			}).Error
	})
}

func (s *NotificationSvc) MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error) {
	//TODO implement me
	panic("implement me")
}

func (d *NotificationSvc) batchCreate(ctx context.Context, datas []dao.Notification, createCallbackLog bool) ([]dao.Notification, error) {
	if len(datas) == 0 {
		return []dao.Notification{}, nil
	}
	if len(datas) > maxBatchSize {
		return nil, fmt.Errorf("一批最多%d个消息", maxBatchSize)
	}
	now := time.Now().UnixMilli()
	for i := range datas {
		datas[i].Ctime, datas[i].Utime = now, now
		datas[i].Version = 1
	}
	// 前一个是库名，后一个是表名
	notiMap := make(map[[2]string][]dao.Notification)
	callbackLogMap := make(map[[2]string][]dao.CallbackLog)
	for idx := range datas {
		data := datas[idx]
		dst := d.notificationShardingSvc.ShardWithID(int64(data.ID))
		v, ok := notiMap[[2]string{dst.DB, dst.Table}]
		if ok {
			v = append(v, data)
		} else {
			v = []dao.Notification{data}
		}
		notiMap[[2]string{dst.DB, dst.Table}] = v
		callBackDst := d.callbackLogShardingSvc.ShardWithID(int64(data.ID))
		vv, ok := callbackLogMap[[2]string{callBackDst.DB, callBackDst.Table}]
		callbackLog := dao.CallbackLog{
			NotificationID: data.ID,
			NextRetryTime:  now,
			Ctime:          now,
			Utime:          now,
		}
		if ok {
			vv = append(vv, callbackLog)
		} else {
			vv = []dao.CallbackLog{callbackLog}
		}
		callbackLogMap[[2]string{callBackDst.DB, callBackDst.Table}] = vv
	}
	var eg errgroup.Group
	for key := range notiMap {
		dbTab := key
		data := notiMap[key]
		eg.Go(func() error {
			gromDb, ok := d.dbs.Load(dbTab[0])
			if !ok {
				return fmt.Errorf("库名%s没找到", dbTab[0])
			}
			return gromDb.WithContext(ctx).Table(dbTab[1]).Create(&data).Error
		})
	}
	if createCallbackLog {
		for key := range callbackLogMap {
			dbTab := key
			callbackLog := callbackLogMap[key]
			eg.Go(func() error {
				gromDb, ok := d.dbs.Load(dbTab[0])
				if !ok {
					return fmt.Errorf("库名%s没找到", dbTab[0])
				}
				return gromDb.WithContext(ctx).Table(dbTab[1]).Create(&callbackLog).Error
			})
		}
	}
	return datas, eg.Wait()
}

func (d *NotificationSvc) batchMarkSuccess(tx *gorm.DB, notiTab, callbackLogTab string, successIDs []uint64) error {
	now := time.Now().Unix()
	err := tx.Model(&dao.Notification{}).
		Table(notiTab).
		Where("id IN ?", successIDs).
		Updates(map[string]any{
			"version": gorm.Expr("version + 1"),
			"utime":   now,
			"status":  domain.SendStatusSucceeded.String(),
		}).Error
	if err != nil {
		return err
	}

	// 要更新 callback log 了
	return tx.Model(&dao.CallbackLog{}).
		Table(callbackLogTab).
		Where("notification_id IN ? ", successIDs).
		Updates(map[string]any{
			"status": domain.CallbackLogStatusPending.String(),
			"utime":  now,
		}).Error
}
