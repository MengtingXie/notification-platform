//go:build e2e

package integration

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/integration/testgrpc"
	"github.com/ego-component/eetcd/registry"
	"github.com/gotomicro/ego/client/egrpc/resolver"
	"gorm.io/gorm"

	tx_notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/tx_notification/v1"
	"gitee.com/flycash/notification-platform/internal/service/config"
	configmocks "gitee.com/flycash/notification-platform/internal/service/config/mocks"
	"gitee.com/flycash/notification-platform/internal/service/notification"
	notificationmocks "gitee.com/flycash/notification-platform/internal/service/notification/mocks"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/domain"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/integration/startup"
	"gitee.com/flycash/notification-platform/internal/service/tx_notification/internal/repository/dao"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ego-component/egorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type TxNotificationServiceTestSuite struct {
	suite.Suite
	db *egorm.Component
}

const bufSize = 1024 * 1024

func (s *TxNotificationServiceTestSuite) SetupSuite() {
	s.db = testioc.InitDB()
	dao.InitTables(s.db)
}

func (s *TxNotificationServiceTestSuite) TearDownTest() {
	// s.db.Exec("TRUNCATE TABLE `tx_notifications`")
}

func (s *TxNotificationServiceTestSuite) TestPrepare() {
	testcases := []struct {
		name            string
		input           domain.TxNotification
		configSvc       func(t *testing.T, ctrl *gomock.Controller) config.Service
		notificationSvc func(t *testing.T, ctrl *gomock.Controller) notification.Service
		after           func(t *testing.T, txId string, now int64)
	}{
		{
			name: "正常响应",
			input: domain.TxNotification{
				Notification: notification.Notification{
					BizID:    3,
					Key:      "case_01",
					Receiver: "18248842099",
					Channel:  "channel_01",
					Template: notification.Template{
						ID:        1,
						VersionID: 10,
						Params: map[string]string{
							"key": "value",
						},
					},
					Status:         notification.StatusPending,
					RetryCount:     9,
					ScheduledETime: 123,
					ScheduledSTime: 321,
					SendTime:       123,
				},
				BizID: 3,
			},
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				mockConfigServices := configmocks.NewMockBusinessConfigService(ctrl)
				mockConfigServices.EXPECT().
					GetByID(gomock.Any(), int64(3)).
					Return(config.BusinessConfig{
						ID:        3,
						TxnConfig: `{"type":"normal","maxRetryTimes":3,"interval":10}`,
					}, nil)
				return mockConfigServices
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				mockNotificationService.
					EXPECT().
					CreateNotification(gomock.Any(), notification.Notification{
						BizID:    3,
						Key:      "case_01",
						Receiver: "18248842099",
						Channel:  "channel_01",
						Template: notification.Template{
							ID:        1,
							VersionID: 10,
							Params: map[string]string{
								"key": "value",
							},
						},
						Status:         notification.StatusPending,
						RetryCount:     9,
						ScheduledETime: 123,
						ScheduledSTime: 321,
						SendTime:       123,
					}).Return(notification.Notification{
					ID:       10123,
					BizID:    3,
					Key:      "case_01",
					Receiver: "18248842099",
					Channel:  "channel_01",
					Template: notification.Template{
						ID:        1,
						VersionID: 10,
						Params: map[string]string{
							"key": "value",
						},
					},
					Status:         notification.StatusPending,
					RetryCount:     9,
					ScheduledETime: 123,
					ScheduledSTime: 321,
					SendTime:       123,
				}, nil)
				return mockNotificationService
			},
			after: func(t *testing.T, txId string, now int64) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				var actual dao.TxNotification
				err := s.db.WithContext(ctx).Where("tx_id = ?", txId).First(&actual).Error
				require.NoError(s.T(), err)
				require.True(t, actual.Ctime > 0)
				require.True(t, actual.Utime > 0)
				nextCheckTime := now + 10000
				assert.Greater(t, actual.Ctime, nextCheckTime)
				actual.Utime = 0
				actual.Ctime = 0
				actual.NextCheckTime = 0
				assert.Equal(t, dao.TxNotification{
					TxID:           txId,
					NotificationID: 10123,
					BizID:          3,
					Status:         domain.TxNotificationStatusPrepare.String(),
				}, actual)
			},
		},
		{
			name: "配置没找到，下一次更新时间为0",
			input: domain.TxNotification{
				Notification: notification.Notification{
					BizID:    4,
					Key:      "case_02",
					Receiver: "18248842099",
					Channel:  "channel_01",
					Template: notification.Template{
						ID:        1,
						VersionID: 10,
						Params: map[string]string{
							"key": "value",
						},
					},
					Status:         notification.StatusPending,
					RetryCount:     9,
					ScheduledETime: 123,
					ScheduledSTime: 321,
					SendTime:       123,
				},
				BizID: 4,
			},
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				mockConfigServices := configmocks.NewMockBusinessConfigService(ctrl)
				mockConfigServices.EXPECT().
					GetByID(gomock.Any(), int64(4)).
					Return(config.BusinessConfig{}, gorm.ErrRecordNotFound)
				return mockConfigServices
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				mockNotificationService.
					EXPECT().
					CreateNotification(gomock.Any(), notification.Notification{
						BizID:    4,
						Key:      "case_02",
						Receiver: "18248842099",
						Channel:  "channel_01",
						Template: notification.Template{
							ID:        1,
							VersionID: 10,
							Params: map[string]string{
								"key": "value",
							},
						},
						Status:         notification.StatusPending,
						RetryCount:     9,
						ScheduledETime: 123,
						ScheduledSTime: 321,
						SendTime:       123,
					}).Return(notification.Notification{
					ID:       10124,
					BizID:    4,
					Key:      "case_02",
					Receiver: "18248842099",
					Channel:  "channel_01",
					Template: notification.Template{
						ID:        1,
						VersionID: 10,
						Params: map[string]string{
							"key": "value",
						},
					},
					Status:         notification.StatusPending,
					RetryCount:     9,
					ScheduledETime: 123,
					ScheduledSTime: 321,
					SendTime:       123,
				}, nil)
				return mockNotificationService
			},
			after: func(t *testing.T, txId string, now int64) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				var actual dao.TxNotification
				err := s.db.WithContext(ctx).Where("tx_id = ?", txId).First(&actual).Error
				require.NoError(s.T(), err)
				require.True(t, actual.Ctime > 0)
				require.True(t, actual.Utime > 0)
				actual.Utime = 0
				actual.Ctime = 0
				assert.Equal(t, dao.TxNotification{
					TxID:           txId,
					NotificationID: 10124,
					BizID:          4,
					Status:         domain.TxNotificationStatusPrepare.String(),
				}, actual)
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			now := time.Now().Unix()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			svc := startup.InitTxNotificationService(notification.Module{
				Svc: tc.notificationSvc(t, ctrl),
			}, config.Module{
				Svc: tc.configSvc(t, ctrl),
			})
			txid, err := svc.Prepare(ctx, tc.input)
			require.NoError(t, err)
			tc.after(t, txid, now)
		})
	}
}

