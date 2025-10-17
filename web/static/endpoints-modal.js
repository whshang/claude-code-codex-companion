// Endpoints Modal JavaScript - 模态框相关功能

let currentSupportsRuntimeState = 'unknown';

function showAddEndpointModal() {
    editingEndpointName = null;
    originalAuthValue = '';
    isAuthVisible = false;

    document.getElementById('endpointModalTitle').textContent = T('add_endpoint', '添加端点');
document.getElementById('endpointForm').reset();
    document.getElementById('endpoint-enabled').checked = true;
    document.getElementById('endpoint-tags').value = ''; // Clear tags field

    // 清空URL字段
    document.getElementById('endpoint-url-anthropic').value = '';
    document.getElementById('endpoint-url-openai').value = '';

    // Reset auth visibility
    resetAuthVisibility();
    
    // Reset auth visibility
    resetAuthVisibility();
    
    // Clear proxy configuration
    loadProxyConfig(null);
    
    // Set default model rewrite configuration for new endpoints
    const defaultModelRewriteConfig = {
        enabled: true,
        rules: [
            { source_pattern: 'claude-*sonnet*', target_model: 'claude-sonnet-4-5-20250929' },
            { source_pattern: 'claude-*haiku*', target_model: 'claude-haiku-4-5-20251001' },
            { source_pattern: 'gpt-5-codex', target_model: 'gpt-5-codex' },
            { source_pattern: 'gpt-5*', target_model: 'gpt-5' }
        ]
    };
    loadModelRewriteConfig(defaultModelRewriteConfig);

    // Clear default model
    document.getElementById('endpoint-default-model').value = '';
    
    
    // Clear header override configuration
    loadHeaderOverrideConfig(null);
    
    // Clear parameter override configuration
    loadParameterOverrideConfig(null);
    
    // Clear max tokens field name configuration
    document.getElementById('max-tokens-field-name').value = '';
    const responsesModeSelect = document.getElementById('responses-mode');
    if (responsesModeSelect) responsesModeSelect.value = 'auto';
    const countTokensCheckbox = document.getElementById('count-tokens-enabled');
    if (countTokensCheckbox) countTokensCheckbox.checked = true;
    
    // Clear enhanced protection configuration
    document.getElementById('enhanced-protection-enabled').checked = false;
    
    // Check enhanced protection availability based on URL (will be checked again when URL changes)
    checkEnhancedProtectionAvailability();
    
    // Reset to basic configuration tab
    resetModalTabs();
    
    updateSupportsRuntimeIndicator('unknown', 'auto');

    endpointModal.show();
}

