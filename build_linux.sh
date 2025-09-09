#!/bin/bash

echo "=========================================="
echo "Go Agent Linux 完整打包脚本"
echo "=========================================="

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
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

# 获取版本信息
GIT_VERSION=$(git describe --tags --always 2>/dev/null)
if [ -z "$GIT_VERSION" ]; then
    GIT_VERSION="v1.0.0"
fi

# 设置变量
VERSION="$GIT_VERSION"
BUILD_TIME=$(date '+%Y-%m-%d %H:%M:%S')
PACKAGE_NAME="go-agent-linux-$VERSION"
BUILD_DIR="build"
PACKAGE_DIR="$BUILD_DIR/$PACKAGE_NAME"

print_info "开始打包 Go Agent Linux 版本"
print_info "版本: $VERSION"
print_info "构建时间: $BUILD_TIME"
print_info "包名: $PACKAGE_NAME"

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
print_info "Go版本: $(go version)"

# 检查当前目录
if [ ! -f "cmd/agent/main.go" ]; then
    print_error "请在项目根目录运行此脚本"
    exit 1
fi

# 清理旧的构建目录
print_step "清理旧的构建文件..."
rm -rf "$BUILD_DIR"
mkdir -p "$PACKAGE_DIR"/{configs,logs,scripts,docs,systemd}

# 更新依赖
print_step "更新依赖包..."
if ! go mod tidy; then
    print_warning "依赖包更新失败，继续构建..."
fi

# 构建程序 - Linux x64
print_step "构建 Linux x64 可执行文件..."
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64
if go build -ldflags="-s -w -X 'main.Version=$VERSION' -X 'main.BuildTime=$BUILD_TIME'" -o "$PACKAGE_DIR/go-agent" cmd/agent/main.go; then
    print_success "Linux x64 构建完成"
    chmod +x "$PACKAGE_DIR/go-agent"
else
    print_error "Linux x64 构建失败"
    exit 1
fi

# 构建程序 - Linux ARM64
print_step "构建 Linux ARM64 可执行文件..."
export GOARCH=arm64
if go build -ldflags="-s -w -X 'main.Version=$VERSION' -X 'main.BuildTime=$BUILD_TIME'" -o "$PACKAGE_DIR/go-agent-arm64" cmd/agent/main.go; then
    print_success "Linux ARM64 构建完成"
    chmod +x "$PACKAGE_DIR/go-agent-arm64"
else
    print_warning "Linux ARM64 构建失败，跳过..."
fi

