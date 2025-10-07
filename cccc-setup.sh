#!/bin/bash

# CCCC (Claude Code Codex Companion) Unified Configuration Script
# This script configures both Claude Code and Codex to use your CCCC proxy instance
# 一键配置两个CLI工具到CCCC代理

set -e

# Color functions for output
print_info() {
    echo -e "\033[34m[INFO]\033[0m $1"
}

print_success() {
    echo -e "\033[32m[SUCCESS]\033[0m $1"
}

print_warning() {
    echo -e "\033[33m[WARNING]\033[0m $1"
}

print_error() {
    echo -e "\033[31m[ERROR]\033[0m $1"
}

# Function to validate API key format
validate_api_key() {
    local api_key="$1"
    if [[ ! "$api_key" =~ ^[A-Za-z0-9_-]+$ ]]; then
        print_error "Invalid API key format. API key should contain only alphanumeric characters, hyphens, and underscores."
        return 1
    fi
    return 0
}

# Function to test API connection
test_api_connection() {
    local base_url="$1"
    local api_key="$2"

    print_info "Testing CCCC proxy connection..."

    # Test Claude Code endpoint
    local claude_response
    claude_response=$(curl -s -w "%{http_code}" -o /tmp/claude_test_response \
        -X POST "${base_url}/v1/messages" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $api_key" \
        -d '{
            "model": "claude-3-5-sonnet-20241022",
            "max_tokens": 10,
            "messages": [{"role": "user", "content": "Hi"}]
        }' \
        2>/dev/null || echo "000")

    # Test Codex endpoint
    local codex_response
    codex_response=$(curl -s -w "%{http_code}" -o /tmp/codex_test_response \
        -X POST "${base_url}/responses" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $api_key" \
        -d '{
            "model": "gpt-5-codex",
            "instructions": "You are a helpful assistant",
            "input": [{"role": "user", "content": "Hi"}],
            "max_tokens": 10
        }' \
        2>/dev/null || echo "000")

    local claude_success=false
    local codex_success=false

    if [[ "$claude_response" == "200" ]] || [[ "$claude_response" == "201" ]]; then
        print_success "Claude Code endpoint connection successful!"
        claude_success=true
    else
        print_warning "Claude Code endpoint test failed (HTTP $claude_response)"
    fi

    if [[ "$codex_response" == "200" ]] || [[ "$codex_response" == "201" ]]; then
        print_success "Codex endpoint connection successful!"
        codex_success=true
    else
        print_warning "Codex endpoint test failed (HTTP $codex_response)"
    fi

    rm -f /tmp/claude_test_response /tmp/codex_test_response

    if [[ "$claude_success" == true ]] && [[ "$codex_success" == true ]]; then
        print_success "Both endpoints are working correctly!"
        return 0
    elif [[ "$claude_success" == true ]] || [[ "$codex_success" == true ]]; then
        print_warning "Partial connection success - some endpoints may need configuration"
        return 0
    else
        print_error "Both endpoints failed to connect"
        return 1
    fi
}

# Function to backup existing configuration
backup_config() {
    local timestamp=$(date +%Y%m%d_%H%M%S)

    # Backup Claude Code settings
    if [ -f "$HOME/.claude/settings.json" ]; then
        local backup_file="$HOME/.claude/settings.json.backup.$timestamp"
        cp "$HOME/.claude/settings.json" "$backup_file"
        print_info "Backed up Claude settings to: $backup_file"
    fi

    # Backup Codex configuration
    if [ -f "$HOME/.codex/config.toml" ]; then
        local backup_file="$HOME/.codex/config.toml.backup.$timestamp"
        cp "$HOME/.codex/config.toml" "$backup_file"
        print_info "Backed up Codex config to: $backup_file"
    fi

    if [ -f "$HOME/.codex/auth.json" ]; then
        local backup_file="$HOME/.codex/auth.json.backup.$timestamp"
        cp "$HOME/.codex/auth.json" "$backup_file"
        print_info "Backed up Codex auth to: $backup_file"
    fi
}

