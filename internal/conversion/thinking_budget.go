package conversion

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// ThinkingBudgetMapper 思考预算映射器
type ThinkingBudgetMapper struct {
	config *ThinkingBudgetConfig
	logger *logger.Logger
}

// ThinkingBudgetConfig 思考预算配置
type ThinkingBudgetConfig struct {
	// OpenAI reasoning_effort 到 Anthropic thinkingBudget 的映射
	OpenAILowToAnthropicTokens    int `yaml:"openai_low_to_anthropic_tokens" json:"openai_low_to_anthropic_tokens"`
	OpenAIMediumToAnthropicTokens int `yaml:"openai_medium_to_anthropic_tokens" json:"openai_medium_to_anthropic_tokens"`
	OpenAIHighToAnthropicTokens   int `yaml:"openai_high_to_anthropic_tokens" json:"openai_high_to_anthropic_tokens"`

	// Anthropic thinkingBudget 到 OpenAI reasoning_effort 的映射阈值
	AnthropicLowThreshold    int `yaml:"anthropic_low_threshold" json:"anthropic_low_threshold"`
	AnthropicMediumThreshold int `yaml:"anthropic_medium_threshold" json:"anthropic_medium_threshold"`

	// 默认值
	DefaultOpenAIMaxTokens    int `yaml:"default_openai_max_tokens" json:"default_openai_max_tokens"`
	DefaultAnthropicMaxTokens int `yaml:"default_anthropic_max_tokens" json:"default_anthropic_max_tokens"`
}

// NewThinkingBudgetMapper 创建思考预算映射器
func NewThinkingBudgetMapper(config *ThinkingBudgetConfig, logger *logger.Logger) *ThinkingBudgetMapper {
	if config == nil {
		config = &ThinkingBudgetConfig{
			OpenAILowToAnthropicTokens:    4096,
			OpenAIMediumToAnthropicTokens: 8192,
			OpenAIHighToAnthropicTokens:   16384,
			AnthropicLowThreshold:         4096,
			AnthropicMediumThreshold:      16384,
			DefaultOpenAIMaxTokens:        8192,
			DefaultAnthropicMaxTokens:     4096,
		}
	}

	return &ThinkingBudgetMapper{
		config: config,
		logger: logger,
	}
}

// OpenAIReasoningEffortToAnthropicTokens 将OpenAI reasoning_effort转换为Anthropic thinkingBudget
func (m *ThinkingBudgetMapper) OpenAIReasoningEffortToAnthropicTokens(effort string) int {
	switch effort {
	case "low":
		return m.config.OpenAILowToAnthropicTokens
	case "medium":
		return m.config.OpenAIMediumToAnthropicTokens
	case "high":
		return m.config.OpenAIHighToAnthropicTokens
	default:
		// 默认使用medium
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("Unknown reasoning_effort '%s', defaulting to 'medium'", effort), nil)
		}
		return m.config.OpenAIMediumToAnthropicTokens
	}
}

// AnthropicTokensToOpenAIReasoningEffort 将Anthropic thinkingBudget转换为OpenAI reasoning_effort
func (m *ThinkingBudgetMapper) AnthropicTokensToOpenAIReasoningEffort(tokens int) string {
	if tokens <= 0 {
		return "medium" // 默认值
	}

	if tokens <= m.config.AnthropicLowThreshold {
		return "low"
	} else if tokens <= m.config.AnthropicMediumThreshold {
		return "medium"
	} else {
		return "high"
	}
}

// GetDefaultOpenAIMaxTokens 获取默认的OpenAI最大token数
func (m *ThinkingBudgetMapper) GetDefaultOpenAIMaxTokens() int {
	// 优先使用环境变量
	if envVal := os.Getenv("OPENAI_REASONING_MAX_TOKENS"); envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil {
			return val
		}
	}
	return m.config.DefaultOpenAIMaxTokens
}

// GetDefaultAnthropicMaxTokens 获取默认的Anthropic最大token数
func (m *ThinkingBudgetMapper) GetDefaultAnthropicMaxTokens() int {
	// 优先使用环境变量
	if envVal := os.Getenv("ANTHROPIC_MAX_TOKENS"); envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil {
			return val
		}
	}
	return m.config.DefaultAnthropicMaxTokens
}

