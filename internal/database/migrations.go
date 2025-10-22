package database

import (
"database/sql"
"encoding/json"
"fmt"
"log"
"strings"

	_ "github.com/mattn/go-sqlite3"
"gorm.io/driver/sqlite"
"gorm.io/gorm"
)

// MigrateFromPython 从 Python 数据库迁移到 Go GORM
// 功能：
// 1. 读取原 Python SQLite 数据库 (data/channels.db)
// 2. 迁移 channels 表数据到新的 GORM 模型
// 3. 忽略 settings 表（管理员认证已移除）
// 4. 解密加密的 API Key（如果启用了加密）
func MigrateFromPython(oldDBPath, newDBPath, encryptionKey string) error {
	// 打开旧数据库
	oldDB, err := sql.Open("sqlite3", oldDBPath)
	if err != nil {
		return fmt.Errorf("failed to open old database: %v", err)
	}
	defer oldDB.Close()

	// 检查旧数据库表结构
	if err := checkOldDatabaseSchema(oldDB); err != nil {
		return fmt.Errorf("old database schema check failed: %v", err)
	}

	// 初始化新数据库
	newDB, err := gorm.Open(sqlite.Open(newDBPath), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to open new database: %v", err)
	}

	// 自动迁移新数据库结构
	if err := newDB.AutoMigrate(&Channel{}); err != nil {
		return fmt.Errorf("failed to migrate new database: %v", err)
	}

	// 读取并迁移数据
	return migrateChannels(oldDB, newDB, encryptionKey)
}

// checkOldDatabaseSchema 检查旧数据库表结构
func checkOldDatabaseSchema(db *sql.DB) error {
	// 检查 channels 表是否存在
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='channels'").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check channels table: %v", err)
	}
	if count == 0 {
		return fmt.Errorf("channels table not found in old database")
	}

	// 检查必要的列
	requiredColumns := []string{"id", "name", "api_key", "base_url", "model", "provider", "enabled", "created_at", "updated_at"}
	for _, col := range requiredColumns {
		err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('channels') WHERE name=?", col).Scan(&count)
		if err != nil {
			return fmt.Errorf("failed to check column %s: %v", col, err)
		}
		if count == 0 {
			return fmt.Errorf("required column %s not found in channels table", col)
		}
	}

	return nil
}

// migrateChannels 迁移 channels 表数据
func migrateChannels(oldDB *sql.DB, newDB *gorm.DB, encryptionKey string) error {
	// 查询旧数据
	rows, err := oldDB.Query(`
		SELECT id, name, api_key, base_url, model, provider, enabled, created_at, updated_at
		FROM channels
		WHERE enabled = 1
		ORDER BY id
	`)
	if err != nil {
		return fmt.Errorf("failed to query old channels: %v", err)
	}
	defer rows.Close()

	migratedCount := 0
	for rows.Next() {
		var oldChannel struct {
			ID        int
			Name      string
			APIKey    string
			BaseURL   string
			Model     string
			Provider  string
			Enabled   bool
			CreatedAt string
			UpdatedAt string
		}

		if err := rows.Scan(&oldChannel.ID, &oldChannel.Name, &oldChannel.APIKey,
			&oldChannel.BaseURL, &oldChannel.Model, &oldChannel.Provider,
			&oldChannel.Enabled, &oldChannel.CreatedAt, &oldChannel.UpdatedAt); err != nil {
			log.Printf("Warning: failed to scan channel row: %v", err)
			continue
		}

		// 转换数据格式
		newChannel := Channel{
			Name:     oldChannel.Name,
			APIKey:   decryptAPIKey(oldChannel.APIKey, encryptionKey), // 如果需要解密
			BaseURL:  oldChannel.BaseURL,
			Model:    oldChannel.Model,
			Provider: normalizeProvider(oldChannel.Provider),
			Enabled:  oldChannel.Enabled,
		}

		// 设置默认的智能路由配置
		configureSmartRouting(&newChannel)

		// 插入新数据库
		if err := newDB.Create(&newChannel).Error; err != nil {
			log.Printf("Warning: failed to create channel %s: %v", newChannel.Name, err)
			continue
		}

		migratedCount++
		log.Printf("Migrated channel: %s (%s)", newChannel.Name, newChannel.Provider)
	}

	log.Printf("Migration completed: %d channels migrated", migratedCount)
	return nil
}

