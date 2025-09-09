package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-agent/pkg/client"
	"go-agent/pkg/collector"
	"go-agent/pkg/config"
	"go-agent/pkg/logger"
	"go-agent/pkg/services"
	"go-agent/pkg/transport"

	"github.com/robfig/cron/v3"
)

// ItemScheduler 监控项调度器
type ItemScheduler struct {
	ItemID                int64
	ItemName              string
	ItemKey               string
	InfoType              int
	UpdateIntervalSeconds int
	Timeout               int
	ticker                *time.Ticker
	stopChan              chan struct{}
	running               bool
	customTrigger         *CustomTrigger // 自定义触发器
	lastExecutionTime     *time.Time     // 上次执行时间
}

// Scheduler 任务调度器
type Scheduler struct {
	cron             *cron.Cron
	config           *config.Config
	systemCollector  *collector.SystemCollector
	snmpCollector    *collector.SNMPCollector
	scriptCollector  *collector.ScriptCollector
	commandCollector *collector.CommandCollector // 新增命令执行采集器
	httpTransport    *transport.HTTPTransport
	grpcTransport    *transport.GRPCTransport
	// 新增API相关服务
	apiClient        *client.DeviceMonitorClient
	registerService  *services.RegisterService
	heartbeatService *services.HeartbeatService
	configManager    *services.ConfigManager
	metricsSender    *services.MetricsSender
	// 内置键管理器
	builtinKeyManager *collector.BuiltinKeyManager
	// 监控项调度器
	itemSchedulers map[int64]*ItemScheduler
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	mu             sync.RWMutex
	running        bool
}

// New 创建新的调度器
func New() *Scheduler {
	return &Scheduler{
		cron:           cron.New(cron.WithSeconds()),
		itemSchedulers: make(map[int64]*ItemScheduler),
	}
}

// Start 启动调度器
func (s *Scheduler) Start(parentCtx context.Context, cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("调度器已在运行")
	}

	// 将父上下文与调度器上下文合并
	s.ctx, s.cancel = context.WithCancel(parentCtx)
	
	s.config = cfg

	// 初始化采集器
	if err := s.initCollectors(); err != nil {
		return fmt.Errorf("初始化采集器失败: %v", err)
	}

	// 初始化传输器
	if err := s.initTransporters(); err != nil {
		return fmt.Errorf("初始化传输器失败: %v", err)
	}

	// 初始化内置键管理器
	if err := s.initBuiltinKeyManager(); err != nil {
		logger.Warnf("初始化内置键管理器失败: %v", err)
		// 这里只是警告，不阻止启动
	}

	// 首先添加基础的定时任务（系统指标采集等），这些不依赖API
	if err := s.addScheduledJobs(); err != nil {
		return fmt.Errorf("添加定时任务失败: %v", err)
	}

	// 初始化API服务
	if err := s.initAPIServices(); err != nil {
		logger.Warnf("初始化API服务失败: %v，将仅使用本地采集功能", err)
		// 不返回错误，让基础功能继续运行
	} else {
		// 并行启动API服务，避免阻塞主流程
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logger.Errorf("API服务启动出现panic: %v", r)
				}
			}()

			if err := s.startAPIServices(); err != nil {
				logger.Errorf("启动API服务失败: %v", err)
			} else {
				logger.Info("API服务启动成功")

				// API服务启动后，启动监控项调度器
				if err := s.startItemSchedulers(); err != nil {
					logger.Errorf("启动监控项调度器失败: %v", err)
				}
			}
		}()
	}

	// 启动cron调度器
	s.cron.Start()
	s.running = true

	// 增加一个长期运行的任务到WaitGroup，确保Wait()会阻塞
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 等待上下文取消
		<-s.ctx.Done()
		logger.Debug("调度器主循环已停止")
	}()

	logger.Info("调度器已启动")

	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	// 停止cron调度器
	s.cron.Stop()

	// 停止API服务
	s.stopAPIServices()

	// 停止监控项调度器
	s.stopItemSchedulers()

	// 取消上下文
	s.cancel()

	// 等待所有任务完成
	s.wg.Wait()

	s.running = false
	logger.Info("调度器已停止")

	return nil
}

