// ==================================================================================
//
// DEPRECATED: This file contains the ConversionManager, which was designed to
// dynamically switch between 'legacy' and 'unified' conversion pipelines.
// As of the October 2025 refactor, this dynamic behavior has been removed in favor
// of static, route-specific adapters. The ConversionManager is no longer used by
// the main proxy logic and is slated for removal.
// See internal/proxy/server.go for the new static handler implementations.
//
// ==================================================================================
package conversion

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"claude-code-codex-companion/internal/logger"

	"github.com/sirupsen/logrus"
)

// ConversionMode represents runtime mode for format conversion
type ConversionMode string

const (
	ConversionModeLegacy  ConversionMode = "legacy"
	ConversionModeUnified ConversionMode = "unified"
	ConversionModeAuto    ConversionMode = "auto"
)

const (
	defaultFailbackThreshold = 30
	minSampleSize            = 10
)

// ManagerConfig carries runtime options for the conversion manager
type ManagerConfig struct {
	Mode              ConversionMode
	ValidateSwitch    bool
	FailbackThreshold int
}

// ConversionStats exposes runtime statistics for observability
type ConversionStats struct {
	ConfiguredMode     string     `json:"configured_mode"`
	EffectiveMode      string     `json:"effective_mode"`
	UnifiedSuccess     int        `json:"unified_success"`
	UnifiedFailure     int        `json:"unified_failure"`
	LegacySuccess      int        `json:"legacy_success"`
	LegacyFailure      int        `json:"legacy_failure"`
	LastFallbackAt     *time.Time `json:"last_fallback_at,omitempty"`
	LastFallbackReason string     `json:"last_fallback_reason,omitempty"`
}

// ConversionManager coordinates legacy/unified conversion flows with failback
type ConversionManager struct {
	logger *logger.Logger

	mu                sync.RWMutex
	configuredMode    ConversionMode
	effectiveMode     ConversionMode
	failbackThreshold int
	validateSwitch    bool
	stats             ConversionStats
}

// NewConversionManager creates a conversion manager with the provided configuration
func NewConversionManager(log *logger.Logger, cfg ManagerConfig) *ConversionManager {
	mode := normalizeMode(cfg.Mode)
	if mode == "" {
		mode = ConversionModeAuto
	}

	failback := cfg.FailbackThreshold
	if failback <= 0 {
		failback = defaultFailbackThreshold
	}

	manager := &ConversionManager{
		logger:            log,
		configuredMode:    mode,
		failbackThreshold: failback,
		validateSwitch:    cfg.ValidateSwitch,
	}

	manager.resetStatsLocked()
	manager.stats.ConfiguredMode = string(mode)
	manager.stats.EffectiveMode = string(manager.effectiveMode)

	return manager
}

// Convert executes a conversion operation using the configured mode with optional fallback
func (cm *ConversionManager) Convert(operation string, endpoint string, unified func() ([]byte, error), legacy func() ([]byte, error)) ([]byte, ConversionMode, error) {
	if cm == nil {
		// No manager configured, always run unified path
		result, err := unified()
		return result, ConversionModeUnified, err
	}

	cm.mu.RLock()
	currentMode := cm.effectiveMode
	legacyAvailable := legacy != nil
	cm.mu.RUnlock()

	// If effective mode is legacy but we do not have legacy implementation, force unified path
	if currentMode == ConversionModeLegacy && !legacyAvailable {
		currentMode = ConversionModeUnified
	}

	if currentMode == ConversionModeUnified {
		result, err := unified()
		fallbackTriggered := cm.recordUnifiedResult(err == nil, legacyAvailable, operation, endpoint, err)
		if err == nil {
			return result, ConversionModeUnified, nil
		}

		if fallbackTriggered && legacyAvailable {
			legacyResult, legacyErr := legacy()
			cm.recordLegacyResult(legacyErr == nil, operation, endpoint, legacyErr)
			return legacyResult, ConversionModeLegacy, legacyErr
		}

		// unified failed but fallback not triggered or legacy unavailable
		return result, cm.getEffectiveMode(), err
	}

	// Legacy path
	if legacyAvailable {
		result, err := legacy()
		cm.recordLegacyResult(err == nil, operation, endpoint, err)
		return result, ConversionModeLegacy, err
	}

	// No legacy implementation, fallback to unified
	result, err := unified()
	cm.recordUnifiedResult(err == nil, false, operation, endpoint, err)
	return result, ConversionModeUnified, err
}

