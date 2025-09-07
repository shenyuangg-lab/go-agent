#!/bin/bash

echo "=========================================="
echo "Go Agent 服务卸载脚本 - Linux版本"
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
INSTALL_DIR="/opt/go-agent"
SERVICE_USER="goagent"

print_info "准备卸载服务: $SERVICE_NAME"
echo "  安装目录: $INSTALL_DIR"
echo "  服务用户: $SERVICE_USER"

# 确认卸载
echo ""
read -p "确定要卸载 Go Agent 服务吗？这将删除所有服务文件和配置 [y/N]: " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    print_info "卸载已取消"
    exit 0
fi

# 卸载systemd服务
uninstall_systemd_service() {
    print_info "卸载systemd服务..."
    
    # 检查服务是否存在
    if systemctl list-units --full -all | grep -Fq "$SERVICE_NAME.service"; then
        print_info "发现systemd服务，开始卸载..."
        
        # 停止服务
        if systemctl is-active --quiet "$SERVICE_NAME"; then
            print_info "停止服务..."
            systemctl stop "$SERVICE_NAME"
            sleep 2
        fi
        
        # 禁用服务
        if systemctl is-enabled --quiet "$SERVICE_NAME"; then
            print_info "禁用服务自启动..."
            systemctl disable "$SERVICE_NAME"
        fi
        
        # 删除服务文件
        if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
            print_info "删除服务文件..."
            rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        fi
        
        # 重新加载systemd
        print_info "重新加载systemd配置..."
        systemctl daemon-reload
        systemctl reset-failed "$SERVICE_NAME" 2>/dev/null || true
        
        print_success "systemd服务卸载完成"
    else
        print_warning "未找到systemd服务"
    fi
}

# 卸载SysV服务
uninstall_sysv_service() {
    print_info "卸载SysV服务..."
    
    # 检查服务是否存在
    if [ -f "/etc/init.d/$SERVICE_NAME" ]; then
        print_info "发现SysV服务，开始卸载..."
        
        # 停止服务
        service "$SERVICE_NAME" stop 2>/dev/null || true
        sleep 2
        
        # 删除服务
        if command -v chkconfig >/dev/null 2>&1; then
            chkconfig --del "$SERVICE_NAME" 2>/dev/null || true
        elif command -v update-rc.d >/dev/null 2>&1; then
            update-rc.d -f "$SERVICE_NAME" remove 2>/dev/null || true
        fi
        
        # 删除init脚本
        rm -f "/etc/init.d/$SERVICE_NAME"
        
        print_success "SysV服务卸载完成"
    else
        print_warning "未找到SysV服务"
    fi
}

# 删除安装文件
remove_files() {
    print_info "删除安装文件..."
    
    if [ -d "$INSTALL_DIR" ]; then
        print_info "删除安装目录: $INSTALL_DIR"
        rm -rf "$INSTALL_DIR"
        print_success "安装文件删除完成"
    else
        print_warning "未找到安装目录: $INSTALL_DIR"
    fi
}

# 删除服务用户
remove_service_user() {
    print_info "删除服务用户..."
    
    if id "$SERVICE_USER" &>/dev/null; then
        print_info "删除服务用户: $SERVICE_USER"
        
        # 确保用户进程已停止
        pkill -u "$SERVICE_USER" 2>/dev/null || true
        sleep 1
        
        # 删除用户
        userdel "$SERVICE_USER" 2>/dev/null || true
        
        # 删除用户主目录（如果与安装目录不同）
        if [ -d "/home/$SERVICE_USER" ]; then
            rm -rf "/home/$SERVICE_USER"
        fi
        
        print_success "服务用户删除完成"
    else
        print_warning "未找到服务用户: $SERVICE_USER"
    fi
}

# 清理日志文件
cleanup_logs() {
    print_info "清理日志文件..."
    
    # 清理系统日志中的相关条目
    if [ "$SYSTEM_TYPE" = "systemd" ]; then
        journalctl --rotate 2>/dev/null || true
        journalctl --vacuum-time=1s 2>/dev/null || true
    fi
    
    # 清理可能的日志文件
    rm -f /var/log/${SERVICE_NAME}*.log 2>/dev/null || true
    
    print_success "日志清理完成"
}

# 主卸载流程
main() {
    print_info "开始卸载 Go Agent 服务..."
    
    # 根据系统类型卸载服务
    if [ "$SYSTEM_TYPE" = "systemd" ]; then
        uninstall_systemd_service
    elif [ "$SYSTEM_TYPE" = "sysv" ]; then
        uninstall_sysv_service
    else
        print_warning "未知系统类型，跳过服务卸载"
    fi
    
    # 删除安装文件
    remove_files
    
    # 询问是否删除服务用户
    echo ""
    read -p "是否删除服务用户 '$SERVICE_USER'？[y/N]: " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        remove_service_user
    else
        print_info "保留服务用户: $SERVICE_USER"
    fi
    
    # 清理日志
    cleanup_logs
    
    echo ""
    print_success "Go Agent 服务卸载完成！"
    echo ""
    print_info "如果需要重新安装，请运行: sudo ./install_service.sh"
}

# 执行主函数
main

echo ""
echo "=========================================="
echo "服务卸载脚本执行完成"
echo "=========================================="