// Endpoints UI JavaScript - UI 相关功能

// Truncate path, show ... if exceeds specified length
function truncatePath(path, maxLength = 10) {
    if (!path || path.length <= maxLength) {
        return path;
    }
    return path.substring(0, maxLength) + '...';
}

function refreshTooltip(element) {
    if (!element || !window.bootstrap || typeof bootstrap.Tooltip !== 'function') {
        return;
    }
    const instance = bootstrap.Tooltip.getInstance(element);
    if (instance) {
        instance.dispose();
    }
    bootstrap.Tooltip.getOrCreateInstance(element);
}

function rebuildTable(endpoints) {
    const specialTbody = document.getElementById('special-endpoint-list');
    const generalTbody = document.getElementById('general-endpoint-list');
    const specialSection = document.getElementById('special-endpoints-section');

    // Clear existing content
    specialTbody.innerHTML = '';
    generalTbody.innerHTML = '';

    // Sort endpoints by priority before rendering
    const sortedEndpoints = [...endpoints].sort((a, b) => a.priority - b.priority);

    // Separate tagged and untagged endpoints
    const specialEndpoints = sortedEndpoints.filter(endpoint => endpoint.tags && endpoint.tags.length > 0);
    const generalEndpoints = sortedEndpoints.filter(endpoint => !endpoint.tags || endpoint.tags.length === 0);
    
    // Show/hide special endpoint section
    if (specialEndpoints.length > 0) {
        StyleUtils.show(specialSection);
    } else {
        StyleUtils.hide(specialSection);
    }
    
    // Function to create endpoint row
    function createEndpointRow(endpoint, index) {
        const row = document.createElement('tr');
        const autoSort = window.APP_CONFIG && window.APP_CONFIG.autoSortEndpoints;

        // 设置行类名：基础类 + 自动排序模式类 + 禁用状态类
        let rowClasses = ['endpoint-row'];
        if (autoSort) {
            rowClasses.push('auto-sort-mode');
        }
        if (!endpoint.enabled) {
            rowClasses.push('endpoint-disabled');
        }
        row.className = rowClasses.join(' ');
        row.setAttribute('data-endpoint-name', escapeHtml(endpoint.name));
        
        // Build status badge - 三种状态：禁用（灰色）、正常（绿色）、不可用（红色）
        const profileBadges = [];
        if (endpoint.enabled) {
            profileBadges.push('<span class="badge bg-success">Enabled</span>');
        } else {
            profileBadges.push('<span class="badge bg-secondary text-dark">Disabled</span>');
        }

        if (endpoint.status === 'active') {
            profileBadges.push('<span class="badge bg-primary">Active</span>');
        } else if (endpoint.status === 'inactive') {
            profileBadges.push('<span class="badge bg-warning text-dark">Idle</span>');
        } else {
            profileBadges.push('<span class="badge bg-info text-dark">Check</span>');
        }

        const profileBadgesHTML = profileBadges.join(' ');

        // Build URL display: show full URLs (matching dashboard style)
        let urlDisplay = '';
        if (endpoint.url_anthropic && endpoint.url_openai) {
            // Both URLs available
            urlDisplay = `
                <div class="small">
                    <div><span class="badge bg-warning">Anthropic</span> ${escapeHtml(endpoint.url_anthropic)}</div>
                    <div class="mt-1"><span class="badge bg-primary">OpenAI</span> ${escapeHtml(endpoint.url_openai)}</div>
                </div>
            `;
        } else if (endpoint.url_anthropic) {
            // Only Anthropic URL
            urlDisplay = `<span class="badge bg-warning">Anthropic</span> ${escapeHtml(endpoint.url_anthropic)}`;
        } else if (endpoint.url_openai) {
            // Only OpenAI URL
            urlDisplay = `<span class="badge bg-primary">OpenAI</span> ${escapeHtml(endpoint.url_openai)}`;
        } else {
            // Fallback for old single URL field
            urlDisplay = ` ${escapeHtml(endpoint.url || '-')}`;
        }

        // Build auth badge (will be merged into config chips) - 只用颜色区分，不需要图标
        let authTypeBadge;
        if (endpoint.auth_type === 'api_key') {
            authTypeBadge = `<span class="badge bg-primary" data-bs-toggle="tooltip" title="${T('authentication_method_api_key_tooltip')}">${T('auth_type_key')}</span>`;
        } else if (endpoint.auth_type === 'oauth') {
            authTypeBadge = `<span class="badge bg-success" data-bs-toggle="tooltip" title="${T('authentication_method_oauth_tooltip')}">${T('auth_type_oauth')}</span>`;
        } else if (endpoint.auth_type === 'auto') {
            authTypeBadge = `<span class="badge bg-info" data-bs-toggle="tooltip" title="${T('authentication_method_auto_tooltip')}">${T('auth_type_auto')}</span>`;
        } else {
            authTypeBadge = `<span class="badge bg-secondary" data-bs-toggle="tooltip" title="${T('authentication_method_auth_token_tooltip')}">${T('auth_type_auth')}</span>`;
        }

        // Build proxy status display
        let proxyDisplay = '';
        if (endpoint.proxy && endpoint.proxy.type && endpoint.proxy.address) {
            const proxyType = endpoint.proxy.type.toUpperCase();
            const hasAuth = endpoint.proxy.username;
            proxyDisplay = `<span class="badge bg-warning" data-bs-toggle="tooltip" title="${T('proxy_with_auth')}: ${endpoint.proxy.type}://${endpoint.proxy.address}${hasAuth ? ` (${T('proxy_auth')})` : ''}">${proxyType}</span>`;
        } else {
            proxyDisplay = `<span class="text-muted" data-bs-toggle="tooltip" title="${T('no_proxy')}"><i class="fas fa-times-circle"></i></span>`;
        }
        
        // Build tags display
        let tagsDisplay = '';
        if (endpoint.tags && endpoint.tags.length > 0) {
            tagsDisplay = endpoint.tags.map(tag => `<span class="badge bg-info me-1 mb-1" data-bs-toggle="tooltip" title="${T('tags')}">${escapeHtml(tag)}</span>`).join('');
        } else {
            tagsDisplay = `<span class="text-muted" data-bs-toggle="tooltip" title="${T('general_endpoints')}"><i class="fas fa-globe"></i></span>`;
        }

        // Build config chips - 使用span而不是button，因为这些是状态显示，不是可交互按钮
        const chips = [];
        // Auth status - 使用span表示状态信息
        chips.push(authTypeBadge);

        // OpenAI preference - 只有配置了url_openai时才显示
        if (endpoint.url_openai) {
            const pref = (endpoint.openai_preference || 'auto');
            const prefShort = pref === 'chat_completions' ? 'Chat' : (pref === 'responses' ? 'Resp' : 'Auto');
            let prefTip = T('openai_preference_auto_tooltip');
            let prefColor = 'secondary'; // auto 模式用灰色
            if (prefShort === 'Chat') {
                prefTip = T('openai_preference_chat_tooltip');
                prefColor = 'warning'; // chat 模式用黄色
            } else if (prefShort === 'Resp') {
                prefTip = T('openai_preference_resp_tooltip');
                prefColor = 'success'; // responses 模式用绿色
            }
            chips.push(`<span class="badge bg-${prefColor}" data-bs-toggle="tooltip" title="${prefTip}">${prefShort}</span>`);
        }

        // Model rewrite - 在tooltip里展示重写的结果
        const mrEnabled = endpoint.model_rewrite && endpoint.model_rewrite.enabled === true;
        let mrTooltip = T('model_rewrite_disabled');
        if (mrEnabled && endpoint.model_rewrite.rules && endpoint.model_rewrite.rules.length > 0) {
            const rules = endpoint.model_rewrite.rules.map(rule =>
                `${rule.source_pattern} → ${rule.target_model}`
            ).join('; ');
            mrTooltip = `${T('model_rewrite_enabled')} (${rules})`;
        } else if (mrEnabled) {
            mrTooltip = `${T('model_rewrite_enabled')} (no specific rules)`;
        }
        chips.push(`<span class="badge ${mrEnabled ? 'bg-info' : 'bg-secondary'}" data-bs-toggle="tooltip" title="${mrTooltip}">MR</span>`);
        const configDisplay = chips.join(' ');

        const supportsConfig = endpoint.supports_responses;
        const activeSupportsMode = supportsConfig === true ? 'native' : supportsConfig === false ? 'convert' : 'auto';

        // 根据配置状态和学习状态生成综合状态信息 - 优化显示逻辑，避免重复
        const nativeFormat = endpoint.native_codex_format;
        const prefValue = (endpoint.openai_preference || 'auto');

        // 简化状态显示：只显示最重要的信息
        let statusBadges = [];

        // 1. 首先显示配置与学习状态的综合信息（最重要的）
        if (supportsConfig === true && nativeFormat === true) {
            // 都是原生支持
            statusBadges.push('<span class="badge bg-success" data-bs-toggle="tooltip" title="配置与学习状态一致：原生支持 /responses">原生支持</span>');
        } else if (supportsConfig === false && nativeFormat === false) {
            // 都是需要转换
            statusBadges.push('<span class="badge bg-warning text-dark" data-bs-toggle="tooltip" title="配置与学习状态一致：需要转换为 /chat/completions">需要转换</span>');
        } else if (supportsConfig !== null && nativeFormat !== null && supportsConfig !== nativeFormat) {
            // 配置和学习状态不一致，显示冲突警告
            statusBadges.push('<span class="badge bg-danger" data-bs-toggle="tooltip" title="警告：配置与学习状态不一致">状态冲突</span>');
        } else {
            // 至少一个未知，显示详细信息
            if (supportsConfig === true) {
                statusBadges.push('<span class="badge bg-success" data-bs-toggle="tooltip" title="显式声明：始终视为原生支持 /responses">配置: 原生</span>');
            } else if (supportsConfig === false) {
                statusBadges.push('<span class="badge bg-warning text-dark" data-bs-toggle="tooltip" title="显式声明：始终转换为 /chat/completions">配置: 转换</span>');
            } else {
                statusBadges.push('<span class="badge bg-secondary" data-bs-toggle="tooltip" title="显式声明：自动探测并按学习结果切换">配置: 自动</span>');
            }

            if (nativeFormat === true) {
                statusBadges.push('<span class="badge bg-success" data-bs-toggle="tooltip" title="最近探测结果：端点返回原生 /responses">学习: 原生</span>');
            } else if (nativeFormat === false) {
                statusBadges.push('<span class="badge bg-warning text-dark" data-bs-toggle="tooltip" title="最近探测结果：端点需转换为 /chat/completions">学习: 转换</span>');
            } else {
                statusBadges.push('<span class="badge bg-secondary" data-bs-toggle="tooltip" title="尚未探测或缺少近期请求">学习: 待探测</span>');
            }
        }

        // 2. 显示OpenAI偏好（重要配置信息）- 只有配置了url_openai时才显示
        if (endpoint.url_openai) {
            let prefLabel = 'Auto';
            let prefTip2 = 'OpenAI 偏好：auto（自动探测并学习）';
            if (prefValue === 'responses') {
                prefLabel = 'Responses';
                prefTip2 = 'OpenAI 偏好：responses（优先请求 /v1/responses）';
            } else if (prefValue === 'chat_completions') {
                prefLabel = 'Chat';
                prefTip2 = 'OpenAI 偏好：chat_completions（直接请求 /v1/chat/completions）';
            }
            statusBadges.push(`<span class="badge bg-info" data-bs-toggle="tooltip" title="${prefTip2}">偏好: ${prefLabel}</span>`);
        }

        // 3. 支持模式切换按钮组（可交互元素使用button）
        const renderSupportsButton = (mode, tooltip, icon) => {
            const isActive = activeSupportsMode === mode;
            const baseClass = isActive ? 'btn btn-primary btn-sm' : 'btn btn-outline-secondary btn-sm';
            return `<button type="button" class="${baseClass}" data-action="set-supports" data-endpoint="${escapeHtml(endpoint.name)}" data-mode="${mode}" data-bs-toggle="tooltip" title="${tooltip}"><i class="fas ${icon}"></i></button>`;
        };

        const configCellHTML = `
            <div class="config-section">
                ${configDisplay}
            </div>
        `;

        // 构建状态信息
        const statusCellHTML = `
            <div class="status-section">
                ${profileBadgesHTML}
            </div>
        `;

        // 根据自动排序模式决定拖拽图标样式（复用函数开始处定义的autoSort变量）
        const dragHandleHTML = autoSort
            ? '<i class="fas fa-grip-lines text-muted" style="opacity: 0.3; cursor: help;" data-bs-toggle="tooltip" title="自动排序模式：拖拽已禁用"></i>'
            : '<i class="fas fa-arrows-alt text-muted"></i>';

        row.innerHTML = `
            <td class="drag-handle text-center">
                ${dragHandleHTML}
            </td>
            <td>
                <span class="badge bg-info priority-badge">${endpoint.priority}</span>
            </td>
            <td>
                <div class="endpoint-info">
                    <div class="endpoint-name"><strong>${escapeHtml(endpoint.name)}</strong></div>
                    <div class="endpoint-url">${urlDisplay}</div>
                </div>
            </td>
            <td>${proxyDisplay}</td>
            <td>${tagsDisplay}</td>
            <td class="config-cell">${configCellHTML}</td>
            <td class="status-cell">${statusCellHTML}</td>
            <td class="test-cell">
                <div class="test-results-container">
                    <div class="test-result-item">
                        <span class="text-muted">-</span>
                    </div>
                </div>
            </td>
            <td class="action-buttons">
                <div class="actions-grid">
                    <button class="btn ${endpoint.enabled ? 'btn-success' : 'btn-secondary'} btn-sm" data-action="toggle-endpoint" onclick="event.stopPropagation(); toggleEndpointEnabled('${escapeHtml(endpoint.name)}', ${endpoint.enabled})" data-bs-toggle="tooltip" title="${endpoint.enabled ? T('click_to_disable') : T('click_to_enable')}"><i class="fas ${endpoint.enabled ? 'fa-toggle-on' : 'fa-toggle-off'}"></i></button>
                    <button class="btn btn-outline-secondary btn-sm" onclick="event.stopPropagation(); testEndpoint('${escapeHtml(endpoint.name)}');" data-endpoint="${escapeHtml(endpoint.name)}" data-action="test-endpoint" data-bs-toggle="tooltip" title="${T('test_endpoint')}"><i class="fas fa-vial"></i></button>
                    <button class="btn btn-outline-primary btn-sm" onclick="event.stopPropagation(); showEditEndpointModal('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="${T('edit')}"><i class="fas fa-edit"></i></button>
                    <button class="btn btn-outline-info btn-sm" onclick="event.stopPropagation(); copyEndpoint('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="${T('copy')}"><i class="fas fa-copy"></i></button>
                    <button class="btn btn-outline-warning btn-sm" onclick="event.stopPropagation(); resetEndpointStatus('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="${T('reset_status')}"><i class="fas fa-redo"></i></button>
                    <button class="btn btn-outline-danger btn-sm" onclick="event.stopPropagation(); deleteEndpoint('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="${T('delete')}"><i class="fas fa-trash"></i></button>
                </div>
            </td>
        `;
        
        return row;
    }
    
    // Add special endpoints
    specialEndpoints.forEach((endpoint, index) => {
        const row = createEndpointRow(endpoint, index);
        specialTbody.appendChild(row);
    });
    
    // Add general endpoints
    generalEndpoints.forEach((endpoint, index) => {
        const row = createEndpointRow(endpoint, specialEndpoints.length + index);
        generalTbody.appendChild(row);
    });

    // Reinitialize drag-and-drop sorting
    initializeSortable();

    // Restore cached test results if available
    if (typeof restoreCachedTestResults === 'function') {
        restoreCachedTestResults();
    }

    // Initialize Bootstrap tooltips for new badges
    if (window.bootstrap && typeof bootstrap.Tooltip === 'function') {
        const tooltipEls = [].slice.call(document.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipEls.forEach(refreshTooltip);
    }
}

function updateEndpointToggleButton(endpointName, enabled) {
    // Try to find in special endpoint list first
    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    if (!row) {
        // If not found, search in general endpoint list
        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    }

    if (row) {
        const toggleButton = row.querySelector('button[data-action="toggle-endpoint"]');
        if (toggleButton) {
            // Update button class
            toggleButton.className = `btn ${enabled ? 'btn-success' : 'btn-secondary'} btn-sm`;
            // Update button icon
            const icon = toggleButton.querySelector('i');
            icon.className = `fas ${enabled ? 'fa-toggle-on' : 'fa-toggle-off'}`;
            // Update button title
            toggleButton.title = enabled ? T('click_to_disable') : T('click_to_enable');
            toggleButton.setAttribute('data-bs-original-title', toggleButton.title);
            refreshTooltip(toggleButton);
            // Update button onclick
            toggleButton.onclick = function(event) {
                event.stopPropagation();
                toggleEndpointEnabled(endpointName, enabled);
            };
        }
    }
}

// Note: updateEndpointStatusBadge function has been removed as status is now part of the combined status cell
// that gets rebuilt entirely when endpoints are refreshed
