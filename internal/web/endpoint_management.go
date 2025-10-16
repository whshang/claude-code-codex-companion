package web

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

const disabledError = "This feature is disabled due to a major architectural refactoring. Endpoint management is now done directly in the config.yaml file."

func (s *AdminServer) handleToggleEndpoint(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

func (s *AdminServer) handleCopyEndpoint(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

func (s *AdminServer) handleResetEndpointStatus(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

func (s *AdminServer) handleResetAllEndpointsStatus(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

func (s *AdminServer) handleReorderEndpoints(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

func (s *AdminServer) handleSortEndpoints(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
}

// refreshDynamicSorterEndpoints is a stub for a deprecated feature.
func (s *AdminServer) refreshDynamicSorterEndpoints() error {
	return fmt.Errorf(disabledError)
}

// hotUpdateEndpointsWithRetry is a stub for a deprecated feature.
func (s *AdminServer) hotUpdateEndpointsWithRetry(endpoints []interface{}, maxRetries int) error {
	return fmt.Errorf(disabledError)
}