#!/bin/bash

echo "=========================================="
echo "Go Agent 构建脚本 - Linux版本"
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

# 检查Go环境
if ! command -v go &> /dev/null; then
    print_error "未找到Go环境，请先安装Go"
    echo "安装方法:"
    echo "  Ubuntu/Debian: sudo apt install golang-go"
    echo "  CentOS/RHEL: sudo yum install golang"
    echo "  或访问: https://golang.org/dl/"
    exit 1
fi

# 显示Go版本
print_info "检查Go版本..."
go version

# 检查当前目录
if [ ! -f "cmd/agent/main.go" ]; then
    print_error "请在项目根目录运行此脚本"
    exit 1
fi

# 清理旧的构建文件
print_info "清理旧的构建文件..."
rm -f go-agent go-agent-*

# 下载依赖
print_info "下载依赖包..."
if ! go mod tidy; then
    print_error "依赖包下载失败"
    exit 1
fi

# 获取系统信息
GOOS=$(go env GOOS)
GOARCH=$(go env GOARCH)
print_info "当前系统: $GOOS/$GOARCH"

# 构建当前平台版本
print_info "构建当前平台版本..."
if go build -ldflags="-s -w" -o go-agent cmd/agent/main.go; then
    print_success "当前平台版本构建完成: go-agent"
    chmod +x go-agent
else
    print_error "当前平台版本构建失败"
    exit 1
fi

# 交叉编译其他平台版本
echo ""
print_info "开始交叉编译..."

# Linux AMD64
if GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o go-agent-linux-amd64 cmd/agent/main.go; then
    print_success "Linux AMD64 版本构建完成: go-agent-linux-amd64"
    chmod +x go-agent-linux-amd64
else
    print_warning "Linux AMD64 版本构建失败"
fi

# Linux ARM64
if GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o go-agent-linux-arm64 cmd/agent/main.go; then
    print_success "Linux ARM64 版本构建完成: go-agent-linux-arm64"
    chmod +x go-agent-linux-arm64
else
    print_warning "Linux ARM64 版本构建失败"
fi

# Windows AMD64
if GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o go-agent-windows-amd64.exe cmd/agent/main.go; then
    print_success "Windows AMD64 版本构建完成: go-agent-windows-amd64.exe"
else
    print_warning "Windows AMD64 版本构建失败"
fi

# macOS AMD64
if GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o go-agent-darwin-amd64 cmd/agent/main.go; then
    print_success "macOS AMD64 版本构建完成: go-agent-darwin-amd64"
    chmod +x go-agent-darwin-amd64
else
    print_warning "macOS AMD64 版本构建失败"
fi

# 显示构建结果
echo ""
print_info "构建结果:"
ls -la go-agent*

echo ""
print_info "使用方法:"
echo "  直接运行: ./go-agent"
echo "  指定配置: ./go-agent -c configs/config.yaml"
echo "  详细日志: ./go-agent -v"
echo "  查看帮助: ./go-agent --help"

echo ""
echo "=========================================="
echo "构建脚本执行完成"
echo "=========================================="