#!/bin/bash

echo "=========================================="
echo "Go Agent 服务安装脚本 - Linux版本"
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

# 检查是否以root权限运行
if [[ $EUID -ne 0 ]]; then
    print_error "此脚本需要root权限运行"
    print_info "请使用: sudo $0"
    exit 1
fi

# 检测系统类型
detect_system() {
    if command -v systemctl >/dev/null 2>&1; then
        echo "systemd"
    elif command -v service >/dev/null 2>&1; then
        echo "sysv"
    else
        echo "unknown"
    fi
}

SYSTEM_TYPE=$(detect_system)
print_info "检测到系统类型: $SYSTEM_TYPE"

# 设置服务参数
SERVICE_NAME="go-agent"
SERVICE_DISPLAY_NAME="Go Agent Monitor Service"
SERVICE_DESCRIPTION="Go语言编写的系统监控代理服务"
INSTALL_DIR="/opt/go-agent"
SERVICE_USER="goagent"
CURRENT_DIR=$(pwd)
EXECUTABLE="$CURRENT_DIR/go-agent"
CONFIG_FILE="$CURRENT_DIR/configs/config.yaml"

print_info "服务配置:"
echo "  服务名称: $SERVICE_NAME"
echo "  显示名称: $SERVICE_DISPLAY_NAME"
echo "  安装目录: $INSTALL_DIR"
echo "  可执行文件: $EXECUTABLE"
echo "  配置文件: $CONFIG_FILE"
echo "  服务用户: $SERVICE_USER"

# 检查可执行文件
if [ ! -f "$EXECUTABLE" ]; then
    print_error "未找到可执行文件: $EXECUTABLE"
    print_info "请先运行 ./build.sh 构建程序"
    exit 1
fi

# 检查配置文件
if [ ! -f "$CONFIG_FILE" ]; then
    print_error "未找到配置文件: $CONFIG_FILE"
    exit 1
fi

# 创建服务用户
create_service_user() {
    if ! id "$SERVICE_USER" &>/dev/null; then
        print_info "创建服务用户: $SERVICE_USER"
        useradd -r -s /bin/false -d "$INSTALL_DIR" "$SERVICE_USER"
        if [ $? -eq 0 ]; then
            print_success "服务用户创建成功"
        else
            print_error "服务用户创建失败"
            exit 1
        fi
    else
        print_info "服务用户已存在: $SERVICE_USER"
    fi
}

