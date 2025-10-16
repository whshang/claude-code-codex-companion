package web

import (
	"fmt"
)

// saveEndpointsToConfig is a stub for a deprecated feature.
func (s *AdminServer) saveEndpointsToConfig(endpointConfigs []interface{}) error {
	return fmt.Errorf(disabledError)
}

// createEndpointConfigFromRequest is a stub for a deprecated feature.
// It returns nil because the config.EndpointConfig struct no longer exists.
func createEndpointConfigFromRequest(name, urlAnthropic, urlOpenAI, authType, authValue string, enabled bool, priority int, tags []string, proxy interface{}, oauthConfig interface{}, headerOverrides map[string]string, parameterOverrides map[string]string, countTokensEnabled *bool, supportsResponses *bool) interface{} {
	return nil
}

// generateUniqueEndpointName is a stub for a deprecated feature.
// It now simply returns the base name as there is no list to check for uniqueness.
func (s *AdminServer) generateUniqueEndpointName(baseName string) string {
	return baseName
}

// generateEndpointNameWithSuffix is a stub for a deprecated feature.
func generateEndpointNameWithSuffix(baseName string, counter int) string {
	return fmt.Sprintf("%s (%d)", baseName, counter)
}

// generateEndpointNameFormat is a stub for a deprecated feature.
func generateEndpointNameFormat(baseName string, counter int) string {
	return fmt.Sprintf("%s (%d)", baseName, counter)
}