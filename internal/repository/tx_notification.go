package repository

import (
	"context"
	"errors"
	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/notification"

	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/repository/dao"
)

var ErrUpdateStatusFailed = errors.New("没有更新")

type TxNotificationRepository interface {
	Create(ctx context.Context, notification domain.TxNotification) (int64, error)
	Find(ctx context.Context, offset, limit int) ([]domain.TxNotification, error)
	UpdateStatus(ctx context.Context, txIDs int64, status string) error
	UpdateCheckStatus(ctx context.Context, txNotifications []domain.TxNotification) error
	// First 通过事务id查找对应的事务
	First(ctx context.Context, txID int64) (domain.TxNotification, error)
	BatchGetTxNotification(ctx context.Context, txIDs []int64) (map[int64]domain.TxNotification, error)
	GetByBizIDKey(ctx context.Context, bizID int64, key string) (domain.TxNotification, error)
	UpdateNotificationID(ctx context.Context, bizID int64, key string, notificationID uint64) error
}

type txNotificationRepo struct {
	txdao dao.TxNotificationDAO
}

// NewTxNotificationRepository creates a new TxNotificationRepository instance
func NewTxNotificationRepository(txdao dao.TxNotificationDAO) TxNotificationRepository {
	return &txNotificationRepo{
		txdao: txdao,
	}
}

func (t *txNotificationRepo) UpdateNotificationID(ctx context.Context, bizID int64, key string, notificationID uint64) error {
	return t.txdao.UpdateNotificationID(ctx, bizID, key, notificationID)
}

func (t *txNotificationRepo) GetByBizIDKey(ctx context.Context, bizID int64, key string) (domain.TxNotification, error) {
	notifyEntity, err := t.txdao.GetByBizIDKey(ctx, bizID, key)
	if err != nil {
		return domain.TxNotification{}, err
	}
	return t.toDomain(notifyEntity), nil
}

func (t *txNotificationRepo) BatchGetTxNotification(ctx context.Context, txIDs []int64) (map[int64]domain.TxNotification, error) {
	taMap, err := t.txdao.BatchGetTxNotification(ctx, txIDs)
	if err != nil {
		return nil, err
	}
	domainTxnMap := make(map[int64]domain.TxNotification, len(taMap))
	for txid, tx := range taMap {
		domainTxnMap[txid] = t.toDomain(tx)
	}
	return domainTxnMap, nil
}

func (t *txNotificationRepo) First(ctx context.Context, txID int64) (domain.TxNotification, error) {
	noti, err := t.txdao.First(ctx, txID)
	if err != nil {
		return domain.TxNotification{}, err
	}
	return t.toDomain(noti), nil
}

func (t *txNotificationRepo) Create(ctx context.Context, notification domain.TxNotification) (int64, error) {
	// 转换领域模型到DAO对象
	daoNotification := t.toDao(notification)
	// 调用DAO层创建记录
	return t.txdao.Create(ctx, daoNotification)
}

func (t *txNotificationRepo) Find(ctx context.Context, offset, limit int) ([]domain.TxNotification, error) {
	// 调用DAO层查询记录
	daoNotifications, err := t.txdao.Find(ctx, offset, limit)
	if err != nil {
		return nil, err
	}

	// 将DAO对象列表转换为领域模型列表
	result := make([]domain.TxNotification, 0, len(daoNotifications))
	for _, daoNotification := range daoNotifications {
		result = append(result, t.toDomain(daoNotification))
	}
	return result, nil
}

func (t *txNotificationRepo) UpdateStatus(ctx context.Context, txID int64, status string) error {
	// 直接调用DAO层更新状态
	return t.txdao.UpdateStatus(ctx, txID, status)
}

func (t *txNotificationRepo) UpdateCheckStatus(ctx context.Context, txNotifications []domain.TxNotification) error {
	// 将领域模型列表转换为DAO对象列表
	daoNotifications := make([]dao.TxNotification, 0, len(txNotifications))
	for idx := range txNotifications {
		txNotification := txNotifications[idx]
		daoNotifications = append(daoNotifications, t.toDao(txNotification))
	}

	// 调用DAO层更新检查状态
	return t.txdao.UpdateCheckStatus(ctx, daoNotifications)
}

// toDomain 将DAO对象转换为领域模型
func (t *txNotificationRepo) toDomain(daoNotification dao.TxNotification) domain.TxNotification {
	return domain.TxNotification{
		TxID: daoNotification.TxID,
		Notification: notification.Notification{
			ID: daoNotification.NotificationID,
		},
		Key:           daoNotification.Key,
		BizID:         daoNotification.BizID,
		Status:        domain.TxNotificationStatus(daoNotification.Status),
		CheckCount:    daoNotification.CheckCount,
		NextCheckTime: daoNotification.NextCheckTime,
		Ctime:         daoNotification.Ctime,
		Utime:         daoNotification.Utime,
	}
}

// toDao 将领域模型转换为DAO对象
func (t *txNotificationRepo) toDao(domainNotification domain.TxNotification) dao.TxNotification {
	return dao.TxNotification{
		TxID:           domainNotification.TxID,
		Key:            domainNotification.Key,
		NotificationID: domainNotification.Notification.ID,
		BizID:          domainNotification.BizID,
		Status:         string(domainNotification.Status),
		CheckCount:     domainNotification.CheckCount,
		NextCheckTime:  domainNotification.NextCheckTime,
		Ctime:          domainNotification.Ctime,
		Utime:          domainNotification.Utime,
	}
}
