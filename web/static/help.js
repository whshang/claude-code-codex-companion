/* Help Page JavaScript Functions */

// Current client type
let currentClient = 'claude-code';

// Detect OS and set active tab
function detectOS() {
    const userAgent = navigator.userAgent.toLowerCase();
    if (userAgent.indexOf('win') > -1) return 'windows';
    if (userAgent.indexOf('mac') > -1) return 'macos';
    if (userAgent.indexOf('linux') > -1) return 'linux';
    return 'windows'; // default
}

// Switch between Claude Code and Codex
function switchClient(client) {
    currentClient = client;
    console.log('Switched to client:', client);

    // Update description text
    const description = document.querySelector('[data-t="generate_script_description"]');
    if (description) {
        if (client === 'codex') {
            description.textContent = '生成启动脚本来使用此代理服务器。脚本会自动配置 Codex 的 config.toml 和 auth.json 文件，并启动 Codex。';
        } else {
            description.textContent = '生成启动脚本来使用此代理服务器。脚本会自动设置必要的环境变量并启动 Claude Code。';
        }
    }

    // Update smart configuration description
    const smartConfigDesc = document.querySelector('[data-t="smart_configuration_desc"]');
    if (smartConfigDesc) {
        if (client === 'codex') {
            smartConfigDesc.textContent = '脚本会自动配置 Codex 的 config.toml 和 auth.json 文件，确保代理配置始终生效。';
        } else {
            smartConfigDesc.textContent = '脚本会自动检测并处理 Claude Code 设置文件，确保代理配置始终生效。';
        }
    }

    // Toggle config info sections
    const claudeConfigInfo = document.getElementById('claude-config-info');
    const codexConfigInfo = document.getElementById('codex-config-info');
    if (claudeConfigInfo && codexConfigInfo) {
        if (client === 'codex') {
            claudeConfigInfo.style.display = 'none';
            codexConfigInfo.style.display = 'block';
        } else {
            claudeConfigInfo.style.display = 'block';
            codexConfigInfo.style.display = 'none';
        }
    }

    // Toggle script description for all platforms
    const platforms = ['win', 'mac', 'linux'];
    platforms.forEach(platform => {
        const claudeDesc = document.getElementById(`script-description-claude-${platform}`);
        const codexDesc = document.getElementById(`script-description-codex-${platform}`);
        if (claudeDesc && codexDesc) {
            if (client === 'codex') {
                claudeDesc.style.display = 'none';
                codexDesc.style.display = 'inline';
            } else {
                claudeDesc.style.display = 'inline';
                codexDesc.style.display = 'none';
            }
        }
    });

    // Update script titles
    updateScriptTitles(client);

    // Update filenames if in single mode
    const scriptType = document.querySelector('input[name="scriptType"]:checked')?.id;
    if (scriptType === 'scriptSingle') {
        updateFilenames(client);
    }
}

// Update script titles based on client
function updateScriptTitles(client) {
    const clientName = client === 'codex' ? 'Codex' : 'Claude Code';
    const scriptTitleElements = document.querySelectorAll('.card-body h5');
    scriptTitleElements.forEach(el => {
        if (el.textContent.includes('Windows') || el.textContent.includes('macOS') || el.textContent.includes('Linux')) {
            // Don't change these, they're OS names
        }
    });
}

// Select client from top section
function selectClient(client) {
    // Update radio buttons
    if (client === 'codex') {
        document.getElementById('clientCodex').checked = true;
    } else {
        document.getElementById('clientClaudeCode').checked = true;
    }
    switchClient(client);
    
    // Scroll to scripts section
    document.getElementById('scripts').scrollIntoView({ behavior: 'smooth' });
}

