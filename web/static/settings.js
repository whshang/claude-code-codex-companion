// Settings Page JavaScript

let originalConfig = null;
let upstreamErrorRuleCounter = 0;
let retryUpstreamHandlersInitialized = false;

function collectUpstreamErrorRules() {
    const container = document.getElementById('retry-upstream-errors-container');
    if (!container) return [];

    const rows = container.querySelectorAll('.upstream-error-row');
    const rules = [];

    rows.forEach(row => {
        const patternInput = row.querySelector('.upstream-error-pattern');
        const actionSelect = row.querySelector('.upstream-error-action');
        const maxInput = row.querySelector('.upstream-error-max-retries');
        const caseCheckbox = row.querySelector('.upstream-error-case');

        if (!patternInput || !actionSelect) {
            return;
        }

        const pattern = patternInput.value.trim();
        const action = (actionSelect.value || '').trim() || 'switch_endpoint';
        const maxRetriesValue = maxInput ? parseInt(maxInput.value, 10) : 0;
        const caseInsensitive = caseCheckbox ? caseCheckbox.checked : false;

        if (pattern.length === 0) {
            return;
        }

        rules.push({
            pattern,
            action,
            max_retries: Number.isFinite(maxRetriesValue) ? maxRetriesValue : 0,
            case_insensitive: caseInsensitive
        });
    });

    return rules;
}

function createUpstreamErrorRow(rule = {}) {
    const pattern = rule.pattern || rule.Pattern || '';
    const action = (rule.action || rule.Action || 'switch_endpoint').toLowerCase();
    const maxRetries = Number.isFinite(rule.max_retries) ? rule.max_retries : (Number.isFinite(rule.MaxRetries) ? rule.MaxRetries : 0);
    const caseInsensitive = typeof rule.case_insensitive === 'boolean' ? rule.case_insensitive :
        (typeof rule.CaseInsensitive === 'boolean' ? rule.CaseInsensitive : false);

    const card = document.createElement('div');
    card.className = 'card mb-2 upstream-error-row';
    card.setAttribute('data-index', upstreamErrorRuleCounter++);
    card.innerHTML = `
        <div class="card-body p-3">
            <div class="row g-2 align-items-end">
                <div class="col-md-4">
                    <label class="form-label" data-t="retry_pattern">匹配模式</label>
                    <input type="text" class="form-control upstream-error-pattern" placeholder="Contains...">
                </div>
                <div class="col-md-3">
                    <label class="form-label" data-t="retry_action">动作</label>
                    <select class="form-select upstream-error-action">
                        <option value="retry_endpoint" data-t="retry_endpoint">重试当前端点</option>
                        <option value="switch_endpoint" data-t="switch_endpoint">切换下一个端点</option>
                    </select>
                </div>
                <div class="col-md-2">
                    <label class="form-label" data-t="retry_max_retries">最大重试</label>
                    <input type="number" class="form-control upstream-error-max-retries" min="0" max="10" placeholder="0">
                </div>
                <div class="col-md-2">
                    <div class="form-check mt-4">
                        <input class="form-check-input upstream-error-case" type="checkbox">
                        <label class="form-check-label" data-t="retry_case_insensitive">忽略大小写</label>
                    </div>
                </div>
                <div class="col-md-1 text-end">
                    <button type="button" class="btn btn-outline-danger btn-sm remove-upstream-error" title="删除">
                        <i class="fas fa-trash"></i>
                    </button>
                </div>
            </div>
        </div>
    `;

    const patternInput = card.querySelector('.upstream-error-pattern');
    const actionSelect = card.querySelector('.upstream-error-action');
    const maxInput = card.querySelector('.upstream-error-max-retries');
    const caseCheckbox = card.querySelector('.upstream-error-case');

    if (patternInput) patternInput.value = pattern;
    if (actionSelect) actionSelect.value = action === 'retry_endpoint' ? 'retry_endpoint' : 'switch_endpoint';
    if (maxInput) maxInput.value = Number.isFinite(maxRetries) ? maxRetries : 0;
    if (caseCheckbox) caseCheckbox.checked = !!caseInsensitive;

    if (window.I18n && typeof window.I18n.processDataTElements === 'function') {
        window.I18n.processDataTElements();
    }

    return card;
}

