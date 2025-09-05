package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go-agent/pkg/client"

	"github.com/sirupsen/logrus"
)

// AgentStatus agent状态枚举
type AgentStatus string

const (
	StatusOnline  AgentStatus = "ONLINE"
	StatusOffline AgentStatus = "OFFLINE"
	StatusWarning AgentStatus = "WARNING"
)

// HeartbeatService 心跳服务
type HeartbeatService struct {
	client   *client.DeviceMonitorClient
	logger   *logrus.Logger
	status   AgentStatus
	interval time.Duration
	stopChan chan struct{}
	wg       sync.WaitGroup
	mutex    sync.RWMutex
	running  bool
}

// HeartbeatConfig 心跳配置
type HeartbeatConfig struct {
	Interval time.Duration `mapstructure:"interval"`
	Enabled  bool          `mapstructure:"enabled"`
}

// NewHeartbeatService 创建心跳服务
func NewHeartbeatService(client *client.DeviceMonitorClient, logger *logrus.Logger, config *HeartbeatConfig) *HeartbeatService {
	interval := config.Interval
	if interval <= 0 {
		interval = 30 * time.Second // 默认30秒
	}

	return &HeartbeatService{
		client:   client,
		logger:   logger,
		status:   StatusOnline,
		interval: interval,
		stopChan: make(chan struct{}),
		running:  false,
	}
}

// Start 启动心跳服务
func (s *HeartbeatService) Start(ctx context.Context) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.running {
		return fmt.Errorf("心跳服务已在运行")
	}

	s.running = true
	s.wg.Add(1)

	go s.heartbeatLoop(ctx)

	s.logger.Info("心跳服务已启动", map[string]interface{}{
		"interval": s.interval.String(),
	})

	return nil
}

// Stop 停止心跳服务
func (s *HeartbeatService) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.running {
		return nil
	}

	close(s.stopChan)
	s.running = false

	// 等待goroutine结束
	s.wg.Wait()

	s.logger.Info("心跳服务已停止")
	return nil
}

// SetStatus 设置agent状态
func (s *HeartbeatService) SetStatus(status AgentStatus) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.status != status {
		s.logger.Info("Agent状态变更", map[string]interface{}{
			"old_status": s.status,
			"new_status": status,
		})
		s.status = status
	}
}

// GetStatus 获取当前状态
func (s *HeartbeatService) GetStatus() AgentStatus {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.status
}

// SendHeartbeat 发送一次心跳
func (s *HeartbeatService) SendHeartbeat(ctx context.Context) error {
	status := s.GetStatus()

	resp, err := s.client.Heartbeat(ctx, string(status))
	if err != nil {
		s.logger.Error("发送心跳失败", map[string]interface{}{
			"error":  err.Error(),
			"status": status,
		})
		return err
	}

	if resp.Code != 200 {
		err := fmt.Errorf("心跳响应异常: %s", resp.Msg)
		s.logger.Error("心跳响应异常", map[string]interface{}{
			"code": resp.Code,
			"msg":  resp.Msg,
		})
		return err
	}

	s.logger.Debug("心跳发送成功", map[string]interface{}{
		"status": status,
	})

	return nil
}

// heartbeatLoop 心跳循环
func (s *HeartbeatService) heartbeatLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 立即发送一次心跳
	if err := s.SendHeartbeat(ctx); err != nil {
		s.logger.Error("初始心跳发送失败", map[string]interface{}{
			"error": err.Error(),
		})
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("心跳服务因上下文取消而停止")
			return
		case <-s.stopChan:
			s.logger.Info("心跳服务收到停止信号")
			return
		case <-ticker.C:
			if err := s.SendHeartbeat(ctx); err != nil {
				// 连续失败时可以考虑设置为WARNING状态
				s.handleHeartbeatError(err)
			}
		}
	}
}

// handleHeartbeatError 处理心跳错误
func (s *HeartbeatService) handleHeartbeatError(err error) {
	s.logger.Error("心跳失败", map[string]interface{}{
		"error": err.Error(),
	})

	// 可以在这里实现失败策略，例如:
	// 1. 连续失败几次后设置为WARNING状态
	// 2. 实现退避重试
	// 3. 触发告警等
}

// IsRunning 检查服务是否在运行
func (s *HeartbeatService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}