// Wait 等待调度器停止
func (s *Scheduler) Wait() {
	s.wg.Wait()
}

// initCollectors 初始化采集器
func (s *Scheduler) initCollectors() error {
	// 初始化系统采集器
	s.systemCollector = collector.NewSystemCollector(s.config.Collect.System.Enabled)

	// 初始化SNMP采集器
	s.snmpCollector = collector.NewSNMPCollector(
		s.config.Collect.SNMP.Enabled,
		s.config.Collect.SNMP.Targets,
		s.config.Collect.SNMP.Community,
		s.config.Collect.SNMP.Version,
		s.config.Collect.SNMP.Port,
		s.config.Agent.Timeout,
	)

	// 初始化脚本采集器
	s.scriptCollector = collector.NewScriptCollector(
		s.config.Collect.Script.Enabled,
		s.config.Collect.Script.Scripts,
		s.config.Collect.Script.Timeout,
	)

	return nil
}

// initBuiltinKeyManager 初始化内置键管理器
func (s *Scheduler) initBuiltinKeyManager() error {
	s.builtinKeyManager = collector.NewBuiltinKeyManager()
	allKeys := s.builtinKeyManager.GetAllKeys()
	logger.Infof("内置键管理器初始化成功，支持 %d 个内置监控项", len(allKeys))

	// 打印所有支持的键
	for _, key := range allKeys {
		logger.Debugf("支持的内置键: %s - %s", key.Key, key.Description)
	}

	return nil
}

// initTransporters 初始化传输器
func (s *Scheduler) initTransporters() error {
	// 初始化HTTP传输器
	s.httpTransport = transport.NewHTTPTransport(
		s.config.Transport.HTTP.Enabled,
		s.config.Transport.HTTP.URL,
		s.config.Transport.HTTP.Method,
		s.config.Transport.HTTP.Headers,
		s.config.Agent.Timeout,
	)

	// 初始化gRPC传输器
	s.grpcTransport = transport.NewGRPCTransport(
		s.config.Transport.GRPC.Enabled,
		s.config.Transport.GRPC.Server,
		s.config.Transport.GRPC.Port,
		s.config.Agent.Timeout,
	)

	// 如果启用gRPC，尝试连接
	if s.config.Transport.GRPC.Enabled {
		if err := s.grpcTransport.Connect(s.ctx); err != nil {
			logger.Warnf("gRPC连接失败: %v", err)
		}
	}

	return nil
}

// addScheduledJobs 添加定时任务
func (s *Scheduler) addScheduledJobs() error {
	// 计算采集间隔（秒）
	intervalSeconds := int(s.config.Agent.Interval.Seconds())
	if intervalSeconds < 1 {
		intervalSeconds = 30 // 默认30秒
	}

	// 添加系统指标采集任务 - 仅在没有API服务时使用
	// 如果有API服务，系统指标采集将通过监控项调度器处理
	if s.config.Collect.System.Enabled && (s.config.DeviceMonitor == nil || !s.config.DeviceMonitor.Enabled) {
		cronSpec := fmt.Sprintf("*/%d * * * * *", intervalSeconds)
		_, err := s.cron.AddFunc(cronSpec, s.collectAndSendSystemMetrics)
		if err != nil {
			return fmt.Errorf("添加系统指标采集任务失败: %v", err)
		}
		logger.Infof("已添加系统指标采集任务（本地模式），间隔: %s", s.config.Agent.Interval)
	}

	// 添加SNMP采集任务 - 仅在没有API服务时使用
	if s.config.Collect.SNMP.Enabled && (s.config.DeviceMonitor == nil || !s.config.DeviceMonitor.Enabled) {
		cronSpec := fmt.Sprintf("*/%d * * * * *", intervalSeconds)
		_, err := s.cron.AddFunc(cronSpec, s.collectAndSendSNMPMetrics)
		if err != nil {
			return fmt.Errorf("添加SNMP采集任务失败: %v", err)
		}
		logger.Infof("已添加SNMP采集任务（本地模式），间隔: %s", s.config.Agent.Interval)
	}

	// 添加脚本执行任务 - 仅在没有API服务时使用  
	if s.config.Collect.Script.Enabled && (s.config.DeviceMonitor == nil || !s.config.DeviceMonitor.Enabled) {
		cronSpec := fmt.Sprintf("*/%d * * * * *", intervalSeconds)
		_, err := s.cron.AddFunc(cronSpec, s.collectAndSendScriptMetrics)
		if err != nil {
			return fmt.Errorf("添加脚本执行任务失败: %v", err)
		}
		logger.Infof("已添加脚本执行任务（本地模式），间隔: %s", s.config.Agent.Interval)
	}

	return nil
}

