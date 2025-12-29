package aliyun

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cas "github.com/alibabacloud-go/cas-20200407/v3/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
)

// CertProvider 阿里云证书提供商
type CertProvider struct {
	client *cas.Client
}

// NewCertProvider 创建阿里云证书提供商
func NewCertProvider(cfg *config.AliyunConfig) (*CertProvider, error) {
	clientConfig := &openapi.Config{
		AccessKeyId:     tea.String(cfg.AccessKeyID),
		AccessKeySecret: tea.String(cfg.AccessKeySecret),
		Endpoint:        tea.String("cas.aliyuncs.com"),
	}

	client, err := cas.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云CAS客户端失败: %w", err)
	}

	return &CertProvider{client: client}, nil
}

// Name 返回提供商名称
func (p *CertProvider) Name() string {
	return "aliyun"
}

// ApplyCertificate 申请证书
func (p *CertProvider) ApplyCertificate(ctx context.Context, domain string) (string, error) {
	log.Printf("[阿里云] 开始为 %s 申请免费SSL证书...", domain)

	request := &cas.CreateCertificateForPackageRequestRequest{
		Domain:       tea.String(domain),
		ValidateType: tea.String("DNS"),
		ProductCode:  tea.String("digicert-free-1-free"),
	}

	response, err := p.client.CreateCertificateForPackageRequest(request)
	if err != nil {
		return "", fmt.Errorf("创建证书订单失败: %w", err)
	}

	orderID := fmt.Sprintf("%d", tea.Int64Value(response.Body.OrderId))
	log.Printf("[阿里云] 证书订单创建成功，订单ID: %s", orderID)

	return orderID, nil
}

// GetCertificateStatus 获取证书状态
func (p *CertProvider) GetCertificateStatus(ctx context.Context, orderID string) (*provider.CertificateStatus, error) {
	var orderId int64
	fmt.Sscanf(orderID, "%d", &orderId)

	request := &cas.DescribeCertificateStateRequest{
		OrderId: tea.Int64(orderId),
	}

	// 带重试的请求
	var response *cas.DescribeCertificateStateResponse
	var lastErr error
	for retry := 0; retry < 3; retry++ {
		var err error
		response, err = p.client.DescribeCertificateState(request)
		if err == nil {
			break
		}
		lastErr = err
		log.Printf("[阿里云] 获取证书状态失败 (重试 %d/3): %v", retry+1, err)
		time.Sleep(time.Duration(retry+1) * 5 * time.Second)
	}
	if response == nil {
		return nil, fmt.Errorf("获取证书状态失败: %w", lastErr)
	}

	// 映射阿里云状态
	status := mapAliyunStatus(tea.StringValue(response.Body.Type))

	return &provider.CertificateStatus{
		OrderID:      orderID,
		Status:       status,
		RecordDomain: tea.StringValue(response.Body.RecordDomain),
		RecordType:   tea.StringValue(response.Body.RecordType),
		RecordValue:  tea.StringValue(response.Body.RecordValue),
	}, nil
}

// mapAliyunStatus 映射阿里云状态到统一状态
func mapAliyunStatus(aliyunStatus string) string {
	switch aliyunStatus {
	case "domain_verify":
		return "domain_verify"
	case "process", "verify", "payed", "checking":
		return "process"
	case "certificate":
		return "certificate"
	default:
		return aliyunStatus
	}
}

