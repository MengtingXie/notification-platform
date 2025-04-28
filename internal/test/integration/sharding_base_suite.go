package integration

import (
	"gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ecodeclub/ekit/syncx"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

// 提供一些 通用的方法
type BaseShardingSuite struct {
	suite.Suite
	dbs *syncx.Map[string, *gorm.DB]
}

// 比较消息的通用方法
func (s *BaseShardingSuite) assertNotifications(wantNotifications, actualNotifications map[[2]string][]dao.Notification) {
	require.Equal(s.T(), len(wantNotifications), len(actualNotifications))
	for key, wantVal := range wantNotifications {
		actualVal, ok := actualNotifications[key]
		require.True(s.T(), ok)
		require.Equal(s.T(), len(wantVal), len(actualVal))
		for idx := range actualVal {
			wantNotification := wantVal[idx]
			actualNotification := actualVal[idx]
			require.True(s.T(), actualNotification.Ctime > 0)
			require.True(s.T(), actualNotification.Utime > 0)
			actualNotification.Ctime = 0
			actualNotification.Utime = 0
			require.Equal(s.T(), wantNotification, actualNotification)
		}
	}
}


func (s *BaseShardingSuite) assertCallbackLogs(wantLogs, actualLogs map[[2]string][]dao.CallbackLog) {
	require.Equal(s.T(), len(wantLogs), len(actualLogs))
	for key, wantVal := range wantLogs {
		actualVal, ok := actualLogs[key]
		require.True(s.T(), ok)
		require.Equal(s.T(), len(wantVal), len(actualVal))
		for idx := range actualVal {
			wantLog := wantVal[idx]
			actualLog := actualVal[idx]
			require.True(s.T(), actualLog.Ctime > 0)
			require.True(s.T(), actualLog.Utime > 0)
			require.True(s.T(), actualLog.NextRetryTime > 0)
			actualLog.Ctime = 0
			actualLog.Utime = 0
			actualLog.NextRetryTime = 0
			actualLog.ID = 0
			require.Equal(s.T(), wantLog, actualLog)

		}
	}
}

func (s *BaseShardingSuite) assertTxNotifications(wantTxNotifications, actualTxNotifications map[[2]string][]dao.TxNotification) {
	require.Equal(s.T(), len(wantTxNotifications), len(actualTxNotifications))
	for key, wantVal := range wantTxNotifications {
		actualVal, ok := actualTxNotifications[key]
		require.True(s.T(), ok)
		require.Equal(s.T(), len(wantVal), len(actualVal))
		for idx := range actualVal {
			wantNotification := wantVal[idx]
			actualNotification := actualVal[idx]
			require.True(s.T(), actualNotification.Ctime > 0)
			require.True(s.T(), actualNotification.Utime > 0)
			actualNotification.TxID = 0
			actualNotification.Ctime = 0
			actualNotification.Utime = 0
			require.Equal(s.T(), wantNotification, actualNotification)
		}
	}
}