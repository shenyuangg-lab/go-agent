package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// SystemCollector 系统指标采集器
type SystemCollector struct {
	enabled bool
}

// SystemMetrics 系统指标结构
type SystemMetrics struct {
	Timestamp time.Time      `json:"timestamp"`
	Host      HostInfo       `json:"host"`
	CPU       CPUMetrics     `json:"cpu"`
	Memory    MemoryMetrics  `json:"memory"`
	Disk      DiskMetrics    `json:"disk"`
	Network   NetworkMetrics `json:"network"`
}

// HostInfo 主机信息
type HostInfo struct {
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
	Platform string `json:"platform"`
	Uptime   uint64 `json:"uptime"`
}

// CPUMetrics CPU指标
type CPUMetrics struct {
	UsagePercent float64   `json:"usage_percent"`
	Count        int       `json:"count"`
	LoadAvg      []float64 `json:"load_avg"`
}

// MemoryMetrics 内存指标
type MemoryMetrics struct {
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usage_percent"`
}

// DiskMetrics 磁盘指标
type DiskMetrics struct {
	Total        uint64      `json:"total"`
	Used         uint64      `json:"used"`
	Free         uint64      `json:"free"`
	UsagePercent float64     `json:"usage_percent"`
	IOStats      DiskIOStats `json:"io_stats"`
}

// DiskIOStats 磁盘IO统计
type DiskIOStats struct {
	ReadBytes  uint64 `json:"read_bytes"`
	WriteBytes uint64 `json:"write_bytes"`
	ReadCount  uint64 `json:"read_count"`
	WriteCount uint64 `json:"write_count"`
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	BytesSent   uint64 `json:"bytes_sent"`
	BytesRecv   uint64 `json:"bytes_recv"`
	PacketsSent uint64 `json:"packets_sent"`
	PacketsRecv uint64 `json:"packets_recv"`
}

// NewSystemCollector 创建系统采集器
func NewSystemCollector(enabled bool) *SystemCollector {
	return &SystemCollector{
		enabled: enabled,
	}
}

// Collect 采集系统指标
func (c *SystemCollector) Collect(ctx context.Context) (*SystemMetrics, error) {
	if !c.enabled {
		return nil, fmt.Errorf("系统采集器未启用")
	}

	metrics := &SystemMetrics{
		Timestamp: time.Now(),
	}

	// 采集主机信息
	if err := c.collectHostInfo(metrics); err != nil {
		return nil, fmt.Errorf("采集主机信息失败: %v", err)
	}

	// 采集CPU指标
	if err := c.collectCPUMetrics(metrics); err != nil {
		return nil, fmt.Errorf("采集CPU指标失败: %v", err)
	}

	// 采集内存指标
	if err := c.collectMemoryMetrics(metrics); err != nil {
		return nil, fmt.Errorf("采集内存指标失败: %v", err)
	}

	// 采集磁盘指标
	if err := c.collectDiskMetrics(metrics); err != nil {
		return nil, fmt.Errorf("采集磁盘指标失败: %v", err)
	}

	// 采集网络指标
	if err := c.collectNetworkMetrics(metrics); err != nil {
		return nil, fmt.Errorf("采集网络指标失败: %v", err)
	}

	return metrics, nil
}

// collectHostInfo 采集主机信息
func (c *SystemCollector) collectHostInfo(metrics *SystemMetrics) error {
	hostInfo, err := host.Info()
	if err != nil {
		return err
	}

	metrics.Host = HostInfo{
		Hostname: hostInfo.Hostname,
		OS:       hostInfo.OS,
		Platform: hostInfo.Platform,
		Uptime:   hostInfo.Uptime,
	}

	return nil
}

// collectCPUMetrics 采集CPU指标
func (c *SystemCollector) collectCPUMetrics(metrics *SystemMetrics) error {
	// CPU使用率
	usagePercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return err
	}

	// CPU核心数
	count, err := cpu.Counts(false)
	if err != nil {
		return err
	}

	// 系统负载
	loadAvg, err := cpu.LoadAvg()
	if err != nil {
		return err
	}

	metrics.CPU = CPUMetrics{
		UsagePercent: usagePercent[0],
		Count:        count,
		LoadAvg:      loadAvg,
	}

	return nil
}

// collectMemoryMetrics 采集内存指标
func (c *SystemCollector) collectMemoryMetrics(metrics *SystemMetrics) error {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return err
	}

	metrics.Memory = MemoryMetrics{
		Total:        memInfo.Total,
		Used:         memInfo.Used,
		Free:         memInfo.Free,
		UsagePercent: memInfo.UsedPercent,
	}

	return nil
}

// collectDiskMetrics 采集磁盘指标
func (c *SystemCollector) collectDiskMetrics(metrics *SystemMetrics) error {
	// 磁盘使用情况
	diskUsage, err := disk.Usage("/")
	if err != nil {
		return err
	}

	// 磁盘IO统计
	diskIO, err := disk.IOCounters()
	if err != nil {
		return err
	}

	var totalReadBytes, totalWriteBytes, totalReadCount, totalWriteCount uint64
	for _, io := range diskIO {
		totalReadBytes += io.ReadBytes
		totalWriteBytes += io.WriteBytes
		totalReadCount += io.ReadCount
		totalWriteCount += io.WriteCount
	}

	metrics.Disk = DiskMetrics{
		Total:        diskUsage.Total,
		Used:         diskUsage.Used,
		Free:         diskUsage.Free,
		UsagePercent: diskUsage.UsedPercent,
		IOStats: DiskIOStats{
			ReadBytes:  totalReadBytes,
			WriteBytes: totalWriteBytes,
			ReadCount:  totalReadCount,
			WriteCount: totalWriteCount,
		},
	}

	return nil
}

// collectNetworkMetrics 采集网络指标
func (c *SystemCollector) collectNetworkMetrics(metrics *SystemMetrics) error {
	netIO, err := net.IOCounters(false)
	if err != nil {
		return err
	}

	if len(netIO) > 0 {
		metrics.Network = NetworkMetrics{
			BytesSent:   netIO[0].BytesSent,
			BytesRecv:   netIO[0].BytesRecv,
			PacketsSent: netIO[0].PacketsSent,
			PacketsRecv: netIO[0].PacketsRecv,
		}
	}

	return nil
}
