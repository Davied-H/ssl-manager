package core

import (
	"crypto/tls"
	"fmt"
	"log"
	"time"

	domainpkg "ssl-manager/internal/domain"
)

// Validator 证书验证器
type Validator struct{}

// NewValidator 创建验证器
func NewValidator() *Validator {
	return &Validator{}
}

// CheckCertExpiry 检查证书有效期，返回过期时间和证书覆盖的域名列表
func (v *Validator) CheckCertExpiry(domain string) (time.Time, []string, error) {
	conn, err := tls.Dial("tcp", domain+":443", &tls.Config{
		InsecureSkipVerify: true,
	})
	if err != nil {
		return time.Time{}, nil, fmt.Errorf("连接失败: %w", err)
	}
	defer conn.Close()

	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return time.Time{}, nil, fmt.Errorf("未找到证书")
	}

	cert := certs[0]
	// 收集证书覆盖的所有域名（CN + SANs）
	var domains []string
	if cert.Subject.CommonName != "" {
		domains = append(domains, cert.Subject.CommonName)
	}
	domains = append(domains, cert.DNSNames...)

	return cert.NotAfter, domains, nil
}

// NeedRenew 判断是否需要续期（检查过期时间和域名匹配）
func (v *Validator) NeedRenew(domain string, renewDays int) (bool, time.Time, error) {
	expiry, certDomains, err := v.CheckCertExpiry(domain)
	if err != nil {
		log.Printf("无法获取 %s 的证书信息: %v，将尝试申请新证书", domain, err)
		return true, time.Time{}, nil
	}

	// 检查域名是否匹配
	matched := v.matchDomain(certDomains, domain)
	if !matched {
		log.Printf("线上证书域名不匹配 (证书域名: %v, 目标域名: %s)，需要重新申请", certDomains, domain)
		return true, expiry, nil
	}

	daysUntilExpiry := int(time.Until(expiry).Hours() / 24)
	log.Printf("域名 %s 的证书将在 %d 天后过期 (%s)", domain, daysUntilExpiry, expiry.Format("2006-01-02"))

	return daysUntilExpiry <= renewDays, expiry, nil
}

// matchDomain 检查目标域名是否在证书域名列表中匹配
func (v *Validator) matchDomain(certDomains []string, targetDomain string) bool {
	for _, certDomain := range certDomains {
		if domainpkg.MatchDomain(certDomain, targetDomain) {
			return true
		}
	}
	return false
}
