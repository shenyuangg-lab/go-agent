package services

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go-agent/pkg/client"

	"github.com/sirupsen/logrus"
)

// CollectItem 采集项
type CollectItem struct {
	ItemID                int64                        `json:"itemId"`
	ItemName              string                       `json:"itemName"`
	ItemKey               string                       `json:"itemKey"`
	InfoType              int                          `json:"infoType"`
	UpdateIntervalSeconds int                          `json:"updateIntervalSeconds"` // 推送间隔(秒)
	Timeout               int                          `json:"timeout"`
	Intervals             []*client.ItemCustomInterval `json:"intervals"` // 自定义时间间隔
}

// ConfigManager 配置管理器
type ConfigManager struct {
	client         *client.DeviceMonitorClient
	logger         *logrus.Logger
	items          []CollectItem
	mutex          sync.RWMutex
	lastUpdate     time.Time
	refreshChan    chan struct{}
	stopChan       chan struct{}
	wg             sync.WaitGroup
	running        bool
	onConfigUpdate func([]CollectItem) // 配置更新回调
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

// SetConfigUpdateCallback 设置配置更新回调
func (cm *ConfigManager) SetConfigUpdateCallback(callback func([]CollectItem)) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	cm.onConfigUpdate = callback
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
func (cm *ConfigManager) GetItemByID(itemID int64) (*CollectItem, error) {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	for _, item := range cm.items {
		if item.ItemID == itemID {
			return &item, nil
		}
	}

	return nil, fmt.Errorf("未找到采集项: %d", itemID)
}

// GetItemsByType 根据类型获取采集项
func (cm *ConfigManager) GetItemsByType(itemType string) []CollectItem {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	var items []CollectItem
	for _, item := range cm.items {
		if item.ItemKey == itemType {
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
		cm.logger.Error("获取配置失败", map[string]interface{}{
			"error": err.Error(),
		})
		return fmt.Errorf("获取配置失败: %v", err)
	}

	if resp.Code != 200 {
		cm.logger.Error("获取配置响应异常", map[string]interface{}{
			"code": resp.Code,
			"msg":  resp.Msg,
		})
		
		// 检查是否是认证相关错误
		if cm.isAuthenticationError(resp.Code, resp.Msg) {
			cm.logger.Warn("检测到认证错误，配置获取失败可能需要重新注册")
		}
		
		return fmt.Errorf("获取配置响应异常: %s", resp.Msg)
	}

	// 转换响应数据为内部结构
	items := make([]CollectItem, len(resp.Data))
	for i, item := range resp.Data {
		items[i] = CollectItem{
			ItemID:                item.ItemID,
			ItemName:              item.ItemName,
			ItemKey:               item.ItemKey,
			InfoType:              item.InfoType,
			UpdateIntervalSeconds: item.UpdateIntervalSeconds,
			Timeout:               item.Timeout,
			Intervals:             item.Intervals,
		}
	}

	// 更新配置
	cm.mutex.Lock()
	cm.items = items
	cm.lastUpdate = time.Now()

	// 调用配置更新回调
	if cm.onConfigUpdate != nil {
		go cm.onConfigUpdate(items)
	}

	cm.mutex.Unlock()

	cm.logger.Info("配置加载成功", map[string]interface{}{
		"item_count":  len(items),
		"update_time": cm.lastUpdate,
	})

	// 记录配置详情
	for _, item := range items {
		cm.logger.Debug("加载采集项", map[string]interface{}{
			"item_id":                 item.ItemID,
			"item_name":               item.ItemName,
			"item_key":                item.ItemKey,
			"update_interval_seconds": item.UpdateIntervalSeconds,
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

// isAuthenticationError 检查是否是认证错误
func (cm *ConfigManager) isAuthenticationError(code int, msg string) bool {
	// 检查HTTP状态码
	if code == 401 || code == 403 {
		return true
	}
	
	// 检查错误消息中的关键词
	if msg == "" {
		return false
	}
	
	authErrors := []string{
		"unauthorized",
		"forbidden", 
		"token",
		"authentication",
		"not registered",
		"未注册",
		"认证失败",
		"令牌",
		"无权限",
	}
	
	msgLower := strings.ToLower(msg)
	for _, authErr := range authErrors {
		if strings.Contains(msgLower, strings.ToLower(authErr)) {
			return true
		}
	}
	return false
}