// GeminiThinkingBudgetToOpenAI 将Gemini thinkingBudget转换为OpenAI格式
func (m *ThinkingBudgetMapper) GeminiThinkingBudgetToOpenAI(thinkingBudget int) (reasoningEffort string, maxCompletionTokens int) {
	if thinkingBudget <= 0 || thinkingBudget == -1 {
		// 动态思考或未设置，默认为high
		return "high", m.GetDefaultOpenAIMaxTokens()
	}

	// 根据thinkingBudget大小确定reasoning_effort
	if thinkingBudget <= 4096 {
		reasoningEffort = "low"
	} else if thinkingBudget <= 16384 {
		reasoningEffort = "medium"
	} else {
		reasoningEffort = "high"
	}

	// 从环境变量获取阈值进行更精确的映射
	lowThreshold := m.getEnvInt("GEMINI_TO_OPENAI_LOW_REASONING_THRESHOLD", 4096)
	highThreshold := m.getEnvInt("GEMINI_TO_OPENAI_HIGH_REASONING_THRESHOLD", 16384)

	if thinkingBudget <= lowThreshold {
		reasoningEffort = "low"
	} else if thinkingBudget <= highThreshold {
		reasoningEffort = "medium"
	} else {
		reasoningEffort = "high"
	}

	maxCompletionTokens = m.GetDefaultOpenAIMaxTokens()

	if m.logger != nil {
		m.logger.Info(fmt.Sprintf("Gemini thinkingBudget %d -> OpenAI reasoning_effort='%s', max_completion_tokens=%d",
			thinkingBudget, reasoningEffort, maxCompletionTokens))
	}

	return reasoningEffort, maxCompletionTokens
}

// GeminiThinkingBudgetToAnthropic 将Gemini thinkingBudget转换为Anthropic格式
func (m *ThinkingBudgetMapper) GeminiThinkingBudgetToAnthropic(thinkingBudget int) interface{} {
	if thinkingBudget == -1 {
		// 动态思考
		if m.logger != nil {
			m.logger.Info("Gemini thinkingBudget -1 (dynamic) -> Anthropic thinking enabled without budget")
		}
		return map[string]interface{}{
			"type": "enabled",
		}
	} else if thinkingBudget == 0 {
		// 不启用思考
		return nil
	} else {
		// 数值型思考预算
		if m.logger != nil {
			m.logger.Info(fmt.Sprintf("Gemini thinkingBudget %d -> Anthropic thinkingBudget %d", thinkingBudget, thinkingBudget))
		}
		return map[string]interface{}{
			"type":          "enabled",
			"budget_tokens": thinkingBudget,
		}
	}
}

// OpenAIReasoningToAnthropic 将OpenAI reasoning配置转换为Anthropic格式
func (m *ThinkingBudgetMapper) OpenAIReasoningToAnthropic(reasoningEffort string, maxCompletionTokens int) interface{} {
	tokens := m.OpenAIReasoningEffortToAnthropicTokens(reasoningEffort)

	if m.logger != nil {
		m.logger.Info(fmt.Sprintf("OpenAI reasoning_effort='%s' -> Anthropic thinkingBudget %d", reasoningEffort, tokens), nil)
	}

	return map[string]interface{}{
		"type":          "enabled",
		"budget_tokens": tokens,
	}
}

// AnthropicThinkingToOpenAI 将Anthropic thinking配置转换为OpenAI格式
func (m *ThinkingBudgetMapper) AnthropicThinkingToOpenAI(thinking interface{}) (reasoningEffort string, maxCompletionTokens int) {
	if thinkingMap, ok := thinking.(map[string]interface{}); ok {
		thinkingType := m.getStringValue(thinkingMap, "type", "")
		if thinkingType == "enabled" {
			if budgetTokens, ok := thinkingMap["budget_tokens"].(int); ok && budgetTokens > 0 {
				reasoningEffort = m.AnthropicTokensToOpenAIReasoningEffort(budgetTokens)
				maxCompletionTokens = m.GetDefaultOpenAIMaxTokens()

				if m.logger != nil {
					m.logger.Info(fmt.Sprintf("Anthropic thinkingBudget %d -> OpenAI reasoning_effort='%s', max_completion_tokens=%d",
						budgetTokens, reasoningEffort, maxCompletionTokens), nil)
				}
				return reasoningEffort, maxCompletionTokens
			}
		}
	}

	// 默认值
	return "medium", m.GetDefaultOpenAIMaxTokens()
}

// 工具方法

func (m *ThinkingBudgetMapper) getEnvInt(key string, defaultValue int) int {
	if envVal := os.Getenv(key); envVal != "" {
		if val, err := strconv.Atoi(envVal); err == nil {
			return val
		}
	}
	return defaultValue
}

