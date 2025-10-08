/**
 * endpoints-test.js - 端点测试功能
 */

// 测试结果缓存
let testResultsCache = {};
let batchTestButtonBound = false;
let singleTestHandlerBound = false;

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

function handleSingleTestButtonClick(e) {
    const testBtn = e.target.closest('button[data-action="test-endpoint"]');
    if (testBtn) {
        const endpointName = testBtn.dataset.endpoint;
        console.log('Test button clicked:', endpointName);
        if (endpointName) {
            testEndpoint(endpointName);
        } else {
            console.error('No endpoint name found in button data');
        }
    }
}

// 从localStorage加载测试结果缓存
function loadTestResultsCache() {
    const cached = localStorage.getItem('endpointTestResults');
    if (cached) {
        try {
            testResultsCache = JSON.parse(cached);
        } catch (error) {
            console.error('Failed to load test results cache:', error);
            testResultsCache = {};
        }
    }
}

// 保存测试结果到localStorage
function saveTestResultsCache() {
    try {
        localStorage.setItem('endpointTestResults', JSON.stringify(testResultsCache));
    } catch (error) {
        console.error('Failed to save test results cache:', error);
    }
}

/**
 * 格式化响应时间显示
 */
function formatResponseTime(ms) {
    if (ms < 1000) {
        return `${ms}ms`;
    } else {
        return `${(ms / 1000).toFixed(2)}s`;
    }
}

/**
 * 渲染单个测试结果
 */
function renderTestResult(result) {
    if (!result) {
        return '<span class="text-muted">-</span>';
    }

    if (result.success) {
        // 颜色分级：<2s绿色, 2-5s黄色, 5-10s橙色, >10s红色
        const timeClass = result.response_time < 2000 ? 'text-success' :
                         result.response_time < 5000 ? 'text-warning' :
                         result.response_time < 10000 ? 'text-orange' : 'text-danger';
        const escapedUrl = escapeHtml(result.url || '');
        return `<span class="${timeClass}" data-bs-toggle="tooltip" title="${result.format.toUpperCase()}: ${escapedUrl}">
            ${formatResponseTime(result.response_time)}
        </span>`;
    } else {
        const errorMsg = result.error || `HTTP ${result.status_code}`;
        const escapedError = escapeHtml(errorMsg);
        return `<span class="text-danger" data-bs-toggle="tooltip" title="${result.format.toUpperCase()}: ${escapedError}">
            <i class="fas fa-exclamation-circle"></i> ${result.status_code || 'Error'}
        </span>`;
    }
}

/**
 * 渲染端点的所有测试结果
 */
function getEndpointTestResultsView(endpointName) {
    const results = testResultsCache[endpointName];
    if (!results) {
        return {
            html: '<span class="text-muted">-</span>',
            tooltip: 'No tests yet'
        };
    }

    const testResults = Array.isArray(results) ? results : (results.results || results.Results || []);
    if (!testResults || testResults.length === 0) {
        return {
            html: '<span class="text-muted">-</span>',
            tooltip: 'No tests yet'
        };
    }

    const htmlParts = [];
    const summaryParts = [];

    testResults.forEach(result => {
        const formatLabel = result.format === 'anthropic' ? 'A' : 'O';
        const formatBadge = result.format === 'anthropic' ? 'bg-warning' : 'bg-primary';
        const labelText = result.format === 'anthropic' ? 'Anthropic' : 'OpenAI';

        if (result.success) {
            const timeClass = result.response_time < 2000 ? 'text-success' :
                             result.response_time < 5000 ? 'text-warning' :
                             result.response_time < 10000 ? 'text-orange' : 'text-danger';
            const escapedUrl = escapeHtml(result.url || '');
            const timeText = formatResponseTime(result.response_time);
            htmlParts.push(`<div class="test-result-item" data-bs-toggle="tooltip" title="${labelText}: ${escapedUrl}&#10;Response time: ${timeText}">
                <span class="badge ${formatBadge}">${formatLabel}</span>
                <span class="${timeClass}">${timeText}</span>
            </div>`);
            summaryParts.push(`${labelText}: ${timeText}`);
        } else {
            const errorMsg = result.error || `HTTP ${result.status_code || 'Error'}`;
            const escapedUrl = escapeHtml(result.url || '');
            const escapedError = escapeHtml(errorMsg);
            htmlParts.push(`<div class="test-result-item" data-bs-toggle="tooltip" title="${labelText}: ${escapedUrl}&#10;Error: ${escapedError}">
                <span class="badge ${formatBadge}">${formatLabel}</span>
                <span class="text-danger"><i class="fas fa-exclamation-circle"></i> ${result.status_code || 'Err'}</span>
            </div>`);
            summaryParts.push(`${labelText}: ${errorMsg}`);
        }
    });

    return {
        html: `<div class="test-results-container">${htmlParts.join('')}</div>`,
        tooltip: summaryParts.join('\n')
    };
}

