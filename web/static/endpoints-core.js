// Endpoints Core JavaScript - 核心功能和数据管理

let currentEndpoints = [];
let editingEndpointName = null;
let endpointModal = null;
let originalAuthValue = '';
let isAuthVisible = false;

let specialSortableInstance = null;
let generalSortableInstance = null;

document.addEventListener('DOMContentLoaded', function() {
    initializeCommonFeatures();
    endpointModal = new bootstrap.Modal(document.getElementById('endpointModal'));
    loadEndpoints();
    
    // Auto-refresh every 30 seconds (only status updates)
    setInterval(refreshEndpointStatus, 30000);
    
    // Add event listeners for action buttons
    document.addEventListener('click', function(e) {
        const action = e.target.dataset.action || e.target.closest('[data-action]')?.dataset.action;
        if (action === 'show-add-endpoint-modal') {
            showAddEndpointModal();
        } else if (action === 'show-endpoint-wizard') {
            showEndpointWizard();
        } else if (action === 'auto-sort-endpoints') {
            e.preventDefault();
            autoSortEndpoints();
        } else if (action === 'set-supports') {
            e.preventDefault();
            e.stopPropagation();
            const button = e.target.closest('[data-action="set-supports"]');
            if (!button) {
                return;
            }
            const endpointName = button.dataset.endpoint;
            const mode = button.dataset.mode;
            if (endpointName && mode) {
                updateSupportsResponses(endpointName, mode, button);
            }
        }
    });
});

function initializeSortable() {
    // Destroy existing sortable instances
    if (specialSortableInstance) {
        specialSortableInstance.destroy();
        specialSortableInstance = null;
    }
    if (generalSortableInstance) {
        generalSortableInstance.destroy();
        generalSortableInstance = null;
    }

    // Initialize special endpoint list drag-and-drop sorting
    const specialTbody = document.getElementById('special-endpoint-list');
    if (specialTbody && specialTbody.children.length > 0) {
        specialSortableInstance = new Sortable(specialTbody, {
            animation: 150,
            ghostClass: 'sortable-ghost',
            chosenClass: 'sortable-chosen',
            dragClass: 'sortable-drag',
            group: 'special-endpoints', // Restrict to special endpoint group
            onStart: function(evt) {
                StyleUtils.setCursorGrabbing(true);
            },
            onEnd: function (evt) {
                StyleUtils.setCursorGrabbing(false);
                reorderEndpoints();
            }
        });
    }
    
    // Initialize general endpoint list drag-and-drop sorting
    const generalTbody = document.getElementById('general-endpoint-list');
    if (generalTbody && generalTbody.children.length > 0) {
        generalSortableInstance = new Sortable(generalTbody, {
            animation: 150,
            ghostClass: 'sortable-ghost',
            chosenClass: 'sortable-chosen',
            dragClass: 'sortable-drag',
            group: 'general-endpoints', // Restrict to general endpoint group
            onStart: function(evt) {
                StyleUtils.setCursorGrabbing(true);
            },
            onEnd: function (evt) {
                StyleUtils.setCursorGrabbing(false);
                reorderEndpoints();
            }
        });
    }
}

function loadEndpoints() {
    console.log('Loading endpoints...');
    apiRequest('/admin/api/endpoints')
        .then(response => response.json())
        .then(data => {
            console.log('Endpoints loaded:', data.endpoints.length);
            currentEndpoints = data.endpoints;
            rebuildTable(currentEndpoints);
            // Trigger custom event after table is rebuilt
            document.dispatchEvent(new CustomEvent('endpointsLoaded'));
        })
        .catch(error => {
            console.error('Failed to load endpoints:', error);
            showAlert('Failed to load endpoints', 'danger');
        });
}

function refreshEndpointStatus() {
    apiRequest('/admin/api/endpoints')
        .then(response => response.json())
        .then(data => {
            // Check if any endpoints have changed priority
            const endpointsChanged = data.endpoints.some(newEndpoint => {
                const oldEndpoint = currentEndpoints.find(ep => ep.name === newEndpoint.name);
                return oldEndpoint && oldEndpoint.priority !== newEndpoint.priority;
            });

            if (endpointsChanged) {
                // If priorities changed, reload the full table
                currentEndpoints = data.endpoints;
                rebuildTable(currentEndpoints);
            } else {
                // Only update status and statistics, not the full table
                data.endpoints.forEach(endpoint => {
                    // Try to find in special endpoint list
                    let row = document.querySelector(`#special-endpoint-list tr[data-endpoint-name="${endpoint.name}"]`);
                    if (!row) {
                        // If not found, search in general endpoint list
                        row = document.querySelector(`#general-endpoint-list tr[data-endpoint-name="${endpoint.name}"]`);
                    }
                    if (row) {
                        updateEndpointRowStatus(row, endpoint);
                        // Also update priority badge if it exists
                        const priorityBadge = row.querySelector('.priority-badge');
                        if (priorityBadge) {
                            priorityBadge.textContent = endpoint.priority;
                        }
                    }
                });
            }
        })
        .catch(error => console.error('Failed to refresh endpoint status:', error));
}