// Initialize tabs based on detected OS
function initializeOSTabs() {
    const detectedOS = detectOS();
    console.log('Detected OS:', detectedOS);
    
    // Always ensure that the correct tab is shown for the detected OS
    // First, remove active from all installation tabs
    document.querySelectorAll('#osTabs .nav-link').forEach(tab => {
        tab.classList.remove('active');
        tab.setAttribute('aria-selected', 'false');
    });
    document.querySelectorAll('#osTabContent .tab-pane').forEach(pane => {
        pane.classList.remove('show', 'active');
    });
    
    // Then activate the correct installation tab
    const installationTab = document.getElementById(detectedOS + '-tab');
    const installationPane = document.getElementById(detectedOS);
    if (installationTab && installationPane) {
        installationTab.classList.add('active');
        installationTab.setAttribute('aria-selected', 'true');
        installationPane.classList.add('show', 'active');
    }
    
    // Do the same for script tabs
    document.querySelectorAll('#scriptTabs .nav-link').forEach(tab => {
        tab.classList.remove('active');
        tab.setAttribute('aria-selected', 'false');
    });
    document.querySelectorAll('#scriptTabContent .tab-pane').forEach(pane => {
        pane.classList.remove('show', 'active');
    });
    
    // Then activate the correct script tab
    const scriptTab = document.getElementById(detectedOS + '-script-tab');
    const scriptPane = document.getElementById(detectedOS + '-script');
    if (scriptTab && scriptPane) {
        scriptTab.classList.add('active');
        scriptTab.setAttribute('aria-selected', 'true');
        scriptPane.classList.add('show', 'active');
    }
}

// Copy to clipboard function for help page
function copyToClipboard(button) {
    const codeBlock = button.parentNode.querySelector('.code-block');
    const text = codeBlock.innerText;
    
    navigator.clipboard.writeText(text).then(function() {
        const icon = button.querySelector('i');
        const originalClass = icon.className;
        icon.className = 'fas fa-check';
        button.classList.remove('btn-outline-light');
        button.classList.add('btn-success');
        
        setTimeout(function() {
            icon.className = originalClass;
            button.classList.remove('btn-success');
            button.classList.add('btn-outline-light');
        }, 2000);
    });
}

// Download script function
function downloadScript(platform, scriptType = 'current') {
    let content, filename;
    
    if (scriptType === 'both') {
        // 下载一键配置双客户端的脚本
        switch(platform) {
            case 'windows':
                content = generateSetupWindowsScript();
                filename = 'cccc-setup.bat';
                break;
            case 'macos':
                content = generateSetupUnixScript('macOS');
                filename = 'cccc-setup.command';
                break;
            case 'linux':
                content = generateSetupUnixScript('Linux');
                filename = 'cccc-setup.sh';
                break;
        }
    } else if (scriptType === 'codex' || currentClient === 'codex') {
        // Codex 专用脚本
        switch(platform) {
            case 'windows':
                content = generateCodexWindowsScript();
                filename = 'cccc-codex.bat';
                break;
            case 'macos':
                content = generateCodexUnixScript('macOS');
                filename = 'cccc-codex.command';
                break;
            case 'linux':
                content = generateCodexUnixScript('Linux');
                filename = 'cccc-codex.sh';
                break;
        }
    } else {
        // Claude Code 专用脚本
        switch(platform) {
            case 'windows':
                content = generateEnhancedWindowsScript();
                filename = 'cccc-claude.bat';
                break;
            case 'macos':
                content = generateEnhancedUnixScript('macOS');
                filename = 'cccc-claude.command';
                break;
            case 'linux':
                content = generateEnhancedUnixScript('Linux');
                filename = 'cccc-claude.sh';
                break;
        }
    }

    const blob = new Blob([content], { type: 'text/plain' });
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    window.URL.revokeObjectURL(url);
}

