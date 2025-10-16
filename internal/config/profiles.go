package config

import (
	_ "embed"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed endpoint_profiles.yaml
var embeddedProfiles []byte

// EndpointProfile 端点预设配置结构
type EndpointProfile struct {
	ProfileID            string `yaml:"profile_id" json:"profile_id"`
	DisplayName          string `yaml:"display_name" json:"display_name"`
	URL                  string `yaml:"url" json:"url"`
	EndpointType         string `yaml:"endpoint_type" json:"endpoint_type"`
	AuthType             string `yaml:"auth_type" json:"auth_type"`
	PathPrefix           string `yaml:"path_prefix" json:"path_prefix"`
	RequireDefaultModel  bool   `yaml:"require_default_model" json:"require_default_model"`
	DefaultModelOptions  string `yaml:"default_model_options" json:"default_model_options"`
}

// ProfilesConfig 预设配置文件结构
type ProfilesConfig struct {
	Profiles []EndpointProfile `yaml:"profiles" json:"profiles"`
}

// LoadEmbeddedEndpointProfiles 加载嵌入的端点预设配置
func LoadEmbeddedEndpointProfiles() (*ProfilesConfig, error) {
	var config ProfilesConfig
	err := yaml.Unmarshal(embeddedProfiles, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// ToEndpointConfig has been removed as it generated the deprecated EndpointConfig struct.
// The endpoint wizard feature needs to be refactored to work with the new configuration model.

// GenerateUniqueEndpointName 生成唯一的端点名称
func GenerateUniqueEndpointName(displayName string, existingNames []string) string {
	// 将显示名称转换为合适的端点名称
	baseName := normalizeEndpointName(displayName)
	
	// 检查重复并生成唯一名称
	return ensureUniqueEndpointName(baseName, existingNames)
}

// ValidateAndGenerateEndpointName 验证并生成端点名称
func ValidateAndGenerateEndpointName(baseName string, existingNames []string) string {
	// 标准化名称
	normalized := normalizeEndpointName(baseName)
	
	// 确保唯一性
	return ensureUniqueEndpointName(normalized, existingNames)
}

// normalizeEndpointName 标准化端点名称
func normalizeEndpointName(displayName string) string {
	// 移除特殊字符，保留字母、数字、空格、横线
	var normalized strings.Builder
	for _, r := range displayName {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || 
		   (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
			normalized.WriteRune(r)
		}
	}
	
	name := normalized.String()
	
	// 转换为小写并处理空格和多余字符
	name = strings.ToLower(name)
	name = strings.TrimSpace(name)
	
	// 将连续的空格、横线、下划线转换为单个横线
	name = strings.ReplaceAll(name, " ", "-")
	name = strings.ReplaceAll(name, "_", "-")
	
	// 移除重复的横线
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	
	// 移除首尾的横线
	name = strings.Trim(name, "-")
	
	// 如果名称为空，使用默认名称
	if name == "" {
		name = "endpoint"
	}
	
	return name
}

// ensureUniqueEndpointName 确保端点名称唯一性
func ensureUniqueEndpointName(baseName string, existingNames []string) string {
	uniqueName := baseName
	counter := 1
	
	// 创建现有名称的map以提高查询效率
	existingSet := make(map[string]bool)
	for _, name := range existingNames {
		existingSet[name] = true
	}
	
	// 如果名称已存在，添加数字后缀
	for existingSet[uniqueName] {
		uniqueName = baseName + "-" + strconv.Itoa(counter)
		counter++
	}
	
	return uniqueName
}

// GetProfileByID 根据ID获取预设配置
func (pc *ProfilesConfig) GetProfileByID(profileID string) *EndpointProfile {
	for i := range pc.Profiles {
		if pc.Profiles[i].ProfileID == profileID {
			return &pc.Profiles[i]
		}
	}
	return nil
}

// GetDefaultModelOptions 获取默认模型选项列表
func (p *EndpointProfile) GetDefaultModelOptions() []string {
	if p.DefaultModelOptions == "" {
		return []string{}
	}
	
	options := strings.Split(p.DefaultModelOptions, ",")
	result := make([]string, 0, len(options))
	
	for _, option := range options {
		trimmed := strings.TrimSpace(option)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	
	return result
}