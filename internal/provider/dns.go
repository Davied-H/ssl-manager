package provider

import "context"

// DNSProvider DNS提供商接口
type DNSProvider interface {
	// Name 返回提供商名称
	Name() string

	// AddRecord 添加DNS记录
	// domain: 主域名 (如 example.com)
	// rr: 主机记录/子域名 (如 _dnsauth.www)
	// recordType: 记录类型 (如 TXT)
	// value: 记录值
	AddRecord(ctx context.Context, domain, rr, recordType, value string) error

	// UpdateRecord 更新DNS记录
	UpdateRecord(ctx context.Context, domain, recordID, rr, recordType, value string) error

	// DeleteRecord 删除DNS记录
	DeleteRecord(ctx context.Context, domain, recordID string) error

	// FindRecord 查找DNS记录
	FindRecord(ctx context.Context, domain, rr, recordType string) (*DNSRecord, error)

	// ListRecords 列出DNS记录
	ListRecords(ctx context.Context, domain string) ([]*DNSRecord, error)
}
