package database

import (
	"time"

	"gorm.io/gorm"
)

// Channel 渠道配置模型（融合原 Python 的 channels 表）
type Channel struct {
	ID          uint           `gorm:"primarykey"`
	Name        string         `gorm:"type:varchar(255);not null;index"`
	APIKey      string         `gorm:"type:text;not null"` // 加密存储
	BaseURL     string         `gorm:"type:varchar(500);not null"`
	Model       string         `gorm:"type:varchar(255)"`
	Provider    string         `gorm:"type:varchar(50);not null;index"` // openai/anthropic/gemini

	// 新增字段以支持 CCCC 的智能路由
	ClientType   string        `gorm:"type:varchar(50)"` // claude_code/codex/openai/universal
	NativeFormat bool          `gorm:"default:false"`
	TargetFormat string        `gorm:"type:varchar(50)"` // anthropic/openai_chat/openai_responses/gemini

	// 高级配置
	Priority     int           `gorm:"default:100"`
	Enabled      bool          `gorm:"default:true;index"`
	ProxyURL     string        `gorm:"type:varchar(500)"` // 代理支持

	// 模型映射配置 (JSON 存储)
	ModelMapping  string       `gorm:"type:text"` // JSON: {"source_pattern": "claude-*", "target_model": "..."}

	// 思考预算映射 (JSON 存储)
	ThinkingBudgetConfig string `gorm:"type:text"` // JSON 配置

	// 元数据
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

// ModelMappingRule 模型映射规则
type ModelMappingRule struct {
	SourcePattern string `json:"source_pattern"`
	TargetModel   string `json:"target_model"`
}

// ThinkingBudgetMapping 思考预算映射配置
type ThinkingBudgetMapping struct {
	OpenAILowToAnthropicTokens    int `json:"openai_low_to_anthropic_tokens"`
	OpenAIMediumToAnthropicTokens int `json:"openai_medium_to_anthropic_tokens"`
	OpenAIHighToAnthropicTokens   int `json:"openai_high_to_anthropic_tokens"`
	// ...其他映射字段
}
