# CLAUDE.md

此文件为在该代码库中工作的 Claude Code (claude.ai/code) 提供指导。

## 命令

### 构建和运行
- 构建: `go build -o go-agent cmd/agent/main.go`
- 使用默认配置运行: `./go-agent`
- 使用自定义配置运行: `./go-agent -c /path/to/config.yaml`
- 详细日志运行: `./go-agent -v`
- 安装依赖: `go mod tidy`

### 开发
- 格式化代码: `go fmt ./...`
- 检查代码: `go vet ./...` 
- 运行所有模块: `go run ./cmd/agent/main.go`

## 架构

这是一个使用 Cobra CLI 框架构建的 Go 监控代理，用于收集系统指标并通过 HTTP/gRPC 传输。

### 核心组件

**入口点**: `cmd/agent/main.go` - 协调所有组件的 Cobra CLI 应用程序

**配置**: `pkg/config/` - 基于 Viper 的配置管理，支持 YAML
- 默认配置文件: `configs/config.yaml`
- 支持代理设置、采集目标、传输方法和日志选项

**调度器**: `pkg/scheduler/scheduler.go` - 基于 Cron 的任务调度器，协调所有采集和传输活动

**采集器**: `pkg/collector/`
- `system.go` - 使用 gopsutil 的系统指标（CPU、内存、磁盘、网络）
- `snmp.go` - 使用 gosnmp 的 SNMP 设备监控（v1/v2c/v3）
- `script.go` - 自定义脚本执行和结果采集

**传输**: `pkg/transport/`
- `http.go` - 带重试机制的 HTTP POST 数据传输
- `grpc.go` - gRPC 数据传输（可选）

**日志**: `pkg/logger/logger.go` - 使用 logrus 的结构化日志，支持 JSON/文本输出

**服务**: `pkg/services/` - 业务服务层
- `heartbeat.go` - 心跳服务，定期向监控平台发送状态
- `config_manager.go` - 配置管理服务，支持动态配置更新
- `metrics_sender.go` - 指标发送服务，负责批量发送采集的指标
- `register.go` - 注册服务，向监控平台注册代理

**客户端**: `pkg/client/device_monitor.go` - 设备监控平台客户端，提供注册、心跳、配置获取、指标上报等API接口

### 配置结构

代理使用分层 YAML 配置，主要包含以下部分：
- `agent`: 核心代理设置（名称、间隔、超时）
- `collect`: 采集模块（系统、snmp、脚本）
- `transport`: 数据传输（http、grpc）
- `device_monitor`: 设备监控API配置（基础URL、超时、心跳间隔等）
- `log`: 日志配置（级别、格式、输出）

### 数据流

1. 启动时，代理向设备监控平台注册（如果启用）
2. 调度器根据配置的间隔启动 cron 作业
3. 采集器从各自的来源收集指标
4. 传输模块将收集的数据发送到配置的端点
5. 心跳服务定期向监控平台报告代理状态
6. 配置管理服务定期从平台获取最新配置
7. 所有活动通过集中化日志记录器记录

### 添加新组件

**新采集器**: 在 `pkg/collector/` 中实现 `Collect(ctx context.Context)` 方法，然后在调度器中注册

**新传输器**: 在 `pkg/transport/` 中实现 `Send(ctx context.Context, data interface{}, dataType string, metadata map[string]interface{})` 方法，然后在调度器中注册

**新服务**: 在 `pkg/services/` 中创建新的业务服务，实现相应的启动和停止方法，然后在调度器中集成

### 测试

目前项目没有测试文件。建议为核心组件添加单元测试：
- 运行测试: `go test ./...`
- 运行覆盖率测试: `go test -cover ./...`
- 运行基准测试: `go test -bench=. ./...`