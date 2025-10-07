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
                }
            });
        })
        .catch(error => console.error('Failed to refresh endpoint status:', error));
}

function updateEndpointRowStatus(row, endpoint) {
    // 使用data属性选择器,而不是依赖列的位置
    const statusCell = row.querySelector('[data-cell-type="status"]');
    const enabledCell = row.querySelector('[data-cell-type="enabled"]');

    if (!statusCell) return; // 如果找不到状态单元格,直接返回

    // Update status - 三种状态：禁用（灰色）、正常（绿色）、不可用（红色）
    let statusBadge = '';
    if (!endpoint.enabled) {
        // 如果端点被禁用，显示灰色的"禁用"状态
        statusBadge = '<span class="badge bg-secondary"><i class="fas fa-ban"></i> ' + T('disabled', '禁用') + '</span>';
    } else if (endpoint.status === 'active') {
        // 如果端点已启用且状态为活跃，显示绿色的"正常"状态
        statusBadge = '<span class="badge bg-success"><i class="fas fa-check-circle"></i> ' + T('normal', '正常') + '</span>';
    } else if (endpoint.status === 'inactive') {
        // 如果端点已启用但状态为不活跃，显示红色的"不可用"状态
        statusBadge = '<span class="badge bg-danger"><i class="fas fa-times-circle"></i> ' + T('unavailable', '不可用') + '</span>';
    } else {
        // 其他状态（如检测中）
        statusBadge = '<span class="badge bg-warning"><i class="fas fa-clock"></i> ' + T('detecting', '检测中') + '</span>';
    }
    statusCell.innerHTML = statusBadge;

    // 同时更新启用状态列
    if (enabledCell) {
        const enabledBadge = endpoint.enabled
            ? '<span class="badge bg-success"><i class="fas fa-toggle-on"></i> ' + T('enabled', '已启用') + '</span>'
            : '<span class="badge bg-secondary"><i class="fas fa-toggle-off"></i> ' + T('disabled', '已禁用') + '</span>';
        enabledCell.innerHTML = enabledBadge;
    }
}

function refreshTable() {
    // Reload endpoint data instead of refreshing the entire page
    loadEndpoints();
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

// Export loadEndpoints for use by the wizard
window.loadEndpointData = loadEndpoints;