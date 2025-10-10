package i18n

import (
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	// Default values for i18n parameters
	LangQueryParam = "lang"
	LangCookieName = "claude_proxy_lang"  
	LangHeaderName = "Accept-Language"
)

// Detector handles language detection from various sources
type Detector struct {
	defaultLang Language
}

// NewDetector creates a new language detector
func NewDetector(defaultLang Language) *Detector {
	return &Detector{
		defaultLang: defaultLang,
	}
}

// DetectLanguage detects user's preferred language from request
// Priority: URL parameter > Cookie > Accept-Language header > Default
func (d *Detector) DetectLanguage(c *gin.Context) Language {
	// 1. Check URL parameter
	if langParam := c.Query(LangQueryParam); langParam != "" {
		if IsValidLanguage(langParam) {
			// Set cookie for future requests
			d.setLanguageCookie(c, Language(langParam))
			return Language(langParam)
		}
	}
	
	// 2. Check cookie
	if cookie, err := c.Cookie(LangCookieName); err == nil {
		if IsValidLanguage(cookie) {
			return Language(cookie)
		}
	}
	
	// 3. Check Accept-Language header
	if acceptLang := c.GetHeader(LangHeaderName); acceptLang != "" {
		if lang := d.parseAcceptLanguage(acceptLang); lang != "" {
			d.setLanguageCookie(c, lang)
			return lang
		}
	}
	
	// 4. Return default language
	return d.defaultLang
}

// parseAcceptLanguage parses Accept-Language header and returns the best match
func (d *Detector) parseAcceptLanguage(acceptLang string) Language {
	// Parse Accept-Language header (e.g., "en-US,en;q=0.9,zh-CN;q=0.8,zh;q=0.7")
	languages := strings.Split(acceptLang, ",")
	
	for _, lang := range languages {
		// Remove quality factor (e.g., "en;q=0.9" -> "en")
		lang = strings.TrimSpace(strings.Split(lang, ";")[0])
		
		// Normalize language code
		switch {
		case strings.HasPrefix(strings.ToLower(lang), "zh-cn"), strings.HasPrefix(strings.ToLower(lang), "zh_cn"):
			return LanguageZhCN
		case strings.HasPrefix(strings.ToLower(lang), "zh"):
			return LanguageZhCN
		case strings.HasPrefix(strings.ToLower(lang), "en"):
			return LanguageEn
		case strings.HasPrefix(strings.ToLower(lang), "de"):
			return LanguageDe
		case strings.HasPrefix(strings.ToLower(lang), "es"):
			return LanguageEs
		case strings.HasPrefix(strings.ToLower(lang), "it"):
			return LanguageIt
		case strings.HasPrefix(strings.ToLower(lang), "ja"):
			return LanguageJa
		case strings.HasPrefix(strings.ToLower(lang), "ko"):
			return LanguageKo
		case strings.HasPrefix(strings.ToLower(lang), "pt"):
			return LanguagePt
		case strings.HasPrefix(strings.ToLower(lang), "ru"):
			return LanguageRu
		default:
			continue
		}
	}
	
	return ""
}

// setLanguageCookie sets the language preference cookie
func (d *Detector) setLanguageCookie(c *gin.Context, lang Language) {
	c.SetCookie(LangCookieName, string(lang), 86400*365, "/", "", false, false) // 1 year
}

// GetLanguageFromContext gets the current language from gin context
func GetLanguageFromContext(c *gin.Context) Language {
	if lang, exists := c.Get("current_language"); exists {
		if l, ok := lang.(Language); ok {
			return l
		}
	}
	return LanguageZhCN // fallback
}

// SetLanguageToContext sets the current language to gin context
func SetLanguageToContext(c *gin.Context, lang Language) {
	c.Set("current_language", lang)
}