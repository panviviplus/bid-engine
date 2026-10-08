package storage

import (
	"sync"

	"gorm.io/gorm"
)

var (
	db   *gorm.DB            // 默认数据库句柄
	dbs  map[string]*gorm.DB // key:config->db->name
	lock sync.RWMutex
)

func init() {
	dbs = make(map[string]*gorm.DB, 0)
}