function renderEndpointTestResults(endpointName) {
    return getEndpointTestResultsView(endpointName).html;
}

function applyResponseCellContent(endpointName, responseCell) {
    if (!responseCell) return;
    const view = getEndpointTestResultsView(endpointName);
    responseCell.innerHTML = view.html;
    setupResponseCellTooltip(responseCell, view.tooltip);
    if (window.bootstrap && typeof bootstrap.Tooltip === 'function') {
        const tooltipEls = [].slice.call(responseCell.querySelectorAll('[data-bs-toggle="tooltip"]'));
        tooltipEls.forEach(refreshTooltip);
    }
}

function setupResponseCellTooltip(element, text) {
    if (!element) return;
    element.setAttribute('data-bs-toggle', 'tooltip');
    element.setAttribute('title', text || '');
    element.setAttribute('data-bs-original-title', text || '');
    refreshTooltip(element);
}

function setResponseCellError(responseCell, message) {
    if (!responseCell) return;
    const text = message || 'Test failed';
    responseCell.innerHTML = `<span class="text-danger"><i class="fas fa-exclamation-circle"></i> ${escapeHtml(text)}</span>`;
    setupResponseCellTooltip(responseCell, text);
}

/**
 * 测试单个端点
 */
async function testEndpoint(endpointName) {
    console.log('Starting test for endpoint:', endpointName);
    const btn = document.querySelector(`button[data-endpoint="${endpointName}"][data-action="test-endpoint"]`);
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i>';
        btn.title = '测试中...';
        btn.setAttribute('data-bs-original-title', btn.title);
        refreshTooltip(btn);
    }

    // 更新UI显示测试中状态
    const responseCell = document.querySelector(`tr[data-endpoint-name="${endpointName}"] .response-cell`);
    if (responseCell) {
        responseCell.innerHTML = '<span class="text-muted"><i class="fas fa-spinner fa-spin"></i> 测试中...</span>';
        setupResponseCellTooltip(responseCell, 'Testing...');
    }

    try {
        const response = await apiRequest(`/admin/api/endpoints/${encodeURIComponent(endpointName)}/test`, {
            method: 'POST'
        });

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}`);
        }

        const result = await response.json();
        console.log('Test result for', endpointName, ':', result);

        // 缓存测试结果
        testResultsCache[endpointName] = result;
        saveTestResultsCache();

        // 更新UI显示测试结果
        if (responseCell) {
            applyResponseCellContent(endpointName, responseCell);
        }
    } catch (error) {
        console.warn('Test endpoint error:', error);
        if (responseCell) {
            setResponseCellError(responseCell, '测试失败');
        }
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '<i class="fas fa-vial"></i>';
            btn.title = '测试端点';
            btn.setAttribute('data-bs-original-title', btn.title);
            refreshTooltip(btn);
        }
}
}

/**
 * 批量测试所有端点（流式更新）
 */
async function testAllEndpoints() {
    const btn = document.querySelector('button[data-action="test-all-endpoints"]');
    if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<i class="fas fa-spinner fa-spin"></i> 批量测试中...';
    }

    // 更新所有端点的响应列显示测试中状态
    const allRows = document.querySelectorAll('tr[data-endpoint-name]');
    allRows.forEach(row => {
        const responseCell = row.querySelector('.response-cell');
        if (responseCell) {
            responseCell.innerHTML = '<span class="text-muted"><i class="fas fa-spinner fa-spin"></i> 测试中...</span>';
            setupResponseCellTooltip(responseCell, 'Testing...');
        }
    });

    try {
        // 使用 EventSource 接收流式测试结果
        const eventSource = new EventSource('/admin/api/endpoints/test-all-stream');
        let completedCount = 0;
        const totalCount = allRows.length;

        eventSource.onmessage = function(event) {
            try {
                const result = JSON.parse(event.data);

                // 缓存测试结果
                testResultsCache[result.endpoint_name] = result;

                // 实时更新UI
                const responseCell = document.querySelector(`tr[data-endpoint-name="${result.endpoint_name}"] .response-cell`);
            if (responseCell) {
                applyResponseCellContent(result.endpoint_name, responseCell);
            }

                // 更新进度
                completedCount++;
                if (btn) {
                    btn.innerHTML = `<i class="fas fa-spinner fa-spin"></i> 测试中 (${completedCount}/${totalCount})`;
                }

                console.log('Test result received:', result.endpoint_name);
            } catch (err) {
                console.error('Failed to parse test result:', err);
            }
        };

        eventSource.addEventListener('done', function(event) {
            console.log('All tests completed');
            eventSource.close();

            // 保存缓存
            saveTestResultsCache();

            // 恢复按钮状态
            if (btn) {
                btn.disabled = false;
                btn.innerHTML = '<i class="fas fa-vial"></i> 批量测试';
            }
        });

        eventSource.onerror = function(err) {
            console.error('EventSource error:', err);
            eventSource.close();

            // 对所有仍在测试中的端点显示失败状态
            document.querySelectorAll('tr[data-endpoint-name] .response-cell').forEach(cell => {
                if (cell.innerHTML.includes('fa-spinner')) {
                    setResponseCellError(cell, '测试失败');
                }
            });

            // 恢复按钮状态
            if (btn) {
                btn.disabled = false;
                btn.innerHTML = '<i class="fas fa-vial"></i> 批量测试';
            }
        };

    } catch (error) {
        console.error('Batch test error:', error);

        // 对所有端点显示测试失败状态
        document.querySelectorAll('tr[data-endpoint-name] .response-cell').forEach(cell => {
            if (cell.innerHTML.includes('fa-spinner')) {
                setResponseCellError(cell, '测试失败');
            }
        });

        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '<i class="fas fa-vial"></i> 批量测试';
        }
    }
}

/**
 * 恢复缓存的测试结果显示
 */
function restoreCachedTestResults() {
    document.querySelectorAll('tr[data-endpoint-name]').forEach(row => {
        const endpointName = row.getAttribute('data-endpoint-name');
        if (endpointName && testResultsCache[endpointName]) {
            const responseCell = row.querySelector('.response-cell');
            if (responseCell) {
                applyResponseCellContent(endpointName, responseCell);
            }
        }
    });
}

/**
 * 初始化测试功能
 */
function initEndpointTesting() {
    const batchBtn = document.querySelector('button[data-action="test-all-endpoints"]');
    if (batchBtn && !batchBtn.dataset.testBound) {
        batchBtn.addEventListener('click', testAllEndpoints);
        batchBtn.dataset.testBound = 'true';
    }

    if (!singleTestHandlerBound) {
        document.addEventListener('click', handleSingleTestButtonClick);
        singleTestHandlerBound = true;
    }
}


// 页面加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    console.log('DOMContentLoaded - initializing endpoint testing');
    // 加载测试结果缓存
    loadTestResultsCache();

    // 初始化测试功能
    initEndpointTesting();

    // 恢复已缓存的测试结果显示
    document.querySelectorAll('tr[data-endpoint-name]').forEach(row => {
        const endpointName = row.getAttribute('data-endpoint-name');
        if (endpointName && testResultsCache[endpointName]) {
            const responseCell = row.querySelector('.response-cell');
            if (responseCell) {
                applyResponseCellContent(endpointName, responseCell);
            }
        }
    });
});

// 监听端点加载完成事件
document.addEventListener('endpointsLoaded', () => {
    console.log('Endpoints loaded - reinitializing testing');
    // 重新初始化测试功能，确保所有动态添加的按钮都能响应
    initEndpointTesting();

    // 重新应用缓存的测试结果
    document.querySelectorAll('tr[data-endpoint-name]').forEach(row => {
        const endpointName = row.getAttribute('data-endpoint-name');
        if (endpointName && testResultsCache[endpointName]) {
            const responseCell = row.querySelector('.response-cell');
            if (responseCell) {
                applyResponseCellContent(endpointName, responseCell);
            }
        }
    });
});
