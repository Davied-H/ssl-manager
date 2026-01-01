package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"text/template"
	"time"

	"ssl-manager/internal/config"
)

// EventType 事件类型
type EventType string

const (
	EventCertExpiring   EventType = "cert_expiring"   // 证书即将过期
	EventCertRenewed    EventType = "cert_renewed"    // 证书申请/续期成功
	EventCertFailed     EventType = "cert_failed"     // 证书申请失败
	EventDNSValidationTimeout EventType = "dns_timeout" // DNS 验证超时
)

// EventData 事件数据
type EventData struct {
	Event     string                 `json:"event"`      // 事件类型
	Domain    string                 `json:"domain"`     // 域名
	Timestamp string                 `json:"timestamp"`  // 时间戳
	Message   string                 `json:"message"`    // 消息
	Data      map[string]interface{} `json:"data,omitempty"` // 额外数据
}

// WebhookNotifier Webhook 通知器
type WebhookNotifier struct {
	config *config.WebhookConfig
	client *http.Client
}

// NewWebhookNotifier 创建 Webhook 通知器
func NewWebhookNotifier(cfg *config.WebhookConfig) *WebhookNotifier {
	if cfg == nil || !cfg.Enabled {
		return nil
	}

	timeout := 30 * time.Second
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &WebhookNotifier{
		config: cfg,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// ShouldNotify 检查是否应该发送该事件的通知
func (w *WebhookNotifier) ShouldNotify(eventType EventType) bool {
	if w == nil || w.config == nil || !w.config.Enabled {
		return false
	}

	// 如果没有配置事件列表，则发送所有事件
	if len(w.config.Events) == 0 {
		return true
	}

	eventStr := string(eventType)
	for _, e := range w.config.Events {
		if e == eventStr {
			return true
		}
	}

	return false
}

// Notify 发送通知
func (w *WebhookNotifier) Notify(ctx context.Context, eventType EventType, domain, message string, data map[string]interface{}) error {
	if w == nil || w.config == nil || !w.config.Enabled {
		return nil
	}

	if !w.ShouldNotify(eventType) {
		return nil
	}

	eventData := EventData{
		Event:     string(eventType),
		Domain:    domain,
		Timestamp: time.Now().Format(time.RFC3339),
		Message:   message,
		Data:      data,
	}

	var body []byte
	var err error

	// 如果配置了自定义模板，使用模板生成请求体
	if w.config.BodyTemplate != "" {
		body, err = w.renderTemplate(w.config.BodyTemplate, eventData)
		if err != nil {
			log.Printf("渲染 Webhook 请求体模板失败: %v", err)
			// 模板渲染失败，使用默认 JSON 格式
			body, err = json.Marshal(eventData)
			if err != nil {
				return fmt.Errorf("序列化事件数据失败: %w", err)
			}
		}
	} else {
		// 使用默认 JSON 格式
		body, err = json.Marshal(eventData)
		if err != nil {
			return fmt.Errorf("序列化事件数据失败: %w", err)
		}
	}

	// 重试机制
	retries := w.config.Retries
	if retries <= 0 {
		retries = 3
	}

	var lastErr error
	for i := 0; i < retries; i++ {
		if i > 0 {
			// 指数退避：1s, 2s, 4s
			backoff := time.Duration(1<<uint(i-1)) * time.Second
			log.Printf("Webhook 通知失败，%v 后重试 (第 %d/%d 次)...", backoff, i+1, retries)
			time.Sleep(backoff)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", w.config.URL, bytes.NewBuffer(body))
		if err != nil {
			lastErr = fmt.Errorf("创建请求失败: %w", err)
			continue
		}

		// 设置请求头
		req.Header.Set("Content-Type", "application/json")
		for key, value := range w.config.Headers {
			req.Header.Set(key, value)
		}

		resp, err := w.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("发送请求失败: %w", err)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("Webhook 通知发送成功: %s (事件: %s, 域名: %s)", w.config.URL, eventType, domain)
			return nil
		}

		lastErr = fmt.Errorf("Webhook 返回错误状态码: %d", resp.StatusCode)
	}

	log.Printf("Webhook 通知发送失败 (已重试 %d 次): %v", retries, lastErr)
	return lastErr
}

// renderTemplate 渲染模板
func (w *WebhookNotifier) renderTemplate(tmplStr string, data EventData) ([]byte, error) {
	// 将 EventData 转换为模板可用的格式
	tmplData := map[string]interface{}{
		"Event":     data.Event,
		"Domain":    data.Domain,
		"Timestamp": data.Timestamp,
		"Message":   data.Message,
		"Data":      data.Data,
	}

	// 定义自定义函数
	funcMap := template.FuncMap{
		"toJson": func(v interface{}) string {
			b, err := json.Marshal(v)
			if err != nil {
				return "null"
			}
			return string(b)
		},
	}

	// 解析模板
	tmpl, err := template.New("webhook").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("解析模板失败: %w", err)
	}

	// 渲染模板
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return nil, fmt.Errorf("渲染模板失败: %w", err)
	}

	return buf.Bytes(), nil
}

// NotifyCertExpiring 通知证书即将过期
func (w *WebhookNotifier) NotifyCertExpiring(ctx context.Context, domain string, daysRemaining int) error {
	message := fmt.Sprintf("证书即将过期: %s (剩余 %d 天)", domain, daysRemaining)
	data := map[string]interface{}{
		"days_remaining": daysRemaining,
	}
	return w.Notify(ctx, EventCertExpiring, domain, message, data)
}

// NotifyCertRenewed 通知证书申请/续期成功
func (w *WebhookNotifier) NotifyCertRenewed(ctx context.Context, domain string, certID string) error {
	message := fmt.Sprintf("证书申请/续期成功: %s", domain)
	data := map[string]interface{}{
		"cert_id": certID,
	}
	return w.Notify(ctx, EventCertRenewed, domain, message, data)
}

// NotifyCertFailed 通知证书申请失败
func (w *WebhookNotifier) NotifyCertFailed(ctx context.Context, domain string, reason string) error {
	message := fmt.Sprintf("证书申请失败: %s", domain)
	data := map[string]interface{}{
		"reason": reason,
	}
	return w.Notify(ctx, EventCertFailed, domain, message, data)
}

// NotifyDNSValidationTimeout 通知 DNS 验证超时
func (w *WebhookNotifier) NotifyDNSValidationTimeout(ctx context.Context, domain string, orderID string) error {
	message := fmt.Sprintf("DNS 验证超时: %s (订单ID: %s)", domain, orderID)
	data := map[string]interface{}{
		"order_id": orderID,
	}
	return w.Notify(ctx, EventDNSValidationTimeout, domain, message, data)
}

// IsEnabled 检查是否启用
func (w *WebhookNotifier) IsEnabled() bool {
	return w != nil && w.config != nil && w.config.Enabled
}