// DownloadCertificate 下载证书（通过订单ID）
func (p *CertProvider) DownloadCertificate(ctx context.Context, orderID string) (*provider.Certificate, error) {
	var orderId int64
	fmt.Sscanf(orderID, "%d", &orderId)

	request := &cas.DescribeCertificateStateRequest{
		OrderId: tea.Int64(orderId),
	}

	response, err := p.client.DescribeCertificateState(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书状态失败: %w", err)
	}

	stateType := tea.StringValue(response.Body.Type)
	if stateType != "certificate" {
		return nil, fmt.Errorf("证书尚未签发，当前状态: %s", stateType)
	}

	certificate := tea.StringValue(response.Body.Certificate)
	privateKey := tea.StringValue(response.Body.PrivateKey)

	if certificate == "" || privateKey == "" {
		return nil, fmt.Errorf("证书内容为空")
	}

	return &provider.Certificate{
		Certificate: certificate,
		PrivateKey:  privateKey,
		Chain:       certificate, // 阿里云返回的证书已包含证书链
	}, nil
}

// ListCertificates 列出已签发的证书
func (p *CertProvider) ListCertificates(ctx context.Context) ([]*provider.CertificateInfo, error) {
	request := &cas.ListUserCertificateOrderRequest{
		OrderType: tea.String("CERT"),
		Status:    tea.String("ISSUED"),
	}

	response, err := p.client.ListUserCertificateOrder(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书列表失败: %w", err)
	}

	var certs []*provider.CertificateInfo
	for _, cert := range response.Body.CertificateOrderList {
		domain := tea.StringValue(cert.CommonName)
		if domain == "" {
			domain = tea.StringValue(cert.Domain)
		}

		var notAfter time.Time
		if endTime := tea.Int64Value(cert.CertEndTime); endTime > 0 {
			notAfter = time.UnixMilli(endTime)
		}

		var sans []string
		if sansStr := tea.StringValue(cert.Sans); sansStr != "" {
			sans = strings.Split(sansStr, ",")
		}

		certs = append(certs, &provider.CertificateInfo{
			CertID:   fmt.Sprintf("%d", tea.Int64Value(cert.CertificateId)),
			OrderID:  fmt.Sprintf("%d", tea.Int64Value(cert.OrderId)),
			Domain:   domain,
			Sans:     sans,
			NotAfter: notAfter,
			Status:   "issued",
		})
	}

	return certs, nil
}

// FindValidCertificate 查找域名的有效证书
func (p *CertProvider) FindValidCertificate(ctx context.Context, domain string, minDays int) (*provider.CertificateInfo, error) {
	certs, err := p.ListCertificates(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("[阿里云] 共查询到 %d 个已签发证书", len(certs))

	mainDomain := extractMainDomain(domain)

	for _, cert := range certs {
		// 检查域名是否匹配
		matched := cert.Domain == domain ||
			cert.Domain == "*."+mainDomain ||
			containsDomain(cert.Sans, domain)

		daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

		log.Printf("[阿里云] 证书: Domain=%s, Sans=%v, 到期: %s, 剩余: %d 天, 匹配: %v",
			cert.Domain, cert.Sans, cert.NotAfter.Format("2006-01-02"), daysRemaining, matched)

		if matched && daysRemaining > minDays {
			return cert, nil
		}
	}

	return nil, nil
}

// GetCertificateDetail 获取证书详情
func (p *CertProvider) GetCertificateDetail(ctx context.Context, certID string) (*provider.Certificate, error) {
	var certId int64
	fmt.Sscanf(certID, "%d", &certId)

	request := &cas.GetUserCertificateDetailRequest{
		CertId: tea.Int64(certId),
	}

	response, err := p.client.GetUserCertificateDetail(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书详情失败: %w", err)
	}

	certificate := tea.StringValue(response.Body.Cert)
	privateKey := tea.StringValue(response.Body.Key)

	if certificate == "" {
		return nil, fmt.Errorf("证书内容为空")
	}

	return &provider.Certificate{
		Certificate: certificate,
		PrivateKey:  privateKey,
		Chain:       certificate,
	}, nil
}

// extractMainDomain 从完整域名提取主域名
func extractMainDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return domain
}

// containsDomain 检查域名列表是否包含指定域名
func containsDomain(domains []string, domain string) bool {
	for _, d := range domains {
		if d == domain {
			return true
		}
	}
	return false
}
