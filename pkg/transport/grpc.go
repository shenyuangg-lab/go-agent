package transport

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GRPCTransport gRPC传输器
type GRPCTransport struct {
	enabled bool
	server  string
	port    int
	conn    *grpc.ClientConn
	timeout time.Duration
}

// NewGRPCTransport 创建gRPC传输器
func NewGRPCTransport(enabled bool, server string, port int, timeout time.Duration) *GRPCTransport {
	return &GRPCTransport{
		enabled: enabled,
		server:  server,
		port:    port,
		timeout: timeout,
	}
}

// Connect 连接到gRPC服务器
func (t *GRPCTransport) Connect(ctx context.Context) error {
	if !t.enabled {
		return fmt.Errorf("gRPC传输器未启用")
	}

	if t.server == "" {
		return fmt.Errorf("未配置gRPC服务器地址")
	}

	// 构建服务器地址
	serverAddr := fmt.Sprintf("%s:%d", t.server, t.port)

	// 创建连接
	conn, err := grpc.DialContext(ctx, serverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(t.timeout),
	)
	if err != nil {
		return fmt.Errorf("连接gRPC服务器失败: %v", err)
	}

	t.conn = conn
	return nil
}

// Disconnect 断开gRPC连接
func (t *GRPCTransport) Disconnect() error {
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

// IsConnected 检查是否已连接
func (t *GRPCTransport) IsConnected() bool {
	return t.conn != nil && t.conn.GetState().String() == "READY"
}

// Send 发送数据（需要实现具体的gRPC服务接口）
func (t *GRPCTransport) Send(ctx context.Context, data interface{}, dataType string, metadata map[string]interface{}) error {
	if !t.enabled {
		return fmt.Errorf("gRPC传输器未启用")
	}

	if !t.IsConnected() {
		return fmt.Errorf("gRPC连接未建立")
	}

	// 这里需要根据具体的gRPC服务定义来实现
	// 由于没有具体的proto文件，这里只是框架代码
	return fmt.Errorf("gRPC发送功能需要根据具体的服务定义来实现")
}

// SendBatch 批量发送数据
func (t *GRPCTransport) SendBatch(ctx context.Context, dataList []interface{}, dataType string, metadata map[string]interface{}) error {
	if !t.enabled {
		return fmt.Errorf("gRPC传输器未启用")
	}

	if !t.IsConnected() {
		return fmt.Errorf("gRPC连接未建立")
	}

	// 这里需要根据具体的gRPC服务定义来实现
	// 由于没有具体的proto文件，这里只是框架代码
	return fmt.Errorf("gRPC批量发送功能需要根据具体的服务定义来实现")
}

// GetServer 获取配置的服务器地址
func (t *GRPCTransport) GetServer() string {
	return t.server
}

// SetServer 设置服务器地址
func (t *GRPCTransport) SetServer(server string) {
	t.server = server
}

// GetPort 获取配置的端口
func (t *GRPCTransport) GetPort() int {
	return t.port
}

// SetPort 设置端口
func (t *GRPCTransport) SetPort(port int) {
	t.port = port
}

// IsEnabled 检查是否启用
func (t *GRPCTransport) IsEnabled() bool {
	return t.enabled
}

// SetEnabled 设置启用状态
func (t *GRPCTransport) SetEnabled(enabled bool) {
	t.enabled = enabled
}

// GetConnection 获取gRPC连接
func (t *GRPCTransport) GetConnection() *grpc.ClientConn {
	return t.conn
}

// SetTimeout 设置超时时间
func (t *GRPCTransport) SetTimeout(timeout time.Duration) {
	t.timeout = timeout
}

// GetTimeout 获取超时时间
func (t *GRPCTransport) GetTimeout() time.Duration {
	return t.timeout
}

// HealthCheck 健康检查
func (t *GRPCTransport) HealthCheck(ctx context.Context) error {
	if !t.enabled {
		return fmt.Errorf("gRPC传输器未启用")
	}

	if !t.IsConnected() {
		return fmt.Errorf("gRPC连接未建立")
	}

	// 这里可以实现具体的健康检查逻辑
	// 例如调用gRPC服务的健康检查方法
	return nil
}
