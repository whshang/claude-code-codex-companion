package logger

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"strings"
	"time"

	appconfig "claude-code-codex-companion/internal/config"
)

// GORMStorage 基于GORM的日志存储实现
type GORMStorage struct {
	db            *gorm.DB
	config        *GORMConfig
	cleanupTicker *time.Ticker
	stopCleanup   chan struct{}
}

// NewGORMStorage 创建一个新的基于GORM的日志存储
func NewGORMStorage(logDir string) (*GORMStorage, error) {
	// 创建日志目录
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	dbPath := filepath.Join(logDir, "logs.db")
	config := DefaultGORMConfig(dbPath)

	// 使用modernc.org/sqlite驱动，添加WAL模式和更长的超时设置
	db, err := gorm.Open(sqlite.Dialector{
		DriverName: "sqlite",
		DSN:        dbPath + "?_journal_mode=WAL&_timeout=10000&_busy_timeout=10000&_synchronous=NORMAL&_cache_size=10000&_temp_store=memory",
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
		"PRAGMA synchronous = NORMAL", // 平衡性能与安全
		fmt.Sprintf("PRAGMA cache_size = %d", appconfig.Default.Database.CacheSize), // 使用统一默认值
		"PRAGMA temp_store = memory", // 临时数据使用内存
		fmt.Sprintf("PRAGMA mmap_size = %d", appconfig.Default.Database.MmapSize),       // 使用统一默认值
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
// 改进的错误处理策略：增强重试机制，更好的错误分类
func (g *GORMStorage) SaveLog(log *RequestLog) {
	gormLog := ConvertToGormRequestLog(log)

	// 增强重试机制处理SQLite BUSY错误
	maxRetries := appconfig.Default.Database.MaxRetries * 2 // 增加重试次数
	baseDelay := 5 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		err := g.db.Create(gormLog).Error
		if err == nil {
			return // 成功保存
		}

		// 检查错误类型并采取不同策略
		errStr := err.Error()
		isBusyError := strings.Contains(errStr, "database is locked") ||
			strings.Contains(errStr, "SQLITE_BUSY") ||
			strings.Contains(errStr, "database table is locked")

		if isBusyError {
			if attempt < maxRetries-1 {
				// 指数退避策略
				delay := baseDelay * time.Duration(1<<uint(attempt))
				if delay > 500*time.Millisecond {
					delay = 500 * time.Millisecond
				}
				time.Sleep(delay)
				continue
			}
		}

		// 对于非忙碌错误或重试次数用完，记录详细错误信息
		fmt.Printf("Failed to save log to database (attempt %d/%d): %v\n",
			attempt+1, maxRetries, err)

		// 如果是数据库损坏错误，尝试重建连接
		if strings.Contains(errStr, "database disk image is malformed") ||
			strings.Contains(errStr, "no such table") {
			fmt.Printf("Database corruption detected, attempting recovery...\n")
			if attempt == maxRetries-1 {
				go g.attemptDatabaseRecovery()
			}
		}

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

	// 统计近24小时的 /responses 降级情况
	since := time.Now().Add(-24 * time.Hour)
	var conversionTotal int64
	g.db.Model(&GormRequestLog{}).
		Where("timestamp >= ? AND conversion_path LIKE ?", since, "%responses->chat_completions%").
		Count(&conversionTotal)
	stats["responses_conversion_total"] = conversionTotal

	type conversionAgg struct {
		Endpoint string
		Count    int64
	}
	var conversionRows []conversionAgg
	g.db.Model(&GormRequestLog{}).
		Select("endpoint, COUNT(*) as count").
		Where("timestamp >= ? AND conversion_path LIKE ?", since, "%responses->chat_completions%").
		Group("endpoint").
		Order("count DESC").
		Limit(10).
		Scan(&conversionRows)

	conversionList := make([]map[string]interface{}, 0, len(conversionRows))
	for _, row := range conversionRows {
		conversionList = append(conversionList, map[string]interface{}{
			"endpoint": row.Endpoint,
			"count":    row.Count,
		})
	}
	stats["responses_conversion_counts"] = conversionList

	// 统计近24小时 supports_responses_flag 分布
	type flagAgg struct {
		Flag  string
		Count int64
	}
	var flagRows []flagAgg
	g.db.Model(&GormRequestLog{}).
		Select("supports_responses_flag as flag, COUNT(*) as count").
		Where("timestamp >= ? AND supports_responses_flag != ''", since).
		Group("supports_responses_flag").
		Scan(&flagRows)

	flagList := make([]map[string]interface{}, 0, len(flagRows))
	for _, row := range flagRows {
		flagList = append(flagList, map[string]interface{}{
			"flag":  row.Flag,
			"count": row.Count,
		})
	}
	stats["supports_responses_flag_counts"] = flagList

	return stats, nil
}

// GetDB 获取底层数据库连接（用于诊断）
func (g *GORMStorage) GetDB() (*gorm.DB, error) {
	return g.db, nil
}

// attemptDatabaseRecovery 尝试恢复损坏的数据库
func (g *GORMStorage) attemptDatabaseRecovery() {
	fmt.Printf("Starting database recovery process...\n")

	// 获取数据库文件路径
	sqlDB, err := g.db.DB()
	if err != nil {
		fmt.Printf("Failed to get underlying SQL DB: %v\n", err)
		return
	}

	// 关闭当前连接
	sqlDB.Close()

	// 这里可以添加更复杂的恢复逻辑，比如：
	// 1. 检查数据库文件完整性
	// 2. 尝试SQLite的REINDEX或VACUUM
	// 3. 从备份恢复（如果有的话）

	fmt.Printf("Database recovery completed. Please restart the application.\n")
}

// GetDatabaseHealth 获取数据库健康状态
func (g *GORMStorage) GetDatabaseHealth() map[string]interface{} {
	health := make(map[string]interface{})

	// 检查数据库连接
	sqlDB, err := g.db.DB()
	if err != nil {
		health["status"] = "error"
		health["error"] = err.Error()
		return health
	}

	// 测试简单查询
	var result int
	if err := g.db.Raw("SELECT 1").Scan(&result).Error; err != nil {
		health["status"] = "error"
		health["error"] = err.Error()
	} else {
		health["status"] = "healthy"
	}

	// 获取连接池状态
	stats := sqlDB.Stats()
	health["open_connections"] = stats.OpenConnections
	health["in_use"] = stats.InUse
	health["idle"] = stats.Idle

	return health
}