// ConvertStream executes a streaming conversion operation using the configured mode with optional fallback
// unified and legacy are functions that perform the actual streaming conversion
func (cm *ConversionManager) ConvertStream(operation string, endpoint string, r io.Reader, w io.Writer, unified func(io.Reader, io.Writer) error, legacy func(io.Reader, io.Writer) error) (ConversionMode, error) {
	if cm == nil {
		// No manager configured, always run unified path
		err := unified(r, w)
		return ConversionModeUnified, err
	}

	cm.mu.RLock()
	currentMode := cm.effectiveMode
	legacyAvailable := legacy != nil
	cm.mu.RUnlock()

	// If effective mode is legacy but we do not have legacy implementation, force unified path
	if currentMode == ConversionModeLegacy && !legacyAvailable {
		currentMode = ConversionModeUnified
	}

	if currentMode == ConversionModeUnified {
		err := unified(r, w)
		fallbackTriggered := cm.recordUnifiedResult(err == nil, legacyAvailable, operation, endpoint, err)
		if err == nil {
			return ConversionModeUnified, nil
		}

		// For streaming, we cannot fallback after starting to write
		// So we just return the error
		if fallbackTriggered {
			cm.logger.Info("Streaming conversion failed but cannot fallback (already started writing)", logrus.Fields{
				"operation": operation,
				"endpoint":  endpoint,
				"error":     err,
			})
		}

		return cm.getEffectiveMode(), err
	}

	// Legacy path
	if legacyAvailable {
		err := legacy(r, w)
		cm.recordLegacyResult(err == nil, operation, endpoint, err)
		return ConversionModeLegacy, err
	}

	// No legacy implementation, fallback to unified
	err := unified(r, w)
	cm.recordUnifiedResult(err == nil, false, operation, endpoint, err)
	return ConversionModeUnified, err
}

// GetConfiguredMode returns the configured mode
func (cm *ConversionManager) GetConfiguredMode() ConversionMode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.configuredMode
}

// GetEffectiveMode returns the effective runtime mode
func (cm *ConversionManager) GetEffectiveMode() ConversionMode {
	return cm.getEffectiveMode()
}

// GetStats returns a snapshot of runtime statistics
func (cm *ConversionManager) GetStats() ConversionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	statsCopy := cm.stats
	if cm.stats.LastFallbackAt != nil {
		t := *cm.stats.LastFallbackAt
		statsCopy.LastFallbackAt = &t
	}
	return statsCopy
}