function setupRetryUpstreamErrors(initialRules = []) {
    const container = document.getElementById('retry-upstream-errors-container');
    const addBtn = document.getElementById('add-retry-upstream-error');
    if (!container || !addBtn) {
        return;
    }

    const existingRows = container.querySelectorAll('.upstream-error-row');
    if (existingRows.length > upstreamErrorRuleCounter) {
        upstreamErrorRuleCounter = existingRows.length;
    }

    if (!retryUpstreamHandlersInitialized) {
        addBtn.addEventListener('click', function() {
            container.appendChild(createUpstreamErrorRow({}));
        });

        container.addEventListener('click', function(e) {
            const btn = e.target.closest('.remove-upstream-error');
            if (!btn) return;
            const row = btn.closest('.upstream-error-row');
            if (row) {
                row.remove();
            }
            if (container.querySelectorAll('.upstream-error-row').length === 0) {
                container.appendChild(createUpstreamErrorRow({}));
            }
        });

        retryUpstreamHandlersInitialized = true;
    }

    if (existingRows.length === 0) {
        if (Array.isArray(initialRules) && initialRules.length > 0) {
            initialRules.forEach(rule => {
                container.appendChild(createUpstreamErrorRow(rule));
            });
        } else {
            container.appendChild(createUpstreamErrorRow({}));
        }
    }
}

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

    const initialRetryRules = originalConfig.retry && Array.isArray(originalConfig.retry.upstream_errors)
        ? originalConfig.retry.upstream_errors
        : [];
    setupRetryUpstreamErrors(initialRetryRules);

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
            port: parseInt(document.getElementById('serverPort').value),
            auto_sort_endpoints: document.getElementById('autoSortEndpoints').checked
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
        },
        streaming: {
            timeout: document.getElementById('streamTimeout').value,
            max_retries: parseInt(document.getElementById('maxRetries').value),
            min_chunk_size: parseInt(document.getElementById('minChunkSize').value),
            enable_sse_validation: document.getElementById('enableSSEValidation').checked,
            enable_caching: document.getElementById('enableStreamCaching').checked
        },
        tools: {
            timeout: document.getElementById('toolCallTimeout').value,
            max_parallel: parseInt(document.getElementById('maxParallelTools').value),
            enable_validation: document.getElementById('enableToolValidation').checked,
            enable_caching: document.getElementById('enableToolCaching').checked
        },
        http_client: {
            max_conns_per_host: parseInt(document.getElementById('maxConnsPerHost').value),
            write_buffer_size: parseInt(document.getElementById('writeBufferSize').value),
            read_buffer_size: parseInt(document.getElementById('readBufferSize').value),
            force_attempt_http2: document.getElementById('forceHTTP2').checked,
            enable_compression: document.getElementById('enableCompression').checked,
            enable_keep_alive: document.getElementById('enableKeepAlive').checked
        },
        monitoring: {
            collection_interval: document.getElementById('metricsCollectionInterval').value,
            slow_request_threshold: document.getElementById('slowRequestThreshold').value,
            enable_detailed_metrics: document.getElementById('enableDetailedMetrics').checked,
            enable_request_tracing: document.getElementById('enableRequestTracing').checked
        },
        format_detection: {
            cache_max_size: parseInt(document.getElementById('cacheMaxSize').value),
            lru_cache_size: parseInt(document.getElementById('lruCacheSize').value),
            enable_path_caching: document.getElementById('enablePathCaching').checked,
            enable_body_structure_detection: document.getElementById('enableBodyStructureDetection').checked
        },
        retry: {
            upstream_errors: collectUpstreamErrorRules()
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
        'logging.log_directory',
        // Network and timeout settings that affect connections
        'timeouts.tls_handshake',
        'timeouts.response_header',
        'timeouts.idle_connection',
        'timeouts.health_check_timeout',
        'http_client.max_conns_per_host',
        'http_client.write_buffer_size',
        'http_client.read_buffer_size',
        'http_client.force_attempt_http2'
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
    document.getElementById('autoSortEndpoints').checked = originalConfig.server.auto_sort_endpoints;
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

    // Restore streaming settings
    if (originalConfig.streaming) {
        document.getElementById('streamTimeout').value = originalConfig.streaming.timeout;
        document.getElementById('maxRetries').value = originalConfig.streaming.max_retries;
        document.getElementById('minChunkSize').value = originalConfig.streaming.min_chunk_size;
        document.getElementById('enableSSEValidation').checked = originalConfig.streaming.enable_sse_validation;
        document.getElementById('enableStreamCaching').checked = originalConfig.streaming.enable_caching;
    }

    // Restore tools settings
    if (originalConfig.tools) {
        document.getElementById('toolCallTimeout').value = originalConfig.tools.timeout;
        document.getElementById('maxParallelTools').value = originalConfig.tools.max_parallel;
        document.getElementById('enableToolValidation').checked = originalConfig.tools.enable_validation;
        document.getElementById('enableToolCaching').checked = originalConfig.tools.enable_caching;
    }

    // Restore HTTP client settings
    if (originalConfig.http_client) {
        document.getElementById('maxConnsPerHost').value = originalConfig.http_client.max_conns_per_host;
        document.getElementById('writeBufferSize').value = originalConfig.http_client.write_buffer_size;
        document.getElementById('readBufferSize').value = originalConfig.http_client.read_buffer_size;
        document.getElementById('forceHTTP2').checked = originalConfig.http_client.force_attempt_http2;
        document.getElementById('enableCompression').checked = originalConfig.http_client.enable_compression;
        document.getElementById('enableKeepAlive').checked = originalConfig.http_client.enable_keep_alive;
    }

    // Restore monitoring settings
    if (originalConfig.monitoring) {
        document.getElementById('metricsCollectionInterval').value = originalConfig.monitoring.collection_interval;
        document.getElementById('slowRequestThreshold').value = originalConfig.monitoring.slow_request_threshold;
        document.getElementById('enableDetailedMetrics').checked = originalConfig.monitoring.enable_detailed_metrics;
        document.getElementById('enableRequestTracing').checked = originalConfig.monitoring.enable_request_tracing;
    }

    // Restore format detection settings
    if (originalConfig.format_detection) {
        document.getElementById('cacheMaxSize').value = originalConfig.format_detection.cache_max_size;
        document.getElementById('lruCacheSize').value = originalConfig.format_detection.lru_cache_size;
        document.getElementById('enablePathCaching').checked = originalConfig.format_detection.enable_path_caching;
        document.getElementById('enableBodyStructureDetection').checked = originalConfig.format_detection.enable_body_structure_detection;
    }

    const retryRules = originalConfig.retry && Array.isArray(originalConfig.retry.upstream_errors)
        ? originalConfig.retry.upstream_errors
        : [];
    const retryContainer = document.getElementById('retry-upstream-errors-container');
    if (retryContainer) {
        retryContainer.innerHTML = '';
        upstreamErrorRuleCounter = 0;
    }
    setupRetryUpstreamErrors(retryRules);

    showAlert('配置已重置为初始值', 'info');
}
