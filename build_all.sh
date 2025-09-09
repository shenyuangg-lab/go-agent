#!/bin/bash

echo "=========================================="
echo "Go Agent 跨平台打包脚本"
echo "=========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
PURPLE='\033[0;35m'
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

print_step() {
    echo -e "${CYAN}[步骤]${NC} $1"
}

print_platform() {
    echo -e "${PURPLE}[平台]${NC} $1"
}

# 获取版本信息
GIT_VERSION=$(git describe --tags --always 2>/dev/null)
if [ -z "$GIT_VERSION" ]; then
    GIT_VERSION="v1.0.0"
fi

VERSION="$GIT_VERSION"
BUILD_TIME=$(date '+%Y-%m-%d %H:%M:%S')
BUILD_DIR="build"
RELEASE_DIR="$BUILD_DIR/release"

print_info "Go Agent 跨平台构建"
print_info "版本: $VERSION"
print_info "构建时间: $BUILD_TIME"

# 检查Go环境
if ! command -v go &> /dev/null; then
    print_error "未找到Go环境，请先安装Go"
    exit 1
fi

print_info "Go版本: $(go version)"

# 检查当前目录
if [ ! -f "cmd/agent/main.go" ]; then
    print_error "请在项目根目录运行此脚本"
    exit 1
fi

# 清理构建目录
print_step "清理构建目录..."
rm -rf "$BUILD_DIR"
mkdir -p "$RELEASE_DIR"

# 更新依赖
print_step "更新依赖包..."
go mod tidy

# 定义构建目标
declare -A TARGETS=(
    ["windows/amd64"]="go-agent-windows-amd64.exe"
    ["windows/386"]="go-agent-windows-386.exe"
    ["linux/amd64"]="go-agent-linux-amd64"
    ["linux/386"]="go-agent-linux-386"
    ["linux/arm64"]="go-agent-linux-arm64"
    ["linux/arm"]="go-agent-linux-arm"
    ["darwin/amd64"]="go-agent-darwin-amd64"
    ["darwin/arm64"]="go-agent-darwin-arm64"
    ["freebsd/amd64"]="go-agent-freebsd-amd64"
)

# 构建所有目标平台
print_step "开始跨平台构建..."
export CGO_ENABLED=0

build_count=0
success_count=0

for target in "${!TARGETS[@]}"; do
    GOOS=$(echo $target | cut -d'/' -f1)
    GOARCH=$(echo $target | cut -d'/' -f2)
    OUTPUT="${TARGETS[$target]}"
    
    print_platform "构建 $GOOS/$GOARCH -> $OUTPUT"
    
    export GOOS GOARCH
    
    if go build -ldflags="-s -w -X 'main.Version=$VERSION' -X 'main.BuildTime=$BUILD_TIME'" -o "$RELEASE_DIR/$OUTPUT" cmd/agent/main.go; then
        
        # 设置可执行权限（非Windows平台）
        if [ "$GOOS" != "windows" ]; then
            chmod +x "$RELEASE_DIR/$OUTPUT"
        fi
        
        # 显示文件大小
        size=$(stat -c%s "$RELEASE_DIR/$OUTPUT" 2>/dev/null || stat -f%z "$RELEASE_DIR/$OUTPUT" 2>/dev/null)
        size_mb=$(( size / 1024 / 1024 ))
        print_success "$OUTPUT (${size_mb}MB)"
        
        ((success_count++))
    else
        print_warning "$target 构建失败"
    fi
    
    ((build_count++))
done

echo
print_info "构建统计: $success_count/$build_count 成功"

# 复制通用文件到release目录
print_step "复制通用文件..."
mkdir -p "$RELEASE_DIR/configs"
mkdir -p "$RELEASE_DIR/docs"

