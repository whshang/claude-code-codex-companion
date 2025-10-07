/* Claude Code Companion Web Admin - Shared JavaScript Functions */

// Style utility functions for refactored code
window.StyleUtils = {
    // Display control utilities
    show: function(element) {
        if (element) {
            element.classList.remove('d-none-custom', 'hidden');
            element.classList.add('d-block-custom');
        }
    },
    
    hide: function(element) {
        if (element) {
            element.classList.remove('d-block-custom');
            element.classList.add('d-none-custom');
        }
    },
    
    toggle: function(element) {
        if (element) {
            if (element.classList.contains('d-none-custom')) {
                this.show(element);
            } else {
                this.hide(element);
            }
        }
    },
    
    // Check if element is hidden
    isHidden: function(element) {
        if (!element) return true;
        return element.classList.contains('d-none-custom') || 
               element.classList.contains('hidden') ||
               window.getComputedStyle(element).display === 'none';
    },
    
    // Cursor utilities
    setCursorGrabbing: function(enabled) {
        if (enabled) {
            document.body.classList.add('cursor-grabbing');
        } else {
            document.body.classList.remove('cursor-grabbing');
        }
    },
    
    // Flag styling utilities
    setFlagStyle: function(element, region) {
        if (!element) return;
        
        // Remove all existing flag classes
        element.classList.remove('bg-flag-us', 'bg-flag-cn', 'bg-flag-jp', 'bg-flag-unknown');
        
        // Add appropriate class based on region
        const regionClass = 'bg-flag-' + (region || 'unknown').toLowerCase();
        element.classList.add(regionClass);
    },
    
    // Position utilities
    hideOffscreen: function(element) {
        if (element) {
            element.classList.add('position-hidden');
        }
    },
    
    showOnscreen: function(element) {
        if (element) {
            element.classList.remove('position-hidden');
        }
    },
    
    // Toast positioning utility
    positionToast: function(element) {
        if (element) {
            element.classList.add('toast-position');
        }
    }
};

// CSRF token management
let csrfToken = null;

// Get CSRF token from server
async function getCSRFToken() {
    if (csrfToken) {
        return csrfToken;
    }
    
    try {
        const response = await fetch('/admin/api/csrf-token');
        if (response.ok) {
            const data = await response.json();
            csrfToken = data.csrf_token;
            return csrfToken;
        }
    } catch (error) {
        console.error('Failed to get CSRF token:', error);
    }
    
    return null;
}

// Enhanced API request function with CSRF protection
async function apiRequest(url, options = {}) {
    const token = await getCSRFToken();
    
    const headers = {
        'Content-Type': 'application/json',
        ...options.headers
    };
    
    // Add CSRF token for non-GET requests
    if (options.method && options.method !== 'GET') {
        if (token) {
            headers['X-CSRF-Token'] = token;
        }
    }
    
    const requestOptions = {
        ...options,
        headers
    };
    
    try {
        const response = await fetch(url, requestOptions);
        
        // If CSRF token is invalid, clear cached token for next time
        if (response.status === 403) {
            const errorData = await response.json().catch(() => ({}));
            if (errorData.code === 'CSRF_INVALID') {
                csrfToken = null; // Clear cached token for future requests
            }
        }
        
        return response;
    } catch (error) {
        console.error('API request failed:', error);
        throw error;
    }
}

