@echo off
echo ==========================================
echo Go Agent Windows 完整打包脚本
echo ==========================================

:: 设置编码为UTF-8
chcp 65001 >nul

:: 获取版本信息
for /f "tokens=*" %%i in ('git describe --tags --always 2^>nul') do set GIT_VERSION=%%i
if "%GIT_VERSION%"=="" set GIT_VERSION=v1.0.0

:: 设置变量
set VERSION=%GIT_VERSION%
set BUILD_TIME=%DATE% %TIME%
set PACKAGE_NAME=go-agent-windows-%VERSION%
set BUILD_DIR=build
set PACKAGE_DIR=%BUILD_DIR%\%PACKAGE_NAME%

echo [信息] 开始打包 Go Agent Windows 版本
echo [信息] 版本: %VERSION%
echo [信息] 构建时间: %BUILD_TIME%
echo [信息] 包名: %PACKAGE_NAME%

:: 检查Go环境
go version >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [错误] 未找到Go环境，请先安装Go
    echo 下载地址: https://golang.org/dl/
    pause
    exit /b 1
)

:: 检查源文件
if not exist "cmd\agent\main.go" (
    echo [错误] 请在项目根目录运行此脚本
    pause
    exit /b 1
)

:: 清理旧的构建目录
echo [信息] 清理旧的构建文件...
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
mkdir %BUILD_DIR%
mkdir %PACKAGE_DIR%
mkdir %PACKAGE_DIR%\configs
mkdir %PACKAGE_DIR%\logs
mkdir %PACKAGE_DIR%\scripts
mkdir %PACKAGE_DIR%\docs

:: 更新依赖
echo [信息] 更新依赖包...
go mod tidy
if %ERRORLEVEL% neq 0 (
    echo [警告] 依赖包更新失败，继续构建...
)

:: 构建程序 - Windows 64位
echo [信息] 构建 Windows 64位可执行文件...
set CGO_ENABLED=0
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-s -w -X 'main.Version=%VERSION%' -X 'main.BuildTime=%BUILD_TIME%'" -o %PACKAGE_DIR%\go-agent.exe cmd/agent/main.go
if %ERRORLEVEL% neq 0 (
    echo [错误] Windows 64位构建失败
    pause
    exit /b 1
)

:: 构建程序 - Windows 32位
echo [信息] 构建 Windows 32位可执行文件...
set GOARCH=386
go build -ldflags="-s -w -X 'main.Version=%VERSION%' -X 'main.BuildTime=%BUILD_TIME%'" -o %PACKAGE_DIR%\go-agent-x86.exe cmd/agent/main.go
if %ERRORLEVEL% neq 0 (
    echo [警告] Windows 32位构建失败，跳过...
)

:: 复制配置文件
echo [信息] 复制配置文件...
copy configs\*.yaml %PACKAGE_DIR%\configs\ >nul 2>&1
copy configs\*.yml %PACKAGE_DIR%\configs\ >nul 2>&1
if not exist "%PACKAGE_DIR%\configs\config.yaml" (
    echo [错误] 配置文件复制失败
    pause
    exit /b 1
)

:: 复制文档
echo [信息] 复制文档...
copy *.md %PACKAGE_DIR%\docs\ >nul 2>&1
copy LICENSE %PACKAGE_DIR%\ >nul 2>&1

:: 创建Windows服务管理脚本
echo [信息] 创建服务管理脚本...

:: 创建启动脚本
echo @echo off > %PACKAGE_DIR%\start.bat
echo title Go Agent 监控代理 >> %PACKAGE_DIR%\start.bat
echo echo 启动 Go Agent 监控代理... >> %PACKAGE_DIR%\start.bat
echo go-agent.exe -c configs\config.yaml -v >> %PACKAGE_DIR%\start.bat
echo if %%ERRORLEVEL%% neq 0 pause >> %PACKAGE_DIR%\start.bat

:: 创建服务安装脚本
echo @echo off > %PACKAGE_DIR%\install_service.bat
echo echo 正在安装 Go Agent 服务... >> %PACKAGE_DIR%\install_service.bat
echo sc create "GoAgent" binPath= "%%~dp0go-agent.exe -c %%~dp0configs\config.yaml" start= auto >> %PACKAGE_DIR%\install_service.bat
echo sc description "GoAgent" "Go Agent 监控代理服务" >> %PACKAGE_DIR%\install_service.bat
echo sc start "GoAgent" >> %PACKAGE_DIR%\install_service.bat
echo echo 服务安装完成，已设置为自动启动 >> %PACKAGE_DIR%\install_service.bat
echo pause >> %PACKAGE_DIR%\install_service.bat

:: 创建服务卸载脚本
echo @echo off > %PACKAGE_DIR%\uninstall_service.bat
echo echo 正在卸载 Go Agent 服务... >> %PACKAGE_DIR%\uninstall_service.bat
echo sc stop "GoAgent" >> %PACKAGE_DIR%\uninstall_service.bat
echo sc delete "GoAgent" >> %PACKAGE_DIR%\uninstall_service.bat
echo echo 服务卸载完成 >> %PACKAGE_DIR%\uninstall_service.bat
echo pause >> %PACKAGE_DIR%\uninstall_service.bat

