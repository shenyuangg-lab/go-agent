package collector

import (
	"fmt"
	"runtime"
	"strings"
)

// BuiltinKey 内置指标键
type BuiltinKey struct {
	Key         string       `json:"key"`
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Category    string       `json:"category"`
	Description string       `json:"description"`
	ValueType   string       `json:"value_type"` // numeric, text, log
	Units       string       `json:"units"`
	Interval    int          `json:"interval"` // 默认采集间隔(秒)
	Extractor   KeyExtractor `json:"-"`        // 数据提取函数
}

// KeyExtractor 键值提取函数
type KeyExtractor func(metrics *SystemMetrics) interface{}

// BuiltinKeyManager 内置键管理器
type BuiltinKeyManager struct {
	keys map[string]*BuiltinKey
}

// NewBuiltinKeyManager 创建内置键管理器
func NewBuiltinKeyManager() *BuiltinKeyManager {
	manager := &BuiltinKeyManager{
		keys: make(map[string]*BuiltinKey),
	}
	manager.initBuiltinKeys()
	return manager
}

// GetAllKeys 获取所有内置键
func (m *BuiltinKeyManager) GetAllKeys() []*BuiltinKey {
	keys := make([]*BuiltinKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, key)
	}
	return keys
}

// GetKey 根据键名获取内置键
func (m *BuiltinKeyManager) GetKey(keyName string) (*BuiltinKey, bool) {
	key, exists := m.keys[keyName]
	return key, exists
}

// GetKeysByCategory 根据分类获取键
func (m *BuiltinKeyManager) GetKeysByCategory(category string) []*BuiltinKey {
	var keys []*BuiltinKey
	for _, key := range m.keys {
		if key.Category == category {
			keys = append(keys, key)
		}
	}
	return keys
}

// ExtractValue 提取键值
func (m *BuiltinKeyManager) ExtractValue(keyName string, metrics *SystemMetrics) (interface{}, error) {
	key, exists := m.keys[keyName]
	if !exists {
		return nil, fmt.Errorf("未找到内置键: %s", keyName)
	}

	if key.Extractor == nil {
		return nil, fmt.Errorf("键 %s 没有提取函数", keyName)
	}

	return key.Extractor(metrics), nil
}

// initBuiltinKeys 初始化所有内置键
func (m *BuiltinKeyManager) initBuiltinKeys() {
	// CPU 指标
	m.addCPUKeys()
	// 内存指标
	m.addMemoryKeys()
	// 磁盘指标
	m.addDiskKeys()
	// 网络指标
	m.addNetworkKeys()
	// 主机信息
	m.addHostKeys()
}

// addKey 添加键
func (m *BuiltinKeyManager) addKey(key *BuiltinKey) {
	m.keys[key.Key] = key
}

// addCPUKeys 添加CPU相关键
func (m *BuiltinKeyManager) addCPUKeys() {
	// CPU使用率
	m.addKey(&BuiltinKey{
		Key:         "system.cpu.util",
		Name:        "CPU使用率",
		Type:        "builtin",
		Category:    "cpu",
		Description: "CPU总使用率百分比",
		ValueType:   "numeric",
		Units:       "%",
		Interval:    30,
		Extractor: func(metrics *SystemMetrics) interface{} {
			return metrics.CPU.UsagePercent
		},
	})

	// CPU核心数
	m.addKey(&BuiltinKey{
		Key:         "system.cpu.num",
		Name:        "CPU核心数",
		Type:        "builtin",
		Category:    "cpu",
		Description: "逻辑CPU核心数量",
		ValueType:   "numeric",
		Units:       "",
		Interval:    300, // 5分钟采集一次即可
		Extractor: func(metrics *SystemMetrics) interface{} {
			return metrics.CPU.Count
		},
	})

	// 系统负载平均值（非Windows系统）
	if runtime.GOOS != "windows" {
		loadKeys := []struct {
			key, name, desc string
			index           int
		}{
			{"system.cpu.load1", "1分钟负载", "系统1分钟平均负载", 0},
			{"system.cpu.load5", "5分钟负载", "系统5分钟平均负载", 1},
			{"system.cpu.load15", "15分钟负载", "系统15分钟平均负载", 2},
		}

		for _, load := range loadKeys {
			idx := load.index
			m.addKey(&BuiltinKey{
				Key:         load.key,
				Name:        load.name,
				Type:        "builtin",
				Category:    "cpu",
				Description: load.desc,
				ValueType:   "numeric",
				Units:       "",
				Interval:    60,
				Extractor: func(metrics *SystemMetrics) interface{} {
					if len(metrics.CPU.LoadAvg) > idx {
						return metrics.CPU.LoadAvg[idx]
					}
					return 0.0
				},
			})
		}
	}
}

