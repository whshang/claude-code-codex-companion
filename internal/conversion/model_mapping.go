package conversion

import (
	"fmt"
	"regexp"
	"strings"

	"claude-code-codex-companion/internal/logger"
)

// ModelMapper 模型映射器
type ModelMapper struct {
	logger *logger.Logger
}

// ModelMappingRule 模型映射规则
type ModelMappingRule struct {
	SourcePattern string `json:"source_pattern" yaml:"source_pattern"`
	TargetModel   string `json:"target_model" yaml:"target_model"`
	compiledRegex *regexp.Regexp
}

// NewModelMapper 创建模型映射器
func NewModelMapper(logger *logger.Logger) *ModelMapper {
	return &ModelMapper{
		logger: logger,
	}
}

// MapModel 根据规则映射模型名称
func (m *ModelMapper) MapModel(sourceModel string, rules []ModelMappingRule) string {
	if sourceModel == "" {
		return sourceModel
	}

	// 预编译正则表达式（如果还没编译）
	for i := range rules {
		if rules[i].compiledRegex == nil {
			regex, err := regexp.Compile(rules[i].SourcePattern)
			if err != nil {
				if m.logger != nil {
					m.logger.Error("Invalid regex pattern '%s': %v", err)
				}
				continue
			}
			rules[i].compiledRegex = regex
		}
	}

	// 按顺序应用规则，第一个匹配的规则生效
	for _, rule := range rules {
		if rule.compiledRegex != nil && rule.compiledRegex.MatchString(sourceModel) {
			mappedModel := rule.TargetModel

			// 支持占位符替换
			mappedModel = m.applyPlaceholders(mappedModel, sourceModel, rule.compiledRegex)

			if m.logger != nil {
				m.logger.Debug(fmt.Sprintf("Model mapped: '%s' -> '%s' (rule: '%s')",
					sourceModel, mappedModel, rule.SourcePattern))
			}

			return mappedModel
		}
	}

	// 没有匹配的规则，返回原模型
	if m.logger != nil {
		m.logger.Debug(fmt.Sprintf("No mapping rule matched for model: '%s'", sourceModel))
	}

	return sourceModel
}

// applyPlaceholders 应用占位符替换
func (m *ModelMapper) applyPlaceholders(targetModel, sourceModel string, regex *regexp.Regexp) string {
	// 查找所有 $1, $2 等占位符
	result := targetModel

	// 使用正则表达式替换 $1, $2 等占位符
	submatches := regex.FindStringSubmatch(sourceModel)
	if len(submatches) > 1 {
		for i := 1; i < len(submatches); i++ {
			placeholder := fmt.Sprintf("$%d", i)
			result = strings.ReplaceAll(result, placeholder, submatches[i])
		}
	}

	return result
}

// ValidateRules 验证映射规则
func (m *ModelMapper) ValidateRules(rules []ModelMappingRule) []string {
	var errors []string

	for i, rule := range rules {
		if rule.SourcePattern == "" {
			errors = append(errors, fmt.Sprintf("Rule %d: source_pattern is empty", i+1))
			continue
		}

		if rule.TargetModel == "" {
			errors = append(errors, fmt.Sprintf("Rule %d: target_model is empty", i+1))
			continue
		}

		// 尝试编译正则表达式
		_, err := regexp.Compile(rule.SourcePattern)
		if err != nil {
			errors = append(errors, fmt.Sprintf("Rule %d: invalid regex pattern '%s': %v", i+1, rule.SourcePattern, err))
		}
	}

	return errors
}

// PrecompileRules 预编译所有规则的正则表达式
func (m *ModelMapper) PrecompileRules(rules []ModelMappingRule) {
	for i := range rules {
		if rules[i].compiledRegex == nil {
			regex, err := regexp.Compile(rules[i].SourcePattern)
			if err != nil {
				if m.logger != nil {
					m.logger.Error("Failed to compile regex for pattern '%s': %v", err)
				}
				continue
			}
			rules[i].compiledRegex = regex
		}
	}
}

// GetDefaultMappings 获取默认的模型映射规则
func GetDefaultMappings() []ModelMappingRule {
	return []ModelMappingRule{
		// Claude 模型映射
		{SourcePattern: `^claude-3-5-sonnet`, TargetModel: "claude-3-5-sonnet-20241022"},
		{SourcePattern: `^claude-3-haiku`, TargetModel: "claude-3-haiku-20240307"},
		{SourcePattern: `^claude-3-opus`, TargetModel: "claude-3-opus-20240229"},

		// GPT 模型映射
		{SourcePattern: `^gpt-4o`, TargetModel: "gpt-4o-2024-08-06"},
		{SourcePattern: `^gpt-4o-mini`, TargetModel: "gpt-4o-mini-2024-07-18"},
		{SourcePattern: `^gpt-4-turbo`, TargetModel: "gpt-4-turbo-2024-04-09"},
		{SourcePattern: `^gpt-4`, TargetModel: "gpt-4-0613"},

		// Gemini 模型映射
		{SourcePattern: `^gemini-pro`, TargetModel: "gemini-1.5-pro-latest"},
		{SourcePattern: `^gemini-flash`, TargetModel: "gemini-1.5-flash-latest"},
		{SourcePattern: `^gemini-1\.5-pro`, TargetModel: "gemini-1.5-pro-latest"},
		{SourcePattern: `^gemini-1\.5-flash`, TargetModel: "gemini-1.5-flash-latest"},
	}
}

