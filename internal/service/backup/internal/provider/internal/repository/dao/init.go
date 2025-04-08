package dao

import (
	dao2 "gitee.com/flycash/notification-platform/internal/repository/dao"
	"github.com/ego-component/egorm"
)

func InitTables(db *egorm.Component) error {
	return db.AutoMigrate(&dao2.Provider{})
}
