package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-agent/pkg/client"

	"github.com/sirupsen/logrus"
)

// CollectItem 采集项
type CollectItem struct {
	ItemID   string `json:"itemId"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Interval int    `json:"interval"` // 采集间隔(秒)
}

// ConfigManager 配置管理器
type ConfigManager struct {
	client      *client.DeviceMonitorClient
	logger      *logrus.Logger
	items       []CollectItem
	mutex       sync.RWMutex
	lastUpdate  time.Time
	refreshChan chan struct{}
	stopChan    chan struct{}
	wg          sync.WaitGroup
	running     bool
}

// ConfigManagerConfig 配置管理器配置
type ConfigManagerConfig struct {
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	Enabled         bool          `mapstructure:"enabled"`
}

// NewConfigManager 创建配置管理器
func NewConfigManager(client *client.DeviceMonitorClient, logger *logrus.Logger, config *ConfigManagerConfig) *ConfigManager {
	refreshInterval := config.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = 5 * time.Minute // 默认5分钟刷新一次
	}

	return &ConfigManager{
		client:      client,
		logger:      logger,
		items:       make([]CollectItem, 0),
		refreshChan: make(chan struct{}, 1),
		stopChan:    make(chan struct{}),
		running:     false,
	}
}

// Start 启动配置管理器
func (cm *ConfigManager) Start(ctx context.Context) error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if cm.running {
		return fmt.Errorf("配置管理器已在运行")
	}

	// 初始加载配置
	if err := cm.loadConfig(ctx); err != nil {
		cm.logger.Error("初始加载配置失败", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	cm.running = true
	cm.wg.Add(1)

	go cm.refreshLoop(ctx)

	cm.logger.Info("配置管理器已启动")
	return nil
}

// Stop 停止配置管理器
func (cm *ConfigManager) Stop() error {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()

	if !cm.running {
		return nil
	}

	close(cm.stopChan)
	cm.running = false

	cm.wg.Wait()

	cm.logger.Info("配置管理器已停止")
	return nil
}

// GetItems 获取采集项列表
func (cm *ConfigManager) GetItems() []CollectItem {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	// 返回副本，避免外部修改
	items := make([]CollectItem, len(cm.items))
	copy(items, cm.items)
	return items
}

// GetItemByID 根据ID获取采集项
func (cm *ConfigManager) GetItemByID(itemID string) (*CollectItem, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, item := range cm.items {
		if item.ItemID == itemID {
			return &item, nil
		}
	}

	return nil, fmt.Errorf("未找到采集项: %s", itemID)
}

// GetItemsByType 根据类型获取采集项
func (cm *ConfigManager) GetItemsByType(itemType string) []CollectItem {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var items []CollectItem
	for _, item := range cm.items {
		if item.Type == itemType {
			items = append(items, item)
		}
	}

	return items
}

// RefreshConfig 手动刷新配置
func (cm *ConfigManager) RefreshConfig() {
	select {
	case cm.refreshChan <- struct{}{}:
		cm.logger.Info("触发配置刷新")
	default:
		cm.logger.Debug("配置刷新已在队列中")
	}
}

// GetLastUpdateTime 获取最后更新时间
func (cm *ConfigManager) GetLastUpdateTime() time.Time {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.lastUpdate
}

// loadConfig 加载配置
func (cm *ConfigManager) loadConfig(ctx context.Context) error {
	cm.logger.Debug("开始加载采集配置")

	resp, err := cm.client.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("获取配置失败: %v", err)
	}

	if resp.Code != 1 {
		return fmt.Errorf("获取配置响应异常: %s", resp.Msg)
	}

	// 转换响应数据为内部结构
	items := make([]CollectItem, len(resp.Data))
	for i, item := range resp.Data {
		items[i] = CollectItem{
			ItemID:   item.ItemID,
			Name:     item.Name,
			Type:     item.Type,
			Interval: item.Interval,
		}
	}

	// 更新配置
	cm.mutex.Lock()
	cm.items = items
	cm.lastUpdate = time.Now()
	cm.mutex.Unlock()

	cm.logger.Info("配置加载成功", map[string]interface{}{
		"item_count":  len(items),
		"update_time": cm.lastUpdate,
	})

	// 记录配置详情
	for _, item := range items {
		cm.logger.Debug("加载采集项", map[string]interface{}{
			"item_id":  item.ItemID,
			"name":     item.Name,
			"type":     item.Type,
			"interval": item.Interval,
		})
	}

	return nil
}

// refreshLoop 配置刷新循环
func (cm *ConfigManager) refreshLoop(ctx context.Context) {
	defer cm.wg.Done()

	ticker := time.NewTicker(5 * time.Minute) // 每5分钟自动刷新
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			cm.logger.Info("配置管理器因上下文取消而停止")
			return
		case <-cm.stopChan:
			cm.logger.Info("配置管理器收到停止信号")
			return
		case <-cm.refreshChan:
			if err := cm.loadConfig(ctx); err != nil {
				cm.logger.Error("手动刷新配置失败", map[string]interface{}{
					"error": err.Error(),
				})
			}
		case <-ticker.C:
			if err := cm.loadConfig(ctx); err != nil {
				cm.logger.Error("定时刷新配置失败", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// IsRunning 检查是否在运行
func (cm *ConfigManager) IsRunning() bool {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.running
}

// GetItemCount 获取采集项数量
func (cm *ConfigManager) GetItemCount() int {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return len(cm.items)
}
