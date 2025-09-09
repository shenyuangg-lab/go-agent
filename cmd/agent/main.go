package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"go-agent/pkg/config"
	"go-agent/pkg/logger"
	"go-agent/pkg/scheduler"

	"github.com/spf13/cobra"
)

var (
	configFile string
	verbose    bool
	daemon     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "go-agent",
		Short: "Go Agent - 系统监控和指标采集代理",
		Long: `Go Agent 是一个轻量级的系统监控和指标采集代理，
支持系统指标采集、SNMP采集、脚本执行采集等功能。`,
		RunE: run,
	}

	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "configs/config.yaml", "配置文件路径")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "详细输出")
	rootCmd.PersistentFlags().BoolVarP(&daemon, "daemon", "d", false, "后台运行模式")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// 处理后台运行模式
	if daemon {
		if err := runDaemon(); err != nil {
			return fmt.Errorf("启动后台进程失败: %v", err)
		}
		return nil
	}

	// 初始化基本日志
	if err := logger.Init(verbose); err != nil {
		return fmt.Errorf("初始化日志失败: %v", err)
	}

	// 加载配置
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %v", err)
	}

	// 使用配置重新初始化日志
	if err := logger.InitWithConfig(cfg.Log.Level, cfg.Log.Format, cfg.Log.Output, verbose); err != nil {
		logger.Warnf("重新初始化日志失败: %v", err)
	}

	// 创建可取消的上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 初始化调度器
	sched := scheduler.New()
	if err := sched.Start(ctx, cfg); err != nil {
		return fmt.Errorf("启动调度器失败: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号
	go func() {
		sig := <-sigChan
		logger.Infof("接收到停止信号 %v，正在优雅关闭...", sig)
		
		// 取消上下文，这会通知所有服务停止
		cancel()
		
		// 给各个服务一些时间来响应上下文取消
		shutdownTimer := time.NewTimer(5 * time.Second)
		defer shutdownTimer.Stop()
		
		// 启动优雅关闭goroutine
		go func() {
			<-shutdownTimer.C
			logger.Warn("优雅关闭超时，强制停止调度器")
			if err := sched.Stop(); err != nil {
				logger.Errorf("强制停止调度器失败: %v", err)
			}
		}()
		
		// 正常停止调度器
		if err := sched.Stop(); err != nil {
			logger.Errorf("停止调度器失败: %v", err)
		}
	}()

	// 等待调度器完成
	sched.Wait()
	logger.Info("应用程序已优雅停止")

	return nil
}

// runDaemon 后台运行模式
func runDaemon() error {
	if runtime.GOOS == "windows" {
		return fmt.Errorf("Windows平台不支持daemon模式，请使用服务安装脚本")
	}

	// 创建子进程
	args := make([]string, 0, len(os.Args)-1)
	for _, arg := range os.Args[1:] {
		if arg != "-d" && arg != "--daemon" {
			args = append(args, arg)
		}
	}

	procAttr := &os.ProcAttr{
		Files: []*os.File{nil, nil, nil}, // 重定向标准输入输出
	}

	process, err := os.StartProcess(os.Args[0], append([]string{os.Args[0]}, args...), procAttr)
	if err != nil {
		return fmt.Errorf("无法启动后台进程: %v", err)
	}

	fmt.Printf("后台进程已启动，PID: %d\n", process.Pid)
	
	// 等待一段时间确保子进程启动成功
	time.Sleep(1 * time.Second)
	
	return nil
}