function showEditEndpointModal(endpointName) {
    const endpoint = currentEndpoints.find(ep => ep.name === endpointName);
    if (!endpoint) {
        showAlert('端点未找到', 'danger');
        return;
    }

    editingEndpointName = endpointName;
    originalAuthValue = endpoint.auth_value;
    isAuthVisible = false;
    const responsesModeSelect = document.getElementById('responses-mode');
    
    document.getElementById('endpointModalTitle').textContent = T('edit_endpoint', '编辑端点');
    
    // Populate form
    document.getElementById('endpoint-name').value = endpoint.name;
    // 向后兼容：如果有旧的单一URL字段，填充到Anthropic URL
    if (endpoint.url && !endpoint.url_anthropic && !endpoint.url_openai) {
        document.getElementById('endpoint-url-anthropic').value = endpoint.url;
        document.getElementById('endpoint-url-openai').value = '';
    } else {
        document.getElementById('endpoint-url-anthropic').value = endpoint.url_anthropic || '';
        document.getElementById('endpoint-url-openai').value = endpoint.url_openai || '';
    }
    document.getElementById('endpoint-enabled').checked = endpoint.enabled;
    
    // Then set the auth type after the options are populated
    document.getElementById('endpoint-auth-type').value = endpoint.auth_type;
    
    // Set tags field
    const tagsValue = endpoint.tags && endpoint.tags.length > 0 ? endpoint.tags.join(', ') : '';
    document.getElementById('endpoint-tags').value = tagsValue;

    // 移除客户端选择字段 - 现在自动检测

    // Set auth value or OAuth config based on auth type
    if (endpoint.auth_type === 'oauth' && endpoint.oauth_config) {
        // Load OAuth configuration
        loadOAuthConfig(endpoint.oauth_config);
    } else {
        // Set auth value to asterisks
        document.getElementById('endpoint-auth-value').value = '*'.repeat(Math.min(endpoint.auth_value.length, 50));
        document.getElementById('endpoint-auth-value').type = 'password'; // Ensure it's password type
        document.getElementById('endpoint-auth-value').placeholder = '输入您的 API Key 或 Token';
        resetAuthVisibility();
    }
    
    // Update auth type display
    onAuthTypeChange();
    
    // Load proxy configuration
    loadProxyConfig(endpoint.proxy);
    
    // Load model rewrite configuration
    loadModelRewriteConfig(endpoint.model_rewrite);
    
    // Load default model after loading model rewrite config
    loadDefaultModel(endpoint.model_rewrite);
    
    
    // Load header override configuration
    loadHeaderOverrideConfig(endpoint.header_overrides);
    
    // Load parameter override configuration
    loadParameterOverrideConfig(endpoint.parameter_overrides);
    
    // Load max tokens field name configuration
    const maxTokensFieldName = endpoint.max_tokens_field_name || '';
    document.getElementById('max-tokens-field-name').value = maxTokensFieldName;
    if (responsesModeSelect) {
        let mode = 'auto';
        if (endpoint.supports_responses === true || endpoint.openai_preference === 'responses') {
            mode = 'native';
        } else if (endpoint.supports_responses === false || endpoint.openai_preference === 'chat_completions') {
            mode = 'convert';
        }
        responsesModeSelect.value = mode;
    }
    const countTokensCheckbox = document.getElementById('count-tokens-enabled');
    if (countTokensCheckbox) {
        countTokensCheckbox.checked = endpoint.count_tokens_enabled !== false;
    }

    const runtimeState = endpoint.native_codex_format === true
        ? 'native'
        : endpoint.native_codex_format === false
            ? 'converted'
            : 'unknown';
    const currentMode = responsesModeSelect ? responsesModeSelect.value : 'auto';
    updateSupportsRuntimeIndicator(runtimeState, currentMode);
    
    // Load enhanced protection configuration
    const enhancedProtection = endpoint.enhanced_protection || false;
    document.getElementById('enhanced-protection-enabled').checked = enhancedProtection;
    
    // Check enhanced protection availability based on URL
    checkEnhancedProtectionAvailability();
    
    // Reset to basic configuration tab
    resetModalTabs();
    
    endpointModal.show();
}

// Reset modal tabs to basic configuration
function resetModalTabs() {
    // Reset tab state
    const basicTab = document.getElementById('basic-tab');
    const advancedTab = document.getElementById('advanced-tab');
    const advanced2Tab = document.getElementById('advanced2-tab');
    const basicPane = document.getElementById('basic-tab-pane');
    const advancedPane = document.getElementById('advanced-tab-pane');
    const advanced2Pane = document.getElementById('advanced2-tab-pane');
    
    // Activate basic configuration tab
    basicTab.classList.add('active');
    basicTab.setAttribute('aria-selected', 'true');
    basicPane.classList.add('show', 'active');
    
    // Deactivate advanced configuration tabs
    advancedTab.classList.remove('active');
    advancedTab.setAttribute('aria-selected', 'false');
    advancedPane.classList.remove('show', 'active');
    
    if (advanced2Tab && advanced2Pane) {
        advanced2Tab.classList.remove('active');
        advanced2Tab.setAttribute('aria-selected', 'false');
        advanced2Pane.classList.remove('show', 'active');
    }
}

function updateSupportsRuntimeIndicator(state, mode) {
	const indicator = document.getElementById('supports-runtime-indicator');
	const note = document.getElementById('supports-runtime-note');
	if (!indicator || !note) {
		return;
	}
	currentSupportsRuntimeState = state;

	let badgeClass = 'bg-secondary';
	let text = '上次成功：未知';
	if (state === 'native') {
		badgeClass = 'bg-success';
		text = '上次成功：原生 /responses';
	} else if (state === 'converted') {
		badgeClass = 'bg-warning text-dark';
		text = '上次成功：/chat/completions';
	}

	indicator.className = `badge ${badgeClass}`;
	indicator.textContent = text;

	let strategyText = '当前策略：自动探测';
	if (mode === 'native') {
		strategyText = '当前策略：仅使用上游原生 /responses';
	} else if (mode === 'convert') {
		strategyText = '当前策略：始终向上游发送 /chat/completions';
	}
	note.textContent = strategyText;
}

