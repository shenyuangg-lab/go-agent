package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config 应用配置结构
type Config struct {
	Agent     AgentConfig     `mapstructure:"agent"`
	Collect   CollectConfig   `mapstructure:"collect"`
	Transport TransportConfig `mapstructure:"transport"`
	Log       LogConfig       `mapstructure:"log"`
}

// AgentConfig 代理配置
type AgentConfig struct {
	Name     string        `mapstructure:"name"`
	Interval time.Duration `mapstructure:"interval"`
	Timeout  time.Duration `mapstructure:"timeout"`
}

// CollectConfig 采集配置
type CollectConfig struct {
	System SystemConfig `mapstructure:"system"`
	SNMP   SNMPConfig   `mapstructure:"snmp"`
	Script ScriptConfig `mapstructure:"script"`
}

// SystemConfig 系统指标采集配置
type SystemConfig struct {
	Enabled bool `mapstructure:"enabled"`
	CPU     bool `mapstructure:"cpu"`
	Memory  bool `mapstructure:"memory"`
	Disk    bool `mapstructure:"disk"`
	Network bool `mapstructure:"network"`
}

// SNMPConfig SNMP采集配置
type SNMPConfig struct {
	Enabled   bool     `mapstructure:"enabled"`
	Targets   []string `mapstructure:"targets"`
	Community string   `mapstructure:"community"`
	Version   string   `mapstructure:"version"`
	Port      int      `mapstructure:"port"`
}

// ScriptConfig 脚本执行配置
type ScriptConfig struct {
	Enabled bool          `mapstructure:"enabled"`
	Scripts []string      `mapstructure:"scripts"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// TransportConfig 数据传输配置
type TransportConfig struct {
	HTTP HTTPConfig `mapstructure:"http"`
	GRPC GRPCConfig `mapstructure:"grpc"`
}

// HTTPConfig HTTP上报配置
type HTTPConfig struct {
	Enabled bool              `mapstructure:"enabled"`
	URL     string            `mapstructure:"url"`
	Method  string            `mapstructure:"method"`
	Headers map[string]string `mapstructure:"headers"`
}

// GRPCConfig gRPC上报配置
type GRPCConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Server  string `mapstructure:"server"`
	Port    int    `mapstructure:"port"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// Load 加载配置文件
func Load(configFile string) (*Config, error) {
	viper.SetConfigFile(configFile)
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	// 设置默认值
	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %v", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 验证配置
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("配置验证失败: %v", err)
	}

	return &config, nil
}

// setDefaults 设置默认配置值
func setDefaults() {
	viper.SetDefault("agent.name", "go-agent")
	viper.SetDefault("agent.interval", "30s")
	viper.SetDefault("agent.timeout", "10s")

	viper.SetDefault("collect.system.enabled", true)
	viper.SetDefault("collect.system.cpu", true)
	viper.SetDefault("collect.system.memory", true)
	viper.SetDefault("collect.system.disk", true)
	viper.SetDefault("collect.system.network", true)

	viper.SetDefault("collect.snmp.enabled", false)
	viper.SetDefault("collect.snmp.community", "public")
	viper.SetDefault("collect.snmp.version", "2c")
	viper.SetDefault("collect.snmp.port", 161)

	viper.SetDefault("collect.script.enabled", false)
	viper.SetDefault("collect.script.timeout", "30s")

	viper.SetDefault("transport.http.enabled", true)
	viper.SetDefault("transport.http.method", "POST")
	viper.SetDefault("transport.grpc.enabled", false)
	viper.SetDefault("transport.grpc.port", 9090)

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output", "stdout")
}

// validateConfig 验证配置
func validateConfig(cfg *Config) error {
	// 验证代理配置
	if cfg.Agent.Name == "" {
		return fmt.Errorf("代理名称不能为空")
	}
	if cfg.Agent.Interval <= 0 {
		return fmt.Errorf("采集间隔必须大于0")
	}
	if cfg.Agent.Timeout <= 0 {
		return fmt.Errorf("超时时间必须大于0")
	}

	// 验证HTTP传输配置
	if cfg.Transport.HTTP.Enabled {
		if cfg.Transport.HTTP.URL == "" {
			return fmt.Errorf("HTTP传输器启用时URL不能为空")
		}
		if cfg.Transport.HTTP.Method == "" {
			cfg.Transport.HTTP.Method = "POST"
		}
	}

	// 验证gRPC传输配置
	if cfg.Transport.GRPC.Enabled {
		if cfg.Transport.GRPC.Server == "" {
			return fmt.Errorf("gRPC传输器启用时服务器地址不能为空")
		}
		if cfg.Transport.GRPC.Port <= 0 || cfg.Transport.GRPC.Port > 65535 {
			return fmt.Errorf("gRPC端口必须在1-65535范围内")
		}
	}

	// 验证SNMP配置
	if cfg.Collect.SNMP.Enabled {
		if len(cfg.Collect.SNMP.Targets) == 0 {
			return fmt.Errorf("SNMP采集器启用时目标列表不能为空")
		}
		if cfg.Collect.SNMP.Port <= 0 || cfg.Collect.SNMP.Port > 65535 {
			return fmt.Errorf("SNMP端口必须在1-65535范围内")
		}
	}

	// 验证脚本配置
	if cfg.Collect.Script.Enabled {
		if len(cfg.Collect.Script.Scripts) == 0 {
			return fmt.Errorf("脚本采集器启用时脚本列表不能为空")
		}
		if cfg.Collect.Script.Timeout <= 0 {
			return fmt.Errorf("脚本超时时间必须大于0")
		}
	}

	return nil
}
