package web

import (
	"fmt"

	"claude-code-codex-companion/internal/config"
)

// saveEndpointsToConfig 将端点配置保存到配置文件
func (s *AdminServer) saveEndpointsToConfig(endpointConfigs []config.EndpointConfig) error {
	// 更新配置
	s.config.Endpoints = endpointConfigs
	
	// 保存到文件
	return config.SaveConfig(s.config, s.configFilePath)
}

// createEndpointConfigFromRequest 从请求创建端点配置，自动设置优先级
func createEndpointConfigFromRequest(name, urlAnthropic, urlOpenAI, authType, authValue string, enabled bool, priority int, tags []string, proxy *config.ProxyConfig, oauthConfig *config.OAuthConfig, headerOverrides map[string]string, parameterOverrides map[string]string) config.EndpointConfig {
	return config.EndpointConfig{
		Name:              name,
		URLAnthropic:      urlAnthropic,
		URLOpenAI:         urlOpenAI,
		AuthType:          authType,
		AuthValue:         authValue,
		Enabled:           enabled,
		Priority:          priority,
		Tags:              tags,
		Proxy:             proxy,
		OAuthConfig:        oauthConfig,
		HeaderOverrides:    headerOverrides,
		ParameterOverrides: parameterOverrides,
	}
}

// generateUniqueEndpointName 生成唯一的端点名称，如果存在重名则添加数字后缀
func (s *AdminServer) generateUniqueEndpointName(baseName string) string {
	currentEndpoints := s.config.Endpoints
	
	// 检查基础名称是否已存在
	nameExists := func(name string) bool {
		for _, ep := range currentEndpoints {
			if ep.Name == name {
				return true
			}
		}
		return false
	}
	
	// 如果基础名称不存在，直接返回
	if !nameExists(baseName) {
		return baseName
	}
	
	// 如果存在，添加数字后缀
	counter := 1
	for {
		newName := generateEndpointNameWithSuffix(baseName, counter)
		if !nameExists(newName) {
			return newName
		}
		counter++
	}
}

// generateEndpointNameWithSuffix 生成带数字后缀的端点名称
func generateEndpointNameWithSuffix(baseName string, counter int) string {
	return generateEndpointNameFormat(baseName, counter)
}

// generateEndpointNameFormat 格式化端点名称
func generateEndpointNameFormat(baseName string, counter int) string {
	return fmt.Sprintf("%s (%d)", baseName, counter)
}