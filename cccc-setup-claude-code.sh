#!/bin/bash

# CCCC (Claude Code Codex Companion) Configuration Script for Claude Code
# This script configures Claude Code to use your CCCC proxy instance

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
DEFAULT_BASE_URL="http://localhost:8081"
CLAUDE_CONFIG_DIR="$HOME/.claude"
CLAUDE_SETTINGS_FILE="$CLAUDE_CONFIG_DIR/settings.json"

# Function to print colored output
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Function to check if jq is installed
check_jq() {
    if ! command -v jq &> /dev/null; then
        print_error "jq is required but not installed."
        print_info "Please install jq:"
        print_info "  macOS: brew install jq"
        print_info "  Ubuntu/Debian: sudo apt-get install jq"
        print_info "  CentOS/RHEL: sudo yum install jq"
        exit 1
    fi
}

# Function to backup existing settings
backup_settings() {
    if [ -f "$CLAUDE_SETTINGS_FILE" ]; then
        local backup_file="${CLAUDE_SETTINGS_FILE}.backup.$(date +%Y%m%d_%H%M%S)"
        cp "$CLAUDE_SETTINGS_FILE" "$backup_file"
        print_info "Backed up existing settings to: $backup_file"
    fi
}

# Function to create settings directory
create_settings_dir() {
    if [ ! -d "$CLAUDE_CONFIG_DIR" ]; then
        mkdir -p "$CLAUDE_CONFIG_DIR"
        print_info "Created Claude configuration directory: $CLAUDE_CONFIG_DIR"
    fi
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

# Function to test API connection to CCCC proxy
test_api_connection() {
    local base_url="$1"
    local api_key="$2"

    print_info "Testing CCCC proxy connection..."

    # Test a simple Claude Code request through the proxy
    local response
    response=$(curl -s -w "%{http_code}" -o /tmp/claude_test_response \
        -X POST "${base_url}/v1/messages" \
        -H "Content-Type: application/json" \
        -H "X-API-Key: $api_key" \
        -d '{
            "model": "claude-3-5-sonnet-20241022",
            "max_tokens": 10,
            "messages": [{"role": "user", "content": "Hi"}]
        }' \
        2>/dev/null || echo "000")

    if [ "$response" = "200" ] || [ "$response" = "201" ]; then
        print_success "CCCC proxy connection successful!"
        rm -f /tmp/claude_test_response
        return 0
    elif [ "$response" = "401" ]; then
        print_error "API key authentication failed. Please check your API key."
        rm -f /tmp/claude_test_response
        return 1
    elif [ "$response" = "000" ]; then
        print_error "Cannot connect to CCCC proxy. Please check the URL and ensure the proxy is running."
        rm -f /tmp/claude_test_response
        return 1
    else
        print_error "API test failed with HTTP status: $response"
        rm -f /tmp/claude_test_response
        return 1
    fi
}

# Function to create Claude Code settings for CCCC
create_settings() {
    local base_url="$1"
    local api_key="$2"

    local settings_json
    settings_json=$(cat <<EOF
{
  "env": {
    "ANTHROPIC_BASE_URL": "${base_url}",
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
    )

    # Validate JSON
    if ! echo "$settings_json" | jq . > /dev/null 2>&1; then
        print_error "Generated settings JSON is invalid"
        return 1
    fi

    # Write settings file
    echo "$settings_json" > "$CLAUDE_SETTINGS_FILE"
    print_success "Claude Code settings written to: $CLAUDE_SETTINGS_FILE"
}

# Function to display current settings
display_settings() {
    if [ -f "$CLAUDE_SETTINGS_FILE" ]; then
        print_info "Current Claude Code settings:"
        echo "----------------------------------------"
        cat "$CLAUDE_SETTINGS_FILE" | jq .
        echo "----------------------------------------"
    else
        print_info "No existing Claude Code settings found."
    fi
}

# Main function
main() {
    print_info "CCCC (Claude Code Codex Companion) Configuration Script for Claude Code"
    echo "======================================================="
    echo

    # Check dependencies
    check_jq

    # Parse command line arguments
    local base_url=""
    local api_key=""
    local test_only=false
    local show_settings=false

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
            -s|--show)
                show_settings=true
                shift
                ;;
            -h|--help)
                cat <<EOF
Usage: $0 [OPTIONS]

Options:
  -u, --url URL     Set the CCCC proxy base URL (default: $DEFAULT_BASE_URL)
  -k, --key KEY     Set the API key
  -t, --test        Test proxy connection only (requires -u and -k)
  -s, --show        Show current settings and exit
  -h, --help        Show this help message

Examples:
  $0 --url http://localhost:8081 --key your-api-key-here
  $0 --test --url http://localhost:8081 --key your-api-key-here
  $0 --show

Interactive mode (no arguments):
  $0
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

    # Show settings and exit if requested
    if [ "$show_settings" = true ]; then
        display_settings
        exit 0
    fi

    # Interactive mode if no arguments provided
    if [ -z "$base_url" ] && [ -z "$api_key" ]; then
        print_info "Interactive setup mode"
        echo

        # Get base URL
        read -p "Enter CCCC proxy URL [$DEFAULT_BASE_URL]: " base_url
        if [ -z "$base_url" ]; then
            base_url="$DEFAULT_BASE_URL"
        fi

        # Get API key
        while [ -z "$api_key" ]; do
            read -p "Enter your API key: " api_key
            if [ -z "$api_key" ]; then
                print_warning "API key is required"
            elif ! validate_api_key "$api_key"; then
                api_key=""
            fi
        done
    fi

    # Validate inputs
    if [ -z "$base_url" ] || [ -z "$api_key" ]; then
        print_error "Both URL and API key are required"
        print_info "Use --help for usage information"
        exit 1
    fi

    # Validate API key
    if ! validate_api_key "$api_key"; then
        exit 1
    fi

    # Remove trailing slash from URL
    base_url="${base_url%/}"

    print_info "Configuration:"
    print_info "  Base URL: $base_url"
    print_info "  API Key: ${api_key:0:8}...${api_key: -4}"
    echo

    # Test API connection
    if ! test_api_connection "$base_url" "$api_key"; then
        if [ "$test_only" = true ]; then
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
    if [ "$test_only" = true ]; then
        print_success "Proxy test completed successfully"
        exit 0
    fi

    # Create settings directory
    create_settings_dir

    # Backup existing settings
    backup_settings

    # Create new settings
    if create_settings "$base_url" "$api_key"; then
        echo
        print_success "Claude Code has been configured successfully for CCCC!"
        print_info "You can now use Claude Code with your CCCC proxy."
        print_info ""
        print_info "To verify the setup, run:"
        print_info "  claude --version"
        print_info ""
        print_info "Configuration file location: $CLAUDE_SETTINGS_FILE"

        if [ -f "$CLAUDE_SETTINGS_FILE" ]; then
            echo
            print_info "Current settings:"
            cat "$CLAUDE_SETTINGS_FILE" | jq .
        fi
    else
        print_error "Failed to create Claude Code settings"
        exit 1
    fi
}

# Run main function
main "$@"