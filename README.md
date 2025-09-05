# Go Agent

Go Agent 是一个轻量级的系统监控和指标采集代理，支持多种采集方式和数据传输协议。

## 功能特性

### 🔍 指标采集
- **系统指标**: CPU、内存、磁盘、网络等系统资源监控
- **SNMP采集**: 支持SNMP v1/v2c/v3协议，可监控网络设备
- **脚本执行**: 支持执行自定义脚本并采集结果

### 📡 数据传输
- **HTTP上报**: 支持HTTP POST方式上报数据
- **gRPC上报**: 支持gRPC协议上报数据（可选）
- **批量传输**: 支持批量数据上报，提高传输效率
- **重试机制**: 内置重试机制，确保数据传输可靠性

### ⏰ 任务调度
- **定时采集**: 基于cron的定时任务调度
- **可配置间隔**: 支持自定义采集间隔
- **并发控制**: 支持并发采集，提高效率

### 📝 日志管理
- **结构化日志**: 基于logrus的结构化日志
- **多级别**: 支持debug、info、warn、error等日志级别
- **多格式**: 支持JSON和文本格式输出

## 项目结构


```
go-agent/
├── cmd/agent/           # 主入口 (cobra 命令行)
│   └── main.go
├── pkg/
│   ├── config/          # 配置管理 (viper)
│   │   └── config.go
│   ├── collector/       # 指标采集模块
│   │   ├── system.go    # CPU/内存/磁盘/网络
│   │   ├── snmp.go      # SNMP 采集
│   │   └── script.go    # 脚本执行采集
│   ├── transport/       # 数据上报模块
│   │   ├── http.go      # HTTP 上报
│   │   └── grpc.go      # gRPC 上报 (可选)
│   ├── scheduler/       # 定时任务 (cron)
│   │   └── scheduler.go
│   └── logger/          # 日志
│       └── logger.go
├── configs/
│   └── config.yaml      # 默认配置文件
├── go.mod
├── go.sum
└── README.md
```

## 安装要求

- Go 1.21 或更高版本
- 支持的操作系统: Linux, Windows, macOS

## 快速开始

### 1. 克隆项目

```bash
git clone <repository-url>
cd go-agent
```

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 配置

编辑 `configs/config.yaml` 文件，根据你的需求配置采集目标和上报地址。

### 4. 编译

```bash
go build -o go-agent cmd/agent/main.go
```

### 5. 运行

```bash
# 使用默认配置文件
./go-agent

# 指定配置文件
./go-agent -c /path/to/config.yaml

# 详细输出
./go-agent -v
```

## 配置说明

### 代理配置

```yaml
agent:
  name: "go-agent"        # 代理名称
  interval: "30s"         # 采集间隔
  timeout: "10s"          # 采集超时时间
```

### 采集配置

#### 系统指标

```yaml
collect:
  system:
    enabled: true         # 启用系统指标采集
    cpu: true            # 采集CPU指标
    memory: true         # 采集内存指标
    disk: true           # 采集磁盘指标
    network: true        # 采集网络指标
```

#### SNMP采集

```yaml
collect:
  snmp:
    enabled: false        # 启用SNMP采集
    targets:              # SNMP目标设备
      - "192.168.1.1"
      - "192.168.1.2"
    community: "public"   # SNMP团体名
    version: "2c"         # SNMP版本
    port: 161             # SNMP端口
```

#### 脚本执行

```yaml
collect:
  script:
    enabled: false        # 启用脚本执行
    scripts:              # 要执行的脚本
      - "echo 'Hello'"
      - "date"
    timeout: "30s"        # 执行超时时间
```

### 传输配置

#### HTTP上报

```yaml
transport:
  http:
    enabled: true
    url: "http://localhost:8080/metrics"
    method: "POST"
    headers:
      "Authorization": "Bearer token"
```

#### gRPC上报

```yaml
transport:
  grpc:
    enabled: false
    server: "localhost"
    port: 9090
```

### 日志配置

```yaml
log:
  level: "info"           # 日志级别
  format: "json"          # 日志格式
  output: "stdout"        # 输出目标
```

## 使用示例

### 基本使用

```bash
# 启动代理，使用默认配置
./go-agent

# 指定配置文件
./go-agent -c /etc/go-agent/config.yaml

# 启用详细日志
./go-agent -v
```

### 配置文件示例

```yaml
agent:
  name: "production-agent"
  interval: "60s"
  timeout: "15s"

collect:
  system:
    enabled: true
    cpu: true
    memory: true
    disk: true
    network: true
  
  snmp:
    enabled: true
    targets:
      - "192.168.1.100"
      - "192.168.1.101"
    community: "private"
    version: "2c"
    port: 161
  
  script:
    enabled: true
    scripts:
      - "df -h"
      - "free -m"
    timeout: "30s"

transport:
  http:
    enabled: true
    url: "https://monitoring.example.com/api/metrics"
    method: "POST"
    headers:
      "Authorization": "Bearer your-api-token"
      "X-Agent-ID": "production-001"
  
  grpc:
    enabled: false
    server: "grpc.example.com"
    port: 9090

log:
  level: "info"
  format: "json"
  output: "stdout"
```

## 开发

### 添加新的采集器

1. 在 `pkg/collector/` 目录下创建新的采集器文件
2. 实现 `Collect(ctx context.Context)` 方法
3. 在调度器中注册新的采集器

### 添加新的传输器

1. 在 `pkg/transport/` 目录下创建新的传输器文件
2. 实现 `Send(ctx context.Context, data interface{}, dataType string, metadata map[string]interface{})` 方法
3. 在调度器中注册新的传输器

## 贡献

欢迎提交 Issue 和 Pull Request！

## 许可证

本项目采用 MIT 许可证，详见 [LICENSE](LICENSE) 文件。

## 联系方式

如有问题或建议，请通过以下方式联系：

- 提交 Issue
- 发送邮件
- 参与讨论

---

**注意**: 这是一个示例项目，生产环境使用前请仔细测试和配置。