// Shared utility functions - Enhanced with translation support
function showAlert(message, type = 'info') {
    // Try to translate the message if T function is available
    let translatedMessage = message;
    if (window.T && typeof message === 'string') {
        // Check if message looks like a translation key (no spaces, underscore separated)
        if (!message.includes(' ') && message.includes('_')) {
            translatedMessage = window.T(message, message);
        }
    }
    
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type} alert-dismissible fade show alert-positioned`;
    alertDiv.innerHTML = `
        ${translatedMessage}
        <button type="button" class="btn-close" data-bs-dismiss="alert"></button>
    `;
    
    document.body.appendChild(alertDiv);
    
    // Auto dismiss after 3 seconds
    setTimeout(() => {
        if (alertDiv.parentNode) {
            alertDiv.remove();
        }
    }, 3000);
}

// Format utilities
function formatDuration(ms) {
    return (ms / 1000).toFixed(3) + 's';
}

function formatFileSize(bytes) {
    if (bytes === 0) return '0B';
    if (bytes < 1024) return bytes + 'B';
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + 'K';
    return (bytes / (1024 * 1024)).toFixed(1) + 'M';
}

function formatJson(jsonString) {
    if (!jsonString) return jsonString;
    try {
        const parsed = JSON.parse(jsonString);
        return JSON.stringify(parsed, null, 2);
    } catch {
        return jsonString;
    }
}

// Extract domain from URL
function extractDomain(url) {
    try {
        const urlObj = new URL(url);
        return urlObj.hostname;
    } catch (e) {
        return url; // If not a valid URL, return original content
    }
}

// Truncate domain if exceeds maxLength characters and add ellipsis
function truncateDomain(domain, maxLength = 25) {
    if (!domain || domain.length <= maxLength) {
        return domain;
    }
    return domain.substring(0, maxLength) + '...';
}

// Format URL display: show full URL, truncate if over 60 chars
function formatUrlDisplay(url) {
    const maxLength = 60;
    let displayUrl = url;

    // If URL is too long, truncate middle part
    if (url && url.length > maxLength) {
        const start = url.substring(0, Math.floor(maxLength * 0.4));
        const end = url.substring(url.length - Math.floor(maxLength * 0.6));
        displayUrl = start + '...' + end;
    }

    return {
        display: displayUrl,
        title: url
    };
}

function escapeHtml(text) {
    if (!text) return text;
    // 保持中文字符不变，只转义必要的HTML字符
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

// UTF-8 safe base64 encoding/decoding
function safeBase64Encode(str) {
    try {
        return btoa(encodeURIComponent(str).replace(/%([0-9A-F]{2})/g, function(match, p1) {
            return String.fromCharCode('0x' + p1);
        }));
    } catch (error) {
        console.warn('Base64编码失败，使用备用方法:', error);
        return encodeURIComponent(str);
    }
}

function safeBase64Decode(str) {
    try {
        const decoded = atob(str);
        return decodeURIComponent(Array.prototype.map.call(decoded, function(c) {
            return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
        }).join(''));
    } catch (error) {
        console.warn('Base64解码失败，使用备用方法:', error);
        try {
            return decodeURIComponent(str);
        } catch (e) {
            console.error('All decode methods failed:', e);
            return str;
        }
    }
}

// File utilities
function getFileExtension(content) {
    if (!content) return 'txt';
    
    try {
        JSON.parse(content);
        return 'json';
    } catch {
        if (content.includes('event: ') && content.includes('data: ')) {
            return 'sse';
        }
        return 'txt';
    }
}

function saveAsFileFromButton(button) {
    const filename = button.getAttribute('data-filename');
    const encodedContent = button.getAttribute('data-content');
    
    if (!encodedContent || encodedContent.trim() === '') {
        alert(T('content_empty_cannot_save', '内容为空，无法保存'));
        return;
    }

    try {
        const content = safeBase64Decode(encodedContent);
        
        const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
        const url = URL.createObjectURL(blob);
        
        const downloadLink = document.createElement('a');
        downloadLink.href = url;
        downloadLink.download = filename;
        StyleUtils.hide(downloadLink);
        
        document.body.appendChild(downloadLink);
        downloadLink.click();
        
        setTimeout(() => {
            document.body.removeChild(downloadLink);
            URL.revokeObjectURL(url);
        }, 100);
    } catch (error) {
        console.error('Save file failed:', error);
        alert(T('save_file_failed_check_console', '保存文件失败，请检查浏览器控制台'));
    }
}

// Copy to clipboard functionality
function copyToClipboard(content) {
    if (!content || content.trim() === '') {
        showAlert(T('content_empty_cannot_copy', '内容为空，无法复制'), 'warning');
        return;
    }

    // Try to use modern clipboard API first
    if (navigator.clipboard && window.isSecureContext) {
        navigator.clipboard.writeText(content).then(() => {
            showAlert(T('copied_to_clipboard', '已复制到剪贴板'), 'success');
        }).catch(err => {
            console.error('Clipboard API failed:', err);
            fallbackCopyToClipboard(content);
        });
    } else {
        // Fallback for older browsers or non-secure contexts
        fallbackCopyToClipboard(content);
    }
}

// Copy request ID to clipboard
function copyRequestId(requestId) {
    copyToClipboard(requestId);
}

// Check if response body is from Anthropic API
function isAnthropicResponse(responseBody) {
    if (!responseBody) return false;
    
    try {
        // 尝试解码 base64
        let decodedBody = responseBody;
        try {
            decodedBody = safeBase64Decode(responseBody);
        } catch (e) {
            // 如果不是 base64，就使用原始字符串
        }
        
        // 检查非流式响应
        try {
            const data = JSON.parse(decodedBody);
            return data.type === 'message' && data.role === 'assistant';
        } catch {
            // 检查流式响应（SSE 格式）
            return decodedBody.includes('event: message_start') && 
                   decodedBody.includes('data: {"type"');
        }
    } catch (error) {
        console.error('Error checking if response is Anthropic:', error);
        return false;
    }
}

function fallbackCopyToClipboard(content) {
    try {
        // Create a temporary textarea element
        const textarea = document.createElement('textarea');
        textarea.value = content;
        StyleUtils.hideOffscreen(textarea);
        document.body.appendChild(textarea);
        
        // Select and copy
        textarea.select();
        textarea.setSelectionRange(0, 99999);
        const successful = document.execCommand('copy');
        
        document.body.removeChild(textarea);
        
        if (successful) {
            showAlert(T('copied_to_clipboard', '已复制到剪贴板'), 'success');
        } else {
            showAlert(T('copy_failed_manual', '复制失败，请手动复制'), 'danger');
        }
    } catch (err) {
        console.error('Fallback copy failed:', err);
        showAlert(T('copy_failed_manual', '复制失败，请手动复制'), 'danger');
    }
}

function copyFromButton(button) {
    const encodedContent = button.getAttribute('data-content');
    if (!encodedContent) {
        showAlert(T('no_content_to_copy', '无内容可复制'), 'warning');
        return;
    }
    
    try {
        const content = safeBase64Decode(encodedContent);
        copyToClipboard(content);
    } catch (error) {
        console.error('Decode content failed:', error);
        showAlert(T('content_decode_failed', '内容解码失败'), 'danger');
    }
}

// Bootstrap tooltip initialization helper
function initializeTooltips(container = document) {
    const tooltipTriggerList = [].slice.call(container.querySelectorAll('[data-bs-toggle="tooltip"]'));
    return tooltipTriggerList.map(function (tooltipTriggerEl) {
        return new bootstrap.Tooltip(tooltipTriggerEl);
    });
}

// Language switching functionality - Simple page reload approach
function switchLanguage(lang) {
    // Set language cookie
    const expiryDate = new Date();
    expiryDate.setFullYear(expiryDate.getFullYear() + 1);
    document.cookie = `claude_proxy_lang=${lang}; expires=${expiryDate.toUTCString()}; path=/`;
    
    // Update URL parameter and reload page
    const url = new URL(window.location);
    url.searchParams.set('lang', lang);
    window.location.href = url.toString();
}

function updateLanguageDropdown() {
    // Get language data from data attributes
    const languageDataElement = document.getElementById('languageData');
    let availableLanguages = {};
    let currentLang = null;
    
    if (languageDataElement) {
        try {
            const languageDataStr = languageDataElement.getAttribute('data-available-languages');
            if (languageDataStr) {
                availableLanguages = JSON.parse('{' + languageDataStr + '}');
            }
            currentLang = languageDataElement.getAttribute('data-current-language');
        } catch (error) {
            console.warn('Failed to parse language data from attributes:', error);
        }
    }
    
    // Fallback to global variable if data attributes are not available
    if (Object.keys(availableLanguages).length === 0 && window.availableLanguages) {
        availableLanguages = window.availableLanguages;
    }
    
    // Get current language from I18n system if available
    if (window.I18n && window.I18n.getLanguage && !currentLang) {
        currentLang = window.I18n.getLanguage();
    }
    
    // Get current language from dropdown data attribute first, then fallback to other methods
    const dropdownElement = document.getElementById('languageDropdown');
    if (!currentLang && dropdownElement) {
        currentLang = dropdownElement.getAttribute('data-current-lang');
    }
    
    if (!currentLang) {
        // Fallback: get from URL parameter
        currentLang = new URLSearchParams(window.location.search).get('lang');
    }
    
    if (!currentLang) {
        // Fallback: get from cookie
        const cookies = document.cookie.split(';');
        for (let cookie of cookies) {
            const [name, value] = cookie.trim().split('=');
            if (name === 'claude_proxy_lang') {
                currentLang = value;
                break;
            }
        }
    }
    
    // Default to zh-cn if no language found
    if (!currentLang) {
        currentLang = 'zh-cn';
    }
    
    // Update dropdown display
    const flagElement = document.getElementById('currentLanguageFlag');
    const textElement = document.getElementById('currentLanguageText');
    
    if (flagElement && textElement && Object.keys(availableLanguages).length > 0) {
        const langInfo = availableLanguages[currentLang];
        if (langInfo) {
            flagElement.textContent = langInfo.flag;
            textElement.textContent = langInfo.name;
            flagElement.classList.add('d-inline-block-custom');
            
            // Set flag color based on language
            switch (currentLang) {
                case 'en':
                    StyleUtils.setFlagStyle(flagElement, 'us');
                    break;
                case 'zh-cn':
                default:
                    StyleUtils.setFlagStyle(flagElement, 'cn');
                    break;
            }
        } else {
            // Fallback for unknown language
            flagElement.textContent = '??';
            textElement.textContent = currentLang;
            StyleUtils.setFlagStyle(flagElement, 'unknown');
            flagElement.classList.add('d-inline-block-custom');
        }
    }
}

// Version update check functionality
let versionCheckInterval = null;

// Parse version string and extract date
function parseVersionDate(version) {
    if (!version) return null;
    
    // Version format: 日期-shorthash-release or just 日期-shorthash
    // Example: 20250816-39e4794-dirty or v0.1.0-test
    
    // Remove 'v' prefix if present
    const cleanVersion = version.startsWith('v') ? version.substring(1) : version;
    
    // Split by '-' and get the first part (should be the date)
    const parts = cleanVersion.split('-');
    if (parts.length === 0) return null;
    
    const datePart = parts[0];
    
    // Check if it's a date format (YYYYMMDD)
    if (!/^\d{8}$/.test(datePart)) {
        return null; // Not a date format, skip comparison
    }
    
    try {
        // Parse YYYYMMDD format
        const year = parseInt(datePart.substring(0, 4));
        const month = parseInt(datePart.substring(4, 6)) - 1; // Month is 0-based
        const day = parseInt(datePart.substring(6, 8));
        
        return new Date(year, month, day);
    } catch (error) {
        console.warn('Failed to parse version date:', datePart, error);
        return null;
    }
}

// Determine if update should be shown based on version comparison
function shouldShowUpdate(currentVersion, latestVersion) {
    // If versions are identical, no update needed
    if (currentVersion === latestVersion) {
        return false;
    }
    
    // Try to parse dates from both versions
    const currentDate = parseVersionDate(currentVersion);
    const latestDate = parseVersionDate(latestVersion);
    
    // If either version doesn't have a valid date, fall back to string comparison
    if (!currentDate || !latestDate) {
        return currentVersion !== latestVersion;
    }
    
    // Only show update if latest version date is newer than current version date
    return latestDate > currentDate;
}

async function checkForUpdates() {
    try {
        // Use fetch with timeout and enhanced error handling
        const controller = new AbortController();
        const timeoutId = setTimeout(() => controller.abort(), 10000); // 10 second timeout
        
        const response = await fetch('https://api.github.com/repos/kxn/claude-code-companion/releases/latest', {
            method: 'GET',
            headers: {
                'Accept': 'application/vnd.github.v3+json',
                'User-Agent': 'Claude-Code-Companion-Version-Check'
            },
            mode: 'cors',
            signal: controller.signal
        });
        
        clearTimeout(timeoutId);
        
        if (!response.ok) {
            // Log different types of HTTP errors but don't throw to user
            if (response.status === 403) {
                console.info('GitHub API rate limit reached, skipping version check');
            } else if (response.status >= 500) {
                console.info('GitHub API server error, skipping version check');
            } else {
                console.info(`GitHub API returned ${response.status}, skipping version check`);
            }
            return; // Silent return without affecting other functionality
        }
        
        const data = await response.json();
        const latestVersion = data?.tag_name;
        
        // Validate response data
        if (!latestVersion || typeof latestVersion !== 'string') {
            console.info('Invalid version data from GitHub API, skipping update check');
            return;
        }
        
        // Get current version from the server
        const currentVersionElement = document.getElementById('currentVersion');
        const currentVersion = currentVersionElement ? currentVersionElement.textContent.trim() : '';
        
        console.log('Version check:', { current: currentVersion, latest: latestVersion });
        
        if (latestVersion && currentVersion && shouldShowUpdate(currentVersion, latestVersion)) {
            showUpdateBadge(latestVersion);
        } else {
            hideUpdateBadge();
        }
    } catch (error) {
        // Enhanced silent error handling with categorization
        if (error.name === 'AbortError') {
            console.info('GitHub API request timeout, skipping version check');
        } else if (error.name === 'TypeError' && error.message.includes('fetch')) {
            console.info('Network connectivity issue, skipping version check');
        } else if (error.message.includes('CORS')) {
            console.info('CORS restriction encountered, skipping version check');
        } else if (error instanceof SyntaxError) {
            console.info('Invalid JSON response from GitHub API, skipping version check');
        } else {
            console.info('Version check failed silently:', error.name || 'Unknown error');
        }
        
        // Ensure update badge is hidden if there was an error
        try {
            hideUpdateBadge();
        } catch (badgeError) {
            // Even if hiding badge fails, don't let it break anything
            console.debug('Failed to hide update badge after error, ignoring');
        }
        
        // Silent return - no user notification, no interruption to other functionality
        return;
    }
}

function showUpdateBadge(latestVersion) {
    try {
        const githubLink = document.querySelector('a[href*="github.com/kxn/claude-code-companion"]');
        if (!githubLink) {
            console.debug('GitHub link not found, skipping update badge display');
            return;
        }
        
        // Remove existing badge if any
        const existingBadge = githubLink.querySelector('.update-badge');
        if (existingBadge) {
            existingBadge.remove();
        }
        
        // Create update badge
        const badge = document.createElement('span');
        badge.className = 'update-badge';
        badge.innerHTML = '<i class="fas fa-arrow-up"></i>';
        badge.title = `发现新版本: ${latestVersion}`;
        
        // Position the badge
        githubLink.classList.add('position-relative');
        githubLink.appendChild(badge);
        
        // Update tooltip
        githubLink.title = `${T('version_found', '发现版本')}: ${latestVersion} - ${T('click_to_view_github', '点击查看 GitHub')}`;
    } catch (error) {
        console.debug('Failed to show update badge, ignoring:', error.name || 'Unknown error');
    }
}

function hideUpdateBadge() {
    try {
        const githubLink = document.querySelector('a[href*="github.com/kxn/claude-code-companion"]');
        if (!githubLink) {
            console.debug('GitHub link not found, nothing to hide');
            return;
        }
        
        const badge = githubLink.querySelector('.update-badge');
        if (badge) {
            badge.remove();
        }
        
        // Reset tooltip
        githubLink.title = 'GitHub 仓库';
    } catch (error) {
        console.debug('Failed to hide update badge, ignoring:', error.name || 'Unknown error');
    }
}

function startVersionCheck() {
    // Check immediately with error protection
    try {
        checkForUpdates();
    } catch (error) {
        console.info('Initial version check failed, continuing with interval checks');
    }
    
    // Set up interval to check every 30 minutes (30 * 60 * 1000 ms)
    if (versionCheckInterval) {
        clearInterval(versionCheckInterval);
    }
    
    // Wrap interval callback to prevent any uncaught errors from stopping the timer
    versionCheckInterval = setInterval(() => {
        try {
            checkForUpdates();
        } catch (error) {
            console.info('Scheduled version check failed silently, will retry next interval');
        }
    }, 30 * 60 * 1000);
}

function stopVersionCheck() {
    if (versionCheckInterval) {
        clearInterval(versionCheckInterval);
        versionCheckInterval = null;
    }
}

// Common DOM ready initialization - Enhanced with I18n support
function initializeCommonFeatures() {
    // Initialize I18n system if available
    if (window.I18n && !window.I18n.isInitialized) {
        try {
            // Get initial language from various sources
            const urlLang = new URLSearchParams(window.location.search).get('lang');
            const cookieLang = getCookieValue('claude_proxy_lang');
            const defaultLang = 'zh-cn';
            
            const initialLang = urlLang || cookieLang || defaultLang;
            
            console.log('[I18n] Initializing with language:', initialLang);
            
            window.I18n.init({
                currentLanguage: initialLang,
                debug: true // Enable debug for troubleshooting
            });
            
            // Load any server-provided translations
            loadServerTranslations();
        } catch (error) {
            console.warn('Failed to initialize I18n system:', error);
        }
    }
    
    // Format duration cells
    document.querySelectorAll('.duration-cell').forEach(function(cell) {
        const ms = parseInt(cell.getAttribute('data-ms'));
        if (!isNaN(ms)) {
            cell.textContent = formatDuration(ms);
        }
    });
    
    // Format file size cells
    document.querySelectorAll('.filesize-cell').forEach(function(cell) {
        const bytes = parseInt(cell.getAttribute('data-bytes'));
        if (!isNaN(bytes)) {
            cell.textContent = formatFileSize(bytes);
        }
    });
    
    // Initialize Bootstrap tooltips
    initializeTooltips();
    
    // Update language dropdown
    updateLanguageDropdown();
    
    // Start version checking
    startVersionCheck();
    
    // Add global event listeners for data-action attributes
    document.addEventListener('click', function(e) {
        const action = e.target.dataset.action;
        const langCode = e.target.dataset.langCode;
        
        if (action === 'switch-language' && langCode) {
            e.preventDefault(); // Prevent default link behavior
            switchLanguage(langCode);
        }
    });
}

// Helper function to get cookie value
function getCookieValue(name) {
    const cookies = document.cookie.split(';');
    for (let cookie of cookies) {
        const [cookieName, cookieValue] = cookie.trim().split('=');
        if (cookieName === name) {
            return cookieValue;
        }
    }
    return null;
}

// Load server-provided translations (called during initialization)
async function loadServerTranslations() {
    try {
        const response = await fetch('/admin/api/translations', {
            method: 'GET',
            headers: {
                'Accept': 'application/json'
            }
        });
        
        if (response.ok) {
            const translations = await response.json();
            
            // Add translations to I18n system
            Object.entries(translations).forEach(([lang, langTranslations]) => {
                if (window.I18n && window.I18n.addTranslations) {
                    window.I18n.addTranslations(lang, langTranslations);
                }
            });
            
            console.log('Server translations loaded successfully');
        }
    } catch (error) {
        console.info('Server translations not available, using client-side fallbacks:', error.message);
    }
}