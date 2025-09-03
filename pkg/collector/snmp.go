package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/gosnmp/gosnmp"
)

// SNMPCollector SNMP采集器
type SNMPCollector struct {
	enabled   bool
	targets   []string
	community string
	version   string
	port      int
	timeout   time.Duration
}

// SNMPMetrics SNMP指标结构
type SNMPMetrics struct {
	Timestamp time.Time              `json:"timestamp"`
	Target    string                 `json:"target"`
	Metrics   map[string]interface{} `json:"metrics"`
	Error     string                 `json:"error,omitempty"`
}

// NewSNMPCollector 创建SNMP采集器
func NewSNMPCollector(enabled bool, targets []string, community, version string, port int, timeout time.Duration) *SNMPCollector {
	return &SNMPCollector{
		enabled:   enabled,
		targets:   targets,
		community: community,
		version:   version,
		port:      port,
		timeout:   timeout,
	}
}

// Collect 采集SNMP指标
func (c *SNMPCollector) Collect(ctx context.Context) ([]*SNMPMetrics, error) {
	if !c.enabled {
		return nil, fmt.Errorf("SNMP采集器未启用")
	}

	if len(c.targets) == 0 {
		return nil, fmt.Errorf("未配置SNMP目标")
	}

	var results []*SNMPMetrics
	for _, target := range c.targets {
		metrics, err := c.collectFromTarget(ctx, target)
		if err != nil {
			metrics = &SNMPMetrics{
				Timestamp: time.Now(),
				Target:    target,
				Error:     err.Error(),
			}
		}
		results = append(results, metrics)
	}

	return results, nil
}

// collectFromTarget 从指定目标采集SNMP指标
func (c *SNMPCollector) collectFromTarget(ctx context.Context, target string) (*SNMPMetrics, error) {
	// 创建SNMP客户端
	snmp := &gosnmp.GoSNMP{
		Target:    target,
		Port:      uint16(c.port),
		Community: c.community,
		Version:   c.getSNMPVersion(),
		Timeout:   c.timeout,
		Retries:   3,
	}

	// 连接SNMP设备
	if err := snmp.Connect(); err != nil {
		return nil, fmt.Errorf("连接SNMP设备失败: %v", err)
	}
	defer snmp.Conn.Close()

	// 定义要采集的OID
	oids := []string{
		"1.3.6.1.2.1.1.1.0",    // sysDescr
		"1.3.6.1.2.1.1.3.0",    // sysUptime
		"1.3.6.1.2.1.2.2.1.2",  // ifDescr
		"1.3.6.1.2.1.2.2.1.10", // ifInOctets
		"1.3.6.1.2.1.2.2.1.16", // ifOutOctets
		"1.3.6.1.2.1.2.2.1.5",  // ifSpeed
		"1.3.6.1.2.1.2.2.1.7",  // ifAdminStatus
		"1.3.6.1.2.1.2.2.1.8",  // ifOperStatus
	}

	// 执行SNMP Walk操作
	result, err := snmp.WalkAll(oids[0])
	if err != nil {
		return nil, fmt.Errorf("SNMP Walk失败: %v", err)
	}

	// 解析结果
	metrics := make(map[string]interface{})
	for _, pdu := range result {
		oid := pdu.Name
		value := pdu.Value

		switch pdu.Type {
		case gosnmp.OctetString:
			metrics[oid] = string(value.([]byte))
		case gosnmp.Integer:
			metrics[oid] = value.(int)
		case gosnmp.Counter32, gosnmp.Counter64:
			metrics[oid] = value.(uint)
		case gosnmp.Gauge32:
			metrics[oid] = value.(uint)
		case gosnmp.TimeTicks:
			metrics[oid] = value.(uint)
		default:
			metrics[oid] = fmt.Sprintf("%v", value)
		}
	}

	// 采集系统描述和运行时间
	if err := c.collectSystemInfo(snmp, metrics); err != nil {
		return nil, fmt.Errorf("采集系统信息失败: %v", err)
	}

	return &SNMPMetrics{
		Timestamp: time.Now(),
		Target:    target,
		Metrics:   metrics,
	}, nil
}

// collectSystemInfo 采集系统基本信息
func (c *SNMPCollector) collectSystemInfo(snmp *gosnmp.GoSNMP, metrics map[string]interface{}) error {
	// 系统描述
	sysDescr, err := snmp.Get([]string{"1.3.6.1.2.1.1.1.0"})
	if err == nil && len(sysDescr.Variables) > 0 {
		if sysDescr.Variables[0].Type == gosnmp.OctetString {
			metrics["system.description"] = string(sysDescr.Variables[0].Value.([]byte))
		}
	}

	// 系统运行时间
	sysUptime, err := snmp.Get([]string{"1.3.6.1.2.1.1.3.0"})
	if err == nil && len(sysUptime.Variables) > 0 {
		if sysUptime.Variables[0].Type == gosnmp.TimeTicks {
			metrics["system.uptime"] = sysUptime.Variables[0].Value.(uint)
		}
	}

	return nil
}

// getSNMPVersion 获取SNMP版本
func (c *SNMPCollector) getSNMPVersion() gosnmp.SnmpVersion {
	switch c.version {
	case "1":
		return gosnmp.Version1
	case "2c":
		return gosnmp.Version2c
	case "3":
		return gosnmp.Version3
	default:
		return gosnmp.Version2c
	}
}

// GetTargets 获取配置的目标列表
func (c *SNMPCollector) GetTargets() []string {
	return c.targets
}

// IsEnabled 检查是否启用
func (c *SNMPCollector) IsEnabled() bool {
	return c.enabled
}