# 创建安装目录并复制文件
install_files() {
    print_info "创建安装目录: $INSTALL_DIR"
    mkdir -p "$INSTALL_DIR/configs"
    mkdir -p "$INSTALL_DIR/logs"
    
    print_info "复制文件到安装目录..."
    cp "$EXECUTABLE" "$INSTALL_DIR/"
    cp "$CONFIG_FILE" "$INSTALL_DIR/configs/"
    
    # 复制整个configs目录
    if [ -d "configs" ]; then
        cp -r configs/* "$INSTALL_DIR/configs/"
    fi
    
    # 设置权限
    chown -R "$SERVICE_USER:$SERVICE_USER" "$INSTALL_DIR"
    chmod 755 "$INSTALL_DIR/go-agent"
    chmod 644 "$INSTALL_DIR/configs/"*.yaml
    chmod 755 "$INSTALL_DIR/logs"
    
    print_success "文件安装完成"
}

# 安装systemd服务
install_systemd_service() {
    print_info "创建systemd服务文件..."
    
    cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=$SERVICE_DESCRIPTION
Documentation=https://github.com/your-org/go-agent
After=network.target
Wants=network.target

[Service]
Type=simple
User=$SERVICE_USER
Group=$SERVICE_USER
WorkingDirectory=$INSTALL_DIR
ExecStart=$INSTALL_DIR/go-agent -c $INSTALL_DIR/configs/config.yaml
ExecReload=/bin/kill -HUP \$MAINPID
KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30
Restart=always
RestartSec=5
StartLimitInterval=60
StartLimitBurst=3

# 安全设置
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$INSTALL_DIR
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
RestrictNamespaces=true

# 资源限制
LimitNOFILE=65536
LimitNPROC=4096

# 日志设置
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$SERVICE_NAME

[Install]
WantedBy=multi-user.target
EOF

    print_success "systemd服务文件创建完成"
    
    print_info "重新加载systemd配置..."
    systemctl daemon-reload
    
    print_info "启用服务自启动..."
    systemctl enable "$SERVICE_NAME"
    
    print_info "启动服务..."
    systemctl start "$SERVICE_NAME"
    
    # 等待服务启动
    sleep 3
    
    # 检查服务状态
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_success "服务启动成功！"
        systemctl status "$SERVICE_NAME" --no-pager -l
    else
        print_error "服务启动失败"
        print_info "查看服务日志:"
        journalctl -u "$SERVICE_NAME" --no-pager -l
        exit 1
    fi
}

# 安装SysV服务
install_sysv_service() {
    print_info "创建SysV init脚本..."
    
    cat > /etc/init.d/${SERVICE_NAME} << EOF
#!/bin/bash
# $SERVICE_NAME        $SERVICE_DESCRIPTION
# chkconfig: 35 99 99
# description: $SERVICE_DESCRIPTION
#

. /etc/rc.d/init.d/functions

USER="$SERVICE_USER"
DAEMON="$SERVICE_NAME"
ROOT_DIR="$INSTALL_DIR"

SERVER="\$ROOT_DIR/go-agent"
LOCK_FILE="/var/lock/subsys/\$DAEMON"

do_start() {
    if [ ! -f "\$LOCK_FILE" ] ; then
        echo -n "Starting \$DAEMON: "
        runuser -l "\$USER" -c "\$SERVER -c \$ROOT_DIR/configs/config.yaml" && echo_success || echo_failure
        RETVAL=\$?
        echo
        [ \$RETVAL -eq 0 ] && touch \$LOCK_FILE
    else
        echo "\$DAEMON is locked."
    fi
}
do_stop() {
    echo -n "Shutting down \$DAEMON: "
    pid=\`ps -aefw | grep "\$DAEMON" | grep -v " grep " | awk '{print \$2}'\`
    kill -9 \$pid > /dev/null 2>&1
    [ \$? -eq 0 ] && echo_success || echo_failure
    RETVAL=\$?
    echo
    [ \$RETVAL -eq 0 ] && rm -f \$LOCK_FILE
}

case "\$1" in
    start)
        do_start
        ;;
    stop)
        do_stop
        ;;
    restart)
        do_stop
        do_start
        ;;
    *)
        echo "Usage: \$0 {start|stop|restart}"
        RETVAL=1
esac

exit \$RETVAL
EOF

    chmod 755 /etc/init.d/${SERVICE_NAME}
    
    print_success "SysV init脚本创建完成"
    
    # 根据系统类型添加服务
    if command -v chkconfig >/dev/null 2>&1; then
        chkconfig --add "$SERVICE_NAME"
        chkconfig "$SERVICE_NAME" on
    elif command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d "$SERVICE_NAME" defaults
    fi
    
    print_info "启动服务..."
    service "$SERVICE_NAME" start
    
    sleep 3
    
    if service "$SERVICE_NAME" status >/dev/null 2>&1; then
        print_success "服务启动成功！"
    else
        print_error "服务启动失败"
        exit 1
    fi
}

# 主安装流程
main() {
    print_info "开始安装 Go Agent 服务..."
    
    # 停止现有服务（如果存在）
    if [ "$SYSTEM_TYPE" = "systemd" ]; then
        if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_info "停止现有服务..."
            systemctl stop "$SERVICE_NAME"
        fi
        if systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null; then
            print_info "禁用现有服务..."
            systemctl disable "$SERVICE_NAME"
        fi
    elif [ "$SYSTEM_TYPE" = "sysv" ]; then
        service "$SERVICE_NAME" stop >/dev/null 2>&1
    fi
    
    # 创建服务用户
    create_service_user
    
    # 安装文件
    install_files
    
    # 根据系统类型安装服务
    if [ "$SYSTEM_TYPE" = "systemd" ]; then
        install_systemd_service
    elif [ "$SYSTEM_TYPE" = "sysv" ]; then
        install_sysv_service
    else
        print_error "不支持的系统类型，无法安装服务"
        exit 1
    fi
    
    echo ""
    print_success "Go Agent 服务安装完成！"
    echo ""
    print_info "服务管理命令:"
    if [ "$SYSTEM_TYPE" = "systemd" ]; then
        echo "  查看状态: systemctl status $SERVICE_NAME"
        echo "  启动服务: systemctl start $SERVICE_NAME"
        echo "  停止服务: systemctl stop $SERVICE_NAME"
        echo "  重启服务: systemctl restart $SERVICE_NAME"
        echo "  查看日志: journalctl -u $SERVICE_NAME -f"
        echo "  禁用自启: systemctl disable $SERVICE_NAME"
    else
        echo "  查看状态: service $SERVICE_NAME status"
        echo "  启动服务: service $SERVICE_NAME start"
        echo "  停止服务: service $SERVICE_NAME stop"
        echo "  重启服务: service $SERVICE_NAME restart"
    fi
}

# 执行主函数
main

echo ""
echo "=========================================="
echo "服务安装脚本执行完成"
echo "=========================================="