package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetEndpointProfiles returns an empty list as the wizard is deprecated.
func (s *AdminServer) handleGetEndpointProfiles(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"profiles": []interface{}{},
		"message":  "Endpoint profiles are deprecated in the new static configuration model.",
	})
}

// handleCreateEndpointFromWizard returns an error as the wizard is deprecated.
func (s *AdminServer) handleCreateEndpointFromWizard(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "The endpoint wizard is disabled due to a major architectural refactoring. Endpoints are now managed directly in the config.yaml file."})
}

// handleGenerateEndpointName returns an error as the wizard is deprecated.
func (s *AdminServer) handleGenerateEndpointName(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "The endpoint wizard is disabled due to a major architectural refactoring."})
}

// registerEndpointWizardRoutes registers the (now deprecated) API routes for the endpoint wizard.
func (s *AdminServer) registerEndpointWizardRoutes(api *gin.RouterGroup) {
	api.GET("/endpoint-profiles", s.handleGetEndpointProfiles)
	api.POST("/endpoints/from-wizard", s.handleCreateEndpointFromWizard)
	api.POST("/endpoints/generate-name", s.handleGenerateEndpointName)
}
