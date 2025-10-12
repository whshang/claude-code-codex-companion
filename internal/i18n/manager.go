package i18n

import (
    "fmt"
	"encoding/json"
	"io/ioutil"
	"path/filepath"
	"sync"
	"time"

	"claude-code-codex-companion/internal/webres"
)

// TranslationCache implements caching for processed content
type TranslationCache struct {
	// Cache for processed HTML templates
	templateCache map[string]map[Language]string

	// Cache for individual translations
	translationCache map[string]map[Language]string

	// Cache TTL and cleanup
	ttl         time.Duration
	lastCleanup time.Time

	mu sync.RWMutex
}

// NewTranslationCache creates a new translation cache
func NewTranslationCache(ttl time.Duration) *TranslationCache {
	return &TranslationCache{
		templateCache:    make(map[string]map[Language]string),
		translationCache: make(map[string]map[Language]string),
		ttl:              ttl,
		lastCleanup:      time.Now(),
	}
}

// Manager manages internationalization functionality
type Manager struct {
	config         *Config
	detector       *Detector
	translator     *Translator
	processorChain *ProcessorChain
	cache          *TranslationCache
	translations   map[Language]map[string]string
	mu             sync.RWMutex
}

// NewManager creates a new i18n manager
func NewManager(config *Config) (*Manager, error) {
	if config == nil {
		config = DefaultConfig()
	}

	manager := &Manager{
		config:       config,
		detector:     NewDetector(config.DefaultLanguage),
		translator:   NewTranslator(),
		cache:        NewTranslationCache(30 * time.Minute), // 30 minute cache TTL
		translations: make(map[Language]map[string]string),
	}

	// Initialize processor chain after manager is created
	manager.processorChain = NewProcessorChain(manager)

	// Load translation files
	if err := manager.loadTranslations(); err != nil {
		return nil, fmt.Errorf("failed to load translations: %w", err)
	}

	// Set as global manager
	SetGlobalManager(manager)

	return manager, nil
}

// loadTranslations loads all translation files from the locales directory
func (m *Manager) loadTranslations() error {
	if !m.config.Enabled {
		return nil
	}

	supportedLangs := []Language{LanguageEn, LanguageZhCN, LanguageDe, LanguageEs, LanguageIt, LanguageJa, LanguageKo, LanguagePt, LanguageRu}

	for _, lang := range supportedLangs {
		filename := filepath.Join(m.config.LocalesPath, string(lang)+".json")
		translations, err := m.loadTranslationFile(filename)
		if err != nil {
			// Create empty translation map for this language
			m.translations[lang] = make(map[string]string)
			continue
		}

		m.translations[lang] = translations
	}

	return nil
}

// loadTranslationFile loads a single translation file
func (m *Manager) loadTranslationFile(filename string) (map[string]string, error) {
	// Extract just the filename from full path
	baseFilename := filepath.Base(filename)

	// Try to read from embedded assets first
	data, err := webres.ReadLocaleFile(baseFilename)
	if err != nil {
		// Fallback to file system (for backwards compatibility)
		data, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
	}

	// Parse the JSON structure that includes meta and translations
	var fileContent struct {
		Meta struct {
			Version     string `json:"version"`
			Language    string `json:"language"`
			LastUpdated string `json:"last_updated"`
			TotalKeys   int    `json:"total_keys"`
		} `json:"meta"`
		Translations map[string]string `json:"translations"`
	}

	if err := json.Unmarshal(data, &fileContent); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return fileContent.Translations, nil
}

// GetDetector returns the language detector
func (m *Manager) GetDetector() *Detector {
	return m.detector
}

// GetTranslator returns the translator
func (m *Manager) GetTranslator() *Translator {
	return m.translator
}

// IsEnabled returns whether i18n is enabled
func (m *Manager) IsEnabled() bool {
	return m.config.Enabled
}

// GetDefaultLanguage returns the default language
func (m *Manager) GetDefaultLanguage() Language {
	return m.config.DefaultLanguage
}

// GetTranslation gets a translation for the given text and language
func (m *Manager) GetTranslation(text string, lang Language) string {
	if !m.config.Enabled {
		return text
	}

	// If it's the default language, return original text
	if lang == m.config.DefaultLanguage {
		return text
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if langTranslations, exists := m.translations[lang]; exists {
		if translation, found := langTranslations[text]; found {
			return translation
		}
	}

	// Fallback to original text
	return text
}

// ReloadTranslations reloads all translation files
func (m *Manager) ReloadTranslations() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clear existing translations
	m.translations = make(map[Language]map[string]string)

	// Reload translations
	return m.loadTranslations()
}

// AddTranslation adds a new translation dynamically
func (m *Manager) AddTranslation(lang Language, original, translation string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.translations[lang] == nil {
		m.translations[lang] = make(map[string]string)
	}

	m.translations[lang][original] = translation
}

