package services

import (
	"context"
	"fmt"
	"time"

	"go-agent/pkg/client"

	"github.com/sirupsen/logrus"
)

// RegisterService agent注册服务
type RegisterService struct {
	client *client.DeviceMonitorClient
	logger *logrus.Logger
}

// NewRegisterService 创建注册服务
func NewRegisterService(client *client.DeviceMonitorClient, logger *logrus.Logger) (*RegisterService, error) {
	return &RegisterService{
		client: client,
		logger: logger,
	}, nil
}

// Register 执行注册
func (s *RegisterService) Register(ctx context.Context) error {
	s.logger.Info("开始注册agent")

	resp, err := s.client.Register(ctx)
	if err != nil {
		s.logger.Error("注册失败", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	if resp.Code != 200 {
		err := fmt.Errorf("注册失败: %s", resp.Msg)
		s.logger.Error("注册失败", map[string]interface{}{
			"code": resp.Code,
			"msg":  resp.Msg,
		})
		return err
	}

	s.logger.Info("注册成功", map[string]interface{}{
		"agentId": resp.Data.AgentID,
	})

	return nil
}

// RegisterWithRetry 带重试的注册
func (s *RegisterService) RegisterWithRetry(ctx context.Context, maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		err := s.Register(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		if i < maxRetries {
			s.logger.Warn("注册失败，准备重试", map[string]interface{}{
				"error":       err.Error(),
				"retry_count": i + 1,
				"max_retries": maxRetries,
			})

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				continue
			}
		}
	}

	return fmt.Errorf("重试%d次后注册仍然失败，最后错误: %v", maxRetries, lastErr)
}

// GetAgentID 获取注册后的agentID
func (s *RegisterService) GetAgentID() string {
	return s.client.GetAgentID()
}
