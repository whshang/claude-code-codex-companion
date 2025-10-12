// Settings Page JavaScript

let originalConfig = null;

// Initialize settings page after translations are loaded
function initializeSettingsPage() {
    // Check if required elements are ready
    const saveBtn = document.querySelector('[data-action="save-settings"]');
    const resetBtn = document.querySelector('[data-action="reset-settings"]');

    if (!saveBtn || !resetBtn) {
        console.log('Settings buttons not ready, waiting...');
        setTimeout(initializeSettingsPage, 100);
        return;
    }

    // Check if translation system is ready
    if (typeof T !== 'function' || !window.I18n) {
        console.log('Translation system not ready, waiting...');
        setTimeout(initializeSettingsPage, 100);
        return;
    }

    // Check if translations are loaded for the current language
    const allTranslations = window.I18n.getAllTranslations();
    const currentLang = window.I18n.getLanguage();
    const langKey = (currentLang || '').toLowerCase();
    const isBaseLanguage = langKey === 'en' || langKey === 'en-us';
    if ((!allTranslations[currentLang] || Object.keys(allTranslations[currentLang]).length === 0) && !isBaseLanguage) {
        console.log('Translations not loaded yet, waiting...');
        setTimeout(initializeSettingsPage, 100);
        return;
    }

    console.log('Initializing settings page...');

    // Collect original configuration
    originalConfig = collectFormData();

    // Add event listeners for action buttons
    document.addEventListener('click', function(e) {
        const target = e.target.closest('button');
        if (!target) return;

        const action = target.dataset.action;
        console.log('Settings button clicked with action:', action); // Debug log

        if (action === 'reset-settings') {
            console.log('Calling resetSettings'); // Debug log
            resetSettings();
        } else if (action === 'save-settings') {
            console.log('Calling saveSettings'); // Debug log
            saveSettings();
        }
    });

    // Initialize language switching for settings page
    initializeLanguageSwitching();
}

// Initialize language switching functionality
function initializeLanguageSwitching() {
    // Add event listeners for language switches if they exist
    const languageLinks = document.querySelectorAll('[data-language]');
    languageLinks.forEach(link => {
        link.addEventListener('click', function(e) {
            e.preventDefault();
            const lang = this.dataset.language;
            if (window.I18n && window.I18n.setLanguage) {
                window.I18n.setLanguage(lang);
                // Reload the page to apply new language
                location.reload();
            }
        });
    });
}

// Save original configuration when page loads
document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    initializeSettingsPage();
});

function collectFormData() {
    return {
        server: {
            host: document.getElementById('serverHost').value,
            port: parseInt(document.getElementById('serverPort').value)
        },
        logging: {
            level: document.getElementById('logLevel').value,
            log_request_types: document.getElementById('logRequestTypes').value,
            log_request_body: document.getElementById('logRequestBody').value,
            log_response_body: document.getElementById('logResponseBody').value,
            log_directory: document.getElementById('logDirectory').value
        },
        validation: {
        },
        timeouts: {
            tls_handshake: document.getElementById('tlsHandshake').value,
            response_header: document.getElementById('responseHeader').value,
            idle_connection: document.getElementById('idleConnection').value,
            health_check_timeout: document.getElementById('healthCheckTimeout').value,
            check_interval: document.getElementById('checkInterval').value,
            recovery_threshold: parseInt(document.getElementById('recoveryThreshold').value)
        },
        blacklist: {
            enabled: document.getElementById('blacklistEnabled').checked,
            auto_blacklist: document.getElementById('autoBlacklist').checked,
            business_error_safe: document.getElementById('businessErrorSafe').checked,
            config_error_safe: document.getElementById('configErrorSafe').checked,
            server_error_safe: document.getElementById('serverErrorSafe').checked,
            sse_validation_safe: document.getElementById('sseValidationSafe').checked
        }
    };
}