// Generate enhanced Windows script (Claude Code)
function generateEnhancedWindowsScript() {
    const baseUrl = window.location.protocol + '//' + window.location.host;
    
    return `@echo off
REM CCCC - Claude Code Launcher for Windows
REM Usage: cccc-claude.bat [arguments]
echo Configuring Claude Code for CCCC proxy...

REM Set environment variables
set ANTHROPIC_BASE_URL=${baseUrl}
set ANTHROPIC_AUTH_TOKEN=hello
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
set API_TIMEOUT_MS=600000

REM Update settings.json if Node.js is available
node -e "const fs=require('fs');const path=require('path');const os=require('os');const claudeDir=path.join(os.homedir(),'.claude');const settingsFile=path.join(claudeDir,'settings.json');const targetEnv={'ANTHROPIC_BASE_URL':'${baseUrl}','ANTHROPIC_AUTH_TOKEN':'hello','CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','API_TIMEOUT_MS':'600000'};function processSettings(){if(!fs.existsSync(settingsFile)){console.log('Claude settings file not found, environment variables already set');return true;}try{const content=fs.readFileSync(settingsFile,'utf8');const settings=JSON.parse(content);if(!settings.env)settings.env={};let needsUpdate=false;let backupCreated=false;for(const[key,targetValue]of Object.entries(targetEnv)){const currentValue=settings.env[key];if(currentValue!==targetValue){if(!backupCreated){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');const backupFile=settingsFile+'.backup-'+timestamp;fs.copyFileSync(settingsFile,backupFile);console.log('Backed up settings to: '+backupFile);backupCreated=true;}if(currentValue){console.log('Updating '+key+': '+currentValue+' -> '+targetValue);}else{console.log('Adding '+key+': '+targetValue);}settings.env[key]=targetValue;needsUpdate=true;}}if(needsUpdate){fs.writeFileSync(settingsFile,JSON.stringify(settings,null,2));console.log('Settings updated successfully');}else{console.log('Settings already configured correctly');}return true;}catch(error){console.error('Error processing settings:',error.message);console.log('Environment variables already set as fallback');return false;}}processSettings();" >nul 2>&1

echo Starting Claude Code...
claude %*`;
}

// Generate enhanced Unix script (Linux/macOS) for Claude Code
function generateEnhancedUnixScript(osName) {
    const baseUrl = window.location.protocol + '//' + window.location.host;
    
    return `#!/bin/bash
# CCCC - Claude Code Launcher for ${osName}
# Usage: ./cccc-claude.sh [arguments]
echo "Configuring Claude Code for CCCC proxy..."

# Set environment variables
export ANTHROPIC_BASE_URL="${baseUrl}"
export ANTHROPIC_AUTH_TOKEN="hello"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"
export API_TIMEOUT_MS="600000"

# Update settings.json if Node.js is available
node -e "const fs=require('fs');const path=require('path');const os=require('os');const claudeDir=path.join(os.homedir(),'.claude');const settingsFile=path.join(claudeDir,'settings.json');const targetEnv={'ANTHROPIC_BASE_URL':'${baseUrl}','ANTHROPIC_AUTH_TOKEN':'hello','CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','API_TIMEOUT_MS':'600000'};function processSettings(){if(!fs.existsSync(settingsFile)){console.log('Claude settings file not found, environment variables already set');return true;}try{const content=fs.readFileSync(settingsFile,'utf8');const settings=JSON.parse(content);if(!settings.env)settings.env={};let needsUpdate=false;let backupCreated=false;for(const[key,targetValue]of Object.entries(targetEnv)){const currentValue=settings.env[key];if(currentValue!==targetValue){if(!backupCreated){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');const backupFile=settingsFile+'.backup-'+timestamp;fs.copyFileSync(settingsFile,backupFile);console.log('Backed up settings to: '+backupFile);backupCreated=true;}if(currentValue){console.log('Updating '+key+': '+currentValue+' -> '+targetValue);}else{console.log('Adding '+key+': '+targetValue);}settings.env[key]=targetValue;needsUpdate=true;}}if(needsUpdate){fs.writeFileSync(settingsFile,JSON.stringify(settings,null,2));console.log('Settings updated successfully');}else{console.log('Settings already configured correctly');}return true;}catch(error){console.error('Error processing settings:',error.message);console.log('Environment variables already set as fallback');return false;}}processSettings();" >/dev/null 2>&1

echo "Starting Claude Code..."
exec claude "$@"`;
}

