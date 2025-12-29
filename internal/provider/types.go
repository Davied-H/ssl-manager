package provider

import "time"

// CertificateStatus 证书状态
type CertificateStatus struct {
	OrderID      string // 订单ID
	Status       string // 状态: pending, domain_verify, process, certificate, failed
	Domain       string // 域名
	RecordDomain string // DNS验证记录域名
	RecordType   string // DNS验证记录类型 (TXT)
	RecordValue  string // DNS验证记录值
}

// Certificate 证书内容
type Certificate struct {
	Certificate string // 证书内容 (PEM格式)
	PrivateKey  string // 私钥 (PEM格式)
	Chain       string // 证书链 (可选)
}

// CertificateInfo 证书信息
type CertificateInfo struct {
	CertID    string    // 证书ID
	OrderID   string    // 订单ID
	Domain    string    // 主域名
	Sans      []string  // 备用域名列表
	NotBefore time.Time // 生效时间
	NotAfter  time.Time // 过期时间
	Status    string    // 状态
}

// DNSRecord DNS记录
type DNSRecord struct {
	RecordID string // 记录ID
	Domain   string // 主域名
	RR       string // 主机记录 (子域名)
	Type     string // 记录类型
	Value    string // 记录值
	TTL      int    // TTL
}
