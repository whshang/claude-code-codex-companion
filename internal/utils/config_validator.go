package utils

import (
	"fmt"
)

// EndpointConfigValidator provides unified endpoint configuration validation
type EndpointConfigValidator struct{}

// EndpointConfig interface for endpoint configuration
type EndpointConfig interface {
	GetName() string
	GetURL() string
	GetAuthType() string
	GetAuthValue() string
}

// NewEndpointConfigValidator creates a new endpoint configuration validator
func NewEndpointConfigValidator() *EndpointConfigValidator {
	return &EndpointConfigValidator{}
}

// ValidateEndpoint validates a single endpoint configuration
func (v *EndpointConfigValidator) ValidateEndpoint(endpoint EndpointConfig, index int) error {
	if endpoint.GetName() == "" {
		return fmt.Errorf("endpoint %d: name cannot be empty", index)
	}
	
	if endpoint.GetURL() == "" {
		return fmt.Errorf("endpoint %d: url cannot be empty", index)
	}
	
	if endpoint.GetAuthType() != "" && endpoint.GetAuthType() != "api_key" && endpoint.GetAuthType() != "auth_token" && endpoint.GetAuthType() != "oauth" && endpoint.GetAuthType() != "auto" {
		return fmt.Errorf("endpoint %d: invalid auth_type '%s', must be 'api_key', 'auth_token', 'oauth', 'auto', or empty", index, endpoint.GetAuthType())
	}
	
	// OAuth 认证不需要 auth_value，其他认证类型需要
	if endpoint.GetAuthType() != "oauth" && endpoint.GetAuthValue() == "" {
		return fmt.Errorf("endpoint %d: auth_value cannot be empty for non-oauth authentication", index)
	}
	
	return nil
}

// ValidateEndpoints validates a list of endpoint configurations
func (v *EndpointConfigValidator) ValidateEndpoints(endpoints []EndpointConfig) error {
	if len(endpoints) == 0 {
		return fmt.Errorf("at least one endpoint must be configured")
	}
	
	for i, endpoint := range endpoints {
		if err := v.ValidateEndpoint(endpoint, i); err != nil {
			return err
		}
	}
	
	return nil
}

// ValidateServerConfig validates server configuration
func ValidateServerConfig(host string, port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid server port: %d", port)
	}
	
	return nil
}