// Generate Codex Windows script
function generateCodexWindowsScript() {
    const baseUrl = window.location.protocol + '//' + window.location.host;

    return `@echo off
REM CCCC - Codex Launcher for Windows
REM Usage: cccc-codex.bat [arguments]
echo Configuring Codex for CCCC proxy...

REM Configure Codex config.toml and auth.json
node -e "const fs=require('fs');const path=require('path');const os=require('os');const codexDir=path.join(os.homedir(),'.codex');if(!fs.existsSync(codexDir)){fs.mkdirSync(codexDir,{recursive:true});console.log('Created .codex directory');}const configFile=path.join(codexDir,'config.toml');const authFile=path.join(codexDir,'auth.json');const configContent='model_provider = \\"cccc\\"\\nmodel = \\"gpt-5\\"\\nmodel_reasoning_effort = \\"high\\"\\ndisable_response_storage = true\\n\\n[model_providers.cccc]\\nname = \\"cccc\\"\\nbase_url = \\"${baseUrl}/v1\\"\\nwire_api = \\"responses\\"\\nrequires_openai_auth = true';const authContent={OPENAI_API_KEY:'hello'};if(fs.existsSync(configFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(configFile,path.join(codexDir,'config.toml.backup-'+timestamp));console.log('Backed up existing config.toml');}fs.writeFileSync(configFile,configContent);console.log('Codex config.toml configured successfully');if(fs.existsSync(authFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(authFile,path.join(codexDir,'auth.json.backup-'+timestamp));console.log('Backed up existing auth.json');}fs.writeFileSync(authFile,JSON.stringify(authContent,null,2));console.log('Codex auth.json configured successfully');" 2>nul
if errorlevel 1 (
    echo [ERROR] Codex configuration failed. Please ensure Node.js is installed.
    pause
    exit /b 1
)

echo.
echo Configuration complete!
echo - Model: gpt-5 ^(via CCCC proxy^)
echo - Proxy URL: ${baseUrl}/v1
echo.
echo Starting Codex...
codex %*`;
}

// Generate Codex Unix script (Linux/macOS)
function generateCodexUnixScript(osName) {
    const baseUrl = window.location.protocol + '//' + window.location.host;

    return `#!/bin/bash
# CCCC - Codex Launcher for ${osName}
# Usage: ./cccc-codex.sh [arguments]
echo "Configuring Codex for CCCC proxy..."

# Configure Codex config.toml and auth.json
node -e "const fs=require('fs');const path=require('path');const os=require('os');const codexDir=path.join(os.homedir(),'.codex');if(!fs.existsSync(codexDir)){fs.mkdirSync(codexDir,{recursive:true});console.log('Created .codex directory');}const configFile=path.join(codexDir,'config.toml');const authFile=path.join(codexDir,'auth.json');const configContent='model_provider = \\"cccc\\"\\nmodel = \\"gpt-5\\"\\nmodel_reasoning_effort = \\"high\\"\\ndisable_response_storage = true\\n\\n[model_providers.cccc]\\nname = \\"cccc\\"\\nbase_url = \\"${baseUrl}/v1\\"\\nwire_api = \\"responses\\"\\nrequires_openai_auth = true';const authContent={OPENAI_API_KEY:'hello'};if(fs.existsSync(configFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(configFile,path.join(codexDir,'config.toml.backup-'+timestamp));console.log('Backed up existing config.toml');}fs.writeFileSync(configFile,configContent);console.log('Codex config.toml configured successfully');if(fs.existsSync(authFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(authFile,path.join(codexDir,'auth.json.backup-'+timestamp));console.log('Backed up existing auth.json');}fs.writeFileSync(authFile,JSON.stringify(authContent,null,2));console.log('Codex auth.json configured successfully');" 2>/dev/null

if [ $? -ne 0 ]; then
    echo "[ERROR] Codex configuration failed. Please ensure Node.js is installed."
    exit 1
fi

echo ""
echo "Configuration complete!"
echo "- Model: gpt-5 (via CCCC proxy)"
echo "- Proxy URL: ${baseUrl}/v1"
echo ""
echo "Starting Codex..."
exec codex "$@"`;
}