# Function to create Claude Code settings
create_claude_settings() {
    local base_url="$1"
    local api_key="$2"

    mkdir -p "$HOME/.claude"

    cat > "$HOME/.claude/settings.json" << EOF
{
  "env": {
    "ANTHROPIC_BASE_URL": "$base_url",
    "ANTHROPIC_AUTH_TOKEN": "$api_key",
    "CLAUDE_CODE_MAX_OUTPUT_TOKENS": 20000,
    "DISABLE_TELEMETRY": 1,
    "DISABLE_ERROR_REPORTING": 1,
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": 1,
    "CLAUDE_BASH_MAINTAIN_PROJECT_WORKING_DIR": 1,
    "MAX_THINKING_TOKENS": 12000
  },
  "model": "sonnet"
}
EOF

    print_success "Claude Code settings created successfully"
}

# Function to create Codex configuration
create_codex_config() {
    local base_url="$1"
    local api_key="$2"

    mkdir -p "$HOME/.codex"

    # Create config.toml
    cat > "$HOME/.codex/config.toml" << EOF
model_provider = "cccc"
model = "gpt-5-codex"
model_reasoning_effort = "high"

[model_providers.cccc]
name = "cccc"
base_url = "${base_url}/v1"
wire_api = "responses"
env_key = "OPENAI_API_KEY"
EOF

    # Create auth.json
    cat > "$HOME/.codex/auth.json" << EOF
{
  "OPENAI_API_KEY": "$api_key",
  "CCCC_API_KEY": "$api_key"
}
EOF

    print_success "Codex configuration created successfully"
}

# Function to set environment variables
set_environment_variables() {
    local api_key="$1"

    # Export for current session
    export CCCC_API_KEY="$api_key"

    # Add to shell profile
    local shell_config=""
    local shell_name=""

    if [ -n "$SHELL" ]; then
        shell_name=$(basename "$SHELL")
        case "$shell_name" in
            bash)
                shell_config="$HOME/.bashrc"
                [ -f "$HOME/.bash_profile" ] && shell_config="$HOME/.bash_profile"
                ;;
            zsh)
                shell_config="$HOME/.zshrc"
                ;;
            fish)
                shell_config="$HOME/.config/fish/config.fish"
                ;;
            *)
                shell_config="$HOME/.profile"
                ;;
        esac
    fi

    if [ -n "$shell_config" ]; then
        if [ "$shell_name" = "fish" ] || [[ "$shell_config" == *"fish"* ]]; then
            echo "" >> "$shell_config"
            echo "# CCCC API key" >> "$shell_config"
            echo "set -x CCCC_API_KEY \"$api_key\"" >> "$shell_config"
        else
            echo "" >> "$shell_config"
            echo "# CCCC API key" >> "$shell_config"
            echo "export CCCC_API_KEY=\"$api_key\"" >> "$shell_config"
        fi
        print_info "Added CCCC_API_KEY to $shell_config"
    fi
}

# Main function
main() {
    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════╗"
    echo "║                    CCCC  Unified Configuration                       ║"
    echo "║              Claude Code & Codex Companion Setup                     ║"
    echo "╚══════════════════════════════════════════════════════════════════════╝"
    echo ""

    # Parse command line arguments
    local base_url=""
    local api_key=""
    local test_only=false
    local configure_claude=true
    local configure_codex=true

    while [[ $# -gt 0 ]]; do
        case $1 in
            -u|--url)
                base_url="$2"
                shift 2
                ;;
            -k|--key)
                api_key="$2"
                shift 2
                ;;
            -t|--test)
                test_only=true
                shift
                ;;
            --claude-only)
                configure_codex=false
                shift
                ;;
            --codex-only)
                configure_claude=false
                shift
                ;;
            -h|--help)
                cat << EOF
Usage: $0 [OPTIONS]

