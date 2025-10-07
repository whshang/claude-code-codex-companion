/**
 * endpoints-test.js - 端点测试功能
 */

// 测试结果缓存
let testResultsCache = {};

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
        return `<span class="${timeClass}" title="${result.format.toUpperCase()}: ${escapedUrl}">
            ${formatResponseTime(result.response_time)}
        </span>`;
    } else {
        const errorMsg = result.error || `HTTP ${result.status_code}`;
        const escapedError = escapeHtml(errorMsg);
        return `<span class="text-danger" title="${result.format.toUpperCase()}: ${escapedError}">
            <i class="fas fa-exclamation-circle"></i> ${result.status_code || 'Error'}
        </span>`;
    }
}

/**
 * 渲染端点的所有测试结果
 */
function renderEndpointTestResults(endpointName) {
    const results = testResultsCache[endpointName];
    if (!results) {
        return '<span class="text-muted">-</span>';
    }

    // 兼容不同数据结构：results (数组) 或包含 results 字段的对象
    const testResults = Array.isArray(results) ? results : (results.results || results.Results || []);
    if (!testResults || testResults.length === 0) {
        return '<span class="text-muted">-</span>';
    }

    const resultHtmls = testResults.map(result => {
        const formatLabel = result.format === 'anthropic' ? 'A' : 'O';
        const formatBadge = result.format === 'anthropic' ? 'badge-primary' : 'badge-info';

        if (result.success) {
            // 颜色分级：<2s绿色, 2-5s黄色, 5-10s橙色, >10s红色
            const timeClass = result.response_time < 2000 ? 'text-success' :
                             result.response_time < 5000 ? 'text-warning' :
                             result.response_time < 10000 ? 'text-orange' : 'text-danger';
            const escapedUrl = escapeHtml(result.url || '');
            return `<div class="test-result-item" title="${result.format.toUpperCase()}: ${escapedUrl}&#10;响应时间: ${formatResponseTime(result.response_time)}">
                <span class="badge ${formatBadge}">${formatLabel}</span>
                <span class="${timeClass}">${formatResponseTime(result.response_time)}</span>
            </div>`;
        } else {
            const errorMsg = result.error || `HTTP ${result.status_code}`;
            // 转义HTML以避免JSON中的引号破坏title属性
            const escapedUrl = escapeHtml(result.url || '');
            const escapedError = escapeHtml(errorMsg);
            return `<div class="test-result-item" title="${result.format.toUpperCase()}: ${escapedUrl}&#10;错误: ${escapedError}">
                <span class="badge ${formatBadge}">${formatLabel}</span>
                <span class="text-danger">
                    <i class="fas fa-exclamation-circle"></i> ${result.status_code || 'Error'}
                </span>
            </div>`;
        }
    });

    return `<div class="test-results-container">${resultHtmls.join('')}</div>`;
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
    }

    // 更新UI显示测试中状态
    const responseCell = document.querySelector(`tr[data-endpoint-name="${endpointName}"] .response-cell`);
    if (responseCell) {
        responseCell.innerHTML = '<span class="text-muted"><i class="fas fa-spinner fa-spin"></i> 测试中...</span>';
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
            responseCell.innerHTML = renderEndpointTestResults(endpointName);
        }
    } catch (error) {
        console.error('Test endpoint error:', error);
        if (responseCell) {
            responseCell.innerHTML = '<span class="text-danger"><i class="fas fa-exclamation-circle"></i> 测试失败</span>';
        }
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.innerHTML = '<i class="fas fa-vial"></i>';
            btn.title = '测试端点';
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
                    responseCell.innerHTML = renderEndpointTestResults(result.endpoint_name);
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
                    cell.innerHTML = '<span class="text-danger"><i class="fas fa-exclamation-circle"></i> 测试失败</span>';
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
                cell.innerHTML = '<span class="text-danger"><i class="fas fa-exclamation-circle"></i> 测试失败</span>';
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
                responseCell.innerHTML = renderEndpointTestResults(endpointName);
            }
        }
    });
}

/**
 * 初始化测试功能
 */
function initEndpointTesting() {
    // 批量测试按钮
    document.querySelector('button[data-action="test-all-endpoints"]')?.addEventListener('click', testAllEndpoints);

    // 单个端点测试按钮（使用事件委托）
    document.addEventListener('click', (e) => {
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
    });
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
                responseCell.innerHTML = renderEndpointTestResults(endpointName);
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
                responseCell.innerHTML = renderEndpointTestResults(endpointName);
            }
        }
    });
});
