package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"go-agent/pkg/config"
	"go-agent/pkg/logger"
	"go-agent/pkg/scheduler"

	"github.com/spf13/cobra"
)

var (
	configFile string
	verbose    bool
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

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
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

	// 初始化调度器
	sched := scheduler.New()
	if err := sched.Start(cfg); err != nil {
		return fmt.Errorf("启动调度器失败: %v", err)
	}

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 等待信号或调度器停止
	go func() {
		<-sigChan
		logger.Info("接收到停止信号，正在优雅关闭...")
		if err := sched.Stop(); err != nil {
			logger.Errorf("停止调度器失败: %v", err)
		}
	}()

	// 等待调度器完成
	sched.Wait()
	logger.Info("应用程序已停止")

	return nil
}