// CreateCrossProviderMappings 创建跨提供商的映射规则
func CreateCrossProviderMappings() map[string][]ModelMappingRule {
	return map[string][]ModelMappingRule{
		// 从Claude到其他提供商的映射
		"claude-to-openai": {
			{SourcePattern: `^claude-3-5-sonnet`, TargetModel: "gpt-4o-2024-08-06"},
			{SourcePattern: `^claude-3-haiku`, TargetModel: "gpt-4o-mini-2024-07-18"},
			{SourcePattern: `^claude-3-opus`, TargetModel: "gpt-4-turbo-2024-04-09"},
		},

		// 从OpenAI到其他提供商的映射
		"openai-to-claude": {
			{SourcePattern: `^gpt-4o`, TargetModel: "claude-3-5-sonnet-20241022"},
			{SourcePattern: `^gpt-4o-mini`, TargetModel: "claude-3-haiku-20240307"},
			{SourcePattern: `^gpt-4-turbo`, TargetModel: "claude-3-opus-20240229"},
		},

		"openai-to-gemini": {
			{SourcePattern: `^gpt-4o`, TargetModel: "gemini-1.5-pro-latest"},
			{SourcePattern: `^gpt-4o-mini`, TargetModel: "gemini-1.5-flash-latest"},
			{SourcePattern: `^gpt-4-turbo`, TargetModel: "gemini-1.5-pro-latest"},
		},

		// 从Gemini到其他提供商的映射
		"gemini-to-openai": {
			{SourcePattern: `^gemini-1\.5-pro`, TargetModel: "gpt-4o-2024-08-06"},
			{SourcePattern: `^gemini-1\.5-flash`, TargetModel: "gpt-4o-mini-2024-07-18"},
		},

		"gemini-to-claude": {
			{SourcePattern: `^gemini-1\.5-pro`, TargetModel: "claude-3-5-sonnet-20241022"},
			{SourcePattern: `^gemini-1\.5-flash`, TargetModel: "claude-3-haiku-20240307"},
		},
	}
}

// GetMappingForConversion 根据转换方向获取映射规则
func GetMappingForConversion(fromFormat, toFormat string) []ModelMappingRule {
	key := fmt.Sprintf("%s-to-%s", fromFormat, toFormat)
	mappings := CreateCrossProviderMappings()

	if rules, ok := mappings[key]; ok {
		return rules
	}

	// 如果没有找到特定的映射，返回空的规则列表
	return []ModelMappingRule{}
}

// ModelFamily 表示模型家族
type ModelFamily string

const (
	ModelFamilyClaude  ModelFamily = "claude"
	ModelFamilyGPT     ModelFamily = "gpt"
	ModelFamilyGemini  ModelFamily = "gemini"
)

// DetectModelFamily 检测模型所属的家族
func DetectModelFamily(modelName string) ModelFamily {
	modelName = strings.ToLower(modelName)

	if strings.Contains(modelName, "claude") {
		return ModelFamilyClaude
	}
	if strings.Contains(modelName, "gpt") {
		return ModelFamilyGPT
	}
	if strings.Contains(modelName, "gemini") {
		return ModelFamilyGemini
	}

	return ""
}

// GetEquivalentModel 获取等价模型
func (m *ModelMapper) GetEquivalentModel(modelName string, targetFamily ModelFamily) string {
	sourceFamily := DetectModelFamily(modelName)
	if sourceFamily == "" || sourceFamily == targetFamily {
		return modelName
	}

	// 获取跨提供商映射
	rules := GetMappingForConversion(string(sourceFamily), string(targetFamily))

	if len(rules) == 0 {
		if m.logger != nil {
			m.logger.Error(fmt.Sprintf("No mapping rules found for %s to %s", sourceFamily, targetFamily), nil)
		}
		return modelName
	}

	mappedModel := m.MapModel(modelName, rules)
	if mappedModel != modelName {
		if m.logger != nil {
			m.logger.Info(fmt.Sprintf("Model converted: %s (%s) -> %s (%s)",
				modelName, sourceFamily, mappedModel, targetFamily))
		}
	}

	return mappedModel
}

// MergeRules 合并多个规则列表，后面的规则优先级更高
func MergeRules(ruleLists ...[]ModelMappingRule) []ModelMappingRule {
	var merged []ModelMappingRule

	// 使用map来去重，key为source_pattern，value为规则
	ruleMap := make(map[string]ModelMappingRule)

	for _, rules := range ruleLists {
		for _, rule := range rules {
			ruleMap[rule.SourcePattern] = rule
		}
	}

	// 转换回切片
	for _, rule := range ruleMap {
		merged = append(merged, rule)
	}

	return merged
}

// FilterRulesByTarget 按目标模型过滤规则
func FilterRulesByTarget(rules []ModelMappingRule, targetPattern string) []ModelMappingRule {
	targetRegex, err := regexp.Compile(targetPattern)
	if err != nil {
		return rules
	}

	var filtered []ModelMappingRule
	for _, rule := range rules {
		if targetRegex.MatchString(rule.TargetModel) {
			filtered = append(filtered, rule)
		}
	}

	return filtered
}
