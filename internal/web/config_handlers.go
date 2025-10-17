package web

import (
	"fmt"
	"net/http"

	"claude-code-codex-companion/internal/config"
	"github.com/gin-gonic/gin"
)

// handleGetConfig gets the current configuration.
// NOTE: This now returns the new static configuration. Sensitive values are not redacted.
func (s *AdminServer) handleGetConfig(c *gin.Context) {
	configCopy := *s.config
	c.JSON(http.StatusOK, gin.H{
		"config": configCopy,
	})
}

// handleHotUpdateConfig is a stub for a deprecated feature.
func (s *AdminServer) handleHotUpdateConfig(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Full configuration hot-update is disabled due to architectural refactoring."})
}

// validateConfigUpdate is a stub for a deprecated feature.
func (s *AdminServer) validateConfigUpdate(newConfig *config.Config) error {
	// Validation logic for the dynamic endpoint list is removed.
	return nil
}

// handleUpdateEndpointModelRewrite updates the model rewrite configuration for a specific endpoint
func (s *AdminServer) handleUpdateEndpointModelRewrite(c *gin.Context) {
	endpointID := c.Param("id")
	if endpointID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "endpoint ID is required"})
		return
	}

	// 解析请求体
	var modelRewriteConfig config.ModelRewriteConfig
	if err := c.ShouldBindJSON(&modelRewriteConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid request body: %v", err)})
		return
	}

	// 更新配置文件
	err := s.updateEndpointInConfig(endpointID, func(cfg *config.EndpointConfig) error {
		cfg.ModelRewrite = &modelRewriteConfig
		return nil
	})

	if err != nil {
		s.logger.Error(fmt.Sprintf("Failed to update endpoint '%s' model rewrite configuration", endpointID), err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 热更新配置
	if s.hotUpdateHandler != nil {
		newConfig, err := config.LoadConfig(s.configFilePath)
		if err != nil {
			s.logger.Error("Failed to reload config after model rewrite update", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}

		if err := s.hotUpdateHandler.HotUpdateConfig(newConfig); err != nil {
			s.logger.Error("Failed to hot update configuration", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration updated but hot reload failed"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Endpoint '%s' model rewrite configuration updated successfully", endpointID),
	})
}

// handleTestModelRewrite is a stub for a deprecated feature.
func (s *AdminServer) handleTestModelRewrite(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Per-endpoint model rewrite testing is disabled."})
}
