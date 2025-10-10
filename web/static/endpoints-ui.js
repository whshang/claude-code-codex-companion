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
    
    // Separate tagged and untagged endpoints
    const specialEndpoints = endpoints.filter(endpoint => endpoint.tags && endpoint.tags.length > 0);
    const generalEndpoints = endpoints.filter(endpoint => !endpoint.tags || endpoint.tags.length === 0);
    
    // Show/hide special endpoint section
    if (specialEndpoints.length > 0) {
        StyleUtils.show(specialSection);
    } else {
        StyleUtils.hide(specialSection);
    }
    
    // Function to create endpoint row
    function createEndpointRow(endpoint, index) {
        const row = document.createElement('tr');
        row.className = 'endpoint-row';
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
                    <div><span class="badge bg-warning">Claude</span> ${escapeHtml(endpoint.url_anthropic)}</div>
                    <div class="mt-1"><span class="badge bg-primary">Codex</span> ${escapeHtml(endpoint.url_openai)}</div>
                </div>
            `;
        } else if (endpoint.url_anthropic) {
            // Only Anthropic URL
            urlDisplay = `<span class="badge bg-warning">Claude</span> ${escapeHtml(endpoint.url_anthropic)}`;
        } else if (endpoint.url_openai) {
            // Only OpenAI URL
            urlDisplay = `<span class="badge bg-primary">Codex</span> ${escapeHtml(endpoint.url_openai)}`;
        } else {
            // Fallback for old single URL field
            urlDisplay = ` ${escapeHtml(endpoint.url || '-')}`;
        }

        // Build auth badge (will be merged into config chips)
        let authTypeBadge;
        if (endpoint.auth_type === 'api_key') {
            authTypeBadge = '<span class="badge bg-primary" data-bs-toggle="tooltip" title="认证方式：x-api-key (api_key)"><i class="fas fa-key"></i></span>';
        } else if (endpoint.auth_type === 'oauth') {
            authTypeBadge = '<span class="badge bg-success" data-bs-toggle="tooltip" title="认证方式：OAuth（支持自动刷新）"><i class="fas fa-id-badge"></i></span>';
        } else if (endpoint.auth_type === 'auto') {
            authTypeBadge = '<span class="badge bg-info" data-bs-toggle="tooltip" title="认证方式：auto（自动探测并学习）"><i class="fas fa-magic"></i></span>';
        } else {
            authTypeBadge = '<span class="badge bg-secondary" data-bs-toggle="tooltip" title="认证方式：Authorization Bearer (auth_token)"><i class="fas fa-lock"></i></span>';
        }
        
        // Build proxy status display
        let proxyDisplay = '';
        if (endpoint.proxy && endpoint.proxy.type && endpoint.proxy.address) {
            const proxyType = endpoint.proxy.type.toUpperCase();
            const hasAuth = endpoint.proxy.username;
            proxyDisplay = `<span class="badge bg-warning" data-bs-toggle="tooltip" title="代理: ${endpoint.proxy.type}://${endpoint.proxy.address}${hasAuth ? ' (已认证)' : ''}">${proxyType}</span>`;
        } else {
            proxyDisplay = '<span class="text-muted" data-bs-toggle="tooltip" title="无代理配置"><i class="fas fa-times-circle"></i></span>';
        }
        
        // Build tags display
        let tagsDisplay = '';
        if (endpoint.tags && endpoint.tags.length > 0) {
            tagsDisplay = endpoint.tags.map(tag => `<span class="badge bg-info me-1 mb-1" data-bs-toggle="tooltip" title="标签">${escapeHtml(tag)}</span>`).join('');
        } else {
            tagsDisplay = '<span class="text-muted" data-bs-toggle="tooltip" title="通用端点"><i class="fas fa-globe"></i></span>';
        }

        // Build config chips
        const chips = [];
        // Auth first (merged column)
        chips.push(authTypeBadge);
        // Tool support
        const enhMode = (endpoint.tool_enhancement_mode || 'auto').toLowerCase();
        if (endpoint.native_tool_support === true) {
            chips.push('<span class="badge bg-success" data-bs-toggle="tooltip" title="工具调用：原生支持（不注入增强）"><i class="fas fa-tools"></i></span>');
        } else {
            let modeLabel = enhMode || 'auto';
            let cls = 'bg-info';
            if (modeLabel === 'force') cls = 'bg-warning';
            if (modeLabel === 'disable') cls = 'bg-secondary';
            let tip = '工具增强模式：auto（原生不支持/未知时注入）';
            if (modeLabel === 'force') tip = '工具增强模式：force（始终注入增强）';
            if (modeLabel === 'disable') tip = '工具增强模式：disable（从不注入增强）';
            chips.push(`<span class="badge ${cls}" data-bs-toggle="tooltip" title="${tip}"><i class="fas fa-tools"></i></span>`);
        }
        // OpenAI preference
        const pref = (endpoint.openai_preference || 'auto');
        const prefShort = pref === 'chat_completions' ? 'chat' : (pref === 'responses' ? 'resp' : 'auto');
        let prefTip = 'OpenAI 偏好：auto（自动探测并学习）';
        if (prefShort === 'chat') prefTip = 'OpenAI 偏好：chat_completions（/v1/chat/completions）';
        if (prefShort === 'resp') prefTip = 'OpenAI 偏好：responses（/v1/responses）';
        chips.push(`<span class="badge bg-primary" data-bs-toggle="tooltip" title="${prefTip}"><i class="fas fa-robot"></i></span>`);
        // Model rewrite
        const mrEnabled = endpoint.model_rewrite && endpoint.model_rewrite.enabled === true;
        chips.push(`<span class="badge ${mrEnabled ? 'bg-info' : 'bg-secondary'}" data-bs-toggle="tooltip" title="模型重写：${mrEnabled ? '启用（应用映射规则）' : '关闭'}"><i class="fas fa-exchange-alt"></i></span>`);
        // Proxy auth note already shown in proxyDisplay
        const configDisplay = chips.join(' ');

        const supportsConfig = endpoint.supports_responses;
        const activeSupportsMode = supportsConfig === true ? 'native' : supportsConfig === false ? 'convert' : 'auto';

        const renderSupportsButton = (mode, tooltip, icon) => {
            const isActive = activeSupportsMode === mode;
            const baseClass = isActive ? 'btn btn-primary btn-sm' : 'btn btn-outline-secondary btn-sm';
            return `<button type="button" class="${baseClass}" data-action="set-supports" data-endpoint="${escapeHtml(endpoint.name)}" data-mode="${mode}" data-bs-toggle="tooltip" title="${tooltip}"><i class="fas ${icon}"></i></button>`;
        };

        let configBadge;
        if (supportsConfig === true) {
            configBadge = '<span class="badge bg-success" data-bs-toggle="tooltip" title="显式声明：始终视为原生支持 /responses">配置: 原生</span>';
        } else if (supportsConfig === false) {
            configBadge = '<span class="badge bg-warning text-dark" data-bs-toggle="tooltip" title="显式声明：始终转换为 /chat/completions">配置: 转换</span>';
        } else {
            configBadge = '<span class="badge bg-secondary" data-bs-toggle="tooltip" title="显式声明：自动探测并按学习结果切换">配置: 自动</span>';
        }

        const nativeFormat = endpoint.native_codex_format;
        let runtimeBadge;
        if (nativeFormat === true) {
            runtimeBadge = '<span class="badge bg-success" data-bs-toggle="tooltip" title="最近探测结果：端点返回原生 /responses">学习: 原生</span>';
        } else if (nativeFormat === false) {
            runtimeBadge = '<span class="badge bg-warning text-dark" data-bs-toggle="tooltip" title="最近探测结果：端点需转换为 /chat/completions">学习: 转换</span>';
        } else {
            runtimeBadge = '<span class="badge bg-secondary" data-bs-toggle="tooltip" title="尚未探测或缺少近期请求">学习: 待探测</span>';
        }

        const prefValue = (endpoint.openai_preference || 'auto');
        let prefLabel = 'Auto';
        prefTip = 'OpenAI 偏好：auto（自动探测并学习）';
        if (prefValue === 'responses') {
            prefLabel = 'Responses';
            prefTip = 'OpenAI 偏好：responses（优先请求 /v1/responses）';
        } else if (prefValue === 'chat_completions') {
            prefLabel = 'Chat';
            prefTip = 'OpenAI 偏好：chat_completions（直接请求 /v1/chat/completions）';
        }
        const preferenceBadge = `<span class="badge bg-info" data-bs-toggle="tooltip" title="${prefTip}">偏好: ${prefLabel}</span>`;

        const responseCellHTML = `
            <div class="supports-responses-cell">
                <div class="mb-2 d-flex flex-wrap gap-1">
                    ${configBadge}
                    ${runtimeBadge}
                    ${preferenceBadge}
                </div>
                <div class="btn-group btn-group-sm" role="group" aria-label="supports responses toggle">
                    ${renderSupportsButton('auto', '自动探测：保持代理自适应策略', 'fa-sync')}
                    ${renderSupportsButton('native', '锁定原生：始终请求 /responses', 'fa-plug')}
                    ${renderSupportsButton('convert', '强制转换：始终请求 /chat/completions', 'fa-random')}
                </div>
                <small class="text-muted d-block mt-1">配置影响后续请求；学习状态来自最近健康检查</small>
            </div>
        `;


        row.innerHTML = `
            <td class="drag-handle text-center">
                <i class="fas fa-arrows-alt text-muted"></i>
            </td>
            <td>
                <span class="badge bg-info priority-badge">${endpoint.priority}</span>
            </td>
            <td><strong>${escapeHtml(endpoint.name)}</strong></td>
            <td class="url-cell">${urlDisplay}</td>
            <td>${proxyDisplay}</td>
            <td>${tagsDisplay}</td>
            <td>${configDisplay}</td>
            <td class="response-cell">
                ${responseCellHTML}
            </td>
            <td data-cell-type="status">${profileBadgesHTML}</td>
            <td class="action-buttons">
                <div class="actions-grid">
                    <button class="btn ${endpoint.enabled ? 'btn-success' : 'btn-secondary'} btn-sm" data-action="toggle-endpoint" onclick="event.stopPropagation(); toggleEndpointEnabled('${escapeHtml(endpoint.name)}', ${endpoint.enabled})" data-bs-toggle="tooltip" title="${endpoint.enabled ? '点击禁用' : '点击启用'}"><i class="fas ${endpoint.enabled ? 'fa-toggle-on' : 'fa-toggle-off'}"></i></button>
                    <button class="btn btn-outline-secondary btn-sm" onclick="event.stopPropagation(); testEndpoint('${escapeHtml(endpoint.name)}');" data-endpoint="${escapeHtml(endpoint.name)}" data-action="test-endpoint" data-bs-toggle="tooltip" title="测试端点"><i class="fas fa-vial"></i></button>
                    <button class="btn btn-outline-primary btn-sm" onclick="event.stopPropagation(); showEditEndpointModal('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="编辑"><i class="fas fa-edit"></i></button>
                    <button class="btn btn-outline-info btn-sm" onclick="event.stopPropagation(); copyEndpoint('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="复制"><i class="fas fa-copy"></i></button>
                    <button class="btn btn-outline-warning btn-sm" onclick="event.stopPropagation(); resetEndpointStatus('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="重置状态"><i class="fas fa-redo"></i></button>
                    <button class="btn btn-outline-danger btn-sm" onclick="event.stopPropagation(); deleteEndpoint('${escapeHtml(endpoint.name)}')" data-bs-toggle="tooltip" title="删除"><i class="fas fa-trash"></i></button>
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
            toggleButton.title = enabled ? '点击禁用' : '点击启用';
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

function updateEndpointStatusBadge(endpointName, enabled, status) {
    // Try to find in special endpoint list first
    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    if (!row) {
        // If not found, search in general endpoint list
        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    }

	if (row) {
		const statusCell = row.querySelector('[data-cell-type="status"]');
		if (!statusCell) return;

		const badges = [];
		badges.push(enabled ? '<span class="badge bg-success">Enabled</span>' : '<span class="badge bg-secondary text-dark">Disabled</span>');

		if (status === 'active') {
			badges.push('<span class="badge bg-primary">Active</span>');
		} else if (status === 'inactive') {
			badges.push('<span class="badge bg-warning text-dark">Idle</span>');
		} else {
			badges.push('<span class="badge bg-info text-dark">Check</span>');
		}

		statusCell.innerHTML = badges.join(' ');
	}
}
