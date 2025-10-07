// Endpoints UI JavaScript - UI 相关功能

// Truncate path, show ... if exceeds specified length
function truncatePath(path, maxLength = 10) {
    if (!path || path.length <= maxLength) {
        return path;
    }
    return path.substring(0, maxLength) + '...';
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
        let statusBadge = '';
        if (!endpoint.enabled) {
            // 如果端点被禁用，显示灰色的"禁用"状态
            statusBadge = `<span class="badge bg-secondary"><i class="fas fa-ban"></i> ${T('disabled', '禁用')}</span>`;
        } else if (endpoint.status === 'active') {
            // 如果端点已启用且状态为活跃，显示绿色的"正常"状态
            statusBadge = `<span class="badge bg-success"><i class="fas fa-check-circle"></i> ${T('normal', '正常')}</span>`;
        } else if (endpoint.status === 'inactive') {
            // 如果端点已启用但状态为不活跃，显示红色的"不可用"状态
            statusBadge = `<span class="badge bg-danger"><i class="fas fa-times-circle"></i> ${T('unavailable', '不可用')}</span>`;
        } else {
            // 其他状态（如检测中）
            statusBadge = `<span class="badge bg-warning"><i class="fas fa-clock"></i> ${T('detecting', '检测中')}</span>`;
        }
        
        // Build enabled status badge
        const enabledBadge = endpoint.enabled
            ? `<span class="badge bg-success"><i class="fas fa-toggle-on"></i> ${T('enabled', '已启用')}</span>`
            : `<span class="badge bg-secondary"><i class="fas fa-toggle-off"></i> ${T('disabled', '已禁用')}</span>`;

        // Build URL display: show full URLs (matching dashboard style)
        let urlDisplay = '';
        if (endpoint.url_anthropic && endpoint.url_openai) {
            // Both URLs available
            urlDisplay = `
                <div class="small">
                    <div><span class="badge bg-primary me-1">Claude</span><code>${escapeHtml(endpoint.url_anthropic)}</code></div>
                    <div class="mt-1"><span class="badge bg-warning me-1">Codex</span><code>${escapeHtml(endpoint.url_openai)}</code></div>
                </div>
            `;
        } else if (endpoint.url_anthropic) {
            // Only Anthropic URL
            urlDisplay = `<span class="badge bg-primary me-1">Claude</span><code>${escapeHtml(endpoint.url_anthropic)}</code>`;
        } else if (endpoint.url_openai) {
            // Only OpenAI URL
            urlDisplay = `<span class="badge bg-warning me-1">Codex</span><code>${escapeHtml(endpoint.url_openai)}</code>`;
        } else {
            // Fallback for old single URL field
            urlDisplay = `<code>${escapeHtml(endpoint.url || '-')}</code>`;
        }

        // Build auth type badge
        let authTypeBadge;
        if (endpoint.auth_type === 'api_key') {
            authTypeBadge = '<span class="badge bg-primary">api_key</span>';
        } else if (endpoint.auth_type === 'oauth') {
            authTypeBadge = '<span class="badge bg-success">oauth</span>';
        } else if (endpoint.auth_type === 'auto') {
            authTypeBadge = '<span class="badge bg-info">auto</span>';
        } else {
            authTypeBadge = '<span class="badge bg-secondary">auth_token</span>';
        }
        
        // Build proxy status display
        let proxyDisplay = '';
        if (endpoint.proxy && endpoint.proxy.type && endpoint.proxy.address) {
            const proxyType = endpoint.proxy.type.toUpperCase();
            const hasAuth = endpoint.proxy.username ? ' 🔐' : '';
            proxyDisplay = `<span class="badge bg-warning" title="${T('proxy_with_auth', '代理')}: ${endpoint.proxy.type}://${endpoint.proxy.address}">${proxyType}${hasAuth}</span>`;
        } else {
            proxyDisplay = `<span class="text-muted">${T('no_proxy', '无')}</span>`;
        }
        
        // Build tags display
        let tagsDisplay = '';
        if (endpoint.tags && endpoint.tags.length > 0) {
            tagsDisplay = endpoint.tags.map(tag => `<span class="badge bg-info me-1 mb-1">${escapeHtml(tag)}</span>`).join('');
        } else {
            tagsDisplay = `<span class="text-muted">${T('general', '通用')}</span>`;
        }


        row.innerHTML = `
            <td class="drag-handle text-center">
                <i class="fas fa-arrows-alt text-muted"></i>
            </td>
            <td>
                <span class="badge bg-info priority-badge">${endpoint.priority}</span>
            </td>
            <td><strong>${escapeHtml(endpoint.name)}</strong></td>
            <td>${urlDisplay}</td>
            <td>${authTypeBadge}</td>
            <td>${proxyDisplay}</td>
            <td>${tagsDisplay}</td>
            <td class="response-cell">
                <span class="text-muted">-</span>
            </td>
            <td data-cell-type="status">${statusBadge}</td>
            <td data-cell-type="enabled">${enabledBadge}</td>
            <td class="action-buttons">
                <div class="btn-group btn-group-sm" role="group">
                    <button class="btn ${endpoint.enabled ? 'btn-success' : 'btn-secondary'} btn-sm"
                            onclick="event.stopPropagation(); toggleEndpointEnabled('${escapeHtml(endpoint.name)}', ${endpoint.enabled})"
                            title="${endpoint.enabled ? '点击禁用' : '点击启用'}">
                        <i class="fas ${endpoint.enabled ? 'fa-toggle-on' : 'fa-toggle-off'}"></i>
                    </button>
                    <button class="btn btn-outline-secondary btn-sm"
                            onclick="event.stopPropagation(); testEndpoint('${escapeHtml(endpoint.name)}');"
                            data-endpoint="${escapeHtml(endpoint.name)}"
                            data-action="test-endpoint"
                            title="测试端点">
                        <i class="fas fa-vial"></i>
                    </button>
                    <button class="btn btn-outline-primary btn-sm"
                            onclick="event.stopPropagation(); showEditEndpointModal('${escapeHtml(endpoint.name)}')"
                            title="编辑">
                        <i class="fas fa-edit"></i>
                    </button>
                    <button class="btn btn-outline-info btn-sm"
                            onclick="event.stopPropagation(); copyEndpoint('${escapeHtml(endpoint.name)}')"
                            title="复制">
                        <i class="fas fa-copy"></i>
                    </button>
                    <button class="btn btn-outline-warning btn-sm"
                            onclick="event.stopPropagation(); resetEndpointStatus('${escapeHtml(endpoint.name)}')"
                            title="重置状态">
                        <i class="fas fa-redo"></i>
                    </button>
                    <button class="btn btn-outline-danger btn-sm"
                            onclick="event.stopPropagation(); deleteEndpoint('${escapeHtml(endpoint.name)}')"
                            title="删除">
                        <i class="fas fa-trash"></i>
                    </button>
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
}

function updateEndpointToggleButton(endpointName, enabled) {
    // Try to find in special endpoint list first
    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    if (!row) {
        // If not found, search in general endpoint list
        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    }
    
    if (row) {
        const toggleButton = row.querySelector('.btn-group button:first-child');
        if (toggleButton) {
            // Update button class
            toggleButton.className = `btn ${enabled ? 'btn-success' : 'btn-secondary'} btn-sm`;
            // Update button icon
            const icon = toggleButton.querySelector('i');
            icon.className = `fas ${enabled ? 'fa-toggle-on' : 'fa-toggle-off'}`;
            // Update button title
            toggleButton.title = enabled ? '点击禁用' : '点击启用';
            // Update button onclick
            toggleButton.onclick = function(event) {
                event.stopPropagation();
                toggleEndpointEnabled(endpointName, enabled);
            };
        }
    }
}

function updateEndpointEnabledBadge(endpointName, enabled) {
    // Try to find in special endpoint list first
    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    if (!row) {
        // If not found, search in general endpoint list
        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpointName}"]`);
    }

    if (row) {
        const enabledCell = row.querySelector('[data-cell-type="enabled"]'); // 使用data属性选择器
        const enabledBadge = enabled
            ? '<span class="badge bg-success"><i class="fas fa-toggle-on"></i> ' + T('enabled', '已启用') + '</span>'
            : '<span class="badge bg-secondary"><i class="fas fa-toggle-off"></i> ' + T('disabled', '已禁用') + '</span>';
        enabledCell.innerHTML = enabledBadge;
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
        const statusCell = row.querySelector('[data-cell-type="status"]'); // 使用data属性选择器
        let statusBadge = '';
        if (!enabled) {
            // 如果端点被禁用，显示灰色的"禁用"状态
            statusBadge = '<span class="badge bg-secondary"><i class="fas fa-ban"></i> ' + T('disabled', '禁用') + '</span>';
        } else if (status === 'active') {
            // 如果端点已启用且状态为活跃，显示绿色的"正常"状态
            statusBadge = '<span class="badge bg-success"><i class="fas fa-check-circle"></i> ' + T('normal', '正常') + '</span>';
        } else if (status === 'inactive') {
            // 如果端点已启用但状态为不活跃，显示红色的"不可用"状态
            statusBadge = '<span class="badge bg-danger"><i class="fas fa-times-circle"></i> ' + T('unavailable', '不可用') + '</span>';
        } else {
            // 其他状态（如检测中）
            statusBadge = '<span class="badge bg-warning"><i class="fas fa-clock"></i> ' + T('detecting', '检测中') + '</span>';
        }
        statusCell.innerHTML = statusBadge;
    }
}