:: 创建Windows安装说明
echo [信息] 创建安装说明...
echo # Go Agent Windows 安装包 > %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ## 版本信息 >> %PACKAGE_DIR%\README_WINDOWS.md
echo - 版本: %VERSION% >> %PACKAGE_DIR%\README_WINDOWS.md
echo - 构建时间: %BUILD_TIME% >> %PACKAGE_DIR%\README_WINDOWS.md
echo - 平台: Windows x64/x86 >> %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ## 快速开始 >> %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ### 方式一：直接运行 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 1. 双击 `start.bat` 启动程序 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 2. 或在命令行运行: `go-agent.exe -c configs\config.yaml` >> %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ### 方式二：安装为Windows服务（推荐） >> %PACKAGE_DIR%\README_WINDOWS.md
echo 1. 以**管理员身份**运行 `install_service.bat` >> %PACKAGE_DIR%\README_WINDOWS.md
echo 2. 服务将自动启动并设置为开机自启 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 3. 可通过Windows服务管理器管理服务状态 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 4. 卸载服务请运行 `uninstall_service.bat` >> %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ## 命令行参数 >> %PACKAGE_DIR%\README_WINDOWS.md
echo - `-c configs\config.yaml` : 指定配置文件 >> %PACKAGE_DIR%\README_WINDOWS.md
echo - `-v` : 详细日志输出 >> %PACKAGE_DIR%\README_WINDOWS.md
echo - `-d` : 后台运行（不适用于Windows） >> %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ## 配置文件 >> %PACKAGE_DIR%\README_WINDOWS.md
echo - `configs\config.yaml` - 主配置文件 >> %PACKAGE_DIR%\README_WINDOWS.md
echo - `configs\command_mapping.yaml` - 命令映射配置 >> %PACKAGE_DIR%\README_WINDOWS.md
echo - `configs\builtin_monitoring_items.yaml` - 内置监控项配置 >> %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ## 日志文件 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 程序运行日志保存在 `logs\` 目录下 >> %PACKAGE_DIR%\README_WINDOWS.md
echo. >> %PACKAGE_DIR%\README_WINDOWS.md
echo ## 故障排除 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 1. 服务无法启动：检查配置文件路径和权限 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 2. 网络连接失败：检查防火墙和监控平台地址 >> %PACKAGE_DIR%\README_WINDOWS.md
echo 3. 查看详细日志：使用 `-v` 参数运行 >> %PACKAGE_DIR%\README_WINDOWS.md

:: 创建版本信息文件
echo [信息] 创建版本信息...
echo Go Agent Windows Version > %PACKAGE_DIR%\VERSION.txt
echo Version: %VERSION% >> %PACKAGE_DIR%\VERSION.txt
echo Build Time: %BUILD_TIME% >> %PACKAGE_DIR%\VERSION.txt
echo Platform: Windows x64/x86 >> %PACKAGE_DIR%\VERSION.txt
go version >> %PACKAGE_DIR%\VERSION.txt

:: 显示文件列表
echo [信息] 打包内容:
dir %PACKAGE_DIR% /b

:: 显示文件大小
echo.
echo [信息] 可执行文件信息:
if exist "%PACKAGE_DIR%\go-agent.exe" (
    for %%I in ("%PACKAGE_DIR%\go-agent.exe") do echo go-agent.exe: %%~zI 字节
)
if exist "%PACKAGE_DIR%\go-agent-x86.exe" (
    for %%I in ("%PACKAGE_DIR%\go-agent-x86.exe") do echo go-agent-x86.exe: %%~zI 字节
)

:: 创建ZIP压缩包
echo.
echo [信息] 创建ZIP压缩包...
powershell -command "Compress-Archive -Path '%PACKAGE_DIR%\*' -DestinationPath '%BUILD_DIR%\%PACKAGE_NAME%.zip' -Force" >nul 2>&1
if %ERRORLEVEL% equ 0 (
    echo [成功] ZIP压缩包创建成功: %BUILD_DIR%\%PACKAGE_NAME%.zip
) else (
    echo [警告] ZIP压缩包创建失败，使用传统方式
    if exist "%ProgramFiles%\7-Zip\7z.exe" (
        "%ProgramFiles%\7-Zip\7z.exe" a -tzip %BUILD_DIR%\%PACKAGE_NAME%.zip %PACKAGE_DIR%\* >nul
        if %ERRORLEVEL% equ 0 (
            echo [成功] 7-Zip压缩包创建成功
        )
    )
)

:: 显示打包结果
echo.
echo ==========================================
echo 🎉 Windows打包完成！
echo ==========================================
echo.
echo 📦 输出目录: %BUILD_DIR%\%PACKAGE_NAME%
echo 🗜️ 压缩包: %BUILD_DIR%\%PACKAGE_NAME%.zip
echo.
echo 📋 包含文件:
echo   ✅ go-agent.exe           (Windows x64主程序)
if exist "%PACKAGE_DIR%\go-agent-x86.exe" echo   ✅ go-agent-x86.exe       (Windows x86主程序)
echo   ✅ configs\               (配置文件目录)  
echo   ✅ start.bat              (快速启动脚本)
echo   ✅ install_service.bat    (服务安装脚本)
echo   ✅ uninstall_service.bat  (服务卸载脚本)
echo   ✅ README_WINDOWS.md      (Windows安装说明)
echo   ✅ docs\                  (文档目录)
echo   ✅ logs\                  (日志目录)
echo.
echo 🚀 使用方法:
echo   1. 解压到目标目录
echo   2. 双击 start.bat 或以管理员身份运行 install_service.bat
echo   3. 编辑 configs\config.yaml 配置监控平台地址
echo.
echo ⚠️  注意: 安装为服务需要管理员权限
echo ==========================================

pause