// collectAndSendSystemMetrics 采集并发送系统指标
func (s *Scheduler) collectAndSendSystemMetrics() {
	s.wg.Add(1)
	defer s.wg.Done()

	ctx, cancel := context.WithTimeout(s.ctx, s.config.Agent.Timeout)
	defer cancel()

	logger.Debug("开始执行系统指标采集和上报任务")

	// 采集系统指标
	metrics, err := s.systemCollector.Collect(ctx)
	if err != nil {
		logger.Errorf("采集系统指标失败: %v", err)
		return
	}

	logger.Debugf("系统指标采集成功: CPU=%.2f%%, Memory=%.2f%%", metrics.CPU.UsagePercent, metrics.Memory.UsagePercent)

	// 优先使用数据中心API上报（如果可用）
	if s.apiClient != nil && s.apiClient.GetAgentID() != "" {
		logger.Debug("使用数据中心API上报系统指标")
		s.sendSystemMetricsToDataCenter(ctx, metrics)
	} else {
		logger.Debug("数据中心API不可用，使用传统HTTP传输")
		// 发送到HTTP服务器（传统方式）
		if s.config.Transport.HTTP.Enabled {
			if err := s.httpTransport.Send(ctx, metrics, "system", nil); err != nil {
				logger.Errorf("发送系统指标到HTTP失败: %v", err)
			} else {
				logger.Debug("系统指标已发送到HTTP服务器")
			}
		}

		// 发送到gRPC服务器
		if s.config.Transport.GRPC.Enabled && s.grpcTransport.IsConnected() {
			if err := s.grpcTransport.Send(ctx, metrics, "system", nil); err != nil {
				logger.Errorf("发送系统指标到gRPC失败: %v", err)
			} else {
				logger.Debug("系统指标已发送到gRPC服务器")
			}
		}
	}
}

// sendSystemMetricsToDataCenter 使用数据中心API发送系统指标
func (s *Scheduler) sendSystemMetricsToDataCenter(ctx context.Context, metrics *collector.SystemMetrics) {
	// 定义基础监控项映射（固定ItemID）
	baseMetrics := []struct {
		itemID   int64
		itemKey  string
		getValue func() interface{}
	}{
		{1430255329320961, "system.cpu.util", func() interface{} { return metrics.CPU.UsagePercent }},
		{1430255329320962, "system.cpu.num", func() interface{} { return metrics.CPU.Count }},
		{1430255329320963, "vm.memory.size[total]", func() interface{} { return metrics.Memory.Total }},
		{1430255329320964, "vm.memory.util", func() interface{} { return metrics.Memory.UsagePercent }},
		{1430255329320965, "system.hostname", func() interface{} { return metrics.Host.Hostname }},
	}

	// 逐个发送指标
	successCount := 0
	for _, metric := range baseMetrics {
		value := metric.getValue()
		logger.Debugf("准备上报监控项: %s = %v", metric.itemKey, value)

		resp, err := s.apiClient.SendSingleMetric(ctx, metric.itemID, value)
		if err != nil {
			logger.Errorf("上报监控项失败 %s: %v", metric.itemKey, err)
			continue
		}

		if resp.Code != 200 {
			logger.Errorf("上报监控项响应异常 %s: %s", metric.itemKey, resp.Msg)
			continue
		}

		logger.Debugf("监控项上报成功: %s", metric.itemKey)
		successCount++
	}

	logger.Infof("系统指标上报完成: 成功 %d/%d 项", successCount, len(baseMetrics))
}