// Generate Setup Windows script (Both clients)
function generateSetupWindowsScript() {
    const baseUrl = window.location.protocol + '//' + window.location.host;
    
    return `@echo off
REM CCCC - Setup Script for Windows (Claude Code + Codex)
REM This script configures both Claude Code and Codex to use CCCC proxy
echo.
echo ========================================
echo  CCCC Setup - Windows
echo ========================================
echo.

echo Configuring Claude Code...
echo ----------------------------
set ANTHROPIC_BASE_URL=${baseUrl}
set ANTHROPIC_AUTH_TOKEN=hello
set CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1
set API_TIMEOUT_MS=600000

node -e "const fs=require('fs');const path=require('path');const os=require('os');const claudeDir=path.join(os.homedir(),'.claude');if(!fs.existsSync(claudeDir)){fs.mkdirSync(claudeDir,{recursive:true});}const settingsFile=path.join(claudeDir,'settings.json');const targetEnv={'ANTHROPIC_BASE_URL':'${baseUrl}','ANTHROPIC_AUTH_TOKEN':'hello','CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','API_TIMEOUT_MS':'600000'};let settings={};if(fs.existsSync(settingsFile)){try{settings=JSON.parse(fs.readFileSync(settingsFile,'utf8'));}catch(e){settings={};}}if(!settings.env)settings.env={};let needsUpdate=false;let backupCreated=false;for(const[key,targetValue]of Object.entries(targetEnv)){if(settings.env[key]!==targetValue){if(!backupCreated&&fs.existsSync(settingsFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');const backupFile=settingsFile+'.backup-'+timestamp;fs.copyFileSync(settingsFile,backupFile);console.log('Backed up Claude settings to:',backupFile);backupCreated=true;}settings.env[key]=targetValue;needsUpdate=true;}}if(needsUpdate){fs.writeFileSync(settingsFile,JSON.stringify(settings,null,2));console.log('Claude Code configured successfully');}else{console.log('Claude Code already configured');}" 2>nul
if errorlevel 1 echo [Note] Claude settings.json update failed, but environment variables are set

echo.
echo Configuring Codex...
echo --------------------
node -e "const fs=require('fs');const path=require('path');const os=require('os');const codexDir=path.join(os.homedir(),'.codex');if(!fs.existsSync(codexDir)){fs.mkdirSync(codexDir,{recursive:true});console.log('Created .codex directory');}const configFile=path.join(codexDir,'config.toml');const authFile=path.join(codexDir,'auth.json');const configContent='model_provider = \\"cccc\\"\\nmodel = \\"gpt-5\\"\\nmodel_reasoning_effort = \\"high\\"\\ndisable_response_storage = true\\n\\n[model_providers.cccc]\\nname = \\"cccc\\"\\nbase_url = \\"${baseUrl}/v1\\"\\nwire_api = \\"responses\\"\\nrequires_openai_auth = true';const authContent={OPENAI_API_KEY:'hello'};if(fs.existsSync(configFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(configFile,path.join(codexDir,'config.toml.backup-'+timestamp));console.log('Backed up existing config.toml');}fs.writeFileSync(configFile,configContent);console.log('Codex config.toml configured successfully');if(fs.existsSync(authFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(authFile,path.join(codexDir,'auth.json.backup-'+timestamp));console.log('Backed up existing auth.json');}fs.writeFileSync(authFile,JSON.stringify(authContent,null,2));console.log('Codex auth.json configured successfully');" 2>nul
if errorlevel 1 echo [Note] Codex configuration failed, please configure manually

echo.
echo ========================================
echo  Setup Complete!
echo ========================================
echo.
echo Claude Code: Use 'claude' command as usual
echo Codex: Use 'codex' command as usual
echo.
echo Both clients are now configured to use CCCC proxy at ${baseUrl}
echo.
pause`;
}

