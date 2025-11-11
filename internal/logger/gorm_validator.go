package logger

import (
	"database/sql"
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

// ValidateGORMCompatibility 验证GORM与modernc.org/sqlite的兼容性
func ValidateGORMCompatibility() error {
	// 首先使用标准sql包测试modernc.org/sqlite连接
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Errorf("modernc.org/sqlite connection failed: %v", err)
	}
	defer sqlDB.Close()
	
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("modernc.org/sqlite ping failed: %v", err)
	}
	
	// 使用已有连接创建GORM实例
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        ":memory:",
	}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		return fmt.Errorf("GORM connection failed: %v", err)
	}
	
	// 验证底层驱动
	gormSqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying database: %v", err)
	}
	defer gormSqlDB.Close()
	
	// 测试基础SQL操作
	if err := gormSqlDB.Ping(); err != nil {
		return fmt.Errorf("database connection test failed: %v", err)
	}
	
	// 测试自动迁移
	if err := db.AutoMigrate(&GormRequestLog{}); err != nil {
		return fmt.Errorf("auto migration failed: %v", err)
	}
	
	// 测试基础CRUD操作
	testLog := &GormRequestLog{
		RequestID: "test-compatibility-123",
		Endpoint:  "test-endpoint",
		Method:    "GET",
		Path:      "/test",
	}
	
	// 测试创建
	if err := db.Create(testLog).Error; err != nil {
		return fmt.Errorf("failed to create test record: %v", err)
	}
	
	// 测试查询
	var foundLog GormRequestLog
	if err := db.Where("request_id = ?", "test-compatibility-123").First(&foundLog).Error; err != nil {
		return fmt.Errorf("failed to query test record: %v", err)
	}
	
	// 测试删除
	if err := db.Delete(&foundLog).Error; err != nil {
		return fmt.Errorf("failed to delete test record: %v", err)
	}
	
	fmt.Println("✅ GORM compatibility with modernc.org/sqlite validation passed")
	return nil
}