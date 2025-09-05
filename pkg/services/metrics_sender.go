package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-agent/pkg/client"

	"github.com/sirupsen/logrus"
)

// MetricData 指标数据
type MetricData struct {
	ItemID    int64                  `json:"itemId"`
	Timestamp time.Time              `json:"timestamp"`
	Value     interface{}            `json:"value"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// MetricsSender 指标发送器

type MetricsSender struct {
	client        *client.DeviceMonitorClient
	logger        *logrus.Logger
	buffer        []MetricData
	bufferSize    int
	flushInterval time.Duration
	mutex         sync.RWMutex
	stopChan      chan struct{}
	flushChan     chan struct{}
	wg            sync.WaitGroup
	running       bool
}

// MetricsSenderConfig 指标发送器配置
type MetricsSenderConfig struct {
	BufferSize    int           `mapstructure:"buffer_size"`
	FlushInterval time.Duration `mapstructure:"flush_interval"`
	Enabled       bool          `mapstructure:"enabled"`
}

// NewMetricsSender 创建指标发送器
func NewMetricsSender(client *client.DeviceMonitorClient, logger *logrus.Logger, config *MetricsSenderConfig) *MetricsSender {
	bufferSize := config.BufferSize
	if bufferSize <= 0 {
		bufferSize = 100 // 默认缓冲区大小
	}

	flushInterval := config.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 10 * time.Second // 默认10秒刷新一次
	}

	return &MetricsSender{
		client:        client,
		logger:        logger,
		buffer:        make([]MetricData, 0, bufferSize),
		bufferSize:    bufferSize,
		flushInterval: flushInterval,
		stopChan:      make(chan struct{}),
		flushChan:     make(chan struct{}, 1),
		running:       false,
	}
}

// Start 启动指标发送器
func (ms *MetricsSender) Start(ctx context.Context) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if ms.running {
		return fmt.Errorf("指标发送器已在运行")
	}

	ms.running = true
	ms.wg.Add(1)

	go ms.flushLoop(ctx)

	ms.logger.Info("指标发送器已启动", map[string]interface{}{
		"buffer_size":    ms.bufferSize,
		"flush_interval": ms.flushInterval.String(),
	})

	return nil
}

// Stop 停止指标发送器
func (ms *MetricsSender) Stop() error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if !ms.running {
		return nil
	}

	close(ms.stopChan)
	ms.running = false

	ms.wg.Wait()

	// 停止前刷新剩余数据
	if len(ms.buffer) > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ms.flushBuffer(ctx)
	}

	ms.logger.Info("指标发送器已停止")
	return nil
}

// SendMetric 发送单个指标
func (ms *MetricsSender) SendMetric(itemID int64, value interface{}, metadata map[string]interface{}) error {
	metric := MetricData{
		ItemID:    itemID,
		Timestamp: time.Now(),
		Value:     value,
		Metadata:  metadata,
	}

	return ms.AddMetric(metric)
}

// AddMetric 添加指标到缓冲区
func (ms *MetricsSender) AddMetric(metric MetricData) error {
	ms.mutex.Lock()
	defer ms.mutex.Unlock()

	if !ms.running {
		return fmt.Errorf("指标发送器未运行")
	}

	ms.buffer = append(ms.buffer, metric)

	ms.logger.Debug("添加指标到缓冲区", map[string]interface{}{
		"item_id":      metric.ItemID,
		"value":        metric.Value,
		"buffer_size":  len(ms.buffer),
		"buffer_limit": ms.bufferSize,
	})

	// 检查是否需要刷新
	if len(ms.buffer) >= ms.bufferSize {
		ms.triggerFlush()
	}

	return nil
}

// SendMetricImmediate 立即发送指标（不通过缓冲区）
func (ms *MetricsSender) SendMetricImmediate(ctx context.Context, itemID int64, value interface{}) error {
	// 处理数组类型的值，只取第一个元素
	processedValue := ms.processValue(value)

	resp, err := ms.client.SendMetrics(ctx, itemID, processedValue)
	if err != nil {
		ms.logger.Error("立即发送指标失败", map[string]interface{}{
			"item_id": itemID,
			"value":   processedValue,
			"error":   err.Error(),
		})
		return err
	}

	if resp.Code != 1 {
		err := fmt.Errorf("指标发送响应异常: %s", resp.Msg)
		ms.logger.Error("指标发送响应异常", map[string]interface{}{
			"item_id": itemID,
			"code":    resp.Code,
			"msg":     resp.Msg,
		})
		return err
	}

	ms.logger.Debug("立即发送指标成功", map[string]interface{}{
		"item_id": itemID,
		"value":   processedValue,
	})

	return nil
}

// Flush 手动刷新缓冲区
func (ms *MetricsSender) Flush() {
	ms.triggerFlush()
}

// triggerFlush 触发刷新
func (ms *MetricsSender) triggerFlush() {
	select {
	case ms.flushChan <- struct{}{}:
		ms.logger.Debug("触发指标缓冲区刷新")
	default:
		ms.logger.Debug("指标刷新已在队列中")
	}
}

// flushLoop 刷新循环
func (ms *MetricsSender) flushLoop(ctx context.Context) {
	defer ms.wg.Done()

	ticker := time.NewTicker(ms.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ms.logger.Info("指标发送器因上下文取消而停止")
			return
		case <-ms.stopChan:
			ms.logger.Info("指标发送器收到停止信号")
			return
		case <-ms.flushChan:
			ms.flushBuffer(ctx)
		case <-ticker.C:
			ms.flushBuffer(ctx)
		}
	}
}

// flushBuffer 刷新缓冲区
func (ms *MetricsSender) flushBuffer(ctx context.Context) {
	ms.mutex.Lock()
	if len(ms.buffer) == 0 {
		ms.mutex.Unlock()
		return
	}

	// 复制缓冲区数据并清空
	metrics := make([]MetricData, len(ms.buffer))
	copy(metrics, ms.buffer)
	ms.buffer = ms.buffer[:0] // 清空缓冲区但保留容量
	ms.mutex.Unlock()

	ms.logger.Debug("开始刷新指标缓冲区", map[string]interface{}{
		"metric_count": len(metrics),
	})

	// 批量发送指标
	successCount := 0
	failureCount := 0

	for _, metric := range metrics {
		err := ms.SendMetricImmediate(ctx, metric.ItemID, metric.Value)
		if err != nil {
			failureCount++
			ms.logger.Error("发送指标失败", map[string]interface{}{
				"item_id": metric.ItemID,
				"error":   err.Error(),
			})
		} else {
			successCount++
		}
	}

	ms.logger.Info("指标缓冲区刷新完成", map[string]interface{}{
		"total_count":   len(metrics),
		"success_count": successCount,
		"failure_count": failureCount,
	})
}

// GetBufferSize 获取当前缓冲区大小
func (ms *MetricsSender) GetBufferSize() int {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	return len(ms.buffer)
}

// GetBufferLimit 获取缓冲区限制
func (ms *MetricsSender) GetBufferLimit() int {
	return ms.bufferSize
}

// IsRunning 检查是否在运行
func (ms *MetricsSender) IsRunning() bool {
	ms.mutex.RLock()
	defer ms.mutex.RUnlock()
	return ms.running
}

// processValue 处理指标值，根据接口要求处理数组类型
func (ms *MetricsSender) processValue(value interface{}) interface{} {
	if value == nil {
		return value
	}

	// 检查是否为数组类型
	switch v := value.(type) {
	case []interface{}:
		// 如果是数组，只取第一个元素
		if len(v) > 0 {
			ms.logger.Debug("处理数组类型值，只取第一个元素", map[string]interface{}{
				"original_value":  v,
				"processed_value": v[0],
			})
			return v[0]
		}
		return nil
	case []string:
		if len(v) > 0 {
			ms.logger.Debug("处理字符串数组类型值，只取第一个元素", map[string]interface{}{
				"original_value":  v,
				"processed_value": v[0],
			})
			return v[0]
		}
		return nil
	case []int:
		if len(v) > 0 {
			ms.logger.Debug("处理整数数组类型值，只取第一个元素", map[string]interface{}{
				"original_value":  v,
				"processed_value": v[0],
			})
			return v[0]
		}
		return nil
	case []float64:
		if len(v) > 0 {
			ms.logger.Debug("处理浮点数组类型值，只取第一个元素", map[string]interface{}{
				"original_value":  v,
				"processed_value": v[0],
			})
			return v[0]
		}
		return nil
	default:
		return value
	}
}