function saveEndpoint() {
    const form = document.getElementById('endpointForm');

    // 自定义URL验证：至少填写一个URL
    const urlAnthropic = document.getElementById('endpoint-url-anthropic').value.trim();
    const urlOpenAI = document.getElementById('endpoint-url-openai').value.trim();

    if (!urlAnthropic && !urlOpenAI) {
        showAlert(T('at_least_one_url_required', '至少需要填写一个URL（Anthropic URL或OpenAI URL）'), 'danger');
        return;
    }

    if (!form.checkValidity()) {
        form.reportValidity();
        return;
    }

    const authType = document.getElementById('endpoint-auth-type').value;
    
    // Get auth value or OAuth config based on auth type
    let authValue = '';
    let oauthConfig = null;
    
    if (authType === 'oauth') {
        // Collect OAuth configuration
        const scopesInput = document.getElementById('oauth-scopes').value.trim();
        const scopes = scopesInput ? scopesInput.split(',').map(s => s.trim()).filter(s => s) : [];
        
        oauthConfig = {
            access_token: document.getElementById('oauth-access-token').value,
            refresh_token: document.getElementById('oauth-refresh-token').value,
            expires_at: parseInt(document.getElementById('oauth-expires-at').value),
            token_url: document.getElementById('oauth-token-url').value,
            client_id: document.getElementById('oauth-client-id').value || '',
            scopes: scopes,
            auto_refresh: document.getElementById('oauth-auto-refresh').checked
        };
        
        // Remove empty optional fields
        if (!oauthConfig.client_id) delete oauthConfig.client_id;
        if (oauthConfig.scopes.length === 0) delete oauthConfig.scopes;
    } else {
        // Get regular auth value
        authValue = document.getElementById('endpoint-auth-value').value;
        if (!isAuthVisible && originalAuthValue && authValue.startsWith('*')) {
            // If showing asterisks and has original value, use original value
            authValue = originalAuthValue;
        }
    }

    // Parse tags field
    const tagsInput = document.getElementById('endpoint-tags').value.trim();
    const tags = tagsInput ? tagsInput.split(',').map(tag => tag.trim()).filter(tag => tag) : [];

    const responsesModeSelect = document.getElementById('responses-mode');
    const data = {
        name: document.getElementById('endpoint-name').value,
        url_anthropic: urlAnthropic === '' ? '' : urlAnthropic || undefined, // Anthropic URL - 明确发送空字符串来清空
        url_openai: urlOpenAI === '' ? '' : urlOpenAI || undefined,       // OpenAI URL - 明确发送空字符串来清空
        // endpoint_type 和 path_prefix 自动推断，不再需要提交
        auth_type: authType,
        auth_value: authValue,
        enabled: document.getElementById('endpoint-enabled').checked,
        tags: tags,
        // 移除 supported_clients - 现在自动检测
        max_tokens_field_name: document.getElementById('max-tokens-field-name').value || '', // New: max tokens field name
        proxy: collectProxyData(), // New: collect proxy configuration
        header_overrides: collectHeaderOverrideData(), // New: collect header override configuration
        parameter_overrides: collectParameterOverrideData(), // New: collect parameter override configuration
        enhanced_protection: document.getElementById('enhanced-protection-enabled').checked, // New: enhanced protection for official accounts
        count_tokens_enabled: document.getElementById('count-tokens-enabled').checked
    };
    if (responsesModeSelect) {
        const mode = responsesModeSelect.value || 'auto';
        if (mode === 'native') {
            data.supports_responses = true;
            data.openai_preference = 'responses';
        } else if (mode === 'convert') {
            data.supports_responses = false;
            data.openai_preference = 'chat_completions';
        } else {
            data.supports_responses = null;
            data.openai_preference = 'auto';
        }
    }
    
    // Add OAuth config if present
    if (oauthConfig) {
        data.oauth_config = oauthConfig;
    }

    const isEditing = editingEndpointName !== null;
    const url = isEditing 
        ? `/admin/api/endpoints/${encodeURIComponent(editingEndpointName)}` 
        : '/admin/api/endpoints';
    const method = isEditing ? 'PUT' : 'POST';

    apiRequest(url, {
        method: method,
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(data)
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            // After successful save, always save model rewrite configuration (including disabled state)
            const modelRewriteConfig = collectModelRewriteData();
            const endpointName = document.getElementById('endpoint-name').value;

            // 始终保存模型重写配置，即使是禁用状态或空规则
            const saveModelRewrite = saveModelRewriteConfig(endpointName, modelRewriteConfig);
            
            saveModelRewrite
                .then(() => {
                    endpointModal.hide();
                    showAlert(data.message, 'success');
                    loadEndpoints(); // Reload data instead of refreshing page
                })
                .catch(error => {
                    console.error('Failed to save model rewrite config:', error);
                    showAlert(T('endpoint_save_success_rewrite_failed', '端点保存成功，但模型重写配置保存失败') + ': ' + error.message, 'warning');
                    endpointModal.hide();
                    loadEndpoints();
                });
        }
    })
    .catch(error => {
        console.error('Failed to save endpoint:', error);
        showAlert(T('failed_to_save_endpoint', 'Failed to save endpoint'), 'danger');
    });
}

