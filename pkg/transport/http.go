package transport

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// HTTPTransport HTTP传输器
type HTTPTransport struct {
	enabled bool
	url     string
	method  string
	headers map[string]string
	client  *http.Client
}

// TransportData 传输数据结构
type TransportData struct {
	Timestamp time.Time              `json:"timestamp"`
	Agent     string                 `json:"agent"`
	Type      string                 `json:"type"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// NewHTTPTransport 创建HTTP传输器
func NewHTTPTransport(enabled bool, url, method string, headers map[string]string, timeout time.Duration) *HTTPTransport {
	client := &http.Client{
		Timeout: timeout,
	}

	return &HTTPTransport{
		enabled: enabled,
		url:     url,
		method:  method,
		headers: headers,
		client:  client,
	}
}

// Send 发送数据
func (t *HTTPTransport) Send(ctx context.Context, data interface{}, dataType string, metadata map[string]interface{}) error {
	if !t.enabled {
		return fmt.Errorf("HTTP传输器未启用")
	}

	if t.url == "" {
		return fmt.Errorf("未配置HTTP URL")
	}

	// 构建传输数据
	transportData := TransportData{
		Timestamp: time.Now(),
		Agent:     "go-agent",
		Type:      dataType,
		Data:      data,
		Metadata:  metadata,
	}

	// 序列化数据
	jsonData, err := json.Marshal(transportData)
	if err != nil {
		return fmt.Errorf("序列化数据失败: %v", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, t.method, t.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置默认头部
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-agent/1.0")

	// 设置自定义头部
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// SendBatch 批量发送数据
func (t *HTTPTransport) SendBatch(ctx context.Context, dataList []interface{}, dataType string, metadata map[string]interface{}) error {
	if !t.enabled {
		return fmt.Errorf("HTTP传输器未启用")
	}

	if len(dataList) == 0 {
		return nil
	}

	// 构建批量传输数据
	batchData := map[string]interface{}{
		"timestamp": time.Now(),
		"agent":     "go-agent",
		"type":      dataType,
		"count":     len(dataList),
		"data":      dataList,
		"metadata":  metadata,
	}

	// 序列化数据
	jsonData, err := json.Marshal(batchData)
	if err != nil {
		return fmt.Errorf("序列化批量数据失败: %v", err)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, t.method, t.url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("创建HTTP请求失败: %v", err)
	}

	// 设置默认头部
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "go-agent/1.0")

	// 设置自定义头部
	for key, value := range t.headers {
		req.Header.Set(key, value)
	}

	// 发送请求
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP请求失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// SendWithRetry 带重试的发送
func (t *HTTPTransport) SendWithRetry(ctx context.Context, data interface{}, dataType string, metadata map[string]interface{}, maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		err := t.Send(ctx, data, dataType, metadata)
		if err == nil {
			return nil
		}

		lastErr = err

		// 如果不是最后一次重试，则等待后重试
		if i < maxRetries {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(retryDelay):
				continue
			}
		}
	}

	return fmt.Errorf("重试%d次后仍然失败，最后错误: %v", maxRetries, lastErr)
}

// SetHeaders 设置HTTP头部
func (t *HTTPTransport) SetHeaders(headers map[string]string) {
	t.headers = headers
}

// AddHeader 添加单个HTTP头部
func (t *HTTPTransport) AddHeader(key, value string) {
	if t.headers == nil {
		t.headers = make(map[string]string)
	}
	t.headers[key] = value
}

// RemoveHeader 移除HTTP头部
func (t *HTTPTransport) RemoveHeader(key string) {
	delete(t.headers, key)
}

// GetURL 获取配置的URL
func (t *HTTPTransport) GetURL() string {
	return t.url
}

// SetURL 设置URL
func (t *HTTPTransport) SetURL(url string) {
	t.url = url
}

// IsEnabled 检查是否启用
func (t *HTTPTransport) IsEnabled() bool {
	return t.enabled
}

// SetEnabled 设置启用状态
func (t *HTTPTransport) SetEnabled(enabled bool) {
	t.enabled = enabled
}
