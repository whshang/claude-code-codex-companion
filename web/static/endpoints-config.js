document.addEventListener('DOMContentLoaded', function() {
    const nameInput = document.getElementById('endpoint-name');
    if (nameInput) {
        nameInput.addEventListener('input', () => {
            const v = nameInput.value;
            if (v.includes('/') || v.includes('\\')) {
                nameInput.setCustomValidity('端点名称不能包含 / 或 \\');
            } else {
                nameInput.setCustomValidity('');
            }
        });
    }
});

// Endpoints Config JavaScript - 配置管理功能


function toggleAuthVisibility() {
    const authValueField = document.getElementById('endpoint-auth-value');
    const eyeIcon = document.getElementById('auth-eye-icon');
    
    if (isAuthVisible) {
        // Hide: show asterisks
        if (originalAuthValue) {
            authValueField.value = '*'.repeat(Math.min(originalAuthValue.length, 50));
        }
        authValueField.type = 'password'; // Set to password type
        eyeIcon.className = 'fas fa-eye';
        isAuthVisible = false;
    } else {
        // Show: show real value
        authValueField.value = originalAuthValue;
        authValueField.type = 'text'; // Set to text type
        eyeIcon.className = 'fas fa-eye-slash';
        isAuthVisible = true;
    }
}

function toggleOAuthVisibility(inputId, iconId) {
    const inputField = document.getElementById(inputId);
    const eyeIcon = document.getElementById(iconId);
    
    if (inputField.type === 'password') {
        inputField.type = 'text';
        eyeIcon.className = 'fas fa-eye-slash';
    } else {
        inputField.type = 'password';
        eyeIcon.className = 'fas fa-eye';
    }
}

function loadOAuthConfig(oauthConfig) {
    if (!oauthConfig) {
        // Clear OAuth fields
        document.getElementById('oauth-access-token').value = '';
        document.getElementById('oauth-refresh-token').value = '';
        document.getElementById('oauth-expires-at').value = '';
        document.getElementById('oauth-token-url').value = '';
        document.getElementById('oauth-client-id').value = '';
        document.getElementById('oauth-scopes').value = '';
        document.getElementById('oauth-auto-refresh').checked = true;
        return;
    }
    
    // Load OAuth configuration
    document.getElementById('oauth-access-token').value = oauthConfig.access_token || '';
    document.getElementById('oauth-refresh-token').value = oauthConfig.refresh_token || '';
    document.getElementById('oauth-expires-at').value = oauthConfig.expires_at || '';
    document.getElementById('oauth-token-url').value = oauthConfig.token_url || '';
    document.getElementById('oauth-client-id').value = oauthConfig.client_id || '';
    document.getElementById('oauth-auto-refresh').checked = oauthConfig.auto_refresh !== false;
    
    // Load scopes
    if (oauthConfig.scopes && Array.isArray(oauthConfig.scopes)) {
        document.getElementById('oauth-scopes').value = oauthConfig.scopes.join(', ');
    } else {
        document.getElementById('oauth-scopes').value = '';
    }
}

function resetAuthVisibility() {
    const eyeIcon = document.getElementById('auth-eye-icon');
    eyeIcon.className = 'fas fa-eye';
    isAuthVisible = false;
}

// ===== Endpoint Type and Auth Type Functions =====




function onAuthTypeChange() {
    const authType = document.getElementById('endpoint-auth-type').value;
    const authValueGroup = document.getElementById('auth-value-group');
    const oauthConfigGroup = document.getElementById('oauth-config-group');
    const authValueInput = document.getElementById('endpoint-auth-value');
    
    if (authType === 'oauth') {
        // 显示 OAuth 配置，隐藏认证值输入
        StyleUtils.hide(authValueGroup);
        StyleUtils.show(oauthConfigGroup);
        authValueInput.required = false;
        
        // OAuth 必填字段设置为必填
        document.getElementById('oauth-access-token').required = true;
        document.getElementById('oauth-refresh-token').required = true;
        document.getElementById('oauth-expires-at').required = true;
        document.getElementById('oauth-token-url').required = true;
    } else {
        // 显示认证值输入，隐藏 OAuth 配置
        StyleUtils.show(authValueGroup);
        StyleUtils.hide(oauthConfigGroup);
        authValueInput.required = true;
        
        // OAuth 字段不再必填
        document.getElementById('oauth-access-token').required = false;
        document.getElementById('oauth-refresh-token').required = false;
        document.getElementById('oauth-expires-at').required = false;
        document.getElementById('oauth-token-url').required = false;
    }
}

// Add event delegation for endpoint modal
document.addEventListener('click', function(e) {
    const action = e.target.dataset.action || e.target.closest('[data-action]')?.dataset.action;

    switch (action) {
        case 'toggle-auth-visibility':
            toggleAuthVisibility();
            break;
        case 'toggle-oauth-visibility':
            const button = e.target.closest('[data-action="toggle-oauth-visibility"]');
            const tokenId = button.dataset.tokenId;
            const eyeId = button.dataset.eyeId;
            toggleOAuthVisibility(tokenId, eyeId);
            break;
        case 'add-rewrite-rule':
            addRewriteRule();
            break;
        case 'save-endpoint':
            saveEndpoint();
            break;
        case 'reset-all-endpoints':
            resetAllEndpoints();
            break;
    }
});

// Add event delegation for change events
document.addEventListener('change', function(e) {
    const changeType = e.target.dataset.change;
    
    switch (changeType) {
        case 'endpoint-url-anthropic':
        case 'endpoint-url-openai':
            updateRequiredMarkers();
            break;
        case 'auth-type':
            onAuthTypeChange();
            break;
    }
});