function updateEndpointRowStatus(row, endpoint) {
    // Update status cell using the correct selector
    const statusCell = row.querySelector('.status-cell');
    if (!statusCell) return;

    const badges = [];
    badges.push(endpoint.enabled ? '<span class="badge bg-success">Enabled</span>' : '<span class="badge bg-secondary text-dark">Disabled</span>');

    if (endpoint.status === 'active') {
        badges.push('<span class="badge bg-primary">Active</span>');
    } else if (endpoint.status === 'inactive') {
        badges.push('<span class="badge bg-warning text-dark">Idle</span>');
    } else {
        badges.push('<span class="badge bg-info text-dark">Check</span>');
    }

    statusCell.innerHTML = badges.join(' ');
}

function refreshTable() {
    // Reload endpoint data instead of refreshing the entire page
    loadEndpoints();
}

function buildSupportsUpdatePayload(endpoint, mode) {
    const payload = {
        enabled: endpoint.enabled,
        tags: endpoint.tags ? [...endpoint.tags] : [],
        proxy: endpoint.proxy ? { ...endpoint.proxy } : null,
        header_overrides: endpoint.header_overrides ? { ...endpoint.header_overrides } : {},
        parameter_overrides: endpoint.parameter_overrides ? { ...endpoint.parameter_overrides } : {},
        supports_responses: mode === 'native' ? true : mode === 'convert' ? false : null,
    };

    if (endpoint.model_rewrite) {
        payload.model_rewrite = JSON.parse(JSON.stringify(endpoint.model_rewrite));
    }
    if (typeof endpoint.count_tokens_enabled === 'boolean') {
        payload.count_tokens_enabled = endpoint.count_tokens_enabled;
    }

    let newPreference = endpoint.openai_preference || 'auto';
    if (mode === 'native') {
        newPreference = 'responses';
    } else if (mode === 'convert') {
        newPreference = 'chat_completions';
    } else {
        newPreference = 'auto';
    }
    payload.openai_preference = newPreference;

    return payload;
}

function updateSupportsResponses(endpointName, mode, triggerButton) {
    const endpoint = currentEndpoints.find(ep => ep.name === endpointName);
    if (!endpoint) {
        showAlert(`端点 ${endpointName} 不存在`, 'danger');
        return;
    }

    const payload = buildSupportsUpdatePayload(endpoint, mode);
    const newValue = mode === 'native' ? true : mode === 'convert' ? false : null;
    const newPreference = payload.openai_preference;

    if (triggerButton) {
        triggerButton.setAttribute('disabled', 'disabled');
        triggerButton.classList.add('loading');
    }

    apiRequest(`/admin/api/endpoints/${encodeURIComponent(endpointName)}`, {
        method: 'PUT',
        headers: {
            'Content-Type': 'application/json'
        },
        body: JSON.stringify(payload)
    })
        .then(async response => {
            const data = await response.json().catch(() => ({}));
            if (!response.ok || data.error) {
                const message = data.error || `HTTP ${response.status}`;
                throw new Error(message);
            }
            endpoint.supports_responses = newValue;
            endpoint.openai_preference = newPreference;
            const modeLabel = mode === 'native' ? '显式设置为原生响应' : mode === 'convert' ? '显式设置为强制转换' : '恢复自动探测';
            showAlert(`${endpointName}: ${modeLabel}`, 'success');
            loadEndpoints();
        })
        .catch(error => {
            console.error('Failed to update supports_responses:', error);
            showAlert(`更新 /responses 策略失败：${error.message}`, 'danger');
        })
        .finally(() => {
            if (triggerButton) {
                triggerButton.classList.remove('loading');
                triggerButton.removeAttribute('disabled');
            }
        });
}

// Show endpoint wizard modal
function showEndpointWizard() {
    if (window.endpointWizard) {
        window.endpointWizard.show();
    } else {
        console.error('Endpoint wizard not initialized');
        showAlert('向导功能尚未就绪，请稍后再试', 'warning');
    }
}

// Auto sort endpoints based on their current status
function autoSortEndpoints() {
    const button = document.querySelector('[data-action="auto-sort-endpoints"]');
    if (button) {
        button.setAttribute('disabled', 'disabled');
        button.classList.add('loading');
    }

    apiRequest('/admin/api/endpoints/sort', {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json'
        }
    })
    .then(async response => {
        const data = await response.json().catch(() => ({}));
        if (!response.ok || data.error) {
            const message = data.error || `HTTP ${response.status}`;
            throw new Error(message);
        }

        showAlert('✅ 端点已根据状态排序完成', 'success');
        // Reload endpoints to reflect the new order
        loadEndpoints();

        return data;
    })
    .catch(error => {
        console.error('Failed to auto sort endpoints:', error);
        showAlert(`端点排序失败：${error.message}`, 'danger');
    })
    .finally(() => {
        if (button) {
            button.classList.remove('loading');
            button.removeAttribute('disabled');
        }
    });
}

// Export loadEndpoints for use by the wizard
window.loadEndpointData = loadEndpoints;