// Generate Setup Unix script (Both clients)
function generateSetupUnixScript(osName) {
    const baseUrl = window.location.protocol + '//' + window.location.host;
    
    return `#!/bin/bash
# CCCC - Setup Script for ${osName} (Claude Code + Codex)
# This script configures both Claude Code and Codex to use CCCC proxy

echo ""
echo "========================================"
echo " CCCC Setup - ${osName}"
echo "========================================"
echo ""

echo "Configuring Claude Code..."
echo "----------------------------"
export ANTHROPIC_BASE_URL="${baseUrl}"
export ANTHROPIC_AUTH_TOKEN="hello"
export CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC="1"
export API_TIMEOUT_MS="600000"

node -e "const fs=require('fs');const path=require('path');const os=require('os');const claudeDir=path.join(os.homedir(),'.claude');if(!fs.existsSync(claudeDir)){fs.mkdirSync(claudeDir,{recursive:true});}const settingsFile=path.join(claudeDir,'settings.json');const targetEnv={'ANTHROPIC_BASE_URL':'${baseUrl}','ANTHROPIC_AUTH_TOKEN':'hello','CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC':'1','API_TIMEOUT_MS':'600000'};let settings={};if(fs.existsSync(settingsFile)){try{settings=JSON.parse(fs.readFileSync(settingsFile,'utf8'));}catch(e){settings={};}}if(!settings.env)settings.env={};let needsUpdate=false;let backupCreated=false;for(const[key,targetValue]of Object.entries(targetEnv)){if(settings.env[key]!==targetValue){if(!backupCreated&&fs.existsSync(settingsFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');const backupFile=settingsFile+'.backup-'+timestamp;fs.copyFileSync(settingsFile,backupFile);console.log('Backed up Claude settings to:',backupFile);backupCreated=true;}settings.env[key]=targetValue;needsUpdate=true;}}if(needsUpdate){fs.writeFileSync(settingsFile,JSON.stringify(settings,null,2));console.log('Claude Code configured successfully');}else{console.log('Claude Code already configured');}" 2>/dev/null
[ $? -ne 0 ] && echo "[Note] Claude settings.json update failed, but environment variables are set"

echo ""
echo "Configuring Codex..."
echo "--------------------"
node -e "const fs=require('fs');const path=require('path');const os=require('os');const codexDir=path.join(os.homedir(),'.codex');if(!fs.existsSync(codexDir)){fs.mkdirSync(codexDir,{recursive:true});console.log('Created .codex directory');}const configFile=path.join(codexDir,'config.toml');const authFile=path.join(codexDir,'auth.json');const configContent='model_provider = \\"cccc\\"\\nmodel = \\"gpt-5\\"\\nmodel_reasoning_effort = \\"high\\"\\ndisable_response_storage = true\\n\\n[model_providers.cccc]\\nname = \\"cccc\\"\\nbase_url = \\"${baseUrl}/v1\\"\\nwire_api = \\"responses\\"\\nrequires_openai_auth = true';const authContent={OPENAI_API_KEY:'hello'};if(fs.existsSync(configFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(configFile,path.join(codexDir,'config.toml.backup-'+timestamp));console.log('Backed up existing config.toml');}fs.writeFileSync(configFile,configContent);console.log('Codex config.toml configured successfully');if(fs.existsSync(authFile)){const timestamp=new Date().toISOString().replace(/[:.]/g,'-');fs.copyFileSync(authFile,path.join(codexDir,'auth.json.backup-'+timestamp));console.log('Backed up existing auth.json');}fs.writeFileSync(authFile,JSON.stringify(authContent,null,2));console.log('Codex auth.json configured successfully');" 2>/dev/null
[ $? -ne 0 ] && echo "[Note] Codex configuration failed, please configure manually"

echo ""
echo "========================================"
echo " Setup Complete!"
echo "========================================"
echo ""
echo "Claude Code: Use 'claude' command as usual"
echo "Codex: Use 'codex' command as usual"
echo ""
echo "Both clients are now configured to use CCCC proxy at ${baseUrl}"
echo ""`;
}

