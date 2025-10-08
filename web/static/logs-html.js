// Logs Page HTML Generation Functions

function generateLogAttemptHtml(log, attemptNum) {
    const isSuccess = log.status_code >= 200 && log.status_code < 300;
    const badgeClass = isSuccess ? 'bg-success' : 'bg-danger';

    // Check if there are data transformations
    const requestChanges = hasRequestChanges(log);
    const responseChanges = hasResponseChanges(log);

    // Use actual attempt number from log if available
    const displayAttemptNum = log.attempt_number || attemptNum;

    // Build client type badge
    let clientBadge = '';
    if (log.client_type === 'claude-code') {
        clientBadge = '<span class="badge bg-primary" title="Claude Code"><i class="fas fa-robot"></i> Claude</span>';
    } else if (log.client_type === 'codex') {
        clientBadge = '<span class="badge bg-success" title="Codex"><i class="fas fa-code"></i> Codex</span>';
    } else if (log.client_type) {
        clientBadge = `<span class="badge bg-secondary">${escapeHtml(log.client_type)}</span>`;
    }

    // Tool calling badges
    let toolBadges = '';
    if (log.tool_enhancement_mode) {
        const modeLabel = log.tool_enhancement_mode.toUpperCase();
        const badgeClass = log.tool_enhancement_applied ? 'bg-warning text-dark' : 'bg-secondary';
        toolBadges += `<span class="badge ${badgeClass}" title="${T('log_tool_mode', '工具增强模式')}">${T('log_tool_mode_short', 'Tool')} ${escapeHtml(modeLabel)}</span>`;
    }
    if (log.tool_enhancement_applied) {
        const count = log.tool_call_count || 0;
        toolBadges += `<span class="badge bg-info text-dark" title="${T('log_tool_prompt_injected', '已注入工具增强提示')}">${T('log_tool_enhanced', 'Enh+')}${count > 0 ? ` (${count})` : ''}</span>`;
    }
    if (log.tool_calls_detected) {
        const count = log.tool_call_count || 0;
        toolBadges += `<span class="badge bg-success" title="${T('log_tool_detected', '检测到工具调用')}">${T('log_tool_calls', 'Tool×')}${count}</span>`;
    }
    if (log.tool_native_support !== undefined && log.tool_native_support !== null) {
        toolBadges += log.tool_native_support ?
            `<span class="badge bg-primary" title="${T('log_tool_native', '端点原生支持工具调用')}">${T('log_tool_native_short', 'Native')}</span>` :
            `<span class="badge bg-danger" title="${T('log_tool_non_native', '端点不支持工具调用')}">${T('log_tool_non_native_short', 'NoNative')}</span>`;
    }

    return `
        <div class="card mb-3">
            <div class="card-header">
                <h6 class="mb-0">
                    ${displayAttemptNum > 1 ? `${T('retry_number', '重试')} #${displayAttemptNum - 1}` : `${T('first_attempt', '首次尝试')}`}: ${escapeHtml(log.endpoint)}
                    <span class="badge ${badgeClass}">${log.status_code}</span>
                    <span class="badge bg-secondary">${log.duration_ms}ms</span>
                    ${clientBadge}
                    ${log.model ?
                        (log.model_rewrite_applied ?
                            `<span class="badge bg-success model-rewritten" title="→ ${escapeHtml(log.rewritten_model)}">${escapeHtml(log.model)}</span>` :
                            `<span class="badge bg-primary">${escapeHtml(log.model)}</span>`
                        ) : ''
                    }
                    ${log.is_streaming ? '<span class="badge bg-info">SSE</span>' : ''}
                    ${log.content_type_override ? `<span class="badge bg-warning text-dark" title="Content-Type覆盖: ${escapeHtml(log.content_type_override)}">${escapeHtml(log.content_type_override)}</span>` : ''}
                    ${requestChanges || responseChanges ? `<span class="badge bg-info">${T('has_modifications', '有修改')}</span>` : ''}
                    ${toolBadges}
                </h6>
            </div>
            <div class="card-body">
                ${log.error ? `<div class="alert alert-danger mb-3"><strong>${T('error', '错误')}:</strong> ${escapeHtml(log.error)}</div>` : ''}
                <!-- Request/Response Tabs -->
                <ul class="nav nav-tabs before-after-tabs" id="logTabs${attemptNum}" role="tablist">
                    <li class="nav-item" role="presentation">
                        <button class="nav-link active" id="request-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#request-${attemptNum}" type="button" role="tab">
                            ${T('request_data', '请求数据')} ${requestChanges ? `<span class="comparison-badge badge bg-warning">${T('modified', '修改')}</span>` : ''}
                        </button>
                    </li>
                    <li class="nav-item" role="presentation">
                        <button class="nav-link" id="response-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#response-${attemptNum}" type="button" role="tab">
                            ${T('response_data', '响应数据')} ${responseChanges ? `<span class="comparison-badge badge bg-warning">${T('modified', '修改')}</span>` : ''}
                        </button>
                    </li>
                </ul>
                
                <div class="tab-content mt-3" id="logTabsContent${attemptNum}">
                    <!-- Request Tab -->
                    <div class="tab-pane fade show active" id="request-${attemptNum}" role="tabpanel">
                        ${generateRequestComparisonHtml(log, attemptNum)}
                    </div>
                    
                    <!-- Response Tab -->  
                    <div class="tab-pane fade" id="response-${attemptNum}" role="tabpanel">
                        ${generateResponseComparisonHtml(log, attemptNum)}
                    </div>
                </div>
            </div>
        </div>`;
}

// Generate content for log attempt in tab (without card wrapper)
function generateLogAttemptContentHtml(log, attemptNum) {
    const isSuccess = log.status_code >= 200 && log.status_code < 300;
    const badgeClass = isSuccess ? 'bg-success' : 'bg-danger';

    // Check if there are data transformations
    const requestChanges = hasRequestChanges(log);
    const responseChanges = hasResponseChanges(log);

    // Use actual attempt number from log if available
    const displayAttemptNum = log.attempt_number || attemptNum;

    // Build client type badge
    let clientBadge = '';
    if (log.client_type === 'claude-code') {
        clientBadge = '<span class="badge bg-primary" title="Claude Code"><i class="fas fa-robot"></i> Claude</span>';
    } else if (log.client_type === 'codex') {
        clientBadge = '<span class="badge bg-success" title="Codex"><i class="fas fa-code"></i> Codex</span>';
    } else if (log.client_type) {
        clientBadge = `<span class="badge bg-secondary">${escapeHtml(log.client_type)}</span>`;
    }

    let toolBadges = '';
    if (log.tool_enhancement_mode) {
        const modeLabel = log.tool_enhancement_mode.toUpperCase();
        const badgeClass = log.tool_enhancement_applied ? 'bg-warning text-dark' : 'bg-secondary';
        toolBadges += `<span class="badge ${badgeClass}" title="${T('log_tool_mode', '工具增强模式')}">${T('log_tool_mode_short', 'Tool')} ${escapeHtml(modeLabel)}</span>`;
    }
    if (log.tool_enhancement_applied) {
        const count = log.tool_call_count || 0;
        toolBadges += `<span class="badge bg-info text-dark" title="${T('log_tool_prompt_injected', '已注入工具增强提示')}">${T('log_tool_enhanced', 'Enh+')}${count > 0 ? ` (${count})` : ''}</span>`;
    }
    if (log.tool_calls_detected) {
        const count = log.tool_call_count || 0;
        toolBadges += `<span class="badge bg-success" title="${T('log_tool_detected', '检测到工具调用')}">${T('log_tool_calls', 'Tool×')}${count}</span>`;
    }
    if (log.tool_native_support !== undefined && log.tool_native_support !== null) {
        toolBadges += log.tool_native_support ?
            `<span class="badge bg-primary" title="${T('log_tool_native', '端点原生支持工具调用')}">${T('log_tool_native_short', 'Native')}</span>` :
            `<span class="badge bg-danger" title="${T('log_tool_non_native', '端点不支持工具调用')}"">${T('log_tool_non_native_short', 'NoNative')}</span>`;
    }

    return `
        ${log.error ? `<div class="alert alert-danger mb-3"><strong>${T('error', '错误')}:</strong> ${escapeHtml(log.error)}</div>` : ''}

        <div class="mb-3">
            <h6 class="mb-2">
                ${displayAttemptNum > 1 ? T('retry_attempt', '重试 #{0}').replace('{0}', displayAttemptNum - 1) : T('first_attempt', '首次尝试')}: ${escapeHtml(log.endpoint)}
                <span class="badge ${badgeClass}">${log.status_code}</span>
                <span class="badge bg-secondary">${log.duration_ms}ms</span>
                ${clientBadge}
                ${log.model ?
                    (log.model_rewrite_applied ?
                        `<span class="badge bg-success model-rewritten" title="→ ${escapeHtml(log.rewritten_model)}">${escapeHtml(log.model)}</span>` :
                        `<span class="badge bg-primary">${escapeHtml(log.model)}</span>`
                    ) : ''
                }
                ${log.is_streaming ? '<span class="badge bg-info">SSE</span>' : ''}
                ${log.content_type_override ? `<span class="badge bg-warning text-dark" title="Content-Type覆盖: ${escapeHtml(log.content_type_override)}">${escapeHtml(log.content_type_override)}</span>` : ''}
                ${requestChanges || responseChanges ? `<span class="badge bg-info">${T('has_modifications', '有修改')}</span>` : ''}
                ${toolBadges}
            </h6>
        </div>
        
        <!-- Request/Response Tabs -->
        <ul class="nav nav-tabs before-after-tabs" id="logTabs${attemptNum}" role="tablist">
            <li class="nav-item" role="presentation">
                <button class="nav-link active" id="request-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#request-${attemptNum}" type="button" role="tab">
                    ${T('request_data', '请求数据')} ${requestChanges ? '<span class="comparison-badge badge bg-warning">' + T('modified', '修改') + '</span>' : ''}
                </button>
            </li>
            <li class="nav-item" role="presentation">
                <button class="nav-link" id="response-tab-${attemptNum}" data-bs-toggle="tab" data-bs-target="#response-${attemptNum}" type="button" role="tab">
                    ${T('response_data', '响应数据')} ${responseChanges ? '<span class="comparison-badge badge bg-warning">' + T('modified', '修改') + '</span>' : ''}
                </button>
            </li>
        </ul>
        
        <div class="tab-content mt-3" id="logTabsContent${attemptNum}">
            <!-- Request Tab -->
            <div class="tab-pane fade show active" id="request-${attemptNum}" role="tabpanel">
                ${generateRequestComparisonHtml(log, attemptNum)}
            </div>
            
            <!-- Response Tab -->  
            <div class="tab-pane fade" id="response-${attemptNum}" role="tabpanel">
                ${generateResponseComparisonHtml(log, attemptNum)}
            </div>
        </div>`;
}