// addMemoryKeys 添加内存相关键
func (m *BuiltinKeyManager) addMemoryKeys() {
	memKeys := []struct {
		key, name, desc, units string
		extractor              KeyExtractor
	}{
		{
			"vm.memory.size[total]", "内存总量", "系统物理内存总量", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Memory.Total },
		},
		{
			"vm.memory.size[used]", "内存已用", "系统已使用的物理内存", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Memory.Used },
		},
		{
			"vm.memory.size[free]", "内存空闲", "系统空闲的物理内存", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Memory.Free },
		},
		{
			"vm.memory.util", "内存使用率", "物理内存使用率百分比", "%",
			func(metrics *SystemMetrics) interface{} { return metrics.Memory.UsagePercent },
		},
	}

	for _, mem := range memKeys {
		m.addKey(&BuiltinKey{
			Key:         mem.key,
			Name:        mem.name,
			Type:        "builtin",
			Category:    "memory",
			Description: mem.desc,
			ValueType:   "numeric",
			Units:       mem.units,
			Interval:    30,
			Extractor:   mem.extractor,
		})
	}
}

// addDiskKeys 添加磁盘相关键
func (m *BuiltinKeyManager) addDiskKeys() {
	// 磁盘空间使用
	diskKeys := []struct {
		key, name, desc, units string
		extractor              KeyExtractor
	}{
		{
			getDiskSpaceKey("total"), "磁盘总空间", "磁盘总空间大小", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.Total },
		},
		{
			getDiskSpaceKey("used"), "磁盘已用空间", "磁盘已使用空间大小", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.Used },
		},
		{
			getDiskSpaceKey("free"), "磁盘空闲空间", "磁盘空闲空间大小", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.Free },
		},
		{
			getDiskUtilKey(), "磁盘使用率", "磁盘空间使用率百分比", "%",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.UsagePercent },
		},
	}

	for _, disk := range diskKeys {
		m.addKey(&BuiltinKey{
			Key:         disk.key,
			Name:        disk.name,
			Type:        "builtin",
			Category:    "disk",
			Description: disk.desc,
			ValueType:   "numeric",
			Units:       disk.units,
			Interval:    60,
			Extractor:   disk.extractor,
		})
	}

	// 磁盘IO统计
	ioKeys := []struct {
		key, name, desc, units string
		extractor              KeyExtractor
	}{
		{
			"vfs.dev.read[,bytes]", "磁盘读字节数", "磁盘读取的总字节数", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.IOStats.ReadBytes },
		},
		{
			"vfs.dev.write[,bytes]", "磁盘写字节数", "磁盘写入的总字节数", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.IOStats.WriteBytes },
		},
		{
			"vfs.dev.read[,ops]", "磁盘读操作数", "磁盘读操作的总次数", "",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.IOStats.ReadCount },
		},
		{
			"vfs.dev.write[,ops]", "磁盘写操作数", "磁盘写操作的总次数", "",
			func(metrics *SystemMetrics) interface{} { return metrics.Disk.IOStats.WriteCount },
		},
	}

	for _, io := range ioKeys {
		m.addKey(&BuiltinKey{
			Key:         io.key,
			Name:        io.name,
			Type:        "builtin",
			Category:    "disk",
			Description: io.desc,
			ValueType:   "numeric",
			Units:       io.units,
			Interval:    30,
			Extractor:   io.extractor,
		})
	}
}