function deleteEndpoint(endpointName) {
    if (!confirm(T('confirm_delete_endpoint', '确定要删除端点 "{0}" 吗？').replace('{0}', endpointName))) {
        return;
    }

    apiRequest(`/admin/api/endpoints/${encodeURIComponent(endpointName)}`, {
        method: 'DELETE'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            showAlert(data.message, 'success');
            loadEndpoints(); // Reload data instead of refreshing page
        }
    })
    .catch(error => {
        console.error('Failed to delete endpoint:', error);
        showAlert(T('failed_to_delete_endpoint', 'Failed to delete endpoint'), 'danger');
    });
}

function copyEndpoint(endpointName) {
    if (!confirm(T('confirm_copy_endpoint', '确定要复制端点 "{0}" 吗？').replace('{0}', endpointName))) {
        return;
    }

    apiRequest(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/copy`, {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            showAlert(data.message, 'success');
            loadEndpoints(); // Reload data to show newly copied endpoint
        }
    })
    .catch(error => {
        console.error('Failed to copy endpoint:', error);
        showAlert(T('failed_to_copy_endpoint', 'Failed to copy endpoint'), 'danger');
    });
}

const togglePending = new Set();

async function toggleEndpointEnabled(endpointName, currentEnabled) {
    if (togglePending.has(endpointName)) {
        return;
    }

    const newEnabled = !currentEnabled;
    const actionText = newEnabled ? '启用' : '禁用';

    const currentEndpoint = currentEndpoints.find(ep => ep.name === endpointName);
    const currentStatus = currentEndpoint ? currentEndpoint.status : 'unknown';

    try {
        togglePending.add(endpointName);
        const response = await apiRequest(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/toggle`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ enabled: newEnabled })
        });

        const data = await response.json().catch(() => ({}));

        if (!response.ok || data.error) {
            const msg = data.error || `HTTP ${response.status}`;
            throw new Error(msg);
        }

        showAlert(T('endpoint_action_success', '端点 "{0}" 已{1}').replace('{0}', endpointName).replace('{1}', actionText), 'success');
        updateEndpointToggleButton(endpointName, newEnabled);
        if (currentEndpoint) {
            currentEndpoint.enabled = newEnabled;
        }

        // 端点状态变化后，只在启用自动排序时才触发动态排序
        if (window.APP_CONFIG && window.APP_CONFIG.autoSortEndpoints && window.autoSortEndpoints) {
            window.autoSortEndpoints();
        }
    } catch (error) {
        console.warn('Failed to toggle endpoint:', error);
        const msg = error && error.message ? error.message : error;
        showAlert(T('endpoint_action_failed', '{0}端点失败').replace('{0}', actionText) + `: ${msg}`, 'danger');
    }
    finally {
        togglePending.delete(endpointName);
    }
}