// collectAndSendSNMPMetrics 采集并发送SNMP指标
func (s *Scheduler) collectAndSendSNMPMetrics() {
	s.wg.Add(1)
	defer s.wg.Done()

	ctx, cancel := context.WithTimeout(s.ctx, s.config.Agent.Timeout)
	defer cancel()

	// 采集SNMP指标
	metrics, err := s.snmpCollector.Collect(ctx)
	if err != nil {
		logger.Errorf("采集SNMP指标失败: %v", err)
		return
	}

	// 发送到HTTP服务器
	if s.config.Transport.HTTP.Enabled {
		if err := s.httpTransport.SendBatch(ctx, s.convertToInterfaceSlice(metrics), "snmp", nil); err != nil {
			logger.Errorf("发送SNMP指标到HTTP失败: %v", err)
		} else {
			logger.Debug("SNMP指标已发送到HTTP服务器")
		}
	}

	// 发送到gRPC服务器
	if s.config.Transport.GRPC.Enabled && s.grpcTransport.IsConnected() {
		if err := s.grpcTransport.SendBatch(ctx, s.convertToInterfaceSlice(metrics), "snmp", nil); err != nil {
			logger.Errorf("发送SNMP指标到gRPC失败: %v", err)
		} else {
			logger.Debug("SNMP指标已发送到gRPC服务器")
		}
	}
}

// collectAndSendScriptMetrics 采集并发送脚本执行结果
func (s *Scheduler) collectAndSendScriptMetrics() {
	s.wg.Add(1)
	defer s.wg.Done()

	ctx, cancel := context.WithTimeout(s.ctx, s.config.Agent.Timeout)
	defer cancel()

	// 执行脚本并采集结果
	metrics, err := s.scriptCollector.Collect(ctx)
	if err != nil {
		logger.Errorf("执行脚本失败: %v", err)
		return
	}

	// 发送到HTTP服务器
	if s.config.Transport.HTTP.Enabled {
		if err := s.httpTransport.SendBatch(ctx, s.convertToInterfaceSlice(metrics), "script", nil); err != nil {
			logger.Errorf("发送脚本结果到HTTP失败: %v", err)
		} else {
			logger.Debug("脚本结果已发送到HTTP服务器")
		}
	}

	// 发送到gRPC服务器
	if s.config.Transport.GRPC.Enabled && s.grpcTransport.IsConnected() {
		if err := s.grpcTransport.SendBatch(ctx, s.convertToInterfaceSlice(metrics), "script", nil); err != nil {
			logger.Errorf("发送脚本结果到gRPC失败: %v", err)
		} else {
			logger.Debug("脚本结果已发送到gRPC服务器")
		}
	}
}

// convertToInterfaceSlice 将具体类型切片转换为interface{}切片
func (s *Scheduler) convertToInterfaceSlice(slice interface{}) []interface{} {
	switch v := slice.(type) {
	case []*collector.SNMPMetrics:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result
	case []*collector.ScriptMetrics:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = item
		}
		return result
	default:
		return nil
	}
}

// IsRunning 检查调度器是否正在运行
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetStatus 获取调度器状态
func (s *Scheduler) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.cron.Entries()
	var jobCount int
	for _, entry := range entries {
		if entry.ID != 0 {
			jobCount++
		}
	}

	return map[string]interface{}{
		"running":   s.running,
		"job_count": jobCount,
		"next_run":  s.getNextRunTime(entries),
		"collectors": map[string]bool{
			"system": s.systemCollector != nil && s.systemCollector.IsEnabled(),
			"snmp":   s.snmpCollector != nil && s.snmpCollector.IsEnabled(),
			"script": s.scriptCollector != nil && s.scriptCollector.IsEnabled(),
		},
		"transporters": map[string]bool{
			"http": s.httpTransport != nil && s.httpTransport.IsEnabled(),
			"grpc": s.grpcTransport != nil && s.grpcTransport.IsEnabled(),
		},
	}
}

// getNextRunTime 获取下次运行时间
func (s *Scheduler) getNextRunTime(entries []cron.Entry) *time.Time {
	var nextRun *time.Time
	for _, entry := range entries {
		if entry.ID != 0 && (nextRun == nil || entry.Next.Before(*nextRun)) {
			nextRun = &entry.Next
		}
	}
	return nextRun
}

