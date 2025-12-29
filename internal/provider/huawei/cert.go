package huawei

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	scm "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/scm/v3"
	scmModel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/scm/v3/model"
	scmRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/scm/v3/region"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
)

// CertProvider 华为云证书提供商
type CertProvider struct {
	client *scm.ScmClient
}

// NewCertProvider 创建华为云证书提供商
func NewCertProvider(cfg *config.HuaweiConfig) (*CertProvider, error) {
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.AccessKey).
		WithSk(cfg.SecretKey).
		Build()

	region := cfg.Region
	if region == "" {
		region = "cn-north-4"
	}

	regionObj, err := scmRegion.SafeValueOf(region)
	if err != nil {
		return nil, fmt.Errorf("无效的区域: %s", region)
	}

	client := scm.NewScmClient(
		scm.ScmClientBuilder().
			WithRegion(regionObj).
			WithCredential(auth).
			Build())

	return &CertProvider{client: client}, nil
}

// Name 返回提供商名称
func (p *CertProvider) Name() string {
	return "huawei"
}

// ApplyCertificate 申请证书
// 注意：华为云免费证书需要通过控制台申请，API主要用于管理已有证书
func (p *CertProvider) ApplyCertificate(ctx context.Context, domain string) (string, error) {
	log.Printf("[华为云] 开始为 %s 申请SSL证书...", domain)

	// 华为云SCM API不直接支持申请免费证书
	// 这里返回提示信息
	return "", fmt.Errorf("华为云暂不支持通过API申请免费证书，请在控制台手动申请后使用此工具管理")
}

// GetCertificateStatus 获取证书状态
func (p *CertProvider) GetCertificateStatus(ctx context.Context, certID string) (*provider.CertificateStatus, error) {
	request := &scmModel.ShowCertificateRequest{
		CertificateId: certID,
	}

	response, err := p.client.ShowCertificate(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书状态失败: %w", err)
	}

	status := "certificate" // 华为云只能获取已签发的证书
	if response.Status != nil {
		switch *response.Status {
		case "PAID", "CHECKING":
			status = "process"
		case "ISSUED":
			status = "certificate"
		case "REVOKED", "EXPIRED":
			status = "failed"
		}
	}

	return &provider.CertificateStatus{
		OrderID: certID,
		Status:  status,
	}, nil
}

// DownloadCertificate 下载证书
func (p *CertProvider) DownloadCertificate(ctx context.Context, certID string) (*provider.Certificate, error) {
	request := &scmModel.ExportCertificateRequest{
		CertificateId: certID,
	}

	response, err := p.client.ExportCertificate(request)
	if err != nil {
		return nil, fmt.Errorf("下载证书失败: %w", err)
	}

	var certificate, privateKey, chain string

	if response.Certificate != nil {
		certificate = *response.Certificate
	}
	if response.PrivateKey != nil {
		privateKey = *response.PrivateKey
	}
	if response.CertificateChain != nil {
		chain = *response.CertificateChain
	}

	if certificate == "" {
		return nil, fmt.Errorf("证书内容为空")
	}

	return &provider.Certificate{
		Certificate: certificate,
		PrivateKey:  privateKey,
		Chain:       chain,
	}, nil
}

// ListCertificates 列出已签发的证书
func (p *CertProvider) ListCertificates(ctx context.Context) ([]*provider.CertificateInfo, error) {
	request := &scmModel.ListCertificatesRequest{}

	response, err := p.client.ListCertificates(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书列表失败: %w", err)
	}

	var certs []*provider.CertificateInfo
	if response.Certificates != nil {
		for _, cert := range *response.Certificates {
			// 只返回已签发的证书
			if cert.Status != "ISSUED" {
				continue
			}

			var notAfter time.Time
			if cert.ExpireTime != "" {
				notAfter, _ = time.Parse("2006-01-02 15:04:05", cert.ExpireTime)
			}

			var sans []string
			if cert.Sans != "" {
				sans = strings.Split(cert.Sans, ",")
			}

			certs = append(certs, &provider.CertificateInfo{
				CertID:   cert.Id,
				Domain:   cert.Domain,
				Sans:     sans,
				NotAfter: notAfter,
				Status:   "issued",
			})
		}
	}

	return certs, nil
}

// FindValidCertificate 查找域名的有效证书
func (p *CertProvider) FindValidCertificate(ctx context.Context, domain string, minDays int) (*provider.CertificateInfo, error) {
	certs, err := p.ListCertificates(ctx)
	if err != nil {
		return nil, err
	}

	log.Printf("[华为云] 共查询到 %d 个已签发证书", len(certs))

	mainDomain := extractMainDomain(domain)

	for _, cert := range certs {
		matched := cert.Domain == domain ||
			cert.Domain == "*."+mainDomain ||
			containsDomain(cert.Sans, domain)

		daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

		log.Printf("[华为云] 证书: Domain=%s, Sans=%v, 到期: %s, 剩余: %d 天, 匹配: %v",
			cert.Domain, cert.Sans, cert.NotAfter.Format("2006-01-02"), daysRemaining, matched)

		if matched && daysRemaining > minDays {
			return cert, nil
		}
	}

	return nil, nil
}

// GetCertificateDetail 获取证书详情
func (p *CertProvider) GetCertificateDetail(ctx context.Context, certID string) (*provider.Certificate, error) {
	return p.DownloadCertificate(ctx, certID)
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
