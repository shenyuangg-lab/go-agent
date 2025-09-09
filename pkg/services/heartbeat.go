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

// AgentStatus agent状态枚举
type AgentStatus string

const (
	StatusOnline  AgentStatus = "ONLINE"
	StatusOffline AgentStatus = "OFFLINE"
	StatusWarning AgentStatus = "WARNING"
)

// HeartbeatService 心跳服务
type HeartbeatService struct {
	client           *client.DeviceMonitorClient
	logger           *logrus.Logger
	status           AgentStatus
	interval         time.Duration
	stopChan         chan struct{}
	wg               sync.WaitGroup
	mutex            sync.RWMutex
	running          bool
	registerService  *RegisterService      // 注册服务引用
	configManager    *ConfigManager       // 配置管理器引用
	failureCount     int                  // 连续失败次数
	lastFailureTime  time.Time           // 最后失败时间
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
		client:       client,
		logger:       logger,
		status:       StatusOnline,
		interval:     interval,
		stopChan:     make(chan struct{}),
		running:      false,
		failureCount: 0,
	}
}

// SetRegisterService 设置注册服务引用
func (s *HeartbeatService) SetRegisterService(registerService *RegisterService) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.registerService = registerService
}

// SetConfigManager 设置配置管理器引用
func (s *HeartbeatService) SetConfigManager(configManager *ConfigManager) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.configManager = configManager
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

	// 心跳成功，重置失败计数
	s.mutex.Lock()
	if s.failureCount > 0 {
		s.logger.Debug("心跳恢复正常，重置失败计数", map[string]interface{}{
			"previous_failure_count": s.failureCount,
		})
		s.failureCount = 0
		// 如果之前是WARNING状态，恢复为ONLINE
		if s.status == StatusWarning {
			s.status = StatusOnline
		}
	}
	s.mutex.Unlock()

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
	s.mutex.Lock()
	s.failureCount++
	s.lastFailureTime = time.Now()
	failureCount := s.failureCount
	s.mutex.Unlock()

	s.logger.Error("心跳失败", map[string]interface{}{
		"error":         err.Error(),
		"failure_count": failureCount,
	})

	// 检查是否是认证相关错误（401、403等）
	errorStr := err.Error()
	if s.isAuthenticationError(errorStr) {
		s.logger.Warn("检测到认证错误，可能需要重新注册")
		s.attemptReregistration()
		return
	}

	// 连续失败处理策略
	if failureCount >= 3 {
		s.logger.Warn("心跳连续失败超过3次，设置状态为WARNING")
		s.SetStatus(StatusWarning)
		
		// 如果失败次数过多，尝试重新注册
		if failureCount >= 10 {
			s.logger.Error("心跳连续失败超过10次，尝试重新注册")
			s.attemptReregistration()
		}
	}
}

// isAuthenticationError 检查是否是认证错误
func (s *HeartbeatService) isAuthenticationError(errorStr string) bool {
	// 检查常见的认证错误关键词
	authErrors := []string{
		"401",
		"403", 
		"unauthorized",
		"forbidden",
		"token",
		"authentication",
		"not registered",
		"未注册",
		"认证失败",
		"令牌",
	}
	
	errorLower := strings.ToLower(errorStr)
	for _, authErr := range authErrors {
		if strings.Contains(errorLower, strings.ToLower(authErr)) {
			return true
		}
	}
	return false
}

// attemptReregistration 尝试重新注册
func (s *HeartbeatService) attemptReregistration() {
	if s.registerService == nil {
		s.logger.Error("注册服务未设置，无法重新注册")
		return
	}

	s.logger.Info("开始重新注册流程")
	
	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 尝试重新注册
	if err := s.registerService.RegisterWithRetry(ctx, 3, 5*time.Second); err != nil {
		s.logger.Error("重新注册失败", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	s.logger.Info("重新注册成功")
	
	// 重置失败计数
	s.mutex.Lock()
	s.failureCount = 0
	s.mutex.Unlock()
	
	// 设置状态回到在线
	s.SetStatus(StatusOnline)

	// 触发配置重新加载
	if s.configManager != nil {
		s.logger.Info("触发配置重新加载")
		s.configManager.RefreshConfig()
	}
}

// IsRunning 检查服务是否在运行
func (s *HeartbeatService) IsRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.running
}
