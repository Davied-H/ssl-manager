package tencent

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	ssl "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ssl/v20191205"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
)

// CertProvider 腾讯云证书提供商
type CertProvider struct {
	client *ssl.Client
}

// NewCertProvider 创建腾讯云证书提供商
func NewCertProvider(cfg *config.TencentConfig) (*CertProvider, error) {
	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "ssl.tencentcloudapi.com"

	region := cfg.Region
	if region == "" {
		region = "ap-guangzhou"
	}

	client, err := ssl.NewClient(credential, region, cpf)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云SSL客户端失败: %w", err)
	}

	return &CertProvider{client: client}, nil
}

// Name 返回提供商名称
func (p *CertProvider) Name() string {
	return "tencent"
}

// ApplyCertificate 申请证书
func (p *CertProvider) ApplyCertificate(ctx context.Context, domain string) (string, error) {
	log.Printf("[腾讯云] 开始为 %s 申请免费SSL证书...", domain)

	request := ssl.NewApplyCertificateRequest()
	request.DvAuthMethod = common.StringPtr("DNS_AUTO")
	request.DomainName = common.StringPtr(domain)

	response, err := p.client.ApplyCertificate(request)
	if err != nil {
		return "", fmt.Errorf("申请证书失败: %w", err)
	}

	certID := *response.Response.CertificateId
	log.Printf("[腾讯云] 证书申请成功，CertificateId: %s", certID)

	return certID, nil
}

// GetCertificateStatus 获取证书状态
func (p *CertProvider) GetCertificateStatus(ctx context.Context, certID string) (*provider.CertificateStatus, error) {
	request := ssl.NewDescribeCertificateRequest()
	request.CertificateId = common.StringPtr(certID)

	response, err := p.client.DescribeCertificate(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书状态失败: %w", err)
	}

	// 映射腾讯云状态
	status := mapTencentStatus(*response.Response.Status)

	var recordDomain, recordType, recordValue string

	// 获取DNS验证信息
	if response.Response.DvAuthDetail != nil && len(response.Response.DvAuthDetail.DvAuths) > 0 {
		dvAuth := response.Response.DvAuthDetail.DvAuths[0]
		if dvAuth.DvAuthSubDomain != nil {
			recordDomain = *dvAuth.DvAuthSubDomain
		}
		if dvAuth.DvAuthVerifyType != nil {
			recordType = *dvAuth.DvAuthVerifyType
		}
		if dvAuth.DvAuthValue != nil {
			recordValue = *dvAuth.DvAuthValue
		}
	}

	return &provider.CertificateStatus{
		OrderID:      certID,
		Status:       status,
		RecordDomain: recordDomain,
		RecordType:   recordType,
		RecordValue:  recordValue,
	}, nil
}

// mapTencentStatus 映射腾讯云状态到统一状态
func mapTencentStatus(status uint64) string {
	// 腾讯云状态码:
	// 0: 审核中
	// 1: 已通过
	// 2: 审核失败
	// 3: 已过期
	// 4: DNS记录添加中
	// 5: 企业证书，待提交
	// 6: 订单取消中
	// 7: 已取消
	// 8: 已提交资料，待上传确认函
	// 9: 证书吊销中
	// 10: 已吊销
	// 11: 重颁发中
	// 12: 待上传吊销确认函
	switch status {
	case 0, 4, 5, 8:
		return "domain_verify"
	case 1:
		return "certificate"
	case 2, 7, 10:
		return "failed"
	default:
		return "process"
	}
}

