#!/bin/bash

# CCCC (Claude Code Codex Companion) Configuration Script for Codex
# This script configures Codex to use your CCCC proxy instance

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

# Default values
DEFAULT_BASE_URL="http://localhost:8081"
BASE_URL=""
API_KEY=""
TEST_ONLY=false
SHOW_SETTINGS=false

# Function to show help
show_help() {
    cat << EOF
CCCC (Claude Code Codex Companion) Configuration Script for Codex

Usage: $0 [OPTIONS]

Options:
  --url URL        Set the CCCC proxy base URL (default: $DEFAULT_BASE_URL)
  --key KEY        Set the API key
  --test           Test proxy connection only (requires --url and --key)
  --show           Show current settings and exit
  --help           Show this help message

Examples:
  $0 --url http://localhost:8081 --key your-api-key-here
  $0 --test --url http://localhost:8081 --key your-api-key-here
  $0 --show

Interactive mode (no arguments):
  $0
EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --url)
            BASE_URL="$2"
            shift 2
            ;;
        --key)
            API_KEY="$2"
            shift 2
            ;;
        --test)
            TEST_ONLY=true
            shift
            ;;
        --show)
            SHOW_SETTINGS=true
            shift
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Function to backup existing configuration
backup_config() {
    if [ -f "$HOME/.codex/config.toml" ]; then
        local timestamp=$(date +%Y%m%d_%H%M%S)
        local backup_file="$HOME/.codex/config.toml.backup.$timestamp"
        local backup_auth_file="$HOME/.codex/auth.json.backup.$timestamp"
        cp "$HOME/.codex/config.toml" "$backup_file"
        if [ -f "$HOME/.codex/auth.json" ]; then
            cp "$HOME/.codex/auth.json" "$backup_auth_file"
            print_info "Backed up existing auth file to: $backup_auth_file"
        fi
        print_info "Backed up existing configuration to: $backup_file"
    fi
}

