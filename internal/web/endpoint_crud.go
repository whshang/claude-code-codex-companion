package web

import (
	"fmt"
	"net/http"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/endpoint"

	"github.com/gin-gonic/gin"
)

// Endpoint is an alias for endpoint.Endpoint for internal use
type Endpoint = endpoint.Endpoint

// handleGetEndpoints returns the list of all endpoints
func (s *AdminServer) handleGetEndpoints(c *gin.Context) {
	endpoints := s.endpointManager.GetAllEndpoints()
	c.JSON(http.StatusOK, gin.H{
		"endpoints": endpoints,
	})
}

// handleUpdateEndpoints is a stub for a deprecated feature.
func (s *AdminServer) handleUpdateEndpoints(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

// handleCreateEndpoint is a stub for a deprecated feature.
func (s *AdminServer) handleCreateEndpoint(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

// handleUpdateEndpoint updates a specific endpoint's configuration
func (s *AdminServer) handleUpdateEndpoint(c *gin.Context) {
	endpointID := c.Param("id")
	if endpointID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint ID is required"})
		return
	}

	// 解析请求体
	var updateReq struct {
		Enabled             *bool                      `json:"enabled"`
		Tags                []string                   `json:"tags"`
		Proxy               *config.ProxyConfig        `json:"proxy"`
		HeaderOverrides     map[string]string          `json:"header_overrides"`
		ParameterOverrides  map[string]string          `json:"parameter_overrides"`
		SupportsResponses   *bool                      `json:"supports_responses"`
		ModelRewrite        *config.ModelRewriteConfig `json:"model_rewrite"`
		CountTokensEnabled  *bool                      `json:"count_tokens_enabled"`
		OpenAIPreference    string                     `json:"openai_preference"`
	}

	if err := c.ShouldBindJSON(&updateReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 查找端点
	var endpoint *Endpoint
	allEndpoints := s.endpointManager.GetAllEndpoints()
	for _, ep := range allEndpoints {
		if ep.Name == endpointID {
			endpoint = ep
			break
		}
	}

	if endpoint == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("endpoint '%s' not found", endpointID)})
		return
	}

	// 更新配置文件
	err := s.updateEndpointInConfig(endpointID, func(cfg *config.EndpointConfig) error {
		// 更新各个字段
		if updateReq.Enabled != nil {
			cfg.Enabled = *updateReq.Enabled
		}
		if updateReq.Tags != nil {
			cfg.Tags = updateReq.Tags
		}
		if updateReq.Proxy != nil {
			cfg.Proxy = updateReq.Proxy
		}
		if updateReq.HeaderOverrides != nil {
			cfg.HeaderOverrides = updateReq.HeaderOverrides
		}
		if updateReq.ParameterOverrides != nil {
			cfg.ParameterOverrides = updateReq.ParameterOverrides
		}
		if updateReq.SupportsResponses != nil {
			cfg.SupportsResponses = updateReq.SupportsResponses
		}
		if updateReq.ModelRewrite != nil {
			cfg.ModelRewrite = updateReq.ModelRewrite
		}
		if updateReq.CountTokensEnabled != nil {
			cfg.CountTokensEnabled = updateReq.CountTokensEnabled
		}
		if updateReq.OpenAIPreference != "" {
			cfg.OpenAIPreference = updateReq.OpenAIPreference
		}

		return nil
	})

	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to update endpoint '%s' configuration", endpointID), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 热更新配置（重新加载端点）
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after update", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}

		if err := s.hotUpdateHandler.HotUpdateConfig(newConfig); err != nil {
			s.logger.Error("Failed to hot update configuration", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}
	}

	// 返回更新后的端点
	var updatedEndpoint *Endpoint
	allEndpointsAfterUpdate := s.endpointManager.GetAllEndpoints()
	for _, ep := range allEndpointsAfterUpdate {
		if ep.Name == endpointID {
			updatedEndpoint = ep
			break
		}
	}

	if updatedEndpoint == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "endpoint updated but not found after reload"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"endpoint": updatedEndpoint,
		"message":  fmt.Sprintf("Endpoint '%s' updated successfully", endpointID),
	})
}

// updateEndpointInConfig 安全地更新指定端点的配置并持久化到config.yaml
func (s *AdminServer) updateEndpointInConfig(endpointName string, updateFunc func(*config.EndpointConfig) error) error {
	// 查找对应的端点配置
	for i := range s.config.Endpoints {
		if s.config.Endpoints[i].Name == endpointName {
			// 应用更新函数
			if err := updateFunc(&s.config.Endpoints[i]); err != nil {
				return err
			}

			// 保存到配置文件
			return config.SaveConfig(s.config, s.configFilePath)
		}
	}

	return fmt.Errorf("endpoint not found: %s", endpointName)
}

// handleDeleteEndpoint is a stub for a deprecated feature.
func (s *AdminServer) handleDeleteEndpoint(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

// normalizeOpenAIPreference is a helper for a deprecated feature.
func normalizeOpenAIPreference(value string) (string, error) {
	// No-op, kept for compatibility if anything still calls it.
	return "auto", nil
}