// DownloadCertificate 下载证书
func (p *CertProvider) DownloadCertificate(ctx context.Context, certID string) (*provider.Certificate, error) {
	request := ssl.NewDownloadCertificateRequest()
	request.CertificateId = common.StringPtr(certID)

	response, err := p.client.DownloadCertificate(request)
	if err != nil {
		return nil, fmt.Errorf("下载证书失败: %w", err)
	}

	// 腾讯云返回的是Base64编码的ZIP文件内容
	// 这里需要解析ZIP获取证书和私钥
	// 实际实现可能需要下载并解压

	// 尝试使用 DescribeCertificateDetail 获取证书内容
	detailRequest := ssl.NewDescribeCertificateDetailRequest()
	detailRequest.CertificateId = common.StringPtr(certID)

	detailResponse, err := p.client.DescribeCertificateDetail(detailRequest)
	if err != nil {
		return nil, fmt.Errorf("获取证书详情失败: %w", err)
	}

	var certificate, privateKey string

	if detailResponse.Response.CertificatePublicKey != nil {
		certificate = *detailResponse.Response.CertificatePublicKey
	}
	if detailResponse.Response.CertificatePrivateKey != nil {
		privateKey = *detailResponse.Response.CertificatePrivateKey
	}

	// 如果上面获取不到，使用下载内容（需要解码）
	if certificate == "" && response.Response.Content != nil {
		log.Printf("[腾讯云] 证书内容需要从ZIP文件中提取")
		// 这里返回提示，实际使用时需要解压ZIP
		return nil, fmt.Errorf("腾讯云返回ZIP格式证书，请从控制台手动下载或使用SDK解压")
	}

	return &provider.Certificate{
		Certificate: certificate,
		PrivateKey:  privateKey,
		Chain:       certificate,
	}, nil
}

// ListCertificates 列出已签发的证书
func (p *CertProvider) ListCertificates(ctx context.Context) ([]*provider.CertificateInfo, error) {
	request := ssl.NewDescribeCertificatesRequest()
	request.Limit = common.Uint64Ptr(100)

	response, err := p.client.DescribeCertificates(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书列表失败: %w", err)
	}

	var certs []*provider.CertificateInfo
	for _, cert := range response.Response.Certificates {
		// 只返回已签发的证书
		if cert.Status == nil || *cert.Status != 1 {
			continue
		}

		var notAfter time.Time
		if cert.CertEndTime != nil {
			notAfter, _ = time.Parse("2006-01-02 15:04:05", *cert.CertEndTime)
		}

		var sans []string
		if cert.SubjectAltName != nil {
			for _, s := range cert.SubjectAltName {
				if s != nil {
					sans = append(sans, *s)
				}
			}
		}

		domain := ""
		if cert.Domain != nil {
			domain = *cert.Domain
		}

		certs = append(certs, &provider.CertificateInfo{
			CertID:   *cert.CertificateId,
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

	log.Printf("[腾讯云] 共查询到 %d 个已签发证书", len(certs))

	mainDomain := extractMainDomain(domain)

	for _, cert := range certs {
		matched := cert.Domain == domain ||
			cert.Domain == "*."+mainDomain ||
			containsDomain(cert.Sans, domain)

		daysRemaining := int(time.Until(cert.NotAfter).Hours() / 24)

		log.Printf("[腾讯云] 证书: Domain=%s, Sans=%v, 到期: %s, 剩余: %d 天, 匹配: %v",
			cert.Domain, cert.Sans, cert.NotAfter.Format("2006-01-02"), daysRemaining, matched)

		if matched && daysRemaining > minDays {
			return cert, nil
		}
	}

	return nil, nil
}

// GetCertificateDetail 获取证书详情
func (p *CertProvider) GetCertificateDetail(ctx context.Context, certID string) (*provider.Certificate, error) {
	request := ssl.NewDescribeCertificateDetailRequest()
	request.CertificateId = common.StringPtr(certID)

	response, err := p.client.DescribeCertificateDetail(request)
	if err != nil {
		return nil, fmt.Errorf("获取证书详情失败: %w", err)
	}

	var certificate, privateKey string

	if response.Response.CertificatePublicKey != nil {
		certificate = *response.Response.CertificatePublicKey
	}
	if response.Response.CertificatePrivateKey != nil {
		privateKey = *response.Response.CertificatePrivateKey
	}

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