// startItemSchedulers 启动监控项调度器
func (s *Scheduler) startItemSchedulers() error {
	if s.configManager == nil {
		logger.Warn("配置管理器为空，跳过启动监控项调度器")
		return nil
	}

	items := s.configManager.GetItems()
	logger.Infof("获取到 %d 个监控项配置", len(items))

	// 更新命令执行采集器的监控项映射
	if s.commandCollector != nil {
		configItems := make([]client.ConfigResponseData, 0, len(items))
		for _, item := range items {
			configItems = append(configItems, client.ConfigResponseData{
				ItemID:                item.ItemID,
				ItemName:              item.ItemName,
				ItemKey:               item.ItemKey,
				InfoType:              item.InfoType,
				UpdateIntervalSeconds: item.UpdateIntervalSeconds,
				Timeout:               item.Timeout,
			})
		}
		s.commandCollector.UpdateMonitorItems(configItems)
		logger.Infof("已更新命令执行采集器的监控项映射: %d 项", len(configItems))
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, item := range items {
		logger.Infof("处理监控项: ID=%d, Name=%s, Key=%s, Interval=%d, CustomIntervals=%d",
			item.ItemID, item.ItemName, item.ItemKey, item.UpdateIntervalSeconds, len(item.Intervals))

		// 创建自定义触发器
		customTrigger := NewCustomTrigger(&item, logger.GetLogger())

		// 如果有间隔配置（默认间隔或自定义间隔），则启动调度器
		if item.UpdateIntervalSeconds > 0 || len(item.Intervals) > 0 {
			scheduler := &ItemScheduler{
				ItemID:                item.ItemID,
				ItemName:              item.ItemName,
				ItemKey:               item.ItemKey,
				InfoType:              item.InfoType,
				UpdateIntervalSeconds: item.UpdateIntervalSeconds,
				Timeout:               item.Timeout,
				stopChan:              make(chan struct{}),
				running:               false,
				customTrigger:         customTrigger,
				lastExecutionTime:     nil,
			}

			s.itemSchedulers[item.ItemID] = scheduler
			s.startItemSchedulerWithCustomTrigger(scheduler)

			if len(item.Intervals) > 0 {
				logger.Infof("启动监控项调度器（自定义间隔）: %s", item.ItemName)
			} else {
				logger.Infof("启动监控项调度器（默认间隔）: %s", item.ItemName)
			}
		} else {
			logger.Warnf("监控项 %s 没有配置任何间隔，跳过启动", item.ItemName)
		}
	}

	logger.Infof("已启动 %d 个监控项调度器", len(s.itemSchedulers))
	return nil
}

// startItemScheduler 启动单个监控项调度器（原方法，向后兼容）
func (s *Scheduler) startItemScheduler(itemScheduler *ItemScheduler) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		interval := time.Duration(itemScheduler.UpdateIntervalSeconds) * time.Second
		itemScheduler.ticker = time.NewTicker(interval)
		itemScheduler.running = true

		logger.Infof("启动监控项调度器: %s (ID: %d, 间隔: %v)",
			itemScheduler.ItemName, itemScheduler.ItemID, interval)

		for {
			select {
			case <-s.ctx.Done():
				itemScheduler.running = false
				logger.Infof("监控项调度器停止: %s", itemScheduler.ItemName)
				return
			case <-itemScheduler.stopChan:
				itemScheduler.running = false
				logger.Infof("监控项调度器停止: %s", itemScheduler.ItemName)
				return
			case <-itemScheduler.ticker.C:
				s.collectAndSendItem(itemScheduler)
			}
		}
	}()
}

