#!/bin/bash

# CCCC Proxy Desktop - 统一启动脚本
# 整合了安全检查、构建、开发、生产等所有功能

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
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

# 显示帮助信息
show_help() {
    echo "CCCC Proxy Desktop - 统一启动脚本"
    echo ""
    echo "用法: $0 [选项] [命令]"
    echo ""
    echo "命令:"
    echo "  dev         启动开发模式"
    echo "  build       构建应用"
    echo "  run         运行已构建的应用"
    echo "  clean       清理构建产物和进程"
    echo "  status      显示当前状态"
    echo ""
    echo "选项:"
    echo "  --help, -h  显示此帮助信息"
    echo "  --open      构建后自动打开应用"
    echo "  --clean     构建前清理"
    echo "  --force     强制执行（忽略安全检查）"
    echo ""
    echo "示例:"
    echo "  $0 dev                    # 开发模式"
    echo "  $0 build --open           # 构建并打开"
    echo "  $0 run                    # 运行应用"
    echo "  $0 clean                  # 清理所有"
    echo ""
}

# 检查现有实例
check_instances() {
    local wails_processes=$(pgrep -f "wails dev" 2>/dev/null || true)
    local app_processes=$(pgrep -f "cccc-proxy.app" 2>/dev/null || true)
    local port_in_use=$(lsof -i :51827 2>/dev/null || true)

    if [[ -n "$wails_processes" || -n "$app_processes" || -n "$port_in_use" ]]; then
        print_warning "检测到现有实例正在运行："

        if [[ -n "$wails_processes" ]]; then
            echo "  - Wails 开发进程: $(echo $wails_processes | wc -w) 个"
        fi

        if [[ -n "$app_processes" ]]; then
            echo "  - 应用进程: $(echo $app_processes | wc -w) 个"
        fi

        if [[ -n "$port_in_use" ]]; then
            echo "  - 单实例检测端口被占用"
        fi

        echo ""
        echo "正在运行的进程："
        ps aux | grep -E "(wails|cccc-proxy)" | grep -v grep || echo "  (无进程信息)"
        echo ""

        if [[ "$FORCE" != "true" ]]; then
            echo "💡 使用 '--force' 选项强制启动，或使用 '$0 clean' 清理所有实例"
            return 1
        fi
    fi

    return 0
}

# 清理所有实例
clean_all() {
    print_info "正在清理所有实例..."

    # 终止 wails dev 进程
    if pgrep -f "wails dev" > /dev/null; then
        print_info "终止 Wails 开发进程..."
        pkill -f "wails dev" || true
        sleep 2

        # 强制终止仍在运行的进程
        if pgrep -f "wails dev" > /dev/null; then
            pkill -9 -f "wails dev" || true
        fi
    fi

    # 终止应用进程
    if pgrep -f "cccc-proxy.app" > /dev/null; then
        print_info "终止应用进程..."
        pkill -f "cccc-proxy.app" || true
        sleep 2

        # 强制终止
        if pgrep -f "cccc-proxy.app" > /dev/null; then
            pkill -9 -f "cccc-proxy.app" || true
        fi
    fi

    # 清理构建产物
    if [[ "$1" == "--build" ]]; then
        print_info "清理构建产物..."
        rm -rf build/bin 2>/dev/null || true
    fi

    print_success "清理完成"
}

# 开发模式
start_dev() {
    print_info "启动开发模式..."

    if ! check_instances; then
        if [[ "$FORCE" != "true" ]]; then
            return 1
        fi
    fi

    # 检查 wails 命令
    if ! command -v wails &> /dev/null; then
        print_error "未找到 wails 命令，请确保已安装 Wails CLI"
        return 1
    fi

    # 设置环境变量，禁用自动重载
    export WAILS_DISABLE_AUTO_RELOAD=true

    print_info "启动 Wails 开发服务器（禁用自动重载）..."
    print_warning "注意：已禁用自动重载功能以避免多实例冲突"
    print_info "如需重启应用，请按 Ctrl+C 然后重新运行 $0 dev"

    # 启动开发服务器，但设置较短的超时时间避免无限循环
    timeout 300s wails dev || {
        print_warning "开发服务器已停止（5分钟超时或手动停止）"
    }
}