func (m *ThinkingBudgetMapper) getStringValue(data map[string]interface{}, key, defaultValue string) string {
	if value, ok := data[key]; ok {
		if str, ok := value.(string); ok {
			return str
		}
	}
	return defaultValue
}

// ValidateConfig 验证配置的有效性
func (c *ThinkingBudgetConfig) ValidateConfig() error {
	if c.OpenAILowToAnthropicTokens <= 0 {
		c.OpenAILowToAnthropicTokens = 4096
	}
	if c.OpenAIMediumToAnthropicTokens <= 0 {
		c.OpenAIMediumToAnthropicTokens = 8192
	}
	if c.OpenAIHighToAnthropicTokens <= 0 {
		c.OpenAIHighToAnthropicTokens = 16384
	}
	if c.AnthropicLowThreshold <= 0 {
		c.AnthropicLowThreshold = 4096
	}
	if c.AnthropicMediumThreshold <= 0 {
		c.AnthropicMediumThreshold = 16384
	}
	if c.DefaultOpenAIMaxTokens <= 0 {
		c.DefaultOpenAIMaxTokens = 8192
	}
	if c.DefaultAnthropicMaxTokens <= 0 {
		c.DefaultAnthropicMaxTokens = 4096
	}
	return nil
}

// GetDefaultConfig 获取默认配置
func GetDefaultThinkingBudgetConfig() *ThinkingBudgetConfig {
	return &ThinkingBudgetConfig{
		OpenAILowToAnthropicTokens:    4096,
		OpenAIMediumToAnthropicTokens: 8192,
		OpenAIHighToAnthropicTokens:   16384,
		AnthropicLowThreshold:         4096,
		AnthropicMediumThreshold:      16384,
		DefaultOpenAIMaxTokens:        8192,
		DefaultAnthropicMaxTokens:     4096,
	}
}

// helper: construct mapper with defaults when config/logger not provided
func newDefaultThinkingMapper(log *logger.Logger) *ThinkingBudgetMapper {
	return NewThinkingBudgetMapper(nil, log)
}

// InternalThinkingFromOpenAI 将 OpenAI reasoning 字段转换为内部 Thinking 表示
func InternalThinkingFromOpenAI(reasoningEffort *string, maxTokens *int) *InternalThinking {
	if (reasoningEffort == nil || strings.TrimSpace(*reasoningEffort) == "") && (maxTokens == nil || *maxTokens == 0) {
		return nil
	}
	thinking := &InternalThinking{
		Provider: "openai",
	}
	if reasoningEffort != nil {
		thinking.Type = strings.TrimSpace(*reasoningEffort)
	}
	if maxTokens != nil {
		thinking.BudgetTokens = *maxTokens
	}
	return thinking
}

// InternalThinkingFromAnthropic 将 Anthropic Thinking 转换为内部表示
func InternalThinkingFromAnthropic(thinking *AnthropicThinking) *InternalThinking {
	if thinking == nil || strings.EqualFold(thinking.Type, "") {
		return nil
	}
	result := &InternalThinking{
		Provider: "anthropic",
		Type:     thinking.Type,
	}
	if thinking.BudgetTokens > 0 {
		result.BudgetTokens = thinking.BudgetTokens
	}
	return result
}

// ApplyInternalThinkingToOpenAI 根据内部 Thinking 更新 OpenAI 请求字段
func ApplyInternalThinkingToOpenAI(req *InternalRequest, out interface{}, mapper *ThinkingBudgetMapper) {
	if req == nil || req.Thinking == nil {
		return
	}
	if mapper == nil {
		mapper = newDefaultThinkingMapper(nil)
	}

	switch target := out.(type) {
	case *OpenAIRequest:
		applyThinkingForOpenAIRequest(req.Thinking, target, mapper)
	case *OpenAIResponsesRequest:
		applyThinkingForOpenAIResponses(req.Thinking, target, mapper)
	}
}

// ApplyInternalThinkingToAnthropic 将内部 Thinking 映射到 Anthropic 请求
func ApplyInternalThinkingToAnthropic(req *InternalRequest, out *AnthropicRequest, mapper *ThinkingBudgetMapper) {
	if req == nil || out == nil {
		return
	}

	thinking := req.Thinking
	// 优先使用已有字段
	if thinking == nil {
		if req.ReasoningEffort != nil || req.MaxReasoningTokens != nil {
			thinking = InternalThinkingFromOpenAI(req.ReasoningEffort, req.MaxReasoningTokens)
		}
	}

	if thinking == nil {
		return
	}

	if mapper == nil {
		mapper = newDefaultThinkingMapper(nil)
	}

	out.Thinking = buildAnthropicThinking(thinking, mapper)
}

