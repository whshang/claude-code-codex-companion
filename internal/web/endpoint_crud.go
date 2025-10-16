package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetEndpoints returns the list of legacy endpoints, which is now always empty.
func (s *AdminServer) handleGetEndpoints(c *gin.Context) {
	// The old endpoint manager now returns an empty list.
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

// handleUpdateEndpoint is a stub for a deprecated feature.
func (s *AdminServer) handleUpdateEndpoint(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": disabledError})
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