Options:
  -u, --url URL         Set the CCCC proxy base URL (default: http://localhost:8081)
  -k, --key KEY         Set the API key
  -t, --test            Test API connection only
  --claude-only         Configure only Claude Code
  --codex-only          Configure only Codex
  -h, --help            Show this help message

Examples:
  $0                                    # Interactive setup for both
  $0 --url http://localhost:8081 --key YOUR_KEY
  $0 --test --url http://localhost:8081 --key YOUR_KEY
  $0 --claude-only                      # Configure only Claude Code
EOF
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                print_info "Use --help for usage information"
                exit 1
                ;;
        esac
    done

    # Interactive mode if no arguments provided
    if [ -z "$base_url" ] && [ -z "$api_key" ]; then
        print_info "交互式配置模式"
        echo ""

        # Get base URL
        read -p "Enter CCCC proxy URL [http://localhost:8081]: " input_url
        base_url="${input_url:-http://localhost:8081}"

        # Get API key
        while [ -z "$api_key" ]; do
            read -p "Enter your API key: " api_key
            if [ -z "$api_key" ]; then
                print_warning "API key is required"
            elif ! validate_api_key "$api_key"; then
                api_key=""
            fi
        done

        # Ask which clients to configure
        if [ "$configure_claude" = true ] && [ "$configure_codex" = true ]; then
            echo ""
            echo "选择要配置的客户端："
            echo "1. 配置 Claude Code 和 Codex（推荐）"
            echo "2. 仅配置 Claude Code"
            echo "3. 仅配置 Codex"
            read -p "请选择 [1]: " choice
            choice=${choice:-1}

            case $choice in
                2)
                    configure_codex=false
                    ;;
                3)
                    configure_claude=false
                    ;;
                *)
                    # Both (default)
                    ;;
            esac
        fi
    fi

    # Validate inputs
    if [ -z "$base_url" ] || [ -z "$api_key" ]; then
        print_error "Both URL and API key are required"
        print_info "Use --help for usage information"
        exit 1
    fi

    # Remove trailing slash from URL
    base_url="${base_url%/}"

    echo ""
    print_info "配置信息："
    print_info "  代理地址: $base_url"
    print_info "  API 密钥: ${api_key:0:8}...${api_key: -4}"
    print_info "  配置目标: $([ "$configure_claude" = true ] && echo "Claude Code "; [ "$configure_codex" = true ] && echo "Codex")"
    echo ""

    # Test API connection
    if ! test_api_connection "$base_url" "$api_key"; then
        if [ "$test_only" = true ]; then
            exit 1
        fi

        read -p "连接测试失败，是否继续配置？(y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "配置已取消"
            exit 1
        fi
    fi

    # Exit if test only
    if [ "$test_only" = true ]; then
        print_success "连接测试完成"
        exit 0
    fi

    # Create backup
    backup_config

    # Configure clients
    if [ "$configure_claude" = true ]; then
        echo ""
        echo "══════════════════════════════════════════════════════════════════════"
        echo "                    配置 Claude Code"
        echo "══════════════════════════════════════════════════════════════════════"
        create_claude_settings "$base_url" "$api_key"
    fi

    if [ "$configure_codex" = true ]; then
        echo ""
        echo "══════════════════════════════════════════════════════════════════════"
        echo "                      配置 Codex"
        echo "══════════════════════════════════════════════════════════════════════"
        create_codex_config "$base_url" "$api_key"
    fi

    # Set environment variables
    set_environment_variables "$api_key"

    echo ""
    echo "╔══════════════════════════════════════════════════════════════════════╗"
    echo "║                    ✅ 配置完成！                                     ║"
    echo "╠══════════════════════════════════════════════════════════════════════╣"

    if [ "$configure_claude" = true ]; then
        echo "║  ✓ Claude Code 已配置完成                                            ║"
        echo "║    使用命令: claude                                                  ║"
        echo "║                                                                      ║"
    fi

    if [ "$configure_codex" = true ]; then
        echo "║  ✓ Codex 已配置完成                                                  ║"
        echo "║    使用命令: codex                                                   ║"
        echo "║                                                                      ║"
    fi

    echo "║  🌐 代理地址: $base_url                                              ║"
    echo "║                                                                      ║"
    echo "║  💡 提示: 如需重新配置，可再次运行本脚本                             ║"
    echo "║     配置已自动备份，可安全恢复                                       ║"
    echo "╚══════════════════════════════════════════════════════════════════════╝"
    echo ""
}

# Run main function
main "$@"