// startItemSchedulerWithCustomTrigger 启动带自定义触发器的监控项调度器
func (s *Scheduler) startItemSchedulerWithCustomTrigger(itemScheduler *ItemScheduler) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		itemScheduler.running = true

		// 计算初始间隔
		nextTime := itemScheduler.customTrigger.NextExecutionTime(nil)

		// 如果没有有效的下次执行时间，退出调度器
		if nextTime.IsZero() {
			logger.Warnf("监控项 %s 没有有效的执行间隔，调度器退出", itemScheduler.ItemName)
			itemScheduler.running = false
			return
		}

		initialDuration := time.Until(nextTime)
		if initialDuration < 0 {
			initialDuration = 0 // 立即执行
		}

		logger.Infof("启动监控项自定义调度器: %s (ID: %d, 初始间隔: %v, 首次执行时间: %v)",
			itemScheduler.ItemName, itemScheduler.ItemID, initialDuration, nextTime)

		// 初始定时器
		timer := time.NewTimer(initialDuration)
		defer timer.Stop()

		for {
			select {
			case <-s.ctx.Done():
				itemScheduler.running = false
				logger.Infof("监控项自定义调度器停止: %s", itemScheduler.ItemName)
				return
			case <-itemScheduler.stopChan:
				itemScheduler.running = false
				logger.Infof("监控项自定义调度器停止: %s", itemScheduler.ItemName)
				return
			case <-timer.C:
				// 检查是否应该执行
				if itemScheduler.customTrigger.ShouldExecuteNow() {
					now := time.Now()
					itemScheduler.lastExecutionTime = &now

					logger.Infof("执行监控项采集: %s (时间: %v)", itemScheduler.ItemName, now)
					s.collectAndSendItem(itemScheduler)

					// 计算下次执行时间
					nextTime := itemScheduler.customTrigger.NextExecutionTime(itemScheduler.lastExecutionTime)
					if nextTime.IsZero() {
						logger.Infof("监控项 %s 没有下次执行时间，调度器退出", itemScheduler.ItemName)
						itemScheduler.running = false
						return
					}

					nextInterval := time.Until(nextTime)
					if nextInterval < 0 {
						nextInterval = time.Second // 最小1秒间隔
					}

					logger.Debugf("监控项 %s 下次执行间隔: %v, 下次执行时间: %v",
						itemScheduler.ItemName, nextInterval, nextTime)

					// 重置定时器
					timer.Reset(nextInterval)
				} else {
					// 如果不应该执行，等待一小段时间后重新检查
					logger.Debugf("监控项 %s 当前不在执行时间范围内，等待1分钟后重新检查", itemScheduler.ItemName)
					timer.Reset(1 * time.Minute)
				}
			}
		}
	}()
}

// collectAndSendItem 采集并发送监控项数据
func (s *Scheduler) collectAndSendItem(itemScheduler *ItemScheduler) {
	ctx, cancel := context.WithTimeout(s.ctx, time.Duration(itemScheduler.Timeout)*time.Second)
	defer cancel()

	logger.Infof("开始采集监控项: %s (ID: %d)", itemScheduler.ItemName, itemScheduler.ItemID)

	// 根据ItemKey采集数据
	logger.Infof("正在采集数据: %s (Key: %s)", itemScheduler.ItemName, itemScheduler.ItemKey)
	value, err := s.collectItemValue(ctx, itemScheduler.ItemKey)
	if err != nil {
		logger.Errorf("采集监控项失败: %s, 错误: %v", itemScheduler.ItemName, err)
		return
	}

	logger.Infof("采集到数据: %s = %v", itemScheduler.ItemName, value)

	// 发送数据
	if s.metricsSender != nil {
		logger.Infof("正在发送数据: %s (ID: %d) = %v", itemScheduler.ItemName, itemScheduler.ItemID, value)
		err = s.metricsSender.SendMetricImmediate(ctx, itemScheduler.ItemID, value)
		if err != nil {
			logger.Errorf("发送监控项数据失败: %s, 错误: %v", itemScheduler.ItemName, err)
		} else {
			logger.Infof("✅ 发送监控项数据成功: %s (ID: %d) = %v", itemScheduler.ItemName, itemScheduler.ItemID, value)
		}
	} else {
		logger.Warn("指标发送器为空，无法发送数据")
	}
}