// Download script based on selected type
function downloadScriptByType(platform) {
    const scriptType = document.querySelector('input[name="scriptType"]:checked').id;
    
    if (scriptType === 'scriptBoth') {
        // Download setup script for both clients
        downloadScript(platform, 'both');
    } else {
        // Download single client script
        downloadScript(platform, currentClient);
    }
}

// Switch script type (single or both)
function switchScriptType() {
    const scriptType = document.querySelector('input[name="scriptType"]:checked').id;
    const clientSelector = document.getElementById('clientSelectorSection');
    
    if (scriptType === 'scriptBoth') {
        // Hide client selector for "both" mode
        clientSelector.style.display = 'none';
        updateFilenames('both');
    } else {
        // Show client selector for single mode
        clientSelector.style.display = 'block';
        updateFilenames(currentClient);
    }
}

// Update filename displays
function updateFilenames(mode) {
    const filenames = {
        'claude-code': {
            windows: 'cccc-claude.bat',
            macos: 'cccc-claude.command',
            linux: 'cccc-claude.sh'
        },
        'codex': {
            windows: 'cccc-codex.bat',
            macos: 'cccc-codex.command',
            linux: 'cccc-codex.sh'
        },
        'both': {
            windows: 'cccc-setup.bat',
            macos: 'cccc-setup.command',
            linux: 'cccc-setup.sh'
        }
    };
    
    const names = filenames[mode];
    document.getElementById('windows-filename').textContent = `(${names.windows})`;
    document.getElementById('macos-filename').textContent = `(${names.macos})`;
    document.getElementById('linux-filename').textContent = `(${names.linux})`;
}

// Initialize help page on DOM ready
document.addEventListener('DOMContentLoaded', function() {
    initializeOSTabs();
    
    // Add script type change listeners
    document.querySelectorAll('input[name="scriptType"]').forEach(radio => {
        radio.addEventListener('change', switchScriptType);
    });
    
    // Check URL parameters for client type
    const urlParams = new URLSearchParams(window.location.search);
    const clientParam = urlParams.get('client');
    if (clientParam === 'codex') {
        selectClient('codex');
    }
});

// Copy setup command to clipboard
function copySetupCommand(clientType) {
    const baseUrl = window.location.protocol + '//' + window.location.host;
    const command = clientType === 'claude-code'
        ? `./cccc-setup-claude-code.sh --url ${baseUrl} --key YOUR_API_KEY`
        : `./cccc-setup-codex.sh --url ${baseUrl} --key YOUR_API_KEY`;

    navigator.clipboard.writeText(command).then(function() {
        // Show success feedback
        const button = event.target.closest('button');
        const originalHTML = button.innerHTML;
        button.innerHTML = '<i class="fas fa-check"></i> 已复制';
        button.classList.remove('btn-primary', 'btn-success');
        button.classList.add('btn-success');

        setTimeout(function() {
            button.innerHTML = originalHTML;
            button.classList.remove('btn-success');
            button.classList.add(clientType === 'claude-code' ? 'btn-primary' : 'btn-success');
        }, 2000);
    }).catch(function(err) {
        console.error('Failed to copy command: ', err);
        alert('复制命令失败，请手动复制');
    });
}