func (s *TxNotificationServiceTestSuite) TestCommit() {
	testcases := []struct {
		name            string
		configSvc       func(t *testing.T, ctrl *gomock.Controller) config.Service
		notificationSvc func(t *testing.T, ctrl *gomock.Controller) notification.Service
		after           func(t *testing.T, bizId int64, key string)
		before          func(t *testing.T) (int64, string)
		checkErr        func(t *testing.T, err error) bool
	}{
		{
			name: "正常提交",
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				mockConfigServices := configmocks.NewMockBusinessConfigService(ctrl)
				return mockConfigServices
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				mockNotificationService.EXPECT().
					UpdateNotificationStatus(gomock.Any(), uint64(10123), notification.StatusSucceeded).
					Return(nil)
				return mockNotificationService
			},
			before: func(t *testing.T) (int64, string) {
				now := time.Now().UnixMilli()
				txn := dao.TxNotification{
					TxID:           "test-tx-id",
					Key:            "case_02",
					NotificationID: 10123,
					BizID:          3,
					Status:         domain.TxNotificationStatusPrepare.String(),
					CheckCount:     0,
					NextCheckTime:  now + 10000,
					Ctime:          now,
					Utime:          now,
				}
				err := s.db.Create(&txn).Error
				require.NoError(t, err)
				return txn.BizID, txn.Key
			},
			after: func(t *testing.T, bizId int64, key string) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				var actual dao.TxNotification
				err := s.db.WithContext(ctx).Where("biz_id = ? AND `key` = ?", bizId, key).First(&actual).Error
				require.NoError(t, err)
				assert.Equal(t, domain.TxNotificationStatusCommit.String(), actual.Status)
			},
			checkErr: func(t *testing.T, err error) bool {
				require.NoError(t, err)
				return false
			},
		},
		{
			name: "错误的事务id",
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				mockConfigServices := configmocks.NewMockBusinessConfigService(ctrl)
				return mockConfigServices
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				// 不需要模拟任何方法调用，因为应该在查找事务时就失败了
				return mockNotificationService
			},
			before: func(t *testing.T) (int64, string) {
				return 33, "123"
			},
			after: func(t *testing.T, bizId int64, key string) {
				// 不需要检查数据库，因为应该没有更新任何记录
			},
			checkErr: func(t *testing.T, err error) bool {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "查找事务失败")
				return true
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bizId, key := tc.before(t)

			svc := startup.InitTxNotificationService(notification.Module{
				Svc: tc.notificationSvc(t, ctrl),
			}, config.Module{
				Svc: tc.configSvc(t, ctrl),
			})

			err := svc.Commit(ctx, bizId, key)
			hasError := tc.checkErr(t, err)
			if !hasError {
				tc.after(t, bizId, key)
			}
		})
	}
}