function saveSettings() {
    console.log('saveSettings called'); // Debug log

    // Check if translation system is ready before using T() function
    if (typeof T !== 'function') {
        console.error('Translation system not ready');
        showAlert('系统未准备好，请稍后再试', 'warning');
        return;
    }

    const config = collectFormData();
    console.log('Collected config:', config); // Debug log

    // Show loading status
    const saveBtn = document.querySelector('[data-action="save-settings"]');
    if (!saveBtn) {
        console.error('Save button not found!');
        return;
    }

    const originalText = saveBtn.innerHTML;
    saveBtn.innerHTML = `<i class="fas fa-spinner fa-spin"></i> ${T('saving', '保存中...')}`;
    saveBtn.disabled = true;

    console.log('Sending API request to /admin/api/settings'); // Debug log

    apiRequest('/admin/api/settings', {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(config)
    })
    .then(response => response.json())
    .then(data => {
        console.log('API response:', data); // Debug log

        if (data.error) {
            throw new Error(data.error);
        }

        // Update original configuration
        originalConfig = config;

        // Check if any settings require restart
        const needsRestart = checkIfRestartRequired(config);

        // Show appropriate success message
        if (needsRestart) {
            showAlert('配置已保存！部分配置需要重启服务后生效。', 'warning', '配置已保存');
        } else {
            showAlert('配置已保存！所有配置已热更新生效。', 'success', '配置已保存');
        }
    })
    .catch(error => {
        console.error('Error saving settings:', error);
        showAlert('保存失败: ' + error.message, 'danger');
    })
    .finally(() => {
        // Restore button state
        saveBtn.innerHTML = originalText;
        saveBtn.disabled = false;
    });
}

// Check if any configuration changes require a restart
function checkIfRestartRequired(newConfig) {
    // These settings require restart
    const restartRequired = [
        // Server settings always require restart when changed
        'server.host',
        'server.port',
        'logging.log_directory'
    ];

    // Check if any restart-required settings have changed
    for (const path of restartRequired) {
        const currentVal = getNestedValue(originalConfig, path);
        const newVal = getNestedValue(newConfig, path);

        if (currentVal !== newVal) {
            return true;
        }
    }

    return false;
}

// Helper function to get nested object values
function getNestedValue(obj, path) {
    return path.split('.').reduce((current, key) => {
        return current && current[key] !== undefined ? current[key] : null;
    }, obj);
}

function resetSettings() {
    console.log('resetSettings called, originalConfig:', originalConfig); // Debug log

    if (!originalConfig) {
        console.warn('No original config found'); // Debug log
        showAlert('没有原始配置可恢复', 'warning');
        return;
    }

    // Restore form values
    document.getElementById('serverHost').value = originalConfig.server.host;
    document.getElementById('serverPort').value = originalConfig.server.port;
    document.getElementById('logLevel').value = originalConfig.logging.level;
    document.getElementById('logRequestTypes').value = originalConfig.logging.log_request_types;
    document.getElementById('logRequestBody').value = originalConfig.logging.log_request_body;
    document.getElementById('logResponseBody').value = originalConfig.logging.log_response_body;
    document.getElementById('logDirectory').value = originalConfig.logging.log_directory;
    document.getElementById('tlsHandshake').value = originalConfig.timeouts.tls_handshake;
    document.getElementById('responseHeader').value = originalConfig.timeouts.response_header;
    document.getElementById('idleConnection').value = originalConfig.timeouts.idle_connection;
    document.getElementById('healthCheckTimeout').value = originalConfig.timeouts.health_check_timeout;
    document.getElementById('checkInterval').value = originalConfig.timeouts.check_interval;
    document.getElementById('recoveryThreshold').value = originalConfig.timeouts.recovery_threshold;

    // Restore blacklist settings
    if (originalConfig.blacklist) {
        document.getElementById('blacklistEnabled').checked = originalConfig.blacklist.enabled;
        document.getElementById('autoBlacklist').checked = originalConfig.blacklist.auto_blacklist;
        document.getElementById('businessErrorSafe').checked = originalConfig.blacklist.business_error_safe;
        document.getElementById('configErrorSafe').checked = originalConfig.blacklist.config_error_safe;
        document.getElementById('serverErrorSafe').checked = originalConfig.blacklist.server_error_safe;
        document.getElementById('sseValidationSafe').checked = originalConfig.blacklist.sse_validation_safe;
    }

    showAlert('配置已重置为初始值', 'info');
}