// addNetworkKeys 添加网络相关键
func (m *BuiltinKeyManager) addNetworkKeys() {
	netKeys := []struct {
		key, name, desc, units string
		extractor              KeyExtractor
	}{
		{
			"net.if.out[,bytes]", "网络发送字节数", "网络接口发送的总字节数", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Network.BytesSent },
		},
		{
			"net.if.in[,bytes]", "网络接收字节数", "网络接口接收的总字节数", "B",
			func(metrics *SystemMetrics) interface{} { return metrics.Network.BytesRecv },
		},
		{
			"net.if.out[,packets]", "网络发送包数", "网络接口发送的总数据包数", "",
			func(metrics *SystemMetrics) interface{} { return metrics.Network.PacketsSent },
		},
		{
			"net.if.in[,packets]", "网络接收包数", "网络接口接收的总数据包数", "",
			func(metrics *SystemMetrics) interface{} { return metrics.Network.PacketsRecv },
		},
	}

	for _, net := range netKeys {
		m.addKey(&BuiltinKey{
			Key:         net.key,
			Name:        net.name,
			Type:        "builtin",
			Category:    "network",
			Description: net.desc,
			ValueType:   "numeric",
			Units:       net.units,
			Interval:    30,
			Extractor:   net.extractor,
		})
	}
}

// addHostKeys 添加主机信息相关键
func (m *BuiltinKeyManager) addHostKeys() {
	hostKeys := []struct {
		key, name, desc, valueType string
		extractor                  KeyExtractor
	}{
		{
			"system.hostname", "主机名", "系统主机名", "text",
			func(metrics *SystemMetrics) interface{} { return metrics.Host.Hostname },
		},
		{
			"system.uname", "操作系统", "操作系统类型", "text",
			func(metrics *SystemMetrics) interface{} { return metrics.Host.OS },
		},
		{
			"system.platform", "系统平台", "系统平台信息", "text",
			func(metrics *SystemMetrics) interface{} { return metrics.Host.Platform },
		},
		{
			"system.uptime", "系统运行时间", "系统启动以来的运行时间", "numeric",
			func(metrics *SystemMetrics) interface{} { return metrics.Host.Uptime },
		},
	}

	for _, host := range hostKeys {
		interval := 300 // 主机信息变化不频繁，5分钟采集一次
		if host.key == "system.uptime" {
			interval = 60 // 运行时间每分钟更新
		}

		m.addKey(&BuiltinKey{
			Key:         host.key,
			Name:        host.name,
			Type:        "builtin",
			Category:    "host",
			Description: host.desc,
			ValueType:   host.valueType,
			Units:       "s",
			Interval:    interval,
			Extractor:   host.extractor,
		})
	}
}

// getDiskSpaceKey 获取磁盘空间键名
func getDiskSpaceKey(mode string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("vfs.fs.size[C:,%s]", mode)
	}
	return fmt.Sprintf("vfs.fs.size[/,%s]", mode)
}

// getDiskUtilKey 获取磁盘使用率键名
func getDiskUtilKey() string {
	if runtime.GOOS == "windows" {
		return "vfs.fs.pused[C:]"
	}
	return "vfs.fs.pused[/]"
}

// FormatKeyName 格式化键名（支持参数替换）
func FormatKeyName(template string, params ...string) string {
	result := template
	for i, param := range params {
		placeholder := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, placeholder, param)
	}
	return result
}

// ValidateKey 验证键名是否符合规范
func ValidateKey(keyName string) bool {
	if keyName == "" {
		return false
	}

	// 基本格式检查
	if strings.Contains(keyName, " ") {
		return false
	}

	// 长度检查
	if len(keyName) > 255 {
		return false
	}

	return true
}