func applyThinkingForOpenAIRequest(thinking *InternalThinking, out *OpenAIRequest, mapper *ThinkingBudgetMapper) {
	if out == nil || thinking == nil {
		return
	}

	switch strings.ToLower(thinking.Provider) {
	case "openai":
		if thinking.Type != "" && out.ReasoningEffort == nil {
			out.ReasoningEffort = ptrString(strings.ToLower(thinking.Type))
		}
		if thinking.BudgetTokens > 0 && out.MaxReasoningTokens == nil {
			out.MaxReasoningTokens = ptrInt(thinking.BudgetTokens)
		}
	case "anthropic", "gemini":
		if mapper == nil {
			mapper = newDefaultThinkingMapper(nil)
		}
		if thinking.BudgetTokens > 0 && out.MaxReasoningTokens == nil {
			out.MaxReasoningTokens = ptrInt(thinking.BudgetTokens)
		}
		if out.MaxReasoningTokens == nil && thinking.BudgetTokens <= 0 {
			out.MaxReasoningTokens = ptrInt(mapper.GetDefaultOpenAIMaxTokens())
		}
		if out.ReasoningEffort == nil {
			effort := mapper.AnthropicTokensToOpenAIReasoningEffort(out.MaxReasoningTokensValue())
			out.ReasoningEffort = ptrString(effort)
		}
	default:
		// 其他 Provider，保留已有字段
	}
}

func applyThinkingForOpenAIResponses(thinking *InternalThinking, out *OpenAIResponsesRequest, mapper *ThinkingBudgetMapper) {
	if out == nil || thinking == nil {
		return
	}
	switch strings.ToLower(thinking.Provider) {
	case "openai":
		if thinking.Type != "" && out.ReasoningEffort == nil {
			out.ReasoningEffort = ptrString(strings.ToLower(thinking.Type))
		}
		if thinking.BudgetTokens > 0 && out.MaxReasoningTokens == nil {
			out.MaxReasoningTokens = ptrInt(thinking.BudgetTokens)
		}
	case "anthropic", "gemini":
		if mapper == nil {
			mapper = newDefaultThinkingMapper(nil)
		}
		if thinking.BudgetTokens > 0 && out.MaxReasoningTokens == nil {
			out.MaxReasoningTokens = ptrInt(thinking.BudgetTokens)
		}
		if out.MaxReasoningTokens == nil && thinking.BudgetTokens <= 0 {
			out.MaxReasoningTokens = ptrInt(mapper.GetDefaultOpenAIMaxTokens())
		}
		if out.ReasoningEffort == nil {
			effort := mapper.AnthropicTokensToOpenAIReasoningEffort(out.MaxReasoningTokensValue())
			out.ReasoningEffort = ptrString(effort)
		}
	default:
	}
}

func buildAnthropicThinking(thinking *InternalThinking, mapper *ThinkingBudgetMapper) *AnthropicThinking {
	if thinking == nil {
		return nil
	}
	switch strings.ToLower(thinking.Provider) {
	case "anthropic":
		return &AnthropicThinking{
			Type:         thinking.Type,
			BudgetTokens: thinking.BudgetTokens,
		}
	case "openai", "gemini":
		if mapper == nil {
			mapper = newDefaultThinkingMapper(nil)
		}
		tokens := thinking.BudgetTokens
		if tokens <= 0 && thinking.Type != "" {
			tokens = mapper.OpenAIReasoningEffortToAnthropicTokens(strings.ToLower(thinking.Type))
		}
		if tokens <= 0 {
			tokens = mapper.config.DefaultAnthropicMaxTokens
		}
		return &AnthropicThinking{
			Type:         "enabled",
			BudgetTokens: tokens,
		}
	default:
		return nil
	}
}

func (out *OpenAIRequest) MaxReasoningTokensValue() int {
	if out == nil || out.MaxReasoningTokens == nil {
		return 0
	}
	return *out.MaxReasoningTokens
}

func (out *OpenAIResponsesRequest) MaxReasoningTokensValue() int {
	if out == nil || out.MaxReasoningTokens == nil {
		return 0
	}
	return *out.MaxReasoningTokens
}
