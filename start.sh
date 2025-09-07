#!/bin/bash

echo "=========================================="
echo "Go Agent 启动脚本 - Linux版本"
echo "=========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_info() {
    echo -e "${BLUE}[信息]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[成功]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[警告]${NC} $1"
}

print_error() {
    echo -e "${RED}[错误]${NC} $1"
}

# 设置默认值
CONFIG_FILE="configs/config.yaml"
EXECUTABLE="go-agent"
VERBOSE=""

# 解析命令行参数
while [[ $# -gt 0 ]]; do
    case $1 in
        -c|--config)
            CONFIG_FILE="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE="-v"
            shift
            ;;
        -h|--help)
            echo "使用方法: $0 [选项]"
            echo ""
            echo "选项:"
            echo "  -c, --config FILE    指定配置文件 (默认: configs/config.yaml)"
            echo "  -v, --verbose        启用详细日志"
            echo "  -h, --help          显示此帮助信息"
            echo ""
            echo "示例:"
            echo "  $0                           # 使用默认配置"
            echo "  $0 -c custom.yaml            # 使用自定义配置"
            echo "  $0 -v                        # 启用详细日志"
            echo "  $0 -c custom.yaml -v         # 使用自定义配置和详细日志"
            exit 0
            ;;
        *)
            print_error "未知选项: $1"
            echo "使用 $0 -h 查看帮助信息"
            exit 1
            ;;
    esac
done

# 检查可执行文件是否存在
if [ ! -f "$EXECUTABLE" ]; then
    print_error "未找到可执行文件: $EXECUTABLE"
    print_info "正在尝试自动构建..."
    
    if [ -f "build.sh" ]; then
        chmod +x build.sh
        if ./build.sh; then
            print_success "自动构建完成"
        else
            print_error "自动构建失败"
            exit 1
        fi
    else
        print_error "未找到构建脚本 build.sh"
        print_info "请手动运行: go build -o go-agent cmd/agent/main.go"
        exit 1
    fi
fi

# 确保可执行文件有执行权限
chmod +x "$EXECUTABLE"

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    print_warning "未找到配置文件: $CONFIG_FILE"
    print_info "将使用默认配置启动"
    CONFIG_ARG=""
else
    CONFIG_ARG="-c $CONFIG_FILE"
    print_success "找到配置文件: $CONFIG_FILE"
fi

# 显示启动信息
print_info "启动参数:"
echo "  可执行文件: $EXECUTABLE"
echo "  配置文件: $CONFIG_FILE"
echo "  工作目录: $(pwd)"
echo "  详细日志: $([ -n "$VERBOSE" ] && echo "启用" || echo "禁用")"
echo "  用户: $(whoami)"
echo "  进程ID: $$"

# 创建日志目录
mkdir -p logs

print_info "正在启动 Go Agent 监控代理..."
echo "=========================================="
echo ""

# 设置信号处理
cleanup() {
    echo ""
    echo "=========================================="
    print_info "收到终止信号，正在优雅关闭..."
    echo "=========================================="
    exit 0
}

trap cleanup SIGINT SIGTERM

# 启动程序
if [ -n "$CONFIG_ARG" ]; then
    ./"$EXECUTABLE" $CONFIG_ARG $VERBOSE
else
    ./"$EXECUTABLE" $VERBOSE
fi

# 检查退出状态
EXIT_CODE=$?
echo ""
echo "=========================================="
if [ $EXIT_CODE -eq 0 ]; then
    print_success "程序正常退出"
else
    print_error "程序异常退出，退出代码: $EXIT_CODE"
fi
echo "=========================================="

exit $EXIT_CODE