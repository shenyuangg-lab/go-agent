package services

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"go-agent/pkg/client"

	"github.com/sirupsen/logrus"
)

// RegisterService agent注册服务
type RegisterService struct {
	client   *client.DeviceMonitorClient
	hostname string
	ipAddr   string
	logger   *logrus.Logger
}

// NewRegisterService 创建注册服务
func NewRegisterService(client *client.DeviceMonitorClient, logger *logrus.Logger) (*RegisterService, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("获取主机名失败: %v", err)
	}

	ipAddr, err := getLocalIP()
	if err != nil {
		return nil, fmt.Errorf("获取本机IP失败: %v", err)
	}

	return &RegisterService{
		client:   client,
		hostname: hostname,
		ipAddr:   ipAddr,
		logger:   logger,
	}, nil
}

// Register 执行注册
func (s *RegisterService) Register(ctx context.Context) error {
	s.logger.Info("开始注册agent", map[string]interface{}{
		"hostname": s.hostname,
		"ip":       s.ipAddr,
	})

	resp, err := s.client.Register(ctx, s.hostname, s.ipAddr)
	if err != nil {
		s.logger.Error("注册失败", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}

	if resp.Code != 1 {
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

// getLocalIP 获取本机IP地址
func getLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		// 如果无法连接外网，尝试获取本地网络接口IP
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return "", err
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String(), nil
				}
			}
		}
		return "", fmt.Errorf("未找到有效的IP地址")
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
