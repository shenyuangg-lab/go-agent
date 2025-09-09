package collector

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"go-agent/pkg/client"
	"go-agent/pkg/services"

	_ "github.com/go-sql-driver/mysql"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// CommandConfig 命令配置结构
type CommandConfig struct {
	Type        string `mapstructure:"type"`
	Command     string `mapstructure:"command"`
	Host        string `mapstructure:"host"`
	Port        int    `mapstructure:"port"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	Database    string `mapstructure:"database"`
	Timeout     int    `mapstructure:"timeout"`
	Description string `mapstructure:"description"`
}

// CommandSettings 全局设置
type CommandSettings struct {
	DefaultTimeout int  `mapstructure:"default_timeout"`
	Enabled        bool `mapstructure:"enabled"`
	RetryCount     int  `mapstructure:"retry_count"`
	RetryInterval  int  `mapstructure:"retry_interval"`
	MaxConcurrent  int  `mapstructure:"max_concurrent"`
}

// CommandCollector 命令执行采集器
type CommandCollector struct {
	commands      map[string]CommandConfig
	settings      CommandSettings
	logger        *logrus.Logger
	deviceClient  *client.DeviceMonitorClient
	metricsSender *services.MetricsSender
	semaphore     chan struct{}    // 并发控制
	monitorItems  map[string]int64 // itemKey -> itemID 映射
	mutex         sync.RWMutex
}

// NewCommandCollector 创建命令执行采集器
func NewCommandCollector(configPath string, logger *logrus.Logger, deviceClient *client.DeviceMonitorClient, metricsSender *services.MetricsSender) (*CommandCollector, error) {
	collector := &CommandCollector{
		commands:      make(map[string]CommandConfig),
		logger:        logger,
		deviceClient:  deviceClient,
		metricsSender: metricsSender,
		monitorItems:  make(map[string]int64),
	}

	// 加载配置
	if err := collector.loadConfig(configPath); err != nil {
		return nil, fmt.Errorf("加载命令映射配置失败: %v", err)
	}

	// 创建并发控制信号量
	collector.semaphore = make(chan struct{}, collector.settings.MaxConcurrent)

	return collector, nil
}

// loadConfig 加载配置文件
func (c *CommandCollector) loadConfig(configPath string) error {
	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	// 加载命令配置
	commandsMap := v.GetStringMap("commands")
	for key := range commandsMap {
		var config CommandConfig
		if err := v.UnmarshalKey(fmt.Sprintf("commands.%s", key), &config); err != nil {
			c.logger.Error("解析命令配置失败", map[string]interface{}{
				"key":   key,
				"error": err.Error(),
			})
			continue
		}
		c.commands[key] = config
	}

	// 加载全局设置
	if err := v.UnmarshalKey("settings", &c.settings); err != nil {
		c.logger.Warn("解析全局设置失败，使用默认值", map[string]interface{}{
			"error": err.Error(),
		})
		c.settings = CommandSettings{
			DefaultTimeout: 30,
			Enabled:        true,
			RetryCount:     2,
			RetryInterval:  5,
			MaxConcurrent:  10,
		}
	}

	c.logger.Info("命令映射配置加载完成", map[string]interface{}{
		"command_count": len(c.commands),
		"enabled":       c.settings.Enabled,
	})

	return nil
}

// UpdateMonitorItems 更新监控项映射
func (c *CommandCollector) UpdateMonitorItems(items []client.ConfigResponseData) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.monitorItems = make(map[string]int64)
	for _, item := range items {
		c.monitorItems[item.ItemKey] = item.ItemID
	}

	c.logger.Info("更新监控项映射", map[string]interface{}{
		"item_count": len(c.monitorItems),
	})
}

// Collect 执行采集
func (c *CommandCollector) Collect(ctx context.Context) {
	if !c.settings.Enabled {
		c.logger.Debug("命令执行采集器已禁用")
		return
	}

	c.mutex.RLock()
	monitorItems := make(map[string]int64)
	for k, v := range c.monitorItems {
		monitorItems[k] = v
	}
	c.mutex.RUnlock()

	if len(monitorItems) == 0 {
		c.logger.Debug("没有可执行的监控项")
		return
	}

	// 统计执行情况
	totalItems := len(monitorItems)
	executedCount := 0
	skippedCount := 0

	var wg sync.WaitGroup
	for itemKey, itemID := range monitorItems {
		// 检查是否有对应的命令配置
		if cmdConfig, exists := c.commands[itemKey]; exists {
			executedCount++
			wg.Add(1)
			go func(key string, id int64, config CommandConfig) {
				defer wg.Done()
				c.executeCommand(ctx, key, id, config)
			}(itemKey, itemID, cmdConfig)
		} else {
			skippedCount++
			c.logger.Debug("跳过未配置命令的监控项", map[string]interface{}{
				"item_key": itemKey,
				"item_id":  itemID,
			})
		}
	}

	wg.Wait()
	c.logger.Info("命令执行采集完成", map[string]interface{}{
		"total_items":    totalItems,
		"executed_count": executedCount,
		"skipped_count":  skippedCount,
	})
}

// executeCommand 执行单个命令
func (c *CommandCollector) executeCommand(ctx context.Context, itemKey string, itemID int64, config CommandConfig) {
	// 并发控制
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return
	}

	timeout := time.Duration(config.Timeout) * time.Second
	if config.Timeout == 0 {
		timeout = time.Duration(c.settings.DefaultTimeout) * time.Second
	}

	var result interface{}
	var err error

	// 重试机制
	for i := 0; i <= c.settings.RetryCount; i++ {
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)

		switch strings.ToLower(config.Type) {
		case "powershell":
			result, err = c.executePowerShell(cmdCtx, config.Command)
		case "cmd":
			result, err = c.executeCmd(cmdCtx, config.Command)
		case "mysql":
			result, err = c.executeMySQL(cmdCtx, config)
		case "script":
			result, err = c.executeScript(cmdCtx, config.Command)
		default:
			err = fmt.Errorf("不支持的命令类型: %s", config.Type)
		}

		cancel()

		if err == nil {
			break
		}

		if i < c.settings.RetryCount {
			c.logger.Warn("命令执行失败，准备重试", map[string]interface{}{
				"item_key":    itemKey,
				"item_id":     itemID,
				"error":       err.Error(),
				"retry_count": i + 1,
				"max_retries": c.settings.RetryCount,
			})
			time.Sleep(time.Duration(c.settings.RetryInterval) * time.Second)
		}
	}

	if err != nil {
		c.logger.Error("命令执行失败", map[string]interface{}{
			"item_key": itemKey,
			"item_id":  itemID,
			"error":    err.Error(),
			"type":     config.Type,
			"command":  config.Command,
		})
		return
	}

	// 发送指标数据
	if c.metricsSender != nil {
		err = c.metricsSender.SendMetricImmediate(ctx, itemID, result)
		if err != nil {
			c.logger.Error("发送指标数据失败", map[string]interface{}{
				"item_key": itemKey,
				"item_id":  itemID,
				"value":    result,
				"error":    err.Error(),
			})
		} else {
			c.logger.Debug("命令执行并发送指标成功", map[string]interface{}{
				"item_key": itemKey,
				"item_id":  itemID,
				"value":    result,
			})
		}
	}
}

// executePowerShell 执行PowerShell命令
func (c *CommandCollector) executePowerShell(ctx context.Context, command string) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "powershell", "-Command", command)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("PowerShell执行失败: %v", err)
	}

	result := strings.TrimSpace(string(output))

	// 尝试将结果转换为数字
	if val, err := strconv.ParseFloat(result, 64); err == nil {
		return val, nil
	}
	if val, err := strconv.ParseInt(result, 10, 64); err == nil {
		return val, nil
	}

	return result, nil
}

// executeCmd 执行CMD命令
func (c *CommandCollector) executeCmd(ctx context.Context, command string) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "cmd", "/C", command)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("CMD执行失败: %v", err)
	}

	result := strings.TrimSpace(string(output))

	// 尝试将结果转换为数字
	if val, err := strconv.ParseFloat(result, 64); err == nil {
		return val, nil
	}
	if val, err := strconv.ParseInt(result, 10, 64); err == nil {
		return val, nil
	}

	return result, nil
}

// executeMySQL 执行MySQL查询
func (c *CommandCollector) executeMySQL(ctx context.Context, config CommandConfig) (interface{}, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		config.Username, config.Password, config.Host, config.Port, config.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("连接MySQL失败: %v", err)
	}
	defer db.Close()

	db.SetMaxIdleConns(2)
	db.SetMaxOpenConns(5)
	db.SetConnMaxLifetime(time.Minute * 3)

	if err = db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("MySQL连接检查失败: %v", err)
	}

	rows, err := db.QueryContext(ctx, config.Command)
	if err != nil {
		return nil, fmt.Errorf("MySQL查询失败: %v", err)
	}
	defer rows.Close()

	// 获取列信息
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("获取列信息失败: %v", err)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("查询结果为空")
	}

	// 读取第一行数据
	if rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("扫描结果失败: %v", err)
		}

		// 如果只有一列，直接返回值
		if len(values) == 1 {
			val := values[0]
			if val == nil {
				return 0, nil
			}

			// 尝试转换为数字
			switch v := val.(type) {
			case []byte:
				str := string(v)
				if num, err := strconv.ParseFloat(str, 64); err == nil {
					return num, nil
				}
				if num, err := strconv.ParseInt(str, 10, 64); err == nil {
					return num, nil
				}
				return str, nil
			case string:
				if num, err := strconv.ParseFloat(v, 64); err == nil {
					return num, nil
				}
				if num, err := strconv.ParseInt(v, 10, 64); err == nil {
					return num, nil
				}
				return v, nil
			case int64:
				return v, nil
			case float64:
				return v, nil
			default:
				return fmt.Sprintf("%v", v), nil
			}
		}

		// 多列结果返回第一列的值
		val := values[0]
		if val == nil {
			return 0, nil
		}
		return fmt.Sprintf("%v", val), nil
	}

	return nil, fmt.Errorf("查询结果为空")
}

// executeScript 执行脚本文件
func (c *CommandCollector) executeScript(ctx context.Context, scriptPath string) (interface{}, error) {
	cmd := exec.CommandContext(ctx, scriptPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("脚本执行失败: %v", err)
	}

	result := strings.TrimSpace(string(output))

	// 尝试将结果转换为数字
	if val, err := strconv.ParseFloat(result, 64); err == nil {
		return val, nil
	}
	if val, err := strconv.ParseInt(result, 10, 64); err == nil {
		return val, nil
	}

	return result, nil
}

// GetCommandCount 获取命令总数
func (c *CommandCollector) GetCommandCount() int {
	return len(c.commands)
}

// GetEnabledStatus 获取启用状态
func (c *CommandCollector) GetEnabledStatus() bool {
	return c.settings.Enabled
}

// ListCommands 列出所有命令
func (c *CommandCollector) ListCommands() map[string]string {
	result := make(map[string]string)
	for key, config := range c.commands {
		result[key] = config.Description
	}
	return result
}

// GetSupportedItemKeys 获取所有支持的itemKey列表
func (c *CommandCollector) GetSupportedItemKeys() []string {
	keys := make([]string, 0, len(c.commands))
	for key := range c.commands {
		keys = append(keys, key)
	}
	return keys
}

// HasCommand 检查是否支持指定的itemKey
func (c *CommandCollector) HasCommand(itemKey string) bool {
	_, exists := c.commands[itemKey]
	return exists
}

// GetCommandConfig 获取指定itemKey的命令配置
func (c *CommandCollector) GetCommandConfig(itemKey string) (CommandConfig, bool) {
	config, exists := c.commands[itemKey]
	return config, exists
}

// GetActiveMonitorItems 获取当前激活的监控项
func (c *CommandCollector) GetActiveMonitorItems() map[string]int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	result := make(map[string]int64)
	for k, v := range c.monitorItems {
		result[k] = v
	}
	return result
}

// GetExecutableItems 获取可执行的监控项（既有监控配置又有命令配置）
func (c *CommandCollector) GetExecutableItems() map[string]int64 {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	result := make(map[string]int64)
	for itemKey, itemID := range c.monitorItems {
		if _, exists := c.commands[itemKey]; exists {
			result[itemKey] = itemID
		}
	}
	return result
}
