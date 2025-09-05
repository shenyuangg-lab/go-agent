package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/host"
)

// DeviceMonitorClient 设备监控API客户端
type DeviceMonitorClient struct {
	baseURL    string
	httpClient *http.Client
	agentID    string
	token      string // JWT认证token
}

// Config 客户端配置
type Config struct {
	BaseURL string        `mapstructure:"base_url"`
	Timeout time.Duration `mapstructure:"timeout"`
	AgentID string        `mapstructure:"agent_id"`
}

// RegisterRequest agent注册请求
type RegisterRequest struct {
	Hostname     string `json:"hostname"`
	IPAddress    string `json:"ipAddress"`
	OSType       string `json:"osType"`
	OSVersion    string `json:"osVersion"`
	AgentVersion string `json:"agentVersion"`
}

// RegisterResponse agent注册响应
type RegisterResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AgentID string `json:"agentId"`
		Token   string `json:"token"`
	} `json:"data"`
}

// HeartbeatRequest 心跳请求
type HeartbeatRequest struct {
	AgentID string `json:"agentId"`
	Status  string `json:"status"` // ONLINE, OFFLINE, WARNING
}

// HeartbeatResponse 心跳响应
type HeartbeatResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// MetricsRequest 指标数据请求
type MetricsRequest struct {
	ItemID    int64       `json:"itemId"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
}

// MetricsResponse 指标数据响应
type MetricsResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// ConfigResponse 配置获取响应
type ConfigResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data []struct {
		ItemID                int64   `json:"itemId"`
		ItemName              string  `json:"itemName"`
		ItemKey               string  `json:"itemkey"`
		InfoType              int     `json:"infoType"`
		UpdateIntervalSeconds int     `json:"updateIntervalseconds"`
		Timeout               int     `json:"timeout"`
		Description           *string `json:"description"`
		Intervals             *string `json:"intervals"`
	} `json:"data"`
}

// NewDeviceMonitorClient 创建设备监控客户端
func NewDeviceMonitorClient(config *Config) *DeviceMonitorClient {
	client := &http.Client{
		Timeout: config.Timeout,
	}

	return &DeviceMonitorClient{
		baseURL:    config.BaseURL,
		httpClient: client,
		agentID:    config.AgentID,
	}
}

// Register agent注册 - 自动获取主机信息
func (c *DeviceMonitorClient) Register(ctx context.Context) (*RegisterResponse, error) {
	// 自动获取主机名
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("获取主机名失败: %v", err)
	}

	// 自动获取IP地址
	ipAddress, err := getLocalIP()
	if err != nil {
		return nil, fmt.Errorf("获取IP地址失败: %v", err)
	}

	// 获取操作系统版本
	osVersion, err := getOSVersion()
	if err != nil {
		osVersion = runtime.GOOS // 如果获取失败，使用GOOS作为后备
	}

	req := &RegisterRequest{
		Hostname:     hostname,
		IPAddress:    ipAddress,
		OSType:       runtime.GOOS,
		OSVersion:    osVersion,
		AgentVersion: "1.0.0",
	}

	var resp RegisterResponse
	err = c.doRequest(ctx, "POST", "/deviceMonitor/agent/register", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("注册失败: %v", err)
	}

	// 保存返回的agentID和token
	if resp.Code == 200 && resp.Data.AgentID != "" {
		c.agentID = resp.Data.AgentID
		c.token = resp.Data.Token
	}

	return &resp, nil
}

// Heartbeat 发送心跳
func (c *DeviceMonitorClient) Heartbeat(ctx context.Context, status string) (*HeartbeatResponse, error) {
	if c.agentID == "" {
		return nil, fmt.Errorf("agentID为空，请先注册")
	}

	req := &HeartbeatRequest{
		AgentID: c.agentID,
		Status:  status,
	}

	var resp HeartbeatResponse
	err := c.doRequest(ctx, "POST", "/deviceMonitor/agent/heartbeat", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("心跳失败: %v", err)
	}

	return &resp, nil
}

// SendMetrics 发送指标数据
func (c *DeviceMonitorClient) SendMetrics(ctx context.Context, itemID int64, value interface{}) (*MetricsResponse, error) {
	req := &MetricsRequest{
		ItemID:    itemID,
		Timestamp: time.Now().Unix(),
		Value:     value,
	}

	var resp MetricsResponse
	err := c.doRequest(ctx, "POST", "/deviceMonitor/agent/metrics", req, &resp)
	if err != nil {
		return nil, fmt.Errorf("发送指标失败: %v", err)
	}

	return &resp, nil
}

// GetConfig 获取采集配置
func (c *DeviceMonitorClient) GetConfig(ctx context.Context) (*ConfigResponse, error) {
	if c.agentID == "" {
		return nil, fmt.Errorf("agentID为空，请先注册")
	}

	var resp ConfigResponse
	url := fmt.Sprintf("/deviceMonitor/agent/config/%s", c.agentID)
	err := c.doRequest(ctx, "GET", url, nil, &resp)
	if err != nil {
		return nil, fmt.Errorf("获取配置失败: %v", err)
	}

	return &resp, nil
}

// SetAgentID 设置agentID
func (c *DeviceMonitorClient) SetAgentID(agentID string) {
	c.agentID = agentID
}

// GetAgentID 获取agentID
func (c *DeviceMonitorClient) GetAgentID() string {
	return c.agentID
}

// GetToken 获取token
func (c *DeviceMonitorClient) GetToken() string {
	return c.token
}

// SetToken 设置token
func (c *DeviceMonitorClient) SetToken(token string) {
	c.token = token
}

// IsAuthenticated 检查是否已认证（有token和agentID）
func (c *DeviceMonitorClient) IsAuthenticated() bool {
	return c.agentID != "" && c.token != ""
}

// doRequest 执行HTTP请求
func (c *DeviceMonitorClient) doRequest(ctx context.Context, method, path string, reqBody, respBody interface{}) error {
	url := c.baseURL + path

	var body *bytes.Buffer
	if reqBody != nil {
		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("序列化请求数据失败: %v", err)
		}
		body = bytes.NewBuffer(jsonData)
	} else {
		body = bytes.NewBuffer(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("创建请求失败: %v", err)
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("User-Agent", "go-agent/1.0")

	// 如果有token，自动添加Authorization头
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}

	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("解析响应失败: %v", err)
		}
	}

	return nil
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

// getOSVersion 获取操作系统版本
func getOSVersion() (string, error) {
	// 使用gopsutil获取详细的操作系统版本信息
	info, err := host.Info()
	if err != nil {
		return "", fmt.Errorf("获取系统信息失败: %v", err)
	}

	// 返回平台版本信息，格式如: "windows-10.0.19041"
	return fmt.Sprintf("%s-%s", info.Platform, info.PlatformVersion), nil
}
