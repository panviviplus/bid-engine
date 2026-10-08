package storage

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	driver "github.com/go-sql-driver/mysql"

	"bid-engine/lib/common/config"
	"bid-engine/lib/encrypt"
)

// MustInitDB 初始化DB
func MustInitDB() {
	systemConf := config.GetConfig()
	if systemConf.Mysql.DSN == "" {
		panic(fmt.Errorf("init db error : mysql.dsn is empty"))
	}
	db = ConnectDB(systemConf.Mysql.DSN)
	db = db.Debug()
}

// GetDB 获取数据库句柄
func GetDB() *gorm.DB {
	if db == nil {
		panic("DB没有初始化")
	}
	return db
}

// ConnectDB 链接数据库
func ConnectDB(dsn string) (db *gorm.DB) {
	// 解析dsn
	cfg, err := driver.ParseDSN(dsn)
	if err != nil {
		panic("dsn invalid")
	}
	//user, err := encrypt.SM4DecryptBase64(cfg.User)
	//if err == nil && user != "" {
	//	cfg.User = user
	//}
	password, err := encrypt.SM4DecryptBase64(cfg.Passwd)
	if err == nil && password != "" {
		cfg.Passwd = password
	}
	dsn = cfg.FormatDSN()
	if strings.HasSuffix(dsn, "sqlite.db") {
		db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	} else {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
	}
	if err != nil {
		panic(fmt.Errorf("connect db fail: %w", err))
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic(fmt.Errorf("connect db fail: %w", err))
	}
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	return db
}
