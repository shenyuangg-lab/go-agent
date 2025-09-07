@echo off
echo ==========================================
echo Go Agent Windows 打包脚本
echo ==========================================

:: 设置编码为UTF-8
chcp 65001 >nul

:: 设置变量
set VERSION=v1.0.0
set PACKAGE_NAME=go-agent-windows-%VERSION%
set BUILD_DIR=build
set PACKAGE_DIR=%BUILD_DIR%\%PACKAGE_NAME%

echo [信息] 开始打包 Go Agent Windows 版本
echo [信息] 版本: %VERSION%
echo [信息] 包名: %PACKAGE_NAME%

:: 清理旧的构建目录
echo [信息] 清理旧的构建文件...
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
mkdir %BUILD_DIR%
mkdir %PACKAGE_DIR%
mkdir %PACKAGE_DIR%\configs
mkdir %PACKAGE_DIR%\logs
mkdir %PACKAGE_DIR%\scripts

:: 构建程序
echo [信息] 构建可执行文件...
go build -ldflags="-s -w -X main.Version=%VERSION%" -o %PACKAGE_DIR%\go-agent.exe cmd/agent/main.go
if %ERRORLEVEL% neq 0 (
    echo [错误] 构建失败
    pause
    exit /b 1
)

:: 复制配置文件
echo [信息] 复制配置文件...
copy configs\*.yaml %PACKAGE_DIR%\configs\
copy configs\*.yml %PACKAGE_DIR%\configs\ 2>nul

:: 复制批处理脚本
echo [信息] 复制管理脚本...
copy *.bat %PACKAGE_DIR%\scripts\
del %PACKAGE_DIR%\scripts\package_windows.bat 2>nul

:: 复制文档
echo [信息] 复制文档...
copy *.md %PACKAGE_DIR%\ 2>nul

:: 创建启动脚本（简化版）
echo [信息] 创建启动脚本...
echo @echo off > %PACKAGE_DIR%\start.bat
echo echo 启动 Go Agent 监控代理... >> %PACKAGE_DIR%\start.bat
echo go-agent.exe -c configs\config.yaml >> %PACKAGE_DIR%\start.bat
echo pause >> %PACKAGE_DIR%\start.bat

:: 创建安装说明
echo [信息] 创建安装说明...
echo # Go Agent Windows 安装包 > %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo ## 快速开始 >> %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo 1. 双击 `start.bat` 启动程序 >> %PACKAGE_DIR%\INSTALL.md
echo 2. 或在命令行运行: `go-agent.exe -c configs\config.yaml` >> %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo ## 安装为Windows服务 >> %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo 1. 以管理员身份运行 `scripts\install_service.bat` >> %PACKAGE_DIR%\INSTALL.md
echo 2. 服务将自动启动并设置为开机自启 >> %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo ## 配置文件 >> %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo - `configs\config.yaml` - 主配置文件 >> %PACKAGE_DIR%\INSTALL.md
echo - `configs\command_mapping.yaml` - 命令映射配置 >> %PACKAGE_DIR%\INSTALL.md
echo - `configs\builtin_monitoring_items.yaml` - 内置监控项配置 >> %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo ## 日志文件 >> %PACKAGE_DIR%\INSTALL.md
echo. >> %PACKAGE_DIR%\INSTALL.md
echo 程序运行日志保存在 `logs\` 目录下 >> %PACKAGE_DIR%\INSTALL.md

:: 创建版本信息文件
echo [信息] 创建版本信息...
echo Go Agent Windows Version > %PACKAGE_DIR%\VERSION.txt
echo Version: %VERSION% >> %PACKAGE_DIR%\VERSION.txt
echo Build Date: %DATE% %TIME% >> %PACKAGE_DIR%\VERSION.txt
echo Platform: Windows x64 >> %PACKAGE_DIR%\VERSION.txt
echo Go Version: >> %PACKAGE_DIR%\VERSION.txt
go version >> %PACKAGE_DIR%\VERSION.txt

:: 显示文件列表
echo [信息] 打包内容:
dir %PACKAGE_DIR% /b /s

:: 创建ZIP压缩包
echo [信息] 创建ZIP压缩包...
if exist "%ProgramFiles%\7-Zip\7z.exe" (
    "%ProgramFiles%\7-Zip\7z.exe" a -tzip %BUILD_DIR%\%PACKAGE_NAME%.zip %PACKAGE_DIR%\*
    if %ERRORLEVEL% equ 0 (
        echo [成功] ZIP压缩包创建成功: %BUILD_DIR%\%PACKAGE_NAME%.zip
    ) else (
        echo [警告] ZIP压缩包创建失败，但文件夹已准备好
    )
) else (
    echo [警告] 未找到7-Zip，跳过压缩包创建
    echo [信息] 可以手动压缩 %PACKAGE_DIR% 文件夹
)

:: 显示打包结果
echo.
echo ==========================================
echo 打包完成！
echo ==========================================
echo.
echo 输出目录: %BUILD_DIR%\%PACKAGE_NAME%
echo 可执行文件大小:
dir %PACKAGE_DIR%\go-agent.exe
echo.
echo 发布文件包含:
echo - go-agent.exe           (主程序)
echo - configs\               (配置文件目录)  
echo - scripts\               (管理脚本)
echo - logs\                  (日志目录)
echo - start.bat              (快速启动脚本)
echo - INSTALL.md             (安装说明)
echo - VERSION.txt            (版本信息)
echo - 其他文档文件
echo.
echo 使用方法:
echo 1. 将整个 %PACKAGE_NAME% 文件夹复制到目标机器
echo 2. 双击 start.bat 或运行 go-agent.exe
echo 3. 或使用 scripts\install_service.bat 安装为系统服务
echo.
if exist %BUILD_DIR%\%PACKAGE_NAME%.zip (
    echo ZIP压缩包: %BUILD_DIR%\%PACKAGE_NAME%.zip
)
echo ==========================================

pause