// collectItemValue 根据ItemKey采集指标值
func (s *Scheduler) collectItemValue(ctx context.Context, itemKey string) (interface{}, error) {
	// 按优先级顺序处理：命令映射 > 内置键 > 硬编码（向后兼容）

	// 1. 首先检查命令执行采集器（最高优先级 - 用户自定义）
	if s.commandCollector != nil && s.commandCollector.GetEnabledStatus() {
		if s.commandCollector.HasCommand(itemKey) {
			logger.Debugf("🎯 使用命令执行采集器处理: %s", itemKey)
			commands := s.commandCollector.ListCommands()
			if description, exists := commands[itemKey]; exists {
				logger.Debugf("找到命令配置: %s - %s", itemKey, description)
				// 返回一个占位值，实际值将由命令执行采集器单独发送
				return fmt.Sprintf("由命令执行采集器处理: %s", itemKey), nil
			}
		}
	}

	// 2. 然后检查内置键管理器（中等优先级 - 标准化处理）
	if s.builtinKeyManager != nil {
		if _, exists := s.builtinKeyManager.GetKey(itemKey); exists {
			logger.Debugf("🔧 使用内置键管理器处理: %s", itemKey)

			// 获取系统指标
			if s.systemCollector != nil && s.systemCollector.IsEnabled() {
				metrics, err := s.systemCollector.Collect(ctx)
				if err != nil {
					return nil, fmt.Errorf("采集系统指标失败: %v", err)
				}

				// 使用内置键管理器提取值
				value, err := s.builtinKeyManager.ExtractValue(itemKey, metrics)
				if err != nil {
					logger.Warnf("内置键管理器提取 %s 失败: %v，尝试其他方式", itemKey, err)
				} else {
					logger.Debugf("内置键管理器成功提取 %s = %v", itemKey, value)
					return value, nil
				}
			}
		}
	}

	// 3. 最后使用硬编码系统采集器（最低优先级 - 向后兼容）
	if s.systemCollector != nil && s.systemCollector.IsEnabled() {
		logger.Debugf("⚙️ 使用硬编码系统采集器（向后兼容）: %s", itemKey)
		metrics, err := s.systemCollector.Collect(ctx)
		if err != nil {
			return nil, fmt.Errorf("采集系统指标失败: %v", err)
		}

		// 硬编码的常用监控项（向后兼容）
		switch itemKey {
		case "system.cpu.util":
			logger.Debugf("硬编码处理 CPU 使用率")
			return metrics.CPU.UsagePercent, nil
		case "system.cpu.num":
			logger.Debugf("硬编码处理 CPU 核心数")
			return metrics.CPU.Count, nil
		case "vm.memory.size[total]":
			logger.Debugf("硬编码处理内存总量")
			return metrics.Memory.Total, nil
		case "vm.memory.util":
			logger.Debugf("硬编码处理内存使用率")
			return metrics.Memory.UsagePercent, nil
		case "system.hostname":
			logger.Debugf("硬编码处理主机名")
			return metrics.Host.Hostname, nil
		default:
			return nil, fmt.Errorf("不支持的监控项: %s", itemKey)
		}
	}

	return nil, fmt.Errorf("所有采集器都未启用或未找到")
}

// stopItemSchedulers 停止所有监控项调度器
func (s *Scheduler) stopItemSchedulers() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, scheduler := range s.itemSchedulers {
		if scheduler.running {
			close(scheduler.stopChan)
			if scheduler.ticker != nil {
				scheduler.ticker.Stop()
			}
		}
	}

	logger.Info("所有监控项调度器已停止")
}

// onConfigUpdate 配置更新回调
func (s *Scheduler) onConfigUpdate(items []services.CollectItem) {
	logger.Info("收到配置更新通知，重新启动监控项调度器", map[string]interface{}{
		"item_count": len(items),
	})

	// 停止现有的监控项调度器
	s.stopItemSchedulers()

	// 清空调度器映射
	s.mu.Lock()
	s.itemSchedulers = make(map[int64]*ItemScheduler)
	s.mu.Unlock()

	// 重新启动监控项调度器
	if err := s.startItemSchedulers(); err != nil {
		logger.Errorf("重新启动监控项调度器失败: %v", err)
	}
}