// decryptAPIKey 简化版：直接返回API Key（不处理加密）
func decryptAPIKey(encryptedKey, encryptionKey string) string {
	// 按"无加密"要求，简化迁移逻辑
	// 直接返回原始API Key，不进行任何解密处理
return encryptedKey
}

// normalizeProvider 标准化 provider 字段
func normalizeProvider(provider string) string {
	provider = strings.ToLower(strings.TrimSpace(provider))
	switch provider {
	case "openai":
		return "openai"
	case "anthropic", "claude":
		return "anthropic"
	case "google", "gemini":
		return "gemini"
	default:
		return provider
	}
}

// configureSmartRouting 根据 provider 设置智能路由配置
func configureSmartRouting(channel *Channel) {
	switch channel.Provider {
	case "anthropic":
		channel.ClientType = "claude_code"
		channel.NativeFormat = true
		channel.TargetFormat = "anthropic"
	case "openai":
		channel.ClientType = "codex"
		channel.NativeFormat = false
		channel.TargetFormat = "openai_chat"
	case "gemini":
		channel.ClientType = "gemini"
		channel.NativeFormat = false
		channel.TargetFormat = "gemini"
	default:
		channel.ClientType = "universal"
		channel.NativeFormat = false
		channel.TargetFormat = "openai_chat"
	}

	// 设置默认优先级
	channel.Priority = 100
}

// GetChannelRepository 返回通道仓库实例
type ChannelRepository struct {
	db *gorm.DB
}

func NewChannelRepository(db *gorm.DB) *ChannelRepository {
	return &ChannelRepository{db: db}
}

// FindAll 获取所有启用的通道
func (r *ChannelRepository) FindAll() ([]Channel, error) {
	var channels []Channel
	err := r.db.Where("enabled = ?", true).Find(&channels).Error
	return channels, err
}

// FindByProvider 根据提供商查找通道
func (r *ChannelRepository) FindByProvider(provider string) ([]Channel, error) {
	var channels []Channel
	err := r.db.Where("provider = ? AND enabled = ?", provider, true).Find(&channels).Error
	return channels, err
}

// Save 保存或更新通道
func (r *ChannelRepository) Save(channel *Channel) error {
	return r.db.Save(channel).Error
}

// Delete 删除通道（软删除）
func (r *ChannelRepository) Delete(id uint) error {
	return r.db.Delete(&Channel{}, id).Error
}

// GetModelMappingRules 解析模型映射配置
func (c *Channel) GetModelMappingRules() ([]ModelMappingRule, error) {
	if c.ModelMapping == "" {
		return nil, nil
	}

	var rules []ModelMappingRule
	err := json.Unmarshal([]byte(c.ModelMapping), &rules)
	return rules, err
}

// SetModelMappingRules 设置模型映射配置
func (c *Channel) SetModelMappingRules(rules []ModelMappingRule) error {
	data, err := json.Marshal(rules)
	if err != nil {
		return err
	}
	c.ModelMapping = string(data)
	return nil
}

// GetThinkingBudgetMapping 解析思考预算映射配置
func (c *Channel) GetThinkingBudgetMapping() (*ThinkingBudgetMapping, error) {
	if c.ThinkingBudgetConfig == "" {
		return &ThinkingBudgetMapping{
			OpenAILowToAnthropicTokens:    1024,
			OpenAIMediumToAnthropicTokens: 2048,
			OpenAIHighToAnthropicTokens:   4096,
		}, nil
	}

	var mapping ThinkingBudgetMapping
	err := json.Unmarshal([]byte(c.ThinkingBudgetConfig), &mapping)
	return &mapping, err
}

// SetThinkingBudgetMapping 设置思考预算映射配置
func (c *Channel) SetThinkingBudgetMapping(mapping *ThinkingBudgetMapping) error {
	data, err := json.Marshal(mapping)
	if err != nil {
		return err
	}
	c.ThinkingBudgetConfig = string(data)
	return nil
}
