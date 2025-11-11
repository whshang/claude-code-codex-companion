package errors

import (
	"errors"
	"testing"
)

func TestNewProxyError(t *testing.T) {
	err := NewProxyError(ErrorTypeNetwork, "test_code", "test message")
	
	if err.Type != ErrorTypeNetwork {
		t.Errorf("Expected type %v, got %v", ErrorTypeNetwork, err.Type)
	}
	
	if err.Code != "test_code" {
		t.Errorf("Expected code 'test_code', got '%s'", err.Code)
	}
	
	if err.Message != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", err.Message)
	}
	
	if err.Timestamp.IsZero() {
		t.Error("Timestamp should be set")
	}
}

func TestWrapError(t *testing.T) {
	cause := errors.New("original error")
	err := WrapError(ErrorTypeInternal, "wrap_test", "wrapped message", cause)
	
	if err.Cause != cause {
		t.Error("Cause not set correctly")
	}
	
	// Test Unwrap
	if errors.Unwrap(err) != cause {
		t.Error("Unwrap() doesn't return original error")
	}
	
	// Test error message includes cause
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Error message is empty")
	}
}

func TestProxyError_WithContext(t *testing.T) {
	err := NewProxyError(ErrorTypeConfig, "test", "test message")
	
	err.WithContext("key1", "value1")
	err.WithContext("key2", 42)
	
	if len(err.Context) != 2 {
		t.Errorf("Expected 2 context items, got %d", len(err.Context))
	}
	
	if err.Context["key1"] != "value1" {
		t.Errorf("Expected context key1='value1', got %v", err.Context["key1"])
	}
	
	if err.Context["key2"] != 42 {
		t.Errorf("Expected context key2=42, got %v", err.Context["key2"])
	}
}

func TestProxyError_WithComponent(t *testing.T) {
	err := NewProxyError(ErrorTypeInternal, "test", "test message")
	err.WithComponent("test_component")
	
	if err.Component != "test_component" {
		t.Errorf("Expected component 'test_component', got '%s'", err.Component)
	}
}

func TestProxyError_WithOperation(t *testing.T) {
	err := NewProxyError(ErrorTypeInternal, "test", "test message")
	err.WithOperation("test_operation")
	
	if err.Operation != "test_operation" {
		t.Errorf("Expected operation 'test_operation', got '%s'", err.Operation)
	}
}

func TestProxyError_Is(t *testing.T) {
	err1 := NewProxyError(ErrorTypeNetwork, "timeout", "timeout occurred")
	err2 := NewProxyError(ErrorTypeNetwork, "timeout", "different message")
	err3 := NewProxyError(ErrorTypeNetwork, "connection", "connection failed")
	
	if !errors.Is(err1, err2) {
		t.Error("Errors with same type and code should be equal")
	}
	
	if errors.Is(err1, err3) {
		t.Error("Errors with different codes should not be equal")
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test network error
	netErr := NewNetworkError("dns_error", "DNS resolution failed", nil)
	if netErr.Type != ErrorTypeNetwork {
		t.Error("NewNetworkError should create network type error")
	}
	
	// Test timeout error
	timeoutErr := NewTimeoutError("http_request", nil)
	if timeoutErr.Type != ErrorTypeTimeout {
		t.Error("NewTimeoutError should create timeout type error")
	}
	if timeoutErr.Operation != "http_request" {
		t.Error("NewTimeoutError should set operation")
	}
	
	// Test config error
	configErr := NewConfigError("server.port", "invalid port")
	if configErr.Type != ErrorTypeConfig {
		t.Error("NewConfigError should create config type error")
	}
	if configErr.Context["field"] != "server.port" {
		t.Error("NewConfigError should set field context")
	}
}