// initAPIServices 初始化API服务
func (s *Scheduler) initAPIServices() error {
	// 检查是否配置了API服务
	if s.config.DeviceMonitor == nil || !s.config.DeviceMonitor.Enabled {
		logger.Info("设备监控API服务未启用，跳过初始化")
		return nil
	}

	// 创建API客户端
	clientConfig := &client.Config{
		BaseURL: s.config.DeviceMonitor.BaseURL,
		Timeout: s.config.DeviceMonitor.Timeout,
		AgentID: s.config.DeviceMonitor.AgentID,
	}
	s.apiClient = client.NewDeviceMonitorClient(clientConfig)

	// 创建注册服务
	registerService, err := services.NewRegisterService(s.apiClient, logger.GetLogger())
	if err != nil {
		return fmt.Errorf("创建注册服务失败: %v", err)
	}
	s.registerService = registerService

	// 创建心跳服务
	heartbeatConfig := &services.HeartbeatConfig{
		Interval: s.config.DeviceMonitor.HeartbeatInterval,
		Enabled:  s.config.DeviceMonitor.Enabled,
	}
	s.heartbeatService = services.NewHeartbeatService(s.apiClient, logger.GetLogger(), heartbeatConfig)

	// 创建配置管理器
	configManagerConfig := &services.ConfigManagerConfig{
		RefreshInterval: s.config.DeviceMonitor.ConfigRefreshInterval,
		Enabled:         s.config.DeviceMonitor.Enabled,
	}
	s.configManager = services.NewConfigManager(s.apiClient, logger.GetLogger(), configManagerConfig)

	// 设置配置更新回调
	s.configManager.SetConfigUpdateCallback(s.onConfigUpdate)
	
	// 设置心跳服务的引用
	if s.heartbeatService != nil {
		s.heartbeatService.SetRegisterService(s.registerService)
		s.heartbeatService.SetConfigManager(s.configManager)
	}

	// 创建指标发送器
	metricsSenderConfig := &services.MetricsSenderConfig{
		BufferSize:    s.config.DeviceMonitor.MetricsBufferSize,
		FlushInterval: s.config.DeviceMonitor.MetricsFlushInterval,
		Enabled:       s.config.DeviceMonitor.Enabled,
	}
	s.metricsSender = services.NewMetricsSender(s.apiClient, logger.GetLogger(), metricsSenderConfig)

	// 初始化命令执行采集器
	commandConfigPath := "configs/command_mapping.yaml"
	commandCollector, err := collector.NewCommandCollector(commandConfigPath, logger.GetLogger(), s.apiClient, s.metricsSender)
	if err != nil {
		logger.Warnf("初始化命令执行采集器失败: %v，将跳过命令执行功能", err)
		s.commandCollector = nil
	} else {
		s.commandCollector = commandCollector
		logger.Info("命令执行采集器初始化完成")
	}

	logger.Info("API服务初始化完成")
	return nil
}

// startAPIServices 启动API服务
func (s *Scheduler) startAPIServices() error {
	if s.config.DeviceMonitor == nil || !s.config.DeviceMonitor.Enabled {
		return nil
	}

	// 注册agent
	if s.registerService != nil {
		if err := s.registerService.RegisterWithRetry(s.ctx, 3, 5*time.Second); err != nil {
			logger.Errorf("Agent注册失败: %v", err)
			return err
		}
		logger.Info("Agent注册成功")
	}

	// 启动心跳服务
	if s.heartbeatService != nil {
		if err := s.heartbeatService.Start(s.ctx); err != nil {
			logger.Errorf("启动心跳服务失败: %v", err)
			return err
		}
	}

	// 启动配置管理器
	if s.configManager != nil {
		if err := s.configManager.Start(s.ctx); err != nil {
			logger.Errorf("启动配置管理器失败: %v", err)
			return err
		}
	}

	// 启动指标发送器
	if s.metricsSender != nil {
		if err := s.metricsSender.Start(s.ctx); err != nil {
			logger.Errorf("启动指标发送器失败: %v", err)
			return err
		}
	}

	logger.Info("API服务启动完成")
	return nil
}

// stopAPIServices 停止API服务
func (s *Scheduler) stopAPIServices() {
	if s.config.DeviceMonitor == nil || !s.config.DeviceMonitor.Enabled {
		return
	}

	// 停止指标发送器
	if s.metricsSender != nil {
		if err := s.metricsSender.Stop(); err != nil {
			logger.Errorf("停止指标发送器失败: %v", err)
		}
	}

	// 停止配置管理器
	if s.configManager != nil {
		if err := s.configManager.Stop(); err != nil {
			logger.Errorf("停止配置管理器失败: %v", err)
		}
	}

	// 停止心跳服务
	if s.heartbeatService != nil {
		if err := s.heartbeatService.Stop(); err != nil {
			logger.Errorf("停止心跳服务失败: %v", err)
		}
	}

	logger.Info("API服务已停止")
}
