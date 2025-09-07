# Go Agent Windows 安装包 
 
## 快速开始 
 
1. 双击 `start.bat` 启动程序 
2. 或在命令行运行: `go-agent.exe -c configs\config.yaml` 
 
## 安装为Windows服务 
 
1. 以管理员身份运行 `scripts\install_service.bat` 
2. 服务将自动启动并设置为开机自启 
 
## 配置文件 
 
- `configs\config.yaml` - 主配置文件 
- `configs\command_mapping.yaml` - 命令映射配置 
- `configs\builtin_monitoring_items.yaml` - 内置监控项配置 
 
## 日志文件 
 
程序运行日志保存在 `logs\` 目录下 