function resetEndpointStatus(endpointName) {
    apiRequest(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/reset-status`, {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            showAlert(T('endpoint_status_reset_success', '端点 "{0}" 状态已重置为正常').replace('{0}', endpointName), 'success');
            // 刷新端点状态显示
            refreshEndpointStatus();
        }
    })
    .catch(error => {
        console.error('Failed to reset endpoint status:', error);
        showAlert(T('reset_endpoint_status_failed', '重置端点状态失败'), 'danger');
    });
}

function resetAllEndpoints() {
    apiRequest('/admin/api/endpoints/reset-all-status', {
        method: 'POST'
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
        } else {
            showAlert(T('all_endpoints_reset_success', '所有端点状态已重置'), 'success');
            // 刷新端点状态显示
            refreshEndpointStatus();
        }
    })
    .catch(error => {
        console.error('Failed to reset all endpoints:', error);
        showAlert(T('reset_all_endpoints_failed', '重置所有端点失败'), 'danger');
    });
}

function reorderEndpoints() {
    // Get special endpoint order
    const specialRows = document.querySelectorAll('#special-endpoint-list tr');
    const specialOrderedNames = Array.from(specialRows).map(row => row.dataset.endpointName);
    
    // Get general endpoint order
    const generalRows = document.querySelectorAll('#general-endpoint-list tr');
    const generalOrderedNames = Array.from(generalRows).map(row => row.dataset.endpointName);
    
    // Merge order: special endpoints first, general endpoints later
    const orderedNames = [...specialOrderedNames, ...generalOrderedNames];
    
    apiRequest('/admin/api/endpoints/reorder', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify({
            ordered_names: orderedNames
        })
    })
    .then(response => response.json())
    .then(data => {
        if (data.error) {
            showAlert(data.error, 'danger');
            loadEndpoints(); // Reload to restore order
        } else {
            showAlert(data.message, 'success');
            // Update priority display, no need to reload entire table
            let priorityIndex = 1;
            
            // Update special endpoint priorities
            specialRows.forEach((row) => {
                const priorityBadge = row.querySelector('.priority-badge');
                if (priorityBadge) {
                    priorityBadge.textContent = priorityIndex++;
                }
            });
            
            // Update general endpoint priorities
            generalRows.forEach((row) => {
                const priorityBadge = row.querySelector('.priority-badge');
                if (priorityBadge) {
                    priorityBadge.textContent = priorityIndex++;
                }
            });
        }
    })
    .catch(error => {
        console.error('Failed to reorder endpoints:', error);
        showAlert('Failed to reorder endpoints', 'danger');
        loadEndpoints(); // Reload to restore order
    });
}

// Check if URL is api.anthropic.com and enable/disable enhanced protection accordingly
function checkEnhancedProtectionAvailability() {
    const urlInput = document.getElementById('endpoint-url');
    const enhancedProtectionCheckbox = document.getElementById('enhanced-protection-enabled');
    
    if (!urlInput || !enhancedProtectionCheckbox) {
        return;
    }
    
    const url = urlInput.value.toLowerCase().trim();
    const isAnthropicOfficial = url.includes('api.anthropic.com');
    
    if (isAnthropicOfficial) {
        // Enable enhanced protection option for api.anthropic.com
        enhancedProtectionCheckbox.disabled = false;
        enhancedProtectionCheckbox.parentElement.parentElement.style.opacity = '1';
    } else {
        // Disable enhanced protection option for non-anthropic endpoints
        enhancedProtectionCheckbox.disabled = true;
        enhancedProtectionCheckbox.checked = false;
        enhancedProtectionCheckbox.parentElement.parentElement.style.opacity = '0.5';
    }
}

// Add event listener for URL input changes
document.addEventListener('DOMContentLoaded', function() {
    const urlInput = document.getElementById('endpoint-url');
    if (urlInput) {
        // Add event listener for input events (real-time typing)
        urlInput.addEventListener('input', checkEnhancedProtectionAvailability);
        // Add event listener for change events (when user leaves the field)
        urlInput.addEventListener('change', checkEnhancedProtectionAvailability);
        // Add event listener for blur events (when field loses focus)
        urlInput.addEventListener('blur', checkEnhancedProtectionAvailability);
    }

	const responsesModeSelect = document.getElementById('responses-mode');
	if (responsesModeSelect) {
		responsesModeSelect.addEventListener('change', function() {
			const mode = responsesModeSelect.value || 'auto';
			updateSupportsRuntimeIndicator(currentSupportsRuntimeState, mode);
		});
	}
});
