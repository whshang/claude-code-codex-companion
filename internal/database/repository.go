package database

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB 全局数据库实例
var DB *gorm.DB

// InitDB 初始化数据库连接
func InitDB(dbPath string) error {
	var err error
	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移表结构
	return DB.AutoMigrate(&Channel{})
}

// GetChannelRepo 获取通道仓库
func GetChannelRepo() *ChannelRepository {
	return NewChannelRepository(DB)
}
