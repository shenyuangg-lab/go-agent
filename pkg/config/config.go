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
