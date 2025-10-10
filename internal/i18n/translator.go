package i18n

import (
	"fmt"
	"regexp"
	"strings"
)

// Translator handles data-t attribute translation processing
type Translator struct {
	// dataPattern matches pattern: data-t="translation_key"
	dataPattern *regexp.Regexp
}

// NewTranslator creates a new translator
func NewTranslator() *Translator {
	// Pattern explanation:
	// data-t="([^"]+)"  - captures the translation key from data-t attribute
	// This pattern will match any element with data-t attribute
	pattern := regexp.MustCompile(`data-t="([^"]+)"`)
	
	return &Translator{
		dataPattern: pattern,
	}
}

// ProcessHTML processes HTML content and replaces data-t translations
func (t *Translator) ProcessHTML(html string, lang Language, getTranslation func(string, Language) string) string {
	if lang == LanguageZhCN {
		// For Chinese (default), return original HTML
		return html
	}
	
	result := html
	
	// Pattern to match: data-t="key">content</tag>
	fullPattern := regexp.MustCompile(`(<[^>]*data-t="([^"]+)"[^>]*>)([^<]*)(</[^>]+>)`)
	matches := fullPattern.FindAllStringSubmatch(result, -1)
	
	// Debug: log pattern matching
	fmt.Printf("Translation processing: lang=%s, matches=%d\n", lang, len(matches))
	
	for _, match := range matches {
		if len(match) >= 5 {
			fullMatch := match[0]      // Full matched string
			openTag := match[1]       // Opening tag with data-t
			translationKey := match[2] // Translation key from data-t
			content := match[3]       // Content between tags
			closeTag := match[4]      // Closing tag
			
			// Get translation using the translation key
			translation := getTranslation(translationKey, lang)
			if translation == translationKey {
				// If no translation found, keep original content
				translation = content
			}
			
			// Replace content while preserving tag structure
			if strings.TrimSpace(content) != "" {
				replacement := openTag + translation + closeTag
				result = strings.Replace(result, fullMatch, replacement, 1)
			}
		}
	}
	
	// Handle self-closing tags with placeholder, alt, title attributes
	selfClosingPattern := regexp.MustCompile(`(<(?:input|img|br|hr|meta)[^>]*data-t="([^"]+)"[^>]*(?:placeholder|alt|title)=")([^"]*)(\"[^>]*>)`)
	selfClosingMatches := selfClosingPattern.FindAllStringSubmatch(result, -1)
	
	for _, match := range selfClosingMatches {
		if len(match) >= 5 {
			fullMatch := match[0]
			beforeAttr := match[1]
			translationKey := match[2]
			originalValue := match[3]
			afterAttr := match[4]
			
			// Get translation
			translation := getTranslation(translationKey, lang)
			if translation == translationKey {
				translation = originalValue
			}
			
			replacement := beforeAttr + translation + afterAttr
			result = strings.Replace(result, fullMatch, replacement, 1)
		}
	}
	
	return result
}

// ExtractTranslations extracts all translatable texts from HTML
func (t *Translator) ExtractTranslations(html string) map[string]string {
	translations := make(map[string]string)
	
	// Pattern to extract data-t keys and their corresponding content
	fullPattern := regexp.MustCompile(`<[^>]*data-t="([^"]+)"[^>]*>([^<]*)</[^>]+>`)
	matches := fullPattern.FindAllStringSubmatch(html, -1)
	
	for _, match := range matches {
		if len(match) >= 3 {
			translationKey := strings.TrimSpace(match[1])
			content := strings.TrimSpace(match[2])
			
			if translationKey != "" && content != "" {
				translations[translationKey] = content
			}
		}
	}
	
	// Also extract from self-closing tags
	selfClosingPattern := regexp.MustCompile(`<(?:input|img|br|hr|meta)[^>]*data-t="([^"]+)"[^>]*(?:placeholder|alt|title)="([^"]*)"[^>]*>`)
	selfClosingMatches := selfClosingPattern.FindAllStringSubmatch(html, -1)
	
	for _, match := range selfClosingMatches {
		if len(match) >= 3 {
			translationKey := strings.TrimSpace(match[1])
			content := strings.TrimSpace(match[2])
			
			if translationKey != "" && content != "" {
				translations[translationKey] = content
			}
		}
	}
	
	return translations
}

// ValidateTranslationMarkers validates data-t translation markers in HTML
func (t *Translator) ValidateTranslationMarkers(html string) []ValidationError {
	var errors []ValidationError
	
	matches := t.dataPattern.FindAllStringSubmatch(html, -1)
	
	for i, match := range matches {
		if len(match) < 2 {
			errors = append(errors, ValidationError{
				Index:   i,
				Message: "Invalid data-t marker format",
				Pattern: match[0],
			})
			continue
		}
		
		translationKey := strings.TrimSpace(match[1])
		
		if translationKey == "" {
			errors = append(errors, ValidationError{
				Index:   i,
				Message: "Empty translation key",
				Pattern: match[0],
			})
		}
	}
	
	return errors
}

// ValidationError represents a validation error in translation markers
type ValidationError struct {
	Index   int    `json:"index"`
	Message string `json:"message"`
	Pattern string `json:"pattern"`
}

// GenerateTranslationTemplate generates a translation template from HTML
func (t *Translator) GenerateTranslationTemplate(html string, targetLang Language) map[string]string {
	// Extract existing translations based on data-t keys and content
	return t.ExtractTranslations(html)
}

// HasTranslationMarkers checks if HTML contains any data-t markers
func (t *Translator) HasTranslationMarkers(html string) bool {
	return t.dataPattern.MatchString(html)
}

// CountTranslationMarkers counts the number of data-t markers in HTML
func (t *Translator) CountTranslationMarkers(html string) int {
	matches := t.dataPattern.FindAllString(html, -1)
	return len(matches)
}