# 复制配置文件
print_step "复制配置文件..."
if ! cp configs/*.yaml configs/*.yml "$PACKAGE_DIR/configs/" 2>/dev/null; then
    print_error "配置文件复制失败"
    exit 1
fi

# 复制文档
print_step "复制文档..."
cp *.md "$PACKAGE_DIR/docs/" 2>/dev/null
cp LICENSE "$PACKAGE_DIR/" 2>/dev/null

# 创建Linux启动脚本
print_step "创建启动脚本..."
cat > "$PACKAGE_DIR/start.sh" << 'EOF'
#!/bin/bash

# Go Agent 启动脚本

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AGENT_BIN="$SCRIPT_DIR/go-agent"
CONFIG_FILE="$SCRIPT_DIR/configs/config.yaml"
PID_FILE="$SCRIPT_DIR/go-agent.pid"

start() {
    if [ -f "$PID_FILE" ] && kill -0 $(cat "$PID_FILE") 2>/dev/null; then
        echo "Go Agent 已在运行 (PID: $(cat $PID_FILE))"
        return 1
    fi
    
    echo "启动 Go Agent..."
    nohup "$AGENT_BIN" -c "$CONFIG_FILE" > "$SCRIPT_DIR/logs/go-agent.log" 2>&1 &
    echo $! > "$PID_FILE"
    echo "Go Agent 已启动 (PID: $(cat $PID_FILE))"
}

stop() {
    if [ ! -f "$PID_FILE" ]; then
        echo "Go Agent 未运行"
        return 1
    fi
    
    PID=$(cat "$PID_FILE")
    if kill -0 "$PID" 2>/dev/null; then
        echo "停止 Go Agent (PID: $PID)..."
        kill -TERM "$PID"
        
        # 等待进程停止
        for i in {1..10}; do
            if ! kill -0 "$PID" 2>/dev/null; then
                break
            fi
            sleep 1
        done
        
        if kill -0 "$PID" 2>/dev/null; then
            echo "强制停止 Go Agent..."
            kill -KILL "$PID"
        fi
        
        rm -f "$PID_FILE"
        echo "Go Agent 已停止"
    else
        echo "进程不存在，清理PID文件"
        rm -f "$PID_FILE"
    fi
}

status() {
    if [ -f "$PID_FILE" ] && kill -0 $(cat "$PID_FILE") 2>/dev/null; then
        echo "Go Agent 正在运行 (PID: $(cat $PID_FILE))"
        return 0
    else
        echo "Go Agent 未运行"
        return 1
    fi
}

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        stop
        sleep 2
        start
        ;;
    status)
        status
        ;;
    *)
        echo "用法: $0 {start|stop|restart|status}"
        echo "或者直接运行: $AGENT_BIN -c $CONFIG_FILE"
        exit 1
        ;;
esac
EOF

chmod +x "$PACKAGE_DIR/start.sh"

# 创建systemd服务文件
print_step "创建systemd服务文件..."
cat > "$PACKAGE_DIR/systemd/go-agent.service" << EOF
[Unit]
Description=Go Agent 监控代理
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/opt/go-agent
ExecStart=/opt/go-agent/go-agent -c /opt/go-agent/configs/config.yaml
ExecReload=/bin/kill -HUP \$MAINPID
KillMode=process
Restart=on-failure
RestartSec=5s
StandardOutput=journal
StandardError=journal
SyslogIdentifier=go-agent

[Install]
WantedBy=multi-user.target
EOF

# 创建安装脚本
print_step "创建安装脚本..."
cat > "$PACKAGE_DIR/install.sh" << 'EOF'
#!/bin/bash

# Go Agent 安装脚本

set -e

INSTALL_DIR="/opt/go-agent"
SERVICE_FILE="/etc/systemd/system/go-agent.service"
USER="root"

echo "=========================================="
echo "Go Agent 安装脚本"
echo "=========================================="

# 检查权限
if [ "$EUID" -ne 0 ]; then
    echo "错误: 请使用root权限运行安装脚本"
    echo "使用方法: sudo ./install.sh"
    exit 1
fi

# 停止现有服务
if systemctl is-active --quiet go-agent; then
    echo "停止现有服务..."
    systemctl stop go-agent
fi

# 创建安装目录
echo "创建安装目录: $INSTALL_DIR"
mkdir -p "$INSTALL_DIR"/{configs,logs}

# 复制文件
echo "复制程序文件..."
cp -r ./* "$INSTALL_DIR/"
chown -R $USER:$USER "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/go-agent"
chmod +x "$INSTALL_DIR/start.sh"

# 安装systemd服务
echo "安装systemd服务..."
cp systemd/go-agent.service "$SERVICE_FILE"
systemctl daemon-reload
systemctl enable go-agent

# 启动服务
echo "启动服务..."
systemctl start go-agent

# 检查服务状态
if systemctl is-active --quiet go-agent; then
    echo "✅ Go Agent 服务安装并启动成功"
    echo "服务状态: $(systemctl is-active go-agent)"
    echo ""
    echo "管理命令:"
    echo "  查看状态: systemctl status go-agent"
    echo "  启动服务: systemctl start go-agent"
    echo "  停止服务: systemctl stop go-agent"
    echo "  重启服务: systemctl restart go-agent"
    echo "  查看日志: journalctl -u go-agent -f"
    echo ""
    echo "配置文件: $INSTALL_DIR/configs/config.yaml"
    echo "日志文件: $INSTALL_DIR/logs/"
else
    echo "❌ 服务启动失败，请检查配置"
    echo "查看日志: journalctl -u go-agent"
    exit 1
fi
EOF

chmod +x "$PACKAGE_DIR/install.sh"

# 创建卸载脚本
print_step "创建卸载脚本..."
cat > "$PACKAGE_DIR/uninstall.sh" << 'EOF'
#!/bin/bash

# Go Agent 卸载脚本

INSTALL_DIR="/opt/go-agent"
SERVICE_FILE="/etc/systemd/system/go-agent.service"

echo "=========================================="
echo "Go Agent 卸载脚本"
echo "=========================================="

# 检查权限
if [ "$EUID" -ne 0 ]; then
    echo "错误: 请使用root权限运行卸载脚本"
    echo "使用方法: sudo ./uninstall.sh"
    exit 1
fi

# 停止服务
if systemctl is-active --quiet go-agent; then
    echo "停止服务..."
    systemctl stop go-agent
fi

# 禁用并删除服务
if [ -f "$SERVICE_FILE" ]; then
    echo "删除systemd服务..."
    systemctl disable go-agent
    rm -f "$SERVICE_FILE"
    systemctl daemon-reload
fi

# 删除安装目录
if [ -d "$INSTALL_DIR" ]; then
    read -p "是否删除安装目录 $INSTALL_DIR? (y/N): " confirm
    if [[ $confirm =~ ^[Yy]$ ]]; then
        echo "删除安装目录..."
        rm -rf "$INSTALL_DIR"
    else
        echo "保留安装目录 $INSTALL_DIR"
    fi
fi

echo "✅ Go Agent 已卸载"
EOF

chmod +x "$PACKAGE_DIR/uninstall.sh"

# 创建Linux安装说明
print_step "创建安装说明..."
cat > "$PACKAGE_DIR/README_LINUX.md" << EOF
# Go Agent Linux 安装包

## 版本信息
- 版本: $VERSION
- 构建时间: $BUILD_TIME
- 平台: Linux x64/ARM64

## 快速安装

### 方式一：自动安装（推荐）
\`\`\`bash
# 解压安装包
tar -xzf go-agent-linux-$VERSION.tar.gz
cd go-agent-linux-$VERSION

# 以root权限安装
sudo ./install.sh
\`\`\`

### 方式二：手动运行
\`\`\`bash
# 启动服务
./start.sh start

# 停止服务
./start.sh stop

# 查看状态
./start.sh status

# 重启服务
./start.sh restart
\`\`\`

### 方式三：直接运行
\`\`\`bash
# 前台运行
./go-agent -c configs/config.yaml -v

# 后台运行
./go-agent -d -c configs/config.yaml
\`\`\`

## 系统服务管理

安装后可以使用systemd管理服务：

\`\`\`bash
# 查看服务状态
systemctl status go-agent

# 启动/停止/重启服务
systemctl start go-agent
systemctl stop go-agent
systemctl restart go-agent

# 查看日志
journalctl -u go-agent -f
\`\`\`

## 文件说明

- \`go-agent\` - 主程序（Linux x64）
- \`go-agent-arm64\` - ARM64版本（如果构建成功）
- \`configs/\` - 配置文件目录
- \`logs/\` - 日志目录
- \`start.sh\` - 启动脚本
- \`install.sh\` - 自动安装脚本
- \`uninstall.sh\` - 卸载脚本
- \`systemd/go-agent.service\` - systemd服务文件

## 配置文件

主配置文件：\`configs/config.yaml\`

编辑配置文件后重启服务：
\`\`\`bash
systemctl restart go-agent
\`\`\`

## 故障排除

1. **服务无法启动**
   \`\`\`bash
   journalctl -u go-agent --no-pager
   \`\`\`

2. **检查配置文件**
   \`\`\`bash
   ./go-agent -c configs/config.yaml --help
   \`\`\`

3. **权限问题**
   \`\`\`bash
   sudo chown -R root:root /opt/go-agent
   sudo chmod +x /opt/go-agent/go-agent
   \`\`\`

4. **网络连接**
   - 检查防火墙设置
   - 验证监控平台地址
   - 确认DNS解析

## 卸载

\`\`\`bash
sudo ./uninstall.sh
\`\`\`
EOF

# 创建版本信息文件
print_step "创建版本信息..."
cat > "$PACKAGE_DIR/VERSION.txt" << EOF
Go Agent Linux Version
Version: $VERSION
Build Time: $BUILD_TIME
Platform: Linux x64/ARM64
Go Version: $(go version)
EOF

# 显示文件列表
print_step "打包内容:"
ls -la "$PACKAGE_DIR"

# 显示文件大小
echo
print_info "可执行文件信息:"
if [ -f "$PACKAGE_DIR/go-agent" ]; then
    size=$(stat -c%s "$PACKAGE_DIR/go-agent")
    echo "  go-agent: $size 字节 ($(( size / 1024 / 1024 ))MB)"
fi
if [ -f "$PACKAGE_DIR/go-agent-arm64" ]; then
    size=$(stat -c%s "$PACKAGE_DIR/go-agent-arm64")
    echo "  go-agent-arm64: $size 字节 ($(( size / 1024 / 1024 ))MB)"
fi

# 创建TAR.GZ压缩包
print_step "创建压缩包..."
cd "$BUILD_DIR"
if tar -czf "$PACKAGE_NAME.tar.gz" "$PACKAGE_NAME"; then
    print_success "压缩包创建成功: $BUILD_DIR/$PACKAGE_NAME.tar.gz"
    
    # 显示压缩包信息
    size=$(stat -c%s "$PACKAGE_NAME.tar.gz")
    echo "  压缩包大小: $size 字节 ($(( size / 1024 / 1024 ))MB)"
else
    print_warning "压缩包创建失败"
fi
cd ..

# 显示打包结果
echo
echo "=========================================="
echo "🎉 Linux打包完成！"
echo "=========================================="
echo
echo "📦 输出目录: $BUILD_DIR/$PACKAGE_NAME"
echo "🗜️ 压缩包: $BUILD_DIR/$PACKAGE_NAME.tar.gz"
echo
echo "📋 包含文件:"
echo "  ✅ go-agent               (Linux x64主程序)"
if [ -f "$PACKAGE_DIR/go-agent-arm64" ]; then
echo "  ✅ go-agent-arm64         (Linux ARM64主程序)"
fi
echo "  ✅ configs/               (配置文件目录)"  
echo "  ✅ start.sh               (启动管理脚本)"
echo "  ✅ install.sh             (自动安装脚本)"
echo "  ✅ uninstall.sh           (卸载脚本)"
echo "  ✅ systemd/go-agent.service (systemd服务文件)"
echo "  ✅ README_LINUX.md        (Linux安装说明)"
echo "  ✅ docs/                  (文档目录)"
echo "  ✅ logs/                  (日志目录)"
echo
echo "🚀 使用方法:"
echo "  1. 解压: tar -xzf $PACKAGE_NAME.tar.gz"
echo "  2. 安装: cd $PACKAGE_NAME && sudo ./install.sh"
echo "  3. 或手动运行: ./start.sh start"
echo
echo "⚠️  注意: 安装为系统服务需要root权限"
echo "=========================================="