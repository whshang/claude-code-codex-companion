/**
 * Enhanced I18n Translation System for Claude Code Companion
 * Provides comprehensive translation functionality for JavaScript/DOM
 */

(function(global) {
    'use strict';
    
    // Translation system core
    const I18n = {
        // Configuration
        config: {
            defaultLanguage: 'zh-cn',
            currentLanguage: 'zh-cn',
            fallbackEnabled: true,
            debug: false
        },
        
        // Translation data storage
        translations: new Map(),
        
        // Cache for processed translations
        cache: new Map(),
        
        // DOM observer for dynamic content
        observer: null,
        
        // Initialize the translation system
        init: function(options = {}) {
            // Merge configuration
            this.config = Object.assign(this.config, options);
            
            // Detect current language
            this.detectLanguage();
            
            // Load translations
            this.loadTranslations();
            
            // Initialize DOM processing
            this.initDOMProcessing();
            
            // Set up dynamic observation
            this.initObserver();
            
            this.log('I18n system initialized');
        },
        
        // Detect current language from various sources
        detectLanguage: function() {
            // Priority: URL parameter > Cookie > HTML lang > Navigator > Default
            const urlLang = this.getURLParameter('lang');
            const cookieLang = this.getCookie('claude_proxy_lang');
            const htmlLang = document.documentElement.lang;
            const navLang = navigator.language;
            
            if (urlLang && this.isValidLanguage(urlLang)) {
                this.setLanguage(urlLang);
            } else if (cookieLang && this.isValidLanguage(cookieLang)) {
                this.setLanguage(cookieLang);
            } else if (htmlLang && this.isValidLanguage(htmlLang)) {
                this.setLanguage(htmlLang);
            } else if (navLang && this.isValidLanguage(navLang)) {
                this.setLanguage(this.normalizeLanguage(navLang));
            }
        },
        
        // Set current language
        setLanguage: function(lang) {
            if (!this.isValidLanguage(lang)) {
                this.log('Invalid language: ' + lang, 'warn');
                return false;
            }
            
            this.config.currentLanguage = lang;
            this.setCookie('claude_proxy_lang', lang);
            document.documentElement.lang = lang;
            
            // Clear cache when language changes
            this.cache.clear();
            
            // Load translations for the new language
            const loadPromise = this.loadLanguageData(lang);
            
            if (loadPromise && typeof loadPromise.then === 'function') {
                loadPromise.then(() => {
                    // Re-process all elements after translations are loaded
                    this.processAllElements();
                });
            } else {
                // Re-process all elements immediately for default language
                this.processAllElements();
            }
            
            this.log('Language set to: ' + lang);
            return true;
        },
        
        // Get current language
        getLanguage: function() {
            return this.config.currentLanguage;
        },
        
        // Load translations from server
        loadTranslations: function() {
            // Load translation data for current language
            if (!this.translations.has(this.config.currentLanguage)) {
                this.loadLanguageData(this.config.currentLanguage);
            }
        },
        
        // Load language data from server
        loadLanguageData: function(lang) {
            if (this.translations.has(lang)) {
                return Promise.resolve();
            }

            // Create empty map first
            this.translations.set(lang, new Map());

            // Always load from server for all languages (including default)
            return fetch(`/admin/api/translations`)
                .then(response => {
                    if (!response.ok) {
                        throw new Error(`Failed to load translations`);
                    }
                    return response.json();
                })
                .then(data => {
                    // Process the translations data for the specific language
                    if (data[lang]) {
                        const langData = this.translations.get(lang);
                        Object.entries(data[lang]).forEach(([key, value]) => {
                            langData.set(key, value);
                        });
                        this.log(`Loaded ${Object.keys(data[lang]).length} translations for ${lang}`);
                    } else {
                        this.log(`No translations found for ${lang}`, 'warn');
                    }
                })
                .catch(error => {
                    this.log(`Failed to load translations for ${lang}: ${error.message}`, 'warn');
                });
        },
        
        // Core translation functions
        
        // Basic translation function
        T: function(key, fallback = null) {
            return this.translate(key, fallback);
        },
        
        // Formatted translation function
        Tf: function(key, fallback = null, ...args) {
            const translation = this.translate(key, fallback);
            if (args.length === 0) {
                return translation;
            }
            
            // Simple sprintf-like formatting
            return this.sprintf(translation, args);
        },
        
        // Main translation function
        translate: function(key, fallback = null) {
            if (!key) return fallback || '';

            // Check cache first
            const cacheKey = this.config.currentLanguage + ':' + key;
            if (this.cache.has(cacheKey)) {
                return this.cache.get(cacheKey);
            }

            // Get translation
            let translation = null;
            const langData = this.translations.get(this.config.currentLanguage);

            if (langData && langData.has(key)) {
                translation = langData.get(key);
            } else if (this.config.fallbackEnabled && fallback) {
                translation = fallback;
            } else {
                translation = key;
            }

            // Cache the result
            this.cache.set(cacheKey, translation);

            return translation;
        },
        
        // DOM manipulation functions
        
        // Translate a specific element
        translateElement: function(selector, key, fallback = null) {
            const elements = typeof selector === 'string' ? 
                document.querySelectorAll(selector) : [selector];
            
            elements.forEach(element => {
                if (element) {
                    const translation = this.translate(key, fallback);
                    element.textContent = translation;
                    element.setAttribute('data-translated', 'true');
                }
            });
        },
        
        // Translate element attribute
        translateAttribute: function(selector, attribute, key, fallback = null) {
            const elements = typeof selector === 'string' ? 
                document.querySelectorAll(selector) : [selector];
            
            elements.forEach(element => {
                if (element) {
                    const translation = this.translate(key, fallback);
                    element.setAttribute(attribute, translation);
                    element.setAttribute('data-attr-translated', 'true');
                }
            });
        },
        
        // Initialize DOM processing
        initDOMProcessing: function() {
            // Process existing data-t elements
            this.processDataTElements();
            
            // Process HTML text markers
            this.processHTMLTextMarkers();
            
            // Process attribute translations
            this.processAttributeTranslations();
        },
        
        // Process all elements (used when language changes)
        processAllElements: function() {
            this.processDataTElements();
            this.processHTMLTextMarkers();
            this.processAttributeTranslations();
        },
        
        // Process data-t elements
        processDataTElements: function() {
            const elements = document.querySelectorAll('[data-t]');
            elements.forEach(element => {
                const key = element.getAttribute('data-t');
                const fallback = element.getAttribute('data-fallback') || element.textContent;
                
                if (key) {
                    const translation = this.translate(key, fallback);
                    element.textContent = translation;
                }
            });
        },
        
        // Process HTML text markers (<!--T:key-->text<!--/T-->)
        processHTMLTextMarkers: function() {
            // This would be processed server-side in the current implementation
            // But we can handle dynamic insertions
            this.log('HTML text markers processed server-side');
        },
        
        // Process attribute translations (data-t-attribute)
        processAttributeTranslations: function() {
            const pattern = /^data-t-(.+)$/;
            const elements = document.querySelectorAll('*');
            
            elements.forEach(element => {
                Array.from(element.attributes).forEach(attr => {
                    const match = attr.name.match(pattern);
                    if (match) {
                        const attrName = match[1];
                        const key = attr.value;
                        const fallback = element.getAttribute(attrName) || '';
                        
                        if (key) {
                            const translation = this.translate(key, fallback);
                            element.setAttribute(attrName, translation);
                        }
                    }
                });
            });
        },
        
        // Initialize mutation observer for dynamic content
        initObserver: function() {
            if (!window.MutationObserver) return;
            
            this.observer = new MutationObserver((mutations) => {
                mutations.forEach((mutation) => {
                    if (mutation.type === 'childList') {
                        mutation.addedNodes.forEach((node) => {
                            if (node.nodeType === Node.ELEMENT_NODE) {
                                this.processNewElement(node);
                            }
                        });
                    }
                });
            });
            
            this.observer.observe(document.body, {
                childList: true,
                subtree: true
            });
        },
        
        // Process newly added elements
        processNewElement: function(element) {
            // Process data-t elements
            if (element.hasAttribute && element.hasAttribute('data-t')) {
                const key = element.getAttribute('data-t');
                const fallback = element.getAttribute('data-fallback') || element.textContent;
                if (key) {
                    const translation = this.translate(key, fallback);
                    element.textContent = translation;
                }
            }
            
            // Process child elements
            const childElements = element.querySelectorAll('[data-t]');
            childElements.forEach(child => {
                const key = child.getAttribute('data-t');
                const fallback = child.getAttribute('data-fallback') || child.textContent;
                if (key) {
                    const translation = this.translate(key, fallback);
                    child.textContent = translation;
                }
            });
        },
        
        // Utility functions
        
        // Simple sprintf implementation
        sprintf: function(format, args) {
            let i = 0;
            return format.replace(/%[sd%]/g, function(match) {
                if (match === '%%') return '%';
                if (i >= args.length) return match;
                return String(args[i++]);
            });
        },
        
        // Check if language is valid
        isValidLanguage: function(lang) {
            const validLangs = ['zh-cn', 'en', 'de', 'es', 'it', 'ja', 'ko', 'pt', 'ru'];
            return validLangs.includes(lang.toLowerCase());
        },
        
        // Normalize language code
        normalizeLanguage: function(lang) {
            lang = lang.toLowerCase();
            if (lang.startsWith('zh')) return 'zh-cn';
            if (lang.startsWith('en')) return 'en';
            if (lang.startsWith('de')) return 'de';
            if (lang.startsWith('es')) return 'es';
            if (lang.startsWith('it')) return 'it';
            if (lang.startsWith('ja')) return 'ja';
            if (lang.startsWith('ko')) return 'ko';
            if (lang.startsWith('pt')) return 'pt';
            if (lang.startsWith('ru')) return 'ru';
            return this.config.defaultLanguage;
        },
        
        // Get URL parameter
        getURLParameter: function(name) {
            const urlParams = new URLSearchParams(window.location.search);
            return urlParams.get(name);
        },
        
        // Cookie management
        getCookie: function(name) {
            const value = `; ${document.cookie}`;
            const parts = value.split(`; ${name}=`);
            if (parts.length === 2) return parts.pop().split(';').shift();
            return null;
        },
        
        setCookie: function(name, value, days = 365) {
            const date = new Date();
            date.setTime(date.getTime() + (days * 24 * 60 * 60 * 1000));
            const expires = `expires=${date.toUTCString()}`;
            document.cookie = `${name}=${value}; ${expires}; path=/`;
        },
        
        // Logging
        log: function(message, level = 'info') {
            if (this.config.debug) {
                console[level](`[I18n] ${message}`);
            }
        },
        
        // Public API for external use
        addTranslations: function(lang, translations) {
            if (!this.translations.has(lang)) {
                this.translations.set(lang, new Map());
            }
            
            const langData = this.translations.get(lang);
            Object.entries(translations).forEach(([key, value]) => {
                langData.set(key, value);
            });
            
            // Clear cache for this language
            this.cache.forEach((value, key) => {
                if (key.startsWith(lang + ':')) {
                    this.cache.delete(key);
                }
            });
            
            this.log(`Added ${Object.keys(translations).length} translations for ${lang}`);
        },
        
        
        // Process option translations  
        processOptionTranslations: function() {
            const elements = document.querySelectorAll('[data-t-option]');
            elements.forEach(element => {
                const key = element.getAttribute('data-t-option');
                const fallback = element.getAttribute('data-fallback') || element.textContent;
                
                if (key) {
                    const translation = this.translate(key, fallback);
                    element.textContent = translation;
                }
            });
        },
        
        // Process tooltip translations with token replacement
        processTooltipTranslations: function() {
            const elements = document.querySelectorAll('[data-t-tooltip]');
            elements.forEach(element => {
                const key = element.getAttribute('data-t-tooltip');
                const tokens = element.getAttribute('data-tokens');
                
                if (key && tokens) {
                    const template = this.translate(key);
                    const translatedText = template.replace('{{tokens}}', tokens);
                    element.setAttribute('title', translatedText);
                }
            });
        },
        
        // Override processDataTElements to include tooltip and option processing
        processDataTElements: function() {
            // Process regular data-t elements
            const elements = document.querySelectorAll('[data-t]');
            elements.forEach(element => {
                const key = element.getAttribute('data-t');
                const fallback = element.getAttribute('data-fallback') || element.textContent;
                
                if (key) {
                    const translation = this.translate(key, fallback);
                    element.textContent = translation;
                }
            });
            
            // Process option translations
            this.processOptionTranslations();
            
            // Process tooltip translations
            this.processTooltipTranslations();
        },
        
        // Get all translations for debugging
        getAllTranslations: function() {
            const result = {};
            this.translations.forEach((langData, lang) => {
                result[lang] = {};
                langData.forEach((value, key) => {
                    result[lang][key] = value;
                });
            });
            return result;
        }
    };
    
    // Global API exposure
    global.I18n = I18n;
    global.T = I18n.T.bind(I18n);
    global.t = I18n.T.bind(I18n);
    global.Tf = I18n.Tf.bind(I18n);
    global.translateElement = I18n.translateElement.bind(I18n);
    global.translateAttribute = I18n.translateAttribute.bind(I18n);
    
    // Auto-initialize when DOM is ready
    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', function() {
            I18n.init();
        });
    } else {
        I18n.init();
    }
    
})(window);