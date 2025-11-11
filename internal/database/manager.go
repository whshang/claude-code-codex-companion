package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// 统一数据库管理器 - 确保Dev模式和生产模式使用相同的数据库路径

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	DataDir    string // 数据目录
	MainDB     string // 主数据库文件名
	LogsDB     string // 日志数据库文件名
	StatisticsDB string // 统计数据库文件名
}

// DefaultDatabaseConfig 返回默认数据库配置
func DefaultDatabaseConfig() *DatabaseConfig {
	return &DatabaseConfig{
		DataDir:       ".cccc-data",
        MainDB:        "main.db",
		LogsDB:        "logs.db",
		StatisticsDB:  "statistics.db",
	}
}

// Manager 统一数据库管理器
type Manager struct {
	config         *DatabaseConfig
	dataDir        string // 绝对路径
	mainDBPath     string
	logsDBPath     string
	statisticsDBPath string

	// 数据库连接
	mainDB       *sql.DB
	logsDB       *sql.DB
	statisticsDB *sql.DB

	mutex sync.RWMutex
}

// NewManager 创建新的数据库管理器
func NewManager(config *DatabaseConfig) (*Manager, error) {
	if config == nil {
		config = DefaultDatabaseConfig()
	}

	// 确定数据目录的绝对路径
	var dataDir string

	if filepath.IsAbs(config.DataDir) {
		dataDir = config.DataDir
	} else {
		// 如果是相对路径，基于项目根目录
		if isWailsEnvironment() {
			// Wails环境：使用用户主目录
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get user home directory: %w", err)
			}
			dataDir = filepath.Join(homeDir, ".cccc-proxy")
		} else {
			// 开发环境：使用当前工作目录的父目录
			cwd, err := os.Getwd()
			if err != nil {
				return nil, fmt.Errorf("failed to get current working directory: %w", err)
			}

			// 如果在wails-app目录下，则使用项目根目录
			if filepath.Base(cwd) == "wails-app" || filepath.Base(filepath.Dir(cwd)) == "wails-app" {
				dataDir = filepath.Join(filepath.Dir(cwd), ".cccc-data")
			} else {
				dataDir = filepath.Join(cwd, config.DataDir)
			}
		}
	}

	// 创建数据目录
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

    manager := &Manager{
        config:           config,
        dataDir:          dataDir,
        mainDBPath:       filepath.Join(dataDir, config.MainDB),
        // 统一存储位置：所有数据库均放在 dataDir 下
        logsDBPath:       filepath.Join(dataDir, config.LogsDB),
        statisticsDBPath: filepath.Join(dataDir, config.StatisticsDB),
    }

    // 确保存储目录存在（与 dataDir 一致）
    logsDir := filepath.Dir(manager.logsDBPath)
    if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

    // 清理旧版主数据库文件（按需）：不做迁移，直接删除避免命名耦合
    // 仅限于当前 dataDir 下的历史文件，避免误删其他位置
    legacyFiles := []string{
        filepath.Join(dataDir, "cccc-proxy.db"),
        filepath.Join(dataDir, "cccc-proxy.db-shm"),
        filepath.Join(dataDir, "cccc-proxy.db-wal"),
    }
    for _, f := range legacyFiles {
        if _, err := os.Stat(f); err == nil {
            _ = os.Remove(f)
        }
    }

	return manager, nil
}

// isWailsEnvironment 检测是否在Wails环境中运行
func isWailsEnvironment() bool {
	// 检查是否存在Wails相关的环境变量或路径
	// Wails应用通常在特定的路径结构中运行
	execPath, _ := os.Executable()
	return filepath.Ext(execPath) == ".app" ||
		   filepath.Base(filepath.Dir(execPath)) == "MacOS" ||
		   os.Getenv("WAILS_ENVIRONMENT") != ""
}

// GetDataDir 获取数据目录路径
func (m *Manager) GetDataDir() string {
	return m.dataDir
}