func (s *TxNotificationServiceTestSuite) TestCancel() {
	testcases := []struct {
		name            string
		configSvc       func(t *testing.T, ctrl *gomock.Controller) config.Service
		notificationSvc func(t *testing.T, ctrl *gomock.Controller) notification.Service
		after           func(t *testing.T, bizId int64, key string)
		before          func(t *testing.T) (int64, string)
		checkErr        func(t *testing.T, err error) bool
	}{
		{
			name: "正常取消",
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				mockConfigServices := configmocks.NewMockBusinessConfigService(ctrl)
				return mockConfigServices
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				mockNotificationService.EXPECT().
					UpdateNotificationStatus(gomock.Any(), uint64(10123), notification.StatusCanceled).
					Return(nil)
				return mockNotificationService
			},
			before: func(t *testing.T) (int64, string) {
				now := time.Now().UnixMilli()
				txn := dao.TxNotification{
					TxID:           "test-tx-id-cancel",
					NotificationID: 10123,
					BizID:          5,
					Key:            "question_02",
					Status:         domain.TxNotificationStatusPrepare.String(),
					CheckCount:     0,
					NextCheckTime:  now + 10000,
					Ctime:          now,
					Utime:          now,
				}
				err := s.db.Create(&txn).Error
				require.NoError(t, err)
				return txn.BizID, txn.Key
			},
			after: func(t *testing.T, bizId int64, key string) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				var actual dao.TxNotification
				err := s.db.WithContext(ctx).Where("biz_id = ? AND `key` = ?", bizId, key).First(&actual).Error
				require.NoError(t, err)
				assert.Equal(t, domain.TxNotificationStatusCancel.String(), actual.Status)
			},
			checkErr: func(t *testing.T, err error) bool {
				require.NoError(t, err)
				return false
			},
		},
		{
			name: "错误的事务id",
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				mockConfigServices := configmocks.NewMockBusinessConfigService(ctrl)
				return mockConfigServices
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				// 不需要模拟任何方法调用，因为应该在查找事务时就失败了
				return mockNotificationService
			},
			before: func(t *testing.T) (int64, string) {
				// 返回一个不存在的事务ID
				return 333, "non-existent-tx-id-cancel"
			},
			after: func(t *testing.T, bizId int64, key string) {
				// 不需要检查数据库，因为应该没有更新任何记录
			},
			checkErr: func(t *testing.T, err error) bool {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "查找事务失败")
				return true
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bizId, key := tc.before(t)

			svc := startup.InitTxNotificationService(notification.Module{
				Svc: tc.notificationSvc(t, ctrl),
			}, config.Module{
				Svc: tc.configSvc(t, ctrl),
			})

			err := svc.Cancel(ctx, bizId, key)
			hasError := tc.checkErr(t, err)
			if !hasError {
				tc.after(t, bizId, key)
			}
		})
	}
}

