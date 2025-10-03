package logger

import (
	"fmt"
	"time"
	"path/filepath"
	"os"
	"strings"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
	
	appconfig "claude-code-codex-companion/internal/config"
)

// GORMStorage 基于GORM的日志存储实现
type GORMStorage struct {
	db             *gorm.DB
	config         *GORMConfig
	cleanupTicker  *time.Ticker
	stopCleanup    chan struct{}
}

// NewGORMStorage 创建一个新的基于GORM的日志存储
func NewGORMStorage(logDir string) (*GORMStorage, error) {
	// 创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}
	
	dbPath := filepath.Join(logDir, "logs.db")
	config := DefaultGORMConfig(dbPath)
	
	// 使用modernc.org/sqlite驱动，添加WAL模式和超时设置
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        dbPath + "?_journal_mode=WAL&_timeout=5000&_busy_timeout=5000",
	}, &gorm.Config{
		Logger: logger.Default.LogMode(config.LogLevel),
		// 禁用外键约束检查（保持与现有数据库一致）
		DisableForeignKeyConstraintWhenMigrating: true,
		// 设置时间函数
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %v", err)
	}
	
	// 配置连接池（modernc.org/sqlite 特定设置）
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	
	sqlDB.SetMaxOpenConns(config.MaxOpenConns)
	sqlDB.SetMaxIdleConns(config.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(config.ConnMaxLifetime)
	
	// 设置SQLite优化参数以减少锁定
	optimizationPragmas := []string{
		"PRAGMA synchronous = NORMAL",     // 平衡性能与安全
		fmt.Sprintf("PRAGMA cache_size = %d", appconfig.Default.Database.CacheSize), // 使用统一默认值
		"PRAGMA temp_store = memory",      // 临时数据使用内存
		fmt.Sprintf("PRAGMA mmap_size = %d", appconfig.Default.Database.MmapSize),   // 使用统一默认值
		fmt.Sprintf("PRAGMA busy_timeout = %d", appconfig.Default.Database.BusyTimeout), // 使用统一默认值
	}
	
	for _, pragma := range optimizationPragmas {
		if err := db.Exec(pragma).Error; err != nil {
			fmt.Printf("Warning: Failed to set pragma %s: %v\n", pragma, err)
		}
	}
	
	storage := &GORMStorage{
		db:          db,
		config:      config,
		stopCleanup: make(chan struct{}),
	}
	
	// 验证表结构兼容性
	if err := validateTableCompatibility(db); err != nil {
		// 如果表不存在，执行自动迁移
		if err := db.AutoMigrate(&GormRequestLog{}); err != nil {
			return nil, fmt.Errorf("failed to migrate database: %v", err)
		}
	}
	
	// 创建优化索引
	if err := createOptimizedIndexes(db); err != nil {
		return nil, fmt.Errorf("failed to create optimized indexes: %v", err)
	}
	
	// 启动后台清理程序
	storage.startBackgroundCleanup()
	
	return storage, nil
}

// SaveLog 保存日志条目到数据库
// 保持与现有实现相同的错误处理策略：静默失败，不阻塞主流程
func (g *GORMStorage) SaveLog(log *RequestLog) {
	gormLog := ConvertToGormRequestLog(log)
	
	// 添加重试机制处理SQLite BUSY错误
	maxRetries := appconfig.Default.Database.MaxRetries
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := g.db.Create(gormLog).Error
		if err == nil {
			return // 成功保存
		}
		
		// 检查是否是SQLite忙碌错误
		if strings.Contains(err.Error(), "database is locked") || 
		   strings.Contains(err.Error(), "SQLITE_BUSY") {
			if attempt < maxRetries-1 {
				// 等待一小段时间后重试
				time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
				continue
			}
		}
		
		// 与现有实现保持一致：只打印错误，不返回
		fmt.Printf("Failed to save log to database: %v\n", err)
		return
	}
}

