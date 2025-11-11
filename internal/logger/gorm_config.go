package logger

import (
	"time"
	"gorm.io/gorm/logger"
)

// GORMConfig contains configuration for GORM storage
type GORMConfig struct {
	DBPath          string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	LogLevel        logger.LogLevel
}

// DefaultGORMConfig returns default configuration for GORM storage
func DefaultGORMConfig(dbPath string) *GORMConfig {
	return &GORMConfig{
		DBPath:          dbPath,
		MaxOpenConns:    1,  // SQLite最佳实践：单连接避免锁定
		MaxIdleConns:    1,  // 保持一个空闲连接
		ConnMaxLifetime: time.Hour,
		LogLevel:        logger.Silent, // 保持静默，不输出GORM日志
	}
}