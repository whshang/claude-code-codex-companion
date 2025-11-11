package proxy

import (
	"github.com/gin-gonic/gin"
)

// sendProxyError sends a standardized error response for proxy failures
func (s *Server) sendProxyError(c *gin.Context, statusCode int, errorType, message string, requestID string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"type":       errorType,
			"message":    message,
			"request_id": requestID,
		},
	})
}