# 构建应用
build_app() {
    local open_after_build="$1"

    print_info "构建应用..."

    # 检查 wails 命令
    if ! command -v wails &> /dev/null; then
        print_error "未找到 wails 命令，请确保已安装 Wails CLI"
        return 1
    fi

    # 构建参数
    local build_args=()
    if [[ "$CLEAN" == "true" ]]; then
        build_args+=(--clean)
    fi

    print_info "执行构建: wails build ${build_args[*]}"
    if wails build "${build_args[@]}"; then
        print_success "构建完成"
        restart_app_after_build "$open_after_build"
    else
        print_error "构建失败"
        return 1
    fi
}

restart_app_after_build() {
    local open_after_build="$1"
    local app_path="build/bin/cccc-proxy.app/Contents/MacOS/cccc-proxy"

    if [[ ! -f "$app_path" ]]; then
        print_error "找不到构建的应用"
        return 1
    fi

    if pgrep -f "cccc-proxy.app" > /dev/null; then
        print_info "检测到正在运行的应用，准备重启..."
        pkill -f "cccc-proxy.app" || true
        sleep 1
    else
        print_info "暂无运行中的应用，准备启动..."
    fi

    if open build/bin/cccc-proxy.app; then
        print_success "应用已启动"
    else
        print_error "应用启动失败"
        return 1
    fi
}

# 运行应用
run_app() {
    print_info "运行应用..."

    if ! check_instances; then
        print_error "已有实例在运行，无法启动新实例"
        return 1
    fi

    if [[ -f "build/bin/cccc-proxy.app/Contents/MacOS/cccc-proxy" ]]; then
        print_info "启动应用..."
        open build/bin/cccc-proxy.app
    else
        print_error "找不到构建的应用，请先运行 '$0 build'"
        return 1
    fi
}

# 显示状态
show_status() {
    print_info "CCCC Proxy Desktop 状态检查"
    echo ""

    # 检查进程
    local wails_count=$(pgrep -f "wails dev" 2>/dev/null | wc -l)
    local app_count=$(pgrep -f "cccc-proxy.app" 2>/dev/null | wc -l)

    echo "📊 进程状态："
    echo "  - Wails 开发进程: $wails_count 个"
    echo "  - 应用进程: $app_count 个"

    # 检查端口
    if lsof -i :51827 &>/dev/null; then
        echo "  - 单实例检测端口: 被占用"
    else
        echo "  - 单实例检测端口: 空闲"
    fi

    # 检查文件
    echo ""
    echo "📁 文件状态："
    if [[ -f "build/bin/cccc-proxy.app/Contents/MacOS/cccc-proxy" ]]; then
        echo "  - 构建产物: 存在"
    else
        echo "  - 构建产物: 不存在"
    fi

    if [[ -f ".cccc-data/main.db" ]]; then
        echo "  - 开发数据库: .cccc-data/main.db"
    else
        echo "  - 开发数据库: 不存在"
    fi

    echo ""
    if [[ $wails_count -gt 0 || $app_count -gt 0 ]]; then
        print_warning "应用正在运行中"
    else
        print_info "应用未运行"
    fi
}

# 主程序
main() {
    # 解析参数
    COMMAND=""
    OPEN_AFTER_BUILD="false"
    CLEAN="false"
    FORCE="false"

    while [[ $# -gt 0 ]]; do
        case $1 in
            --help|-h)
                show_help
                exit 0
                ;;
            --open)
                OPEN_AFTER_BUILD="true"
                shift
                ;;
            --clean)
                CLEAN="true"
                shift
                ;;
            --force)
                FORCE="true"
                shift
                ;;
            dev|build|run|clean|status)
                COMMAND="$1"
                shift
                ;;
            *)
                print_error "未知参数: $1"
                show_help
                exit 1
                ;;
        esac
    done

    # 如果没有命令，显示帮助
    if [[ -z "$COMMAND" ]]; then
        show_help
        exit 1
    fi

    # 确保在正确的目录
    cd "$(dirname "$0")"

    # 执行命令
    case $COMMAND in
        clean)
            clean_all
            ;;
        dev)
            start_dev
            ;;
        build)
            build_app "$OPEN_AFTER_BUILD"
            ;;
        run)
            run_app
            ;;
        status)
            show_status
            ;;
        *)
            print_error "未知命令: $COMMAND"
            show_help
            exit 1
            ;;
    esac
}

# 运行主程序
main "$@"
