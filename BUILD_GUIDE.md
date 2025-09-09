# Go Agent 打包部署指南

## 🎯 概述

Go Agent 提供了完整的跨平台打包解决方案，支持 Windows、Linux、macOS、FreeBSD 等多个平台。

## 📋 打包脚本说明

### 🪟 Windows 平台

#### 1. 快速构建
```cmd
# 基本构建
build.bat windows

# 或直接运行
build_windows.bat
```

#### 2. 输出内容
```
build/go-agent-windows-v1.0.0/
├── go-agent.exe                 # Windows x64 主程序
├── go-agent-x86.exe            # Windows x86 主程序
├── configs/                     # 配置文件目录
├── start.bat                    # 快速启动脚本
├── install_service.bat          # 服务安装脚本
├── uninstall_service.bat        # 服务卸载脚本
├── README_WINDOWS.md            # Windows 安装说明
├── docs/                        # 文档目录
└── logs/                        # 日志目录
```

### 🐧 Linux 平台

#### 1. 快速构建
```bash
# 基本构建
./build.sh linux

# 或直接运行
./build_linux.sh
```

#### 2. 输出内容
```
build/go-agent-linux-v1.0.0/
├── go-agent                     # Linux x64 主程序
├── go-agent-arm64              # Linux ARM64 主程序
├── configs/                     # 配置文件目录
├── start.sh                     # 启动管理脚本
├── install.sh                   # 自动安装脚本
├── uninstall.sh                 # 卸载脚本
├── systemd/go-agent.service     # systemd 服务文件
├── README_LINUX.md              # Linux 安装说明
├── docs/                        # 文档目录
└── logs/                        # 日志目录
```

### 🌐 跨平台构建

#### 1. 构建所有平台
```bash
# 构建所有支持的平台
./build_all.sh

# 构建并测试
./build_all.sh test
```

#### 2. 支持的平台
- **Windows**: amd64, 386
- **Linux**: amd64, 386, arm64, arm
- **macOS**: amd64, arm64 (Intel/Apple Silicon)
- **FreeBSD**: amd64

## 🚀 部署方法

### Windows 部署

#### 方式一：直接运行
```cmd
# 1. 解压到目标目录
# 2. 双击 start.bat
# 3. 或命令行运行
go-agent.exe -c configs\config.yaml
```

#### 方式二：Windows 服务（推荐）
```cmd
# 1. 以管理员身份运行
install_service.bat

# 2. 服务管理
sc start GoAgent          # 启动服务
sc stop GoAgent           # 停止服务
sc query GoAgent          # 查看状态

# 3. 卸载服务
uninstall_service.bat
```

### Linux 部署

#### 方式一：自动安装（推荐）
```bash
# 1. 解压安装包
tar -xzf go-agent-linux-v1.0.0.tar.gz
cd go-agent-linux-v1.0.0

# 2. 自动安装为系统服务
sudo ./install.sh

# 3. 服务管理
systemctl status go-agent    # 查看状态
systemctl start go-agent     # 启动服务
systemctl stop go-agent      # 停止服务
journalctl -u go-agent -f    # 查看日志
```

#### 方式二：手动管理
```bash
# 启动管理
./start.sh start      # 启动
./start.sh stop       # 停止
./start.sh status     # 状态
./start.sh restart    # 重启
```

#### 方式三：直接运行
```bash
# 前台运行
./go-agent -c configs/config.yaml -v

# 后台运行
./go-agent -d -c configs/config.yaml
```

### macOS 部署

```bash
# 设置执行权限
chmod +x go-agent-darwin-amd64

# 运行程序
./go-agent-darwin-amd64 -c configs/config.yaml

# 后台运行
./go-agent-darwin-amd64 -d -c configs/config.yaml
```

## ⚙️ 配置文件

### 主配置文件 `configs/config.yaml`

```yaml
# 关键配置项
device_monitor:
  enabled: true
  base_url: "http://your-monitor-platform:8081/api"  # 监控平台地址
  timeout: "30s"
  heartbeat_interval: "30s"

agent:
  name: "go-agent"
  interval: "30s"
  timeout: "10s"

log:
  level: "info"      # debug, info, warn, error
  format: "text"     # text, json
  output: "stdout"   # stdout, file
```

## 🔧 高级构建选项

### 自定义构建参数

```bash
# 设置版本号
export VERSION="v2.0.0"

# 设置构建标志
export LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.BuildTime=$(date)'"

# 构建
go build -ldflags="$LDFLAGS" -o go-agent cmd/agent/main.go
```

### 精简构建

```bash
# 禁用 CGO，减小体积
export CGO_ENABLED=0

# 去除调试信息
export LDFLAGS="-s -w"

# 构建
go build -ldflags="$LDFLAGS" -o go-agent cmd/agent/main.go
```

## 📦 生产环境部署清单

### 部署前检查
- [ ] Go 版本 >= 1.19
- [ ] 网络连接正常
- [ ] 防火墙配置
- [ ] 权限设置（Linux 服务需要 root 权限）

### 配置检查
- [ ] 监控平台地址配置正确
- [ ] 采集间隔设置合理
- [ ] 日志级别和输出路径
- [ ] 服务端口未被占用

### 部署步骤
1. **下载对应平台的发布包**
2. **解压到目标目录**
3. **编辑配置文件**
4. **安装为系统服务（推荐）**
5. **启动并验证服务状态**
6. **查看日志确认运行正常**

## 🛠️ 故障排除

### 常见问题

1. **服务无法启动**
   ```bash
   # Linux
   journalctl -u go-agent --no-pager
   
   # Windows
   查看 Windows 事件查看器
   ```

2. **网络连接失败**
   ```bash
   # 测试网络连接
   curl -v http://monitor-platform:8081/api/health
   
   # 检查防火墙
   systemctl status firewalld
   ```

3. **权限问题**
   ```bash
   # Linux 设置权限
   sudo chown -R root:root /opt/go-agent
   sudo chmod +x /opt/go-agent/go-agent
   ```

### 调试方法

```bash
# 启用详细日志
go-agent -v -c configs/config.yaml

# 检查配置文件语法
go-agent -c configs/config.yaml --help

# 测试网络连接
go-agent -c configs/config.yaml --test-connection
```

## 📊 性能优化

### 系统资源
- **内存使用**: 通常 < 50MB
- **CPU 使用**: 空闲时 < 1%
- **磁盘空间**: 程序 < 20MB，日志根据配置

### 优化建议
- 合理设置采集间隔（建议 >= 30秒）
- 定期清理日志文件
- 监控系统资源使用情况
- 根据需要调整并发数

## 🔐 安全建议

1. **使用非 root 用户运行**（如可能）
2. **配置防火墙规则**
3. **定期更新到最新版本**
4. **监控异常日志**
5. **备份配置文件**

---

**需要帮助？** 请查看项目文档或联系技术支持。