func (s *TxNotificationServiceTestSuite) TestGetNotification() {
	testcases := []struct {
		name            string
		before          func(t *testing.T) (int64, string)
		notificationSvc func(t *testing.T, ctrl *gomock.Controller) notification.Service
		configSvc       func(t *testing.T, ctrl *gomock.Controller) config.Service
		checkResult     func(t *testing.T, txn domain.TxNotification, err error)
	}{
		{
			name: "正常获取事务通知",
			before: func(t *testing.T) (int64, string) {
				now := time.Now().UnixMilli()
				txn := dao.TxNotification{
					TxID:           "test-tx-id-get",
					Key:            "get-test-case-01",
					NotificationID: 10123,
					BizID:          5,
					Status:         domain.TxNotificationStatusPrepare.String(),
					CheckCount:     0,
					NextCheckTime:  now + 10000,
					Ctime:          now,
					Utime:          now,
				}
				err := s.db.Create(&txn).Error
				require.NoError(t, err)
				return txn.BizID, txn.Key
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				mockNotificationService.EXPECT().
					GetNotificationByID(gomock.Any(), uint64(10123)).
					Return(notification.Notification{
						ID:       10123,
						BizID:    5,
						Key:      "get-test-case-01",
						Receiver: "13800138000",
						Channel:  "SMS",
						Template: notification.Template{
							ID:        1,
							VersionID: 10,
							Params: map[string]string{
								"code": "123456",
							},
						},
						Status:         notification.StatusPending,
						RetryCount:     0,
						ScheduledSTime: 0,
						ScheduledETime: 0,
						SendTime:       0,
					}, nil)
				return mockNotificationService
			},
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				return configmocks.NewMockBusinessConfigService(ctrl)
			},
			checkResult: func(t *testing.T, txn domain.TxNotification, err error) {
				require.NoError(t, err)
				assert.Equal(t, "test-tx-id-get", txn.TxID)
				assert.Equal(t, int64(5), txn.BizID)
				assert.Equal(t, "get-test-case-01", txn.Key)
				assert.Equal(t, domain.TxNotificationStatusPrepare, txn.Status)

				// 验证通知详情
				assert.Equal(t, uint64(10123), txn.Notification.ID)
				assert.Equal(t, int64(5), txn.Notification.BizID)
				assert.Equal(t, "get-test-case-01", txn.Notification.Key)
				assert.Equal(t, "13800138000", txn.Notification.Receiver)
				assert.Equal(t, notification.Channel("SMS"), txn.Notification.Channel)
				assert.Equal(t, notification.StatusPending, txn.Notification.Status)
				assert.Equal(t, int64(1), txn.Notification.Template.ID)
				assert.Equal(t, int64(10), txn.Notification.Template.VersionID)
				assert.Equal(t, map[string]string{"code": "123456"}, txn.Notification.Template.Params)
			},
		},
		{
			name: "事务不存在",
			before: func(t *testing.T) (int64, string) {
				return 999, "non-existent-key"
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				return notificationmocks.NewMockNotificationService(ctrl)
			},
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				return configmocks.NewMockBusinessConfigService(ctrl)
			},
			checkResult: func(t *testing.T, txn domain.TxNotification, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "record not found")
				assert.Equal(t, domain.TxNotification{}, txn)
			},
		},
		{
			name: "通知不存在",
			before: func(t *testing.T) (int64, string) {
				now := time.Now().UnixMilli()
				txn := dao.TxNotification{
					TxID:           "test-tx-id-notfound",
					Key:            "get-test-case-notfound",
					NotificationID: 10456,
					BizID:          6,
					Status:         domain.TxNotificationStatusPrepare.String(),
					CheckCount:     0,
					NextCheckTime:  now + 10000,
					Ctime:          now,
					Utime:          now,
				}
				err := s.db.Create(&txn).Error
				require.NoError(t, err)
				return txn.BizID, txn.Key
			},
			notificationSvc: func(t *testing.T, ctrl *gomock.Controller) notification.Service {
				mockNotificationService := notificationmocks.NewMockNotificationService(ctrl)
				mockNotificationService.EXPECT().
					GetNotificationByID(gomock.Any(), uint64(10456)).
					Return(notification.Notification{}, errors.New("通知不存在"))
				return mockNotificationService
			},
			configSvc: func(t *testing.T, ctrl *gomock.Controller) config.Service {
				return configmocks.NewMockBusinessConfigService(ctrl)
			},
			checkResult: func(t *testing.T, txn domain.TxNotification, err error) {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "通知不存在")
				assert.Equal(t, domain.TxNotification{}, txn)
			},
		},
	}

	for _, tc := range testcases {
		tc := tc
		s.T().Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			bizId, key := tc.before(t)

			svc := startup.InitTxNotificationService(notification.Module{
				Svc: tc.notificationSvc(t, ctrl),
			}, config.Module{
				Svc: tc.configSvc(t, ctrl),
			})

			txn, err := svc.GetNotification(ctx, bizId, key)
			tc.checkResult(t, txn, err)
		})
	}
}