// GetMainDBPath 获取主数据库路径
func (m *Manager) GetMainDBPath() string {
	return m.mainDBPath
}

// GetLogsDBPath 获取日志数据库路径
func (m *Manager) GetLogsDBPath() string {
	return m.logsDBPath
}

// GetStatisticsDBPath 获取统计数据库路径
func (m *Manager) GetStatisticsDBPath() string {
	return m.statisticsDBPath
}

// GetMainDB 获取主数据库连接
func (m *Manager) GetMainDB() (*sql.DB, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.mainDB != nil {
		return m.mainDB, nil
	}

	db, err := sql.Open("sqlite", m.mainDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open main database: %w", err)
	}

	// 优化设置
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA cache_size=10000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set cache size: %w", err)
	}

	m.mainDB = db
	return db, nil
}

// GetLogsDB 获取日志数据库连接
func (m *Manager) GetLogsDB() (*sql.DB, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.logsDB != nil {
		return m.logsDB, nil
	}

	db, err := sql.Open("sqlite", m.logsDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open logs database: %w", err)
	}

	// 优化设置
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA cache_size=10000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set cache size: %w", err)
	}

	m.logsDB = db
	return db, nil
}

// GetStatisticsDB 获取统计数据库连接
func (m *Manager) GetStatisticsDB() (*sql.DB, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.statisticsDB != nil {
		return m.statisticsDB, nil
	}

	db, err := sql.Open("sqlite", m.statisticsDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open statistics database: %w", err)
	}

	// 优化设置
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set WAL mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA synchronous=NORMAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set synchronous mode: %w", err)
	}

	if _, err := db.Exec("PRAGMA cache_size=10000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to set cache size: %w", err)
	}

	m.statisticsDB = db
	return db, nil
}

// Close 关闭所有数据库连接
func (m *Manager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	var errors []error

	if m.mainDB != nil {
		if err := m.mainDB.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close main database: %w", err))
		}
		m.mainDB = nil
	}

	if m.logsDB != nil {
		if err := m.logsDB.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close logs database: %w", err))
		}
		m.logsDB = nil
	}

	if m.statisticsDB != nil {
		if err := m.statisticsDB.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close statistics database: %w", err))
		}
		m.statisticsDB = nil
	}

	if len(errors) > 0 {
		return fmt.Errorf("multiple errors occurred: %v", errors)
	}

	return nil
}

// GetInfo 获取数据库管理器信息
func (m *Manager) GetInfo() map[string]interface{} {
	return map[string]interface{}{
		"data_dir":           m.dataDir,
		"main_db_path":       m.mainDBPath,
		"logs_db_path":       m.logsDBPath,
		"statistics_db_path": m.statisticsDBPath,
		"environment": func() string {
			if isWailsEnvironment() {
				return "wails"
			}
			return "development"
		}(),
	}
}

// 全局数据库管理器实例
var (
	globalManager *Manager
	managerMutex  sync.Mutex
)

// GetGlobalManager 获取全局数据库管理器
func GetGlobalManager() (*Manager, error) {
	managerMutex.Lock()
	defer managerMutex.Unlock()

	if globalManager != nil {
		return globalManager, nil
	}

	var err error
	globalManager, err = NewManager(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create global database manager: %w", err)
	}

	return globalManager, nil
}

// InitializeGlobalManager 初始化全局数据库管理器
func InitializeGlobalManager(config *DatabaseConfig) error {
	managerMutex.Lock()
	defer managerMutex.Unlock()

	if globalManager != nil {
		// 关闭现有管理器
		globalManager.Close()
	}

	var err error
	globalManager, err = NewManager(config)
	if err != nil {
		return fmt.Errorf("failed to initialize global database manager: %w", err)
	}

	return nil
}

// CleanupGlobalManager 清理全局数据库管理器
func CleanupGlobalManager() error {
	managerMutex.Lock()
	defer managerMutex.Unlock()

	if globalManager != nil {
		err := globalManager.Close()
		globalManager = nil
		return err
	}

	return nil
}