// GetLogs 获取日志列表，支持分页和过滤
func (g *GORMStorage) GetLogs(limit, offset int, failedOnly bool) ([]*RequestLog, int, error) {
	var gormLogs []GormRequestLog
	var total int64
	
	query := g.db.Model(&GormRequestLog{})
	
	// 应用过滤条件（与现有逻辑保持一致）
	if failedOnly {
		query = query.Where("status_code >= ? OR error != ?", 400, "")
	}
	
	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %v", err)
	}
	
	// 获取分页数据
	err := query.Order("timestamp DESC").
		Limit(limit).
		Offset(offset).
		Find(&gormLogs).Error
	
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query logs: %v", err)
	}
	
	// 转换为现有的RequestLog格式
	logs := make([]*RequestLog, len(gormLogs))
	for i, gormLog := range gormLogs {
		logs[i] = ConvertFromGormRequestLog(&gormLog)
	}
	
	return logs, int(total), nil
}

// GetAllLogsByRequestID 获取指定request_id的所有日志条目
func (g *GORMStorage) GetAllLogsByRequestID(requestID string) ([]*RequestLog, error) {
	var gormLogs []GormRequestLog
	
	err := g.db.Where("request_id = ?", requestID).
		Order("timestamp ASC").
		Find(&gormLogs).Error
	
	if err != nil {
		return nil, fmt.Errorf("failed to query logs by request ID: %v", err)
	}
	
	// 转换为现有的RequestLog格式
	logs := make([]*RequestLog, len(gormLogs))
	for i, gormLog := range gormLogs {
		logs[i] = ConvertFromGormRequestLog(&gormLog)
	}
	
	return logs, nil
}

// CleanupLogsByDays 清理指定天数之前的日志
func (g *GORMStorage) CleanupLogsByDays(days int) (int64, error) {
	var result *gorm.DB
	
	if days > 0 {
		cutoffTime := time.Now().AddDate(0, 0, -days)
		result = g.db.Where("timestamp < ?", cutoffTime).Delete(&GormRequestLog{})
	} else {
		// 删除所有记录，使用 1=1 作为条件
		result = g.db.Where("1 = 1").Delete(&GormRequestLog{})
	}
	
	if result.Error != nil {
		return 0, fmt.Errorf("failed to cleanup logs: %v", result.Error)
	}
	
	// VACUUM 操作（保持与现有实现一致）
	if result.RowsAffected > 0 {
		if err := g.db.Exec("VACUUM").Error; err != nil {
			fmt.Printf("Failed to vacuum database: %v\n", err)
		}
	}
	
	return result.RowsAffected, nil
}

// Close 关闭数据库连接和清理程序
func (g *GORMStorage) Close() error {
	// 停止后台清理程序
	if g.cleanupTicker != nil {
		g.cleanupTicker.Stop()
	}
	
	select {
	case g.stopCleanup <- struct{}{}:
	default:
	}
	
	// 关闭数据库连接
	sqlDB, err := g.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// startBackgroundCleanup 启动后台清理程序（保持与现有实现一致）
func (g *GORMStorage) startBackgroundCleanup() {
	g.cleanupTicker = time.NewTicker(24 * time.Hour)
	
	go func() {
		for {
			select {
			case <-g.cleanupTicker.C:
				// 清理30天前的日志
				deleted, err := g.CleanupLogsByDays(30)
				if err != nil {
					fmt.Printf("Background cleanup error: %v\n", err)
				} else if deleted > 0 {
					fmt.Printf("Background cleanup: deleted %d old log entries\n", deleted)
				}
			case <-g.stopCleanup:
				return
			}
		}
	}()
}

// GetStats 获取统计信息
func (g *GORMStorage) GetStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	// 总日志数
	var totalLogs int64
	g.db.Model(&GormRequestLog{}).Count(&totalLogs)
	stats["total_logs"] = totalLogs
	
	// 失败日志数
	var failedLogs int64
	g.db.Model(&GormRequestLog{}).Where("status_code >= ? OR error != ?", 400, "").Count(&failedLogs)
	stats["failed_logs"] = failedLogs
	
	// 最早日志时间
	var oldestLog GormRequestLog
	if err := g.db.Order("timestamp ASC").First(&oldestLog).Error; err == nil {
		stats["oldest_log"] = oldestLog.Timestamp
	}
	
	// 数据库大小
	var pageCount, pageSize int
	g.db.Raw("PRAGMA page_count").Scan(&pageCount)
	g.db.Raw("PRAGMA page_size").Scan(&pageSize)
	stats["db_size_bytes"] = pageCount * pageSize
	
	return stats, nil
}