// 回查任务的测试
func (s *TxNotificationServiceTestSuite) TestCheckBackTask() {
	ctrl := gomock.NewController(s.T())
	defer ctrl.Finish()
	configSvc := configmocks.NewMockBusinessConfigService(ctrl)
	notificationSvc := notificationmocks.NewMockNotificationService(ctrl)
	mockConfigMap := s.mockConfigMap()
	mu := &sync.Mutex{}
	notificationMap := map[uint64]string{}
	configSvc.EXPECT().GetByIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, ids []int64) (map[int64]config.BusinessConfig, error) {
		res := make(map[int64]config.BusinessConfig, len(ids))
		for _, id := range ids {
			v, ok := mockConfigMap[id]
			if ok {
				res[id] = v
			}
		}
		return res, nil
	}).AnyTimes()
	// 记录更新的消息
	notificationSvc.EXPECT().BatchUpdateNotificationStatus(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, ids []uint64, status string) error {
			mu.Lock()
			defer mu.Unlock()
			for _, id := range ids {
				notificationMap[id] = status
			}
			return nil
		}).AnyTimes()

	// 测试逻辑比较，简单直接往数据库里插数据模拟各种情况

	now := time.Now().UnixMilli()
	// 事务1的会取出来，并且成功提交
	tx1 := s.MockDaoTxn("tx1", 1, now-(time.Second*11).Milliseconds())
	tx1.Status = domain.TxNotificationStatusPrepare.String()
	tx1.BizID = 1
	// 事务2取不出来，因为已经提交
	tx2 := s.MockDaoTxn("tx2", 2, 0)
	tx2.Status = domain.TxNotificationStatusCommit.String()
	tx2.BizID = 2
	// 事务3取不出来, 因为已经取消
	tx3 := s.MockDaoTxn("tx3", 3, 0)
	tx3.Status = domain.TxNotificationStatusCancel.String()
	tx3.BizID = 3
	// 事务4取不出来, 因为已经失败
	tx4 := s.MockDaoTxn("tx4", 4, 0)
	tx4.Status = domain.TxNotificationStatusFail.String()
	tx4.BizID = 4
	// 事务5取不出来，因为还没有到检查时间
	tx5 := s.MockDaoTxn("tx5", 5, now+(time.Second*30).Milliseconds())
	tx5.Status = domain.TxNotificationStatusPrepare.String()
	tx5.BizID = 5
	// 事务11会取出来，但是他只有最后一次回查机会了，发送会成功
	tx11 := s.MockDaoTxn("tx11", 11, now-(time.Second*11).Milliseconds())
	tx11.Status = domain.TxNotificationStatusPrepare.String()
	tx11.BizID = 1
	tx11.CheckCount = 2
	// 事务22会取出来，但是他只有最后一次回查机会了，发送失败
	tx22 := s.MockDaoTxn("tx22", 22, now-(time.Second*11).Milliseconds())
	tx22.Status = domain.TxNotificationStatusPrepare.String()
	tx22.BizID = 1
	tx22.CheckCount = 2
	// 事务23会取出来，但是他还有好几次次回查机会了，发送失败
	tx23 := s.MockDaoTxn("tx23", 23, now-(time.Second*11).Milliseconds())
	tx23.Status = domain.TxNotificationStatusPrepare.String()
	tx23.BizID = 2
	tx23.CheckCount = 0
	txns := []dao.TxNotification{
		tx1,
		tx2,
		tx3,
		tx4,
		tx5,
		tx11,
		tx22,
		tx23,
	}
	err := s.db.WithContext(context.Background()).Create(txns).Error
	require.NoError(s.T(), err)

	txSvc := startup.InitTxNotificationService(notification.Module{
		Svc: notificationSvc,
	}, config.Module{
		Svc: configSvc,
	})
	// time.Sleep(time.Second * 1000000)

	// 初始化注册中心
	etcdClient := testioc.InitEtcdClient()
	reg := registry.Load("").Build(registry.WithClientEtcd(etcdClient))
	go func() {
		server := testgrpc.NewServer("order.notification.callback.service", reg, &MockGrpcServer{})
		err = server.Start("127.0.0.1:30001")
		if err != nil {
			require.NoError(s.T(), err)
		}
	}()
	// 等待启动完成
	time.Sleep(1 * time.Second)
	resolver.Register("etcd", reg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	nowMill := time.Now().UnixMilli()
	txSvc.StartTask(ctx)
	<-ctx.Done()
	tx1.CheckCount = 1
	tx1.NextCheckTime = 0
	tx1.Status = domain.TxNotificationStatusCommit.String()
	txns[0] = tx1
	tx11.NextCheckTime = 0
	tx11.CheckCount = 3
	tx11.Status = domain.TxNotificationStatusCommit.String()
	txns[5] = tx11
	tx22.NextCheckTime = 0
	tx22.CheckCount = 3
	tx22.Status = domain.TxNotificationStatusFail.String()
	txns[6] = tx22
	tx23.NextCheckTime = nowMill + 30*1000
	tx22.CheckCount = 1
	tx22.Status = domain.TxNotificationStatusPrepare.String()
	txns[7] = tx23
	var notifications []dao.TxNotification
	nctx, ncancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer ncancel()
	err = s.db.WithContext(nctx).Model(&dao.TxNotification{}).Find(&notifications).Error
	require.NoError(s.T(), err)
	txnMap := make(map[string]dao.TxNotification, len(notifications))
	for _, n := range notifications {
		txnMap[n.TxID] = n
	}
	for k, v := range txnMap {
		log.Println("xxxxxxxx tx: ", k, "status: ", v)
	}
	for _, n := range txns {
		txn, ok := txnMap[n.TxID]
		if !ok {
			log.Println("xxxxxxxx", n.TxID)
		}
		require.True(s.T(), ok)
		s.assertTxNotification(txn, n)
	}
	wantNotifications := []notification.Notification{
		{
			ID:     1,
			Status: notification.StatusPending,
		},
		{
			ID:     11,
			Status: notification.StatusPending,
		},
		{
			ID:     22,
			Status: notification.StatusCanceled,
		},
	}
	for _, wantNotification := range wantNotifications {
		actualNotification, ok := notificationMap[wantNotification.ID]
		require.True(s.T(), ok)
		assert.Equal(s.T(), string(wantNotification.Status), actualNotification)
	}
}

