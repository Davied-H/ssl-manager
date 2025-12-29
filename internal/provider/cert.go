package provider

import "context"

// CertProvider 证书提供商接口
type CertProvider interface {
	// Name 返回提供商名称
	Name() string

	// ApplyCertificate 申请证书，返回订单ID
	ApplyCertificate(ctx context.Context, domain string) (orderID string, err error)

	// GetCertificateStatus 获取证书状态
	GetCertificateStatus(ctx context.Context, orderID string) (*CertificateStatus, error)

	// DownloadCertificate 下载证书（通过订单ID）
	DownloadCertificate(ctx context.Context, orderID string) (*Certificate, error)

	// ListCertificates 列出已签发的证书
	ListCertificates(ctx context.Context) ([]*CertificateInfo, error)

	// FindValidCertificate 查找域名的有效证书（剩余有效期大于minDays天）
	FindValidCertificate(ctx context.Context, domain string, minDays int) (*CertificateInfo, error)

	// GetCertificateDetail 获取证书详情（通过证书ID）
	GetCertificateDetail(ctx context.Context, certID string) (*Certificate, error)
}
