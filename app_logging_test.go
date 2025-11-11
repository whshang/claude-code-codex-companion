package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	"claude-code-codex-companion/internal/config"
	"claude-code-codex-companion/internal/endpoint"
	"claude-code-codex-companion/internal/health"
	logger "claude-code-codex-companion/internal/logger"

	_ "modernc.org/sqlite"
)

func TestLogEndpointTestResult(t *testing.T) {
	tempDir := t.TempDir()

	logCfg := logger.LogConfig{
		Level:           "info",
		LogRequestTypes: "all",
		LogRequestBody:  "full",
		LogResponseBody: "full",
		LogDirectory:    tempDir,
	}

	log, err := logger.NewLogger(logCfg)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	app := &App{
		requestLogger: log,
	}

	endpointConfig := config.EndpointConfig{
		Name:         "test-endpoint",
		URLAnthropic: "https://example.com/test",
		AuthType:     "api_key",
		AuthValue:    "dummy",
		Enabled:      true,
		Priority:     1,
		Tags:         []string{"test", "healthcheck"},
	}

	testEndpoint := endpoint.NewEndpoint(endpointConfig)
	testEndpoint.ID = "test-endpoint-id"

	result := &health.HealthCheckResult{
		URL:             endpointConfig.URLAnthropic,
		Method:          "POST",
		StatusCode:      503,
		RequestHeaders:  map[string]string{"Content-Type": "application/json"},
		ResponseHeaders: map[string]string{"Content-Type": "application/json"},
	}

	requestID, requestLog := app.logEndpointTestResult(testEndpoint, result, nil, result.URL)
	if requestID == "" {
		t.Fatal("expected non-empty request ID from logEndpointTestResult")
	}
	if requestLog == nil {
		t.Fatal("expected request log metadata from logEndpointTestResult")
	}

	dbPath := filepath.Join(tempDir, "logs.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("failed to open logs db: %v", err)
	}
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM request_logs").Scan(&count); err != nil {
		t.Fatalf("failed to count request logs: %v", err)
	}

	if count != 1 {
		t.Fatalf("expected exactly one log entry, got %d", count)
	}
}