func (s *TxNotificationServiceTestSuite) mockConfigMap() map[int64]config.BusinessConfig {
	return map[int64]config.BusinessConfig{
		1: {
			TxnConfig: `{
    "serviceName": "order.notification.callback.service",
    "type": "normal",
    "maxRetryTimes": 3,
    "interval": 30
}`,
		},
		2: {
			TxnConfig: `{
    "serviceName": "order.notification.callback.service",
    "type": "normal",
    "maxRetryTimes": 2,
    "interval": 10
}`,
		},
	}
}

func TestTxNotificationServiceSuite(t *testing.T) {
	suite.Run(t, new(TxNotificationServiceTestSuite))
}

func (s *TxNotificationServiceTestSuite) MockDaoTxn(txid string, nid uint64, nextTime int64) dao.TxNotification {
	now := time.Now().UnixMilli()
	return dao.TxNotification{
		TxID:           txid,
		Key:            fmt.Sprintf("%d", nid),
		NotificationID: nid,
		NextCheckTime:  nextTime,
		Ctime:          now,
		Utime:          now,
	}
}

type MockGrpcServer struct {
	tx_notificationv1.UnimplementedBackCheckServiceServer
}

func (m *MockGrpcServer) BackCheck(ctx context.Context, request *tx_notificationv1.BackCheckRequest) (*tx_notificationv1.BackCheckResponse, error) {
	if strings.Contains(request.GetKey(), "1") {
		return &tx_notificationv1.BackCheckResponse{
			Status: tx_notificationv1.BackCheckResponse_SUCCESS,
		}, nil
	}
	if strings.Contains(request.GetKey(), "2") {
		return nil, errors.New("mock error")
	}

	return &tx_notificationv1.BackCheckResponse{
		Status: tx_notificationv1.BackCheckResponse_FAILURE,
	}, nil
}

func (s *TxNotificationServiceTestSuite) assertTxNotification(wantTxn dao.TxNotification, actualTxn dao.TxNotification) {
	assert.Equal(s.T(), wantTxn.TxID, actualTxn.TxID)
	assert.Equal(s.T(), wantTxn.Key, actualTxn.Key)
	assert.Equal(s.T(), wantTxn.NotificationID, actualTxn.NotificationID)
	assert.Equal(s.T(), wantTxn.BizID, actualTxn.BizID)
	assert.Equal(s.T(), wantTxn.Status, actualTxn.Status)
	if actualTxn.NextCheckTime == 0 {
		assert.Equal(s.T(), wantTxn.NextCheckTime, actualTxn.NextCheckTime)
	} else {
		require.LessOrEqual(s.T(), wantTxn.NextCheckTime, actualTxn.NextCheckTime)
	}
	require.True(s.T(), actualTxn.Ctime > 0)
	require.True(s.T(), actualTxn.Utime > 0)
}