// GetAvailableLanguages returns all available languages
func (m *Manager) GetAvailableLanguages() []Language {
	return []Language{LanguageZhCN, LanguageEn, LanguageDe, LanguageEs, LanguageIt, LanguageJa, LanguageKo, LanguagePt, LanguageRu}
}

// GetLanguageInfo returns display information for a language
func (m *Manager) GetLanguageInfo(lang Language) map[string]string {
	switch lang {
	case LanguageZhCN:
		return map[string]string{"flag": "CN", "name": T("language_chinese_name", "中文")}
	case LanguageEn:
		return map[string]string{"flag": "US", "name": "English"}
	case LanguageDe:
		return map[string]string{"flag": "DE", "name": "Deutsch"}
	case LanguageEs:
		return map[string]string{"flag": "ES", "name": "Español"}
	case LanguageIt:
		return map[string]string{"flag": "IT", "name": "Italiano"}
	case LanguageJa:
		return map[string]string{"flag": "JP", "name": "日本語"}
	case LanguageKo:
		return map[string]string{"flag": "KR", "name": "한국어"}
	case LanguagePt:
		return map[string]string{"flag": "PT", "name": "Português"}
	case LanguageRu:
		return map[string]string{"flag": "RU", "name": "Русский"}
	default:
		return map[string]string{"flag": "??", "name": string(lang)}
	}
}

// GetAllTranslations returns all translations for debugging
func (m *Manager) GetAllTranslations() map[Language]map[string]string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to avoid race conditions
	result := make(map[Language]map[string]string)
	for lang, translations := range m.translations {
		result[lang] = make(map[string]string)
		for key, value := range translations {
			result[lang][key] = value
		}
	}

	return result
}

// Enhanced methods for new processor architecture

// GetProcessorChain returns the processor chain
func (m *Manager) GetProcessorChain() *ProcessorChain {
	return m.processorChain
}

// ProcessContent processes content through the processor chain
func (m *Manager) ProcessContent(content string, lang Language, ctx Context) (string, error) {
	if !m.config.Enabled {
		return content, nil
	}

	// Check cache first
	if cached := m.getCachedContent(content, lang); cached != "" {
		return cached, nil
	}

	// Process through chain
	result, err := m.processorChain.Process(content, lang, ctx)
	if err != nil {
		return content, err
	}

	// Cache the result
	m.setCachedContent(content, lang, result)

	return result, nil
}

// ExtractTranslations extracts all translations from content
func (m *Manager) ExtractTranslations(content string) (map[string]string, error) {
	return m.processorChain.ExtractAll(content)
}

// ValidateContent validates translation markers in content
func (m *Manager) ValidateContent(content string) []ProcessorValidationError {
	return m.processorChain.ValidateAll(content)
}

// Cache management methods

// getCachedContent gets cached processed content
func (m *Manager) getCachedContent(content string, lang Language) string {
	m.cache.mu.RLock()
	defer m.cache.mu.RUnlock()

	if langCache, exists := m.cache.templateCache[content]; exists {
		if result, found := langCache[lang]; found {
			return result
		}
	}

	return ""
}

// setCachedContent sets cached processed content
func (m *Manager) setCachedContent(content string, lang Language, result string) {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	if m.cache.templateCache[content] == nil {
		m.cache.templateCache[content] = make(map[Language]string)
	}

	m.cache.templateCache[content][lang] = result

	// Perform periodic cleanup
	if time.Since(m.cache.lastCleanup) > m.cache.ttl {
		go m.cleanupCache()
	}
}

// cleanupCache performs cache cleanup
func (m *Manager) cleanupCache() {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	// Simple cleanup: clear all cache periodically
	// In production, implement proper LRU or TTL-based cleanup
	if time.Since(m.cache.lastCleanup) > m.cache.ttl*2 {
		m.cache.templateCache = make(map[string]map[Language]string)
		m.cache.translationCache = make(map[string]map[Language]string)
		m.cache.lastCleanup = time.Now()
	}
}

// ClearCache clears all caches
func (m *Manager) ClearCache() {
	m.cache.mu.Lock()
	defer m.cache.mu.Unlock()

	m.cache.templateCache = make(map[string]map[Language]string)
	m.cache.translationCache = make(map[string]map[Language]string)
	m.cache.lastCleanup = time.Now()
}

// Advanced translation methods

// GetTranslationWithKey gets translation by specific key (for new translation system)
func (m *Manager) GetTranslationWithKey(key string, lang Language) string {
	if !m.config.Enabled {
		return key
	}

	// If it's the default language, return key as-is for lookup
	if lang == m.config.DefaultLanguage {
		return key
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if langTranslations, exists := m.translations[lang]; exists {
		if translation, found := langTranslations[key]; found {
			return translation
		}
	}

	// Return key if no translation found
	return key
}