cp configs/*.yaml "$RELEASE_DIR/configs/" 2>/dev/null
cp *.md "$RELEASE_DIR/docs/" 2>/dev/null
cp LICENSE "$RELEASE_DIR/" 2>/dev/null

# 创建通用README
cat > "$RELEASE_DIR/README.md" << EOF
# Go Agent 跨平台发布包

## 版本信息
- 版本: $VERSION
- 构建时间: $BUILD_TIME
- 支持平台: Windows, Linux, macOS, FreeBSD

## 可执行文件

### Windows
- \`go-agent-windows-amd64.exe\` - Windows 64位
- \`go-agent-windows-386.exe\` - Windows 32位

### Linux
- \`go-agent-linux-amd64\` - Linux 64位
- \`go-agent-linux-386\` - Linux 32位
- \`go-agent-linux-arm64\` - Linux ARM64
- \`go-agent-linux-arm\` - Linux ARM

### macOS
- \`go-agent-darwin-amd64\` - macOS Intel
- \`go-agent-darwin-arm64\` - macOS Apple Silicon (M1/M2)

### FreeBSD
- \`go-agent-freebsd-amd64\` - FreeBSD 64位

## 使用方法

### Windows
\`\`\`cmd
go-agent-windows-amd64.exe -c configs\config.yaml
\`\`\`

### Linux/macOS/FreeBSD
\`\`\`bash
# 设置执行权限
chmod +x go-agent-linux-amd64

# 运行程序
./go-agent-linux-amd64 -c configs/config.yaml

# 后台运行
./go-agent-linux-amd64 -d -c configs/config.yaml
\`\`\`

## 命令行参数

- \`-c <config>\` : 指定配置文件路径
- \`-v\` : 启用详细日志输出
- \`-d\` : 后台运行模式（仅Linux/macOS/FreeBSD）
- \`--help\` : 显示帮助信息

## 配置文件

主配置文件：\`configs/config.yaml\`

请根据实际环境修改配置文件中的监控平台地址和其他参数。

## 系统要求

- Windows: Windows 7 或更高版本
- Linux: 内核版本 2.6.32 或更高版本
- macOS: macOS 10.12 或更高版本
- FreeBSD: FreeBSD 10 或更高版本

## 技术支持

如有问题，请查看文档目录下的相关文档或联系技术支持。
EOF

# 创建版本信息文件
cat > "$RELEASE_DIR/VERSION.txt" << EOF
Go Agent Cross-Platform Release
Version: $VERSION
Build Time: $BUILD_TIME
Go Version: $(go version)

Supported Platforms:
- Windows (amd64, 386)
- Linux (amd64, 386, arm64, arm)
- macOS (amd64, arm64)
- FreeBSD (amd64)

Build Statistics: $success_count/$build_count targets successful
EOF

# 显示构建结果
print_step "构建结果:"
ls -la "$RELEASE_DIR"

# 计算总大小
total_size=0
for file in "$RELEASE_DIR"/go-agent-*; do
    if [ -f "$file" ]; then
        size=$(stat -c%s "$file" 2>/dev/null || stat -f%z "$file" 2>/dev/null)
        total_size=$((total_size + size))
    fi
done

total_mb=$((total_size / 1024 / 1024))

# 创建压缩包
print_step "创建发布压缩包..."
cd "$BUILD_DIR"
if tar -czf "go-agent-$VERSION-all-platforms.tar.gz" release/; then
    archive_size=$(stat -c%s "go-agent-$VERSION-all-platforms.tar.gz" 2>/dev/null || stat -f%z "go-agent-$VERSION-all-platforms.tar.gz" 2>/dev/null)
    archive_mb=$((archive_size / 1024 / 1024))
    print_success "发布包创建成功: build/go-agent-$VERSION-all-platforms.tar.gz (${archive_mb}MB)"
fi
cd ..

echo
echo "=========================================="
echo "🎉 跨平台构建完成！"
echo "=========================================="
echo
echo "📊 构建统计:"
echo "  ✅ 成功构建: $success_count/$build_count 个目标平台"
echo "  📦 总文件大小: ${total_mb}MB"
echo "  🗜️ 压缩包: build/go-agent-$VERSION-all-platforms.tar.gz"
echo
echo "📁 输出目录: $RELEASE_DIR/"
echo
echo "🚀 部署方法:"
echo "  1. 选择对应平台的可执行文件"
echo "  2. 复制 configs/ 目录到目标机器"
echo "  3. 编辑配置文件并运行程序"
echo
echo "📖 详细说明请查看 $RELEASE_DIR/README.md"
echo "=========================================="

# 如果只有一个参数且为test，则运行简单测试
if [ "$1" = "test" ]; then
    echo
    print_step "运行构建测试..."
    
    # 测试当前平台的可执行文件
    current_os=$(uname -s | tr '[:upper:]' '[:lower:]')
    current_arch=$(uname -m)
    
    case "$current_arch" in
        x86_64) current_arch="amd64" ;;
        i386|i686) current_arch="386" ;;
        aarch64) current_arch="arm64" ;;
        armv7l) current_arch="arm" ;;
    esac
    
    if [ "$current_os" = "darwin" ]; then
        current_os="darwin"
    elif [ "$current_os" = "linux" ]; then
        current_os="linux"
    fi
    
    test_binary="$RELEASE_DIR/go-agent-$current_os-$current_arch"
    if [ "$current_os" = "windows" ]; then
        test_binary="$test_binary.exe"
    fi
    
    if [ -f "$test_binary" ]; then
        print_info "测试二进制文件: $test_binary"
        if "$test_binary" --help >/dev/null 2>&1; then
            print_success "二进制文件测试通过"
        else
            print_warning "二进制文件测试失败"
        fi
    else
        print_warning "未找到当前平台的二进制文件"
    fi
fi