package collector

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ScriptCollector 脚本执行采集器
type ScriptCollector struct {
	enabled bool
	scripts []string
	timeout time.Duration
}

// ScriptMetrics 脚本执行结果
type ScriptMetrics struct {
	Timestamp time.Time `json:"timestamp"`
	Script    string    `json:"script"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	ExitCode  int       `json:"exit_code"`
	Duration  float64   `json:"duration"`
}

// NewScriptCollector 创建脚本采集器
func NewScriptCollector(enabled bool, scripts []string, timeout time.Duration) *ScriptCollector {
	return &ScriptCollector{
		enabled: enabled,
		scripts: scripts,
		timeout: timeout,
	}
}

// Collect 执行脚本并采集结果
func (c *ScriptCollector) Collect(ctx context.Context) ([]*ScriptMetrics, error) {
	if !c.enabled {
		return nil, fmt.Errorf("脚本采集器未启用")
	}

	if len(c.scripts) == 0 {
		return nil, fmt.Errorf("未配置脚本")
	}

	var results []*ScriptMetrics
	for _, script := range c.scripts {
		metrics, err := c.executeScript(ctx, script)
		if err != nil {
			metrics = &ScriptMetrics{
				Timestamp: time.Now(),
				Script:    script,
				Error:     err.Error(),
				ExitCode:  -1,
			}
		}
		results = append(results, metrics)
	}

	return results, nil
}

// executeScript 执行单个脚本
func (c *ScriptCollector) executeScript(ctx context.Context, script string) (*ScriptMetrics, error) {
	startTime := time.Now()

	// 创建带超时的上下文
	execCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	// 解析脚本命令
	args := c.parseScriptCommand(script)
	if len(args) == 0 {
		return nil, fmt.Errorf("无效的脚本命令: %s", script)
	}

	// 创建命令
	cmd := exec.CommandContext(execCtx, args[0], args[1:]...)

	// 执行命令
	output, err := cmd.CombinedOutput()

	// 计算执行时间
	duration := time.Since(startTime).Seconds()

	// 获取退出码
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	// 处理输出
	outputStr := strings.TrimSpace(string(output))

	// 检查是否超时
	if execCtx.Err() == context.DeadlineExceeded {
		return &ScriptMetrics{
			Timestamp: time.Now(),
			Script:    script,
			Error:     "脚本执行超时",
			ExitCode:  -1,
			Duration:  duration,
		}, nil
	}

	return &ScriptMetrics{
		Timestamp: time.Now(),
		Script:    script,
		Output:    outputStr,
		Error: func() string {
			if err != nil {
				return err.Error()
			}
			return ""
		}(),
		ExitCode: exitCode,
		Duration: duration,
	}, nil
}

// parseScriptCommand 解析脚本命令
func (c *ScriptCollector) parseScriptCommand(script string) []string {
	// 简单的命令解析，支持基本的shell命令
	// 这里可以根据需要扩展更复杂的解析逻辑

	// 移除前后空格
	script = strings.TrimSpace(script)

	// 如果是空字符串，返回空切片
	if script == "" {
		return nil
	}

	// 简单的空格分割（不处理引号等复杂情况）
	args := strings.Fields(script)

	// 如果第一个参数是shell，则使用shell执行
	if len(args) > 0 && (args[0] == "sh" || args[0] == "bash" || args[0] == "cmd" || args[0] == "powershell") {
		return args
	}

	// 否则直接返回分割后的参数
	return args
}

// ExecuteScriptWithTimeout 执行脚本并设置超时
func (c *ScriptCollector) ExecuteScriptWithTimeout(ctx context.Context, script string, timeout time.Duration) (*ScriptMetrics, error) {
	// 临时设置超时
	originalTimeout := c.timeout
	c.timeout = timeout
	defer func() { c.timeout = originalTimeout }()

	return c.executeScript(ctx, script)
}

// GetScripts 获取配置的脚本列表
func (c *ScriptCollector) GetScripts() []string {
	return c.scripts
}

// IsEnabled 检查是否启用
func (c *ScriptCollector) IsEnabled() bool {
	return c.enabled
}

// AddScript 添加脚本
func (c *ScriptCollector) AddScript(script string) {
	c.scripts = append(c.scripts, script)
}

// RemoveScript 移除脚本
func (c *ScriptCollector) RemoveScript(script string) {
	for i, s := range c.scripts {
		if s == script {
			c.scripts = append(c.scripts[:i], c.scripts[i+1:]...)
			break
		}
	}
}

// ClearScripts 清空脚本列表
func (c *ScriptCollector) ClearScripts() {
	c.scripts = nil
}