// SetMode updates the configured mode; if validateSwitch is true, a quick validation is performed
func (cm *ConversionManager) SetMode(mode ConversionMode, validate bool) error {
	mode = normalizeMode(mode)
	if mode == "" {
		return fmt.Errorf("invalid conversion mode: %s", mode)
	}

	if validate && cm.validateSwitch && mode != ConversionModeLegacy {
		if err := cm.validateUnifiedPipeline(); err != nil {
			return err
		}
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.configuredMode == mode {
		return nil
	}

	cm.configuredMode = mode
	cm.resetStatsLocked()
	cm.stats.ConfiguredMode = string(mode)
	cm.stats.EffectiveMode = string(cm.effectiveMode)
	return nil
}

// ApplyConfig updates runtime configuration without recreating the manager
func (cm *ConversionManager) ApplyConfig(cfg ManagerConfig) error {
	mode := normalizeMode(cfg.Mode)
	if mode == "" {
		mode = cm.configuredMode
	}

	failback := cfg.FailbackThreshold
	if failback <= 0 {
		failback = defaultFailbackThreshold
	}

	cm.mu.Lock()
	cm.failbackThreshold = failback
	cm.validateSwitch = cfg.ValidateSwitch
	cm.mu.Unlock()

	return cm.SetMode(mode, false)
}

func (cm *ConversionManager) recordUnifiedResult(success bool, legacyAvailable bool, operation string, endpoint string, err error) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if success {
		cm.stats.UnifiedSuccess++
		return false
	}

	cm.stats.UnifiedFailure++

	if err != nil && cm.logger != nil {
		cm.logger.Debug("Unified conversion failed", logrus.Fields{
			"operation": operation,
			"endpoint":  endpoint,
			"error":     err.Error(),
		})
	}

	if !legacyAvailable {
		return false
	}

	if cm.configuredMode != ConversionModeAuto || cm.failbackThreshold <= 0 || cm.effectiveMode != ConversionModeUnified {
		return false
	}

	total := cm.stats.UnifiedSuccess + cm.stats.UnifiedFailure
	if total < minSampleSize {
		return false
	}

	failureRate := float64(cm.stats.UnifiedFailure) / float64(total) * 100
	if failureRate < float64(cm.failbackThreshold) {
		return false
	}

	now := time.Now()
	cm.effectiveMode = ConversionModeLegacy
	cm.stats.EffectiveMode = string(cm.effectiveMode)
	cm.stats.LastFallbackAt = &now
	cm.stats.LastFallbackReason = fmt.Sprintf("failure rate %.1f%% >= %d%%", failureRate, cm.failbackThreshold)

	if cm.logger != nil {
		cm.logger.Info("Unified conversion failure threshold reached, switching to legacy mode", logrus.Fields{
			"operation":          operation,
			"endpoint":           endpoint,
			"failure_rate":       fmt.Sprintf("%.1f%%", failureRate),
			"failback_threshold": cm.failbackThreshold,
		})
	}

	return true
}

func (cm *ConversionManager) recordLegacyResult(success bool, operation string, endpoint string, err error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if success {
		cm.stats.LegacySuccess++
	} else {
		cm.stats.LegacyFailure++
		if err != nil && cm.logger != nil {
			cm.logger.Debug("Legacy conversion failed", logrus.Fields{
				"operation": operation,
				"endpoint":  endpoint,
				"error":     err.Error(),
			})
		}
	}
}

func (cm *ConversionManager) resetStatsLocked() {
	cm.stats = ConversionStats{}
	switch cm.configuredMode {
	case ConversionModeLegacy:
		cm.effectiveMode = ConversionModeLegacy
	case ConversionModeUnified:
		cm.effectiveMode = ConversionModeUnified
	default:
		cm.configuredMode = ConversionModeAuto
		cm.effectiveMode = ConversionModeUnified
	}
}

func (cm *ConversionManager) getEffectiveMode() ConversionMode {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.effectiveMode
}

func (cm *ConversionManager) validateUnifiedPipeline() error {
	sampleChatResponse := []byte(`{"id":"chatcmpl-test","object":"chat.completion","model":"gpt-4","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"ok"}}]}`)
	if _, err := ConvertChatResponseJSONToResponses(sampleChatResponse); err != nil {
		return fmt.Errorf("failed to validate unified conversion (chat->responses): %w", err)
	}

	sampleResponsesRequest := []byte(`{"model":"gpt-4","input":[{"role":"user","content":[{"type":"input_text","text":"hi"}]}]}`)
	if _, err := ConvertResponsesRequestJSONToChat(sampleResponsesRequest); err != nil {
		return fmt.Errorf("failed to validate unified conversion (responses->chat): %w", err)
	}

	return nil
}

func normalizeMode(mode ConversionMode) ConversionMode {
	modeStr := strings.ToLower(string(mode))
	switch modeStr {
	case string(ConversionModeLegacy):
		return ConversionModeLegacy
	case string(ConversionModeUnified):
		return ConversionModeUnified
	case string(ConversionModeAuto), "":
		return ConversionModeAuto
	default:
		return ConversionMode(modeStr)
	}
}