# Function to test API connection to CCCC proxy
test_api_connection() {
    local base_url="$1"
    local api_key="$2"

    print_info "Testing CCCC proxy connection..."

    # Test a simple Codex/Responses request through the proxy
    local response
    response=$(curl -s -w "%{http_code}" -o /tmp/codex_test_response \
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

    if [ "$response" = "200" ] || [ "$response" = "201" ]; then
        print_success "CCCC proxy connection successful!"
        rm -f /tmp/codex_test_response
        return 0
    elif [ "$response" = "401" ]; then
        print_error "API key authentication failed. Please check your API key."
        rm -f /tmp/codex_test_response
        return 1
    elif [ "$response" = "000" ]; then
        print_error "Cannot connect to CCCC proxy. Please check the URL and ensure the proxy is running."
        rm -f /tmp/codex_test_response
        return 1
    else
        print_error "API test failed with HTTP status: $response"
        rm -f /tmp/codex_test_response
        return 1
    fi
}

# Function to create Codex configuration for CCCC
create_codex_config() {
    local base_url="$1"
    local api_key="$2"

    # Create config directory if it doesn't exist
    mkdir -p "$HOME/.codex"

    # Create config.toml - specific to CCCC proxy
    cat > "$HOME/.codex/config.toml" << EOF
model_provider = "cccc"
model = "gpt-5-codex"
model_reasoning_effort = "high"

[model_providers.cccc]
name = "cccc"
base_url = "${base_url}/v1"
wire_api = "responses"
env_key = "CCCC_API_KEY"
EOF

    cat > "$HOME/.codex/auth.json" << EOF
{
  "OPENAI_API_KEY": "$api_key",
  "CCCC_API_KEY": "$api_key"
}
EOF

    print_success "Codex configuration written to: $HOME/.codex/config.toml"
    print_success "Codex auth file written to: $HOME/.codex/auth.json"
    return 0
}

# Function to set environment variable
set_environment_variable() {
    local api_key="$1"

    # Export for current session
    export CCCC_API_KEY="$api_key"

    # Detect shell and add to appropriate config file
    local shell_config=""
    local shell_name=""

    # First check $SHELL to determine user's default shell
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
    # Fallback to checking version variables if $SHELL is not set
    elif [ -n "$BASH_VERSION" ]; then
        shell_config="$HOME/.bashrc"
        [ -f "$HOME/.bash_profile" ] && shell_config="$HOME/.bash_profile"
    elif [ -n "$ZSH_VERSION" ]; then
        shell_config="$HOME/.zshrc"
    elif [ -n "$FISH_VERSION" ]; then
        shell_config="$HOME/.config/fish/config.fish"
    else
        shell_config="$HOME/.profile"
    fi

    print_info "Detected shell: ${shell_name:-$(basename $SHELL 2>/dev/null || echo 'unknown')}"
    print_info "Using config file: $shell_config"

    # Handle Fish shell differently (uses 'set -x' instead of 'export')
    if [ "$shell_name" = "fish" ] || [[ "$shell_config" == *"fish"* ]]; then
        # Fish shell syntax
        if [ -f "$shell_config" ] && grep -q "set -x CCCC_API_KEY" "$shell_config"; then
            # Update existing
            if [[ "$OSTYPE" == "darwin"* ]]; then
                sed -i '' "s/set -x CCCC_API_KEY.*/set -x CCCC_API_KEY \"$api_key\"\//" "$shell_config"
            else
                sed -i "s/set -x CCCC_API_KEY.*/set -x CCCC_API_KEY \"$api_key\"\//" "$shell_config"
            fi
            print_info "Updated CCCC_API_KEY in $shell_config"
        else
            # Add new
            mkdir -p "$(dirname "$shell_config")"
            echo "" >> "$shell_config"
            echo "# CCCC (Claude Code Codex Companion) API key for Codex" >> "$shell_config"
            echo "set -x CCCC_API_KEY \"$api_key\"" >> "$shell_config"
            print_info "Added CCCC_API_KEY to $shell_config"
        fi
    else
        # Bash/Zsh/sh syntax
        if [ -f "$shell_config" ] && grep -q "export CCCC_API_KEY=" "$shell_config"; then
            # Update existing
            if [[ "$OSTYPE" == "darwin"* ]]; then
                # macOS
                sed -i '' "s/export CCCC_API_KEY=.*/export CCCC_API_KEY=\"$api_key\"\//" "$shell_config"
            else
                # Linux
                sed -i "s/export CCCC_API_KEY=.*/export CCCC_API_KEY=\"$api_key\"\//" "$shell_config"
            fi
            print_info "Updated CCCC_API_KEY in $shell_config"
        else
            # Add new
            echo "" >> "$shell_config"
            echo "# CCCC (Claude Code Codex Companion) API key for Codex" >> "$shell_config"
            echo "export CCCC_API_KEY=\"$api_key\"" >> "$shell_config"
            print_info "Added CCCC_API_KEY to $shell_config"
        fi
    fi

    return 0
}

# Function to show current settings
show_current_settings() {
    print_info "Current Codex settings:"
    echo "----------------------------------------"

    if [ -f "$HOME/.codex/config.toml" ]; then
        print_info "Configuration file: $HOME/.codex/config.toml"
        echo ""
        cat "$HOME/.codex/config.toml"
        echo ""
    else
        print_info "No configuration file found at $HOME/.codex/config.toml"
    fi

    echo "----------------------------------------"
    print_info "Environment variable:"

    if [ ! -z "$CCCC_API_KEY" ]; then
        local masked_key="${CCCC_API_KEY:0:8}...${CCCC_API_KEY: -4}"
        print_info "CCCC_API_KEY: $masked_key"
    else
        print_info "CCCC_API_KEY: (not set)"
    fi

    echo "----------------------------------------"
}

# Main function
main() {
    print_info "CCCC (Claude Code Codex Companion) Configuration Script for Codex"
    echo "======================================="
    echo ""

    # Show current settings and exit if requested
    if [ "$SHOW_SETTINGS" = true ]; then
        show_current_settings
        exit 0
    fi

    # Interactive mode if no URL or key provided
    if [ -z "$BASE_URL" ] && [ -z "$API_KEY" ]; then
        print_info "Interactive setup mode"
        echo ""

        # Get base URL
        read -p "Enter CCCC proxy URL [$DEFAULT_BASE_URL]: " input_url
        BASE_URL="${input_url:-$DEFAULT_BASE_URL}"

        # Get API key
        while [ -z "$API_KEY" ]; do
            read -p "Enter your API key: " API_KEY
            if [ -z "$API_KEY" ]; then
                print_warning "API key is required"
            fi
        done
    fi

    # Validate inputs
    if [ -z "$BASE_URL" ] || [ -z "$API_KEY" ]; then
        print_error "Both URL and API key are required"
        print_info "Use --help for usage information"
        exit 1
    fi

    # Remove trailing slash from URL
    BASE_URL="${BASE_URL%/}"

    print_info "Configuration:"
    print_info "  Base URL: $BASE_URL"

    # Mask API key for display
    if [ ${#API_KEY} -gt 12 ]; then
        masked_key="${API_KEY:0:8}...${API_KEY: -4}"
    else
        masked_key="${API_KEY:0:4}..."
    fi
    print_info "  API Key: $masked_key"
    echo ""

    # Test API connection
    if ! test_api_connection "$BASE_URL" "$API_KEY"; then
        if [ "$TEST_ONLY" = true ]; then
            exit 1
        fi

        read -p "Proxy test failed. Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Setup cancelled"
            exit 1
        fi
    fi

    # Exit if test only
    if [ "$TEST_ONLY" = true ]; then
        print_success "Proxy test completed successfully"
        exit 0
    fi

    # Backup existing configuration
    backup_config

    # Create Codex configuration
    if ! create_codex_config "$BASE_URL" "$API_KEY"; then
        print_error "Failed to create Codex configuration"
        exit 1
    fi

    # Set environment variable
    if ! set_environment_variable "$API_KEY"; then
        print_warning "Failed to set environment variable automatically"
        print_info "Please set manually: export CCCC_API_KEY=\"$API_KEY\""
    fi

    echo ""
    print_success "Codex has been configured successfully for CCCC!"
    print_info "You can now use Codex with your CCCC proxy."
    print_info ""
    print_info "To apply the environment variable in your current session, run:"

    # Provide correct command based on detected shell
    local current_shell=$(basename "$SHELL" 2>/dev/null || echo "bash")
    if [ "$current_shell" = "fish" ]; then
        print_info "  set -x CCCC_API_KEY \"$API_KEY\""
    else
        print_info "  export CCCC_API_KEY=\"$API_KEY\""
    fi
    print_info "Or restart your terminal."
    print_info ""
    print_info "Configuration file: $HOME/.codex/config.toml"

    # Show current settings
    echo ""
    show_current_settings
}

# Run main function
main