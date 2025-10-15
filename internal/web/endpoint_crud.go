package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/i18n"
	"claude-code-codex-companion/internal/security"

	"github.com/gin-gonic/gin"
)

// handleGetEndpoints 获取所有端点
func (s *AdminServer) handleGetEndpoints(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	c.JSON(http.StatusOK, gin.H{
		"endpoints": endpoints,
	})
}

// handleUpdateEndpoints 批量更新端点
func (s *AdminServer) handleUpdateEndpoints(c *gin.Context) {
	var request struct {
		Endpoints []config.EndpointConfig `json:"endpoints"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	// 创建新配置，只更新端点部分
	newConfig := *s.config
	newConfig.Endpoints = request.Endpoints

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(request.Endpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update endpoints: " + err.Error(),
		})
		return
	}

	// 如果没有热更新处理器，则使用旧方式
	if s.hotUpdateHandler == nil {
		s.endpointManager.UpdateEndpoints(request.Endpoints)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoints updated successfully"})
}

// handleCreateEndpoint 创建新端点
func (s *AdminServer) handleCreateEndpoint(c *gin.Context) {
	var request struct {
		Name                string              `json:"name" binding:"required"`
		URLAnthropic        string              `json:"url_anthropic"`
		URLOpenAI           string              `json:"url_openai"`
		AuthType            string              `json:"auth_type" binding:"required"`
		AuthValue           string              `json:"auth_value"` // OAuth时不需要
		Enabled             bool                `json:"enabled"`
		Tags                []string            `json:"tags"`
		Proxy               *config.ProxyConfig `json:"proxy,omitempty"`                 // 新增：代理配置
		OAuthConfig         *config.OAuthConfig `json:"oauth_config,omitempty"`          // 新增：OAuth配置
		HeaderOverrides     map[string]string   `json:"header_overrides,omitempty"`      // 新增：HTTP Header覆盖配置
		ParameterOverrides  map[string]string   `json:"parameter_overrides,omitempty"`   // 新增：Request Parameter覆盖配置
		CountTokensEnabled  *bool               `json:"count_tokens_enabled,omitempty"`  // 新增：是否允许 /count_tokens
		SupportsResponses   *bool               `json:"supports_responses,omitempty"`    // 新增：显式声明 /responses 支持
		OpenAIPreference    string              `json:"openai_preference"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 添加安全验证
	if err := security.ValidateEndpointName(request.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "endpoint_name_validation_failed", "端点名称验证失败: ") + err.Error()})
		return
	}

	// 至少需要一个URL
	if request.URLAnthropic == "" && request.URLOpenAI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "at_least_one_url_required", "至少需要填写一个URL（Anthropic URL或OpenAI URL）")})
		return
	}

	// 验证提供的URL
	if request.URLAnthropic != "" {
		if err := security.ValidateURL(request.URLAnthropic); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "anthropic_url_validation_failed", "Anthropic URL验证失败: ") + err.Error()})
			return
		}
	}

	if request.URLOpenAI != "" {
		if err := security.ValidateURL(request.URLOpenAI); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "openai_url_validation_failed", "OpenAI URL验证失败: ") + err.Error()})
			return
		}
	}

	if err := security.ValidateTags(request.Tags); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "tags_validation_failed", "标签验证失败: ") + err.Error()})
		return
	}

	if request.AuthValue != "" {
		if err := security.ValidateAuthToken(request.AuthValue); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "auth_token_validation_failed", "认证令牌验证失败: ") + err.Error()})
			return
		}
	}

	// 验证auth_type
	if request.AuthType != "api_key" && request.AuthType != "auth_token" && request.AuthType != "oauth" && request.AuthType != "auto" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "auth_type must be 'api_key', 'auth_token', 'oauth', or 'auto'"})
		return
	}

	normalizedPreference, err := normalizeOpenAIPreference(request.OpenAIPreference)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证 OAuth 或传统认证配置
	if request.AuthType == "oauth" {
		if request.OAuthConfig == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "oauth_config is required when auth_type is 'oauth'"})
			return
		}
		// 验证OAuth配置
		if err := config.ValidateOAuthConfig(request.OAuthConfig, fmt.Sprintf("endpoint '%s'", request.Name)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid oauth config: " + err.Error()})
			return
		}
	} else {
		// 非 OAuth 认证需要 auth_value
		if request.AuthValue == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "auth_value is required for non-oauth authentication"})
			return
		}
	}

	// 验证代理配置（如果提供）
	if request.Proxy != nil {
		if err := config.ValidateProxyConfig(request.Proxy, fmt.Sprintf("endpoint '%s'", request.Name)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy config: " + err.Error()})
			return
		}
	}

	// 设置默认值 - 移除timeout相关逻辑

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints

	// 新端点的优先级为当前最大优先级+1
	maxPriority := 0
	for _, ep := range currentEndpoints {
		if ep.Priority > maxPriority {
			maxPriority = ep.Priority
		}
	}

	// 创建新端点配置
	var countTokensPtr *bool
	if request.CountTokensEnabled != nil {
		countTokensPtr = new(bool)
		*countTokensPtr = *request.CountTokensEnabled
	}
	newEndpoint := createEndpointConfigFromRequest(
		request.Name, request.URLAnthropic, request.URLOpenAI,
		request.AuthType, request.AuthValue,
		request.Enabled, maxPriority+1, request.Tags, request.Proxy, request.OAuthConfig, request.HeaderOverrides, request.ParameterOverrides, countTokensPtr, request.SupportsResponses)
	newEndpoint.OpenAIPreference = normalizedPreference
	currentEndpoints = append(currentEndpoints, newEndpoint)

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to create endpoint: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message":  "Endpoint created successfully",
		"endpoint": newEndpoint,
	})
}

// handleUpdateEndpoint 更新特定端点
func (s *AdminServer) handleUpdateEndpoint(c *gin.Context) {
	encodedEndpointName := c.Param("id") // 使用名称作为ID
	endpointName, err := url.PathUnescape(encodedEndpointName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endpoint name encoding"})
		return
	}

	var request struct {
		Name                string                     `json:"name"`
		URLAnthropic        string                     `json:"url_anthropic"`
		URLOpenAI           string                     `json:"url_openai"`
		AuthType            string                     `json:"auth_type"`
		AuthValue           string                     `json:"auth_value"`
		Enabled             bool                       `json:"enabled"`
		Tags                []string                   `json:"tags"`
		Proxy               *config.ProxyConfig        `json:"proxy,omitempty"`               // 新增：代理配置
		OAuthConfig         *config.OAuthConfig        `json:"oauth_config,omitempty"`        // 新增：OAuth配置
		HeaderOverrides     map[string]string          `json:"header_overrides,omitempty"`    // 新增：HTTP Header覆盖配置
		ParameterOverrides  map[string]string          `json:"parameter_overrides,omitempty"` // 新增：Request Parameter覆盖配置
		ModelRewrite        *config.ModelRewriteConfig `json:"model_rewrite"`                 // 修改：移除omitempty，允许null值
		CountTokensEnabled  *bool                      `json:"count_tokens_enabled,omitempty"`
		SupportsResponses   *bool                      `json:"supports_responses,omitempty"`
		OpenAIPreference    *string                    `json:"openai_preference,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format: " + err.Error()})
		return
	}

	// 添加安全验证
	if request.Name != "" {
		if err := security.ValidateEndpointName(request.Name); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "endpoint_name_validation_failed", "端点名称验证失败: ") + err.Error()})
			return
		}
	}

	// 验证URL
	if request.URLAnthropic != "" {
		if err := security.ValidateURL(request.URLAnthropic); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "anthropic_url_validation_failed", "Anthropic URL验证失败: ") + err.Error()})
			return
		}
	}

	if request.URLOpenAI != "" {
		if err := security.ValidateURL(request.URLOpenAI); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "openai_url_validation_failed", "OpenAI URL验证失败: ") + err.Error()})
			return
		}
	}

	if len(request.Tags) > 0 {
		if err := security.ValidateTags(request.Tags); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "tags_validation_failed", "标签验证失败: ") + err.Error()})
			return
		}
	}

	if request.AuthValue != "" {
		if err := security.ValidateAuthToken(request.AuthValue); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": i18n.TCtx(c, "auth_token_validation_failed", "认证令牌验证失败: ") + err.Error()})
			return
		}
	}

	// 验证代理配置（如果提供）
	if request.Proxy != nil {
		if err := config.ValidateProxyConfig(request.Proxy, fmt.Sprintf("endpoint '%s'", endpointName)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid proxy config: " + err.Error()})
			return
		}
	}

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints
	found := false

	for i, ep := range currentEndpoints {
		if ep.Name == endpointName {
			// 更新端点，保持原有优先级
			if request.Name != "" {
				currentEndpoints[i].Name = request.Name
			}
			// 允许空字符串更新现有的URL值（用于清除URL）
			currentEndpoints[i].URLAnthropic = request.URLAnthropic
			currentEndpoints[i].URLOpenAI = request.URLOpenAI
			if request.AuthType != "" {
				if request.AuthType != "api_key" && request.AuthType != "auth_token" && request.AuthType != "oauth" && request.AuthType != "auto" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "auth_type must be 'api_key', 'auth_token', 'oauth', or 'auto'"})
					return
				}

				// 验证 OAuth 或传统认证配置
				if request.AuthType == "oauth" {
					if request.OAuthConfig == nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": "oauth_config is required when auth_type is 'oauth'"})
						return
					}
					// 验证OAuth配置
					if err := config.ValidateOAuthConfig(request.OAuthConfig, fmt.Sprintf("endpoint '%s'", endpointName)); err != nil {
						c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid oauth config: " + err.Error()})
						return
					}

					// 检查内存中是否已有更新的 OAuth token（防止覆盖已刷新的token）
					if currentEndpoints[i].AuthType == "oauth" && currentEndpoints[i].OAuthConfig != nil {
						currentExpiresAt := currentEndpoints[i].OAuthConfig.ExpiresAt
						requestExpiresAt := request.OAuthConfig.ExpiresAt

						// 如果内存中的过期时间比 WebUI 发送的更大，说明后台已刷新token，拒绝更新
						if currentExpiresAt > requestExpiresAt && requestExpiresAt > 0 {
							c.JSON(http.StatusConflict, gin.H{
								"error":              "Cannot update OAuth config: token has been refreshed in background. Please reload the page to get the latest configuration.",
								"current_expires_at": currentExpiresAt,
								"request_expires_at": requestExpiresAt,
							})
							return
						}
					}

					// 设置OAuth配置，清空auth_value
					currentEndpoints[i].OAuthConfig = request.OAuthConfig
					currentEndpoints[i].AuthValue = ""
				} else {
					// 非 OAuth 认证，清空OAuth配置
					currentEndpoints[i].OAuthConfig = nil
					if request.AuthValue != "" {
						currentEndpoints[i].AuthValue = request.AuthValue
					}
				}
				currentEndpoints[i].AuthType = request.AuthType
			}
			currentEndpoints[i].Enabled = request.Enabled

			// 更新tags字段
			currentEndpoints[i].Tags = request.Tags

			// 更新代理配置
			currentEndpoints[i].Proxy = request.Proxy

			// 更新HTTP Header覆盖配置
			currentEndpoints[i].HeaderOverrides = request.HeaderOverrides

			// 更新Request Parameter覆盖配置
			currentEndpoints[i].ParameterOverrides = request.ParameterOverrides

			// 更新模型重写配置
			// 前端现在始终发送配置对象（enabled=false或enabled=true+rules），不再发送null
			// 因此简化逻辑：直接使用request中的配置，如果为nil则保持原值不变
			if request.ModelRewrite != nil {
				// 验证模型重写配置
				if err := config.ValidateModelRewriteConfig(request.ModelRewrite, fmt.Sprintf("endpoint '%s'", endpointName)); err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid model rewrite config: " + err.Error()})
					return
				}
				// 始终保存配置对象，即使enabled=false（前端需要显示禁用状态）
				currentEndpoints[i].ModelRewrite = request.ModelRewrite
			}
			if request.CountTokensEnabled != nil {
				ptr := new(bool)
				*ptr = *request.CountTokensEnabled
				currentEndpoints[i].CountTokensEnabled = ptr
			}
			currentEndpoints[i].SupportsResponses = request.SupportsResponses
			if request.OpenAIPreference != nil {
				pref, prefErr := normalizeOpenAIPreference(*request.OpenAIPreference)
				if prefErr != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": prefErr.Error()})
					return
				}
				currentEndpoints[i].OpenAIPreference = pref
			}
			// 如果没有model_rewrite字段，保持原有配置不变

			found = true
			break
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(currentEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update endpoint: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoint updated successfully"})
}

// handleDeleteEndpoint 删除端点
func (s *AdminServer) handleDeleteEndpoint(c *gin.Context) {
	encodedEndpointName := c.Param("id") // 使用名称作为ID
	endpointName, err := url.PathUnescape(encodedEndpointName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid endpoint name encoding"})
		return
	}

	// 获取当前所有端点
	currentEndpoints := s.config.Endpoints
	newEndpoints := make([]config.EndpointConfig, 0)
	found := false

	for _, ep := range currentEndpoints {
		if ep.Name != endpointName {
			newEndpoints = append(newEndpoints, ep)
		} else {
			found = true
		}
	}

	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "Endpoint not found"})
		return
	}

	// 重新计算优先级（按数组顺序）
	for i := range newEndpoints {
		newEndpoints[i].Priority = i + 1
	}

	// 使用热更新机制
	if err := s.hotUpdateEndpoints(newEndpoints); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to delete endpoint: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Endpoint deleted successfully"})
}

func normalizeOpenAIPreference(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "auto", nil
	}

	switch trimmed {
	case "auto", "responses", "chat_completions":
		return trimmed, nil
	default:
		return "", fmt.Errorf("openai_preference must be 'auto', 'responses', or 'chat_completions'")
	}
}
