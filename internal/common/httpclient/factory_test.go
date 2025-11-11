package httpclient

import (
	"testing"
	"time"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	if factory == nil {
		t.Error("NewFactory() returned nil")
	}
	
	// Check if default configs are set
	if len(factory.defaultConfigs) == 0 {
		t.Error("Default configs not set")
	}
	
	// Check specific default config
	proxyConfig, exists := factory.defaultConfigs[ClientTypeProxy]
	if !exists {
		t.Error("Proxy client default config not found")
	}
	
	if proxyConfig.Type != ClientTypeProxy {
		t.Errorf("Expected proxy client type, got %v", proxyConfig.Type)
	}
}

func TestCreateClient(t *testing.T) {
	factory := NewFactory()
	
	config := ClientConfig{
		Type: ClientTypeHealth,
		Timeouts: TimeoutConfig{
			TLSHandshake:   5 * time.Second,
			ResponseHeader: 30 * time.Second,
			IdleConnection: 60 * time.Second,
			OverallRequest: 30 * time.Second,
		},
	}
	
	client, err := factory.CreateClient(config)
	if err != nil {
		t.Errorf("CreateClient() error = %v", err)
	}
	
	if client == nil {
		t.Error("CreateClient() returned nil client")
	}
	
	// Check timeout is set
	if client.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.Timeout)
	}
}

func TestParseTimeoutWithDefault(t *testing.T) {
	testCases := []struct {
		name        string
		value       string
		fieldName   string
		defaultVal  time.Duration
		expected    time.Duration
		expectError bool
	}{
		{
			name:        "valid duration",
			value:       "10s",
			fieldName:   "test",
			defaultVal:  5 * time.Second,
			expected:    10 * time.Second,
			expectError: false,
		},
		{
			name:        "empty string uses default",
			value:       "",
			fieldName:   "test",
			defaultVal:  5 * time.Second,
			expected:    5 * time.Second,
			expectError: false,
		},
		{
			name:        "invalid duration",
			value:       "invalid",
			fieldName:   "test",
			defaultVal:  5 * time.Second,
			expected:    0,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseTimeoutWithDefault(tc.value, tc.fieldName, tc.defaultVal)
			
			if (err != nil) != tc.expectError {
				t.Errorf("ParseTimeoutWithDefault() error = %v, expectError %v", err, tc.expectError)
				return
			}
			
			if !tc.expectError && result != tc.expected {
				t.Errorf("ParseTimeoutWithDefault() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestGetManager(t *testing.T) {
	manager1 := GetManager()
	manager2 := GetManager()
	
	if manager1 != manager2 {
		t.Error("GetManager() should return the same instance (singleton)")
	}
	
	if manager1.factory == nil {
		t.Error("Manager factory not initialized")
	}
}

func TestManager_GetClients(t *testing.T) {
	manager := GetManager()
	
	// Test getting clients before initialization (should auto-initialize)
	proxyClient := manager.GetProxyClient()
	if proxyClient == nil {
		t.Error("GetProxyClient() returned nil")
	}
	
	healthClient := manager.GetHealthClient()
	if healthClient == nil {
		t.Error("GetHealthClient() returned nil")
	}
	
	// They should be different clients
	if proxyClient == healthClient {
		t.Error("Proxy and health clients should be different instances")
	}
}