package web

import (
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

// handleUpdateEndpointModelRewrite is a stub for a deprecated feature.
func (s *AdminServer) handleUpdateEndpointModelRewrite(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Per-endpoint model rewrite configuration is disabled."})
}

// handleTestModelRewrite is a stub for a deprecated feature.
func (s *AdminServer) handleTestModelRewrite(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Per-endpoint model rewrite testing is disabled."})
}
