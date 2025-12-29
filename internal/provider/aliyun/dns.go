package aliyun

import (
	"context"
	"fmt"
	"log"
	"strings"

	alidns "github.com/alibabacloud-go/alidns-20150109/v4/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	"github.com/alibabacloud-go/tea/tea"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
)

// DNSProvider 阿里云DNS提供商
type DNSProvider struct {
	client *alidns.Client
}

// NewDNSProvider 创建阿里云DNS提供商
func NewDNSProvider(cfg *config.AliyunConfig) (*DNSProvider, error) {
	endpoint := "alidns.cn-hangzhou.aliyuncs.com"
	if cfg.Region != "" {
		endpoint = fmt.Sprintf("alidns.%s.aliyuncs.com", cfg.Region)
	}

	clientConfig := &openapi.Config{
		AccessKeyId:     tea.String(cfg.AccessKeyID),
		AccessKeySecret: tea.String(cfg.AccessKeySecret),
		Endpoint:        tea.String(endpoint),
	}

	client, err := alidns.NewClient(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("创建阿里云DNS客户端失败: %w", err)
	}

	return &DNSProvider{client: client}, nil
}

// Name 返回提供商名称
func (p *DNSProvider) Name() string {
	return "aliyun"
}

// AddRecord 添加DNS记录
func (p *DNSProvider) AddRecord(ctx context.Context, domain, rr, recordType, value string) error {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	log.Printf("[阿里云DNS] 添加记录: %s.%s -> %s (类型: %s)", subDomain, mainDomain, value, recordType)

	// 先检查是否已存在相同记录
	existingRecord, err := p.FindRecord(ctx, domain, subDomain, recordType)
	if err != nil {
		log.Printf("[阿里云DNS] 检查现有记录失败: %v", err)
	}

	if existingRecord != nil {
		// 更新现有记录
		return p.UpdateRecord(ctx, domain, existingRecord.RecordID, subDomain, recordType, value)
	}

	// 添加新记录
	request := &alidns.AddDomainRecordRequest{
		DomainName: tea.String(mainDomain),
		RR:         tea.String(subDomain),
		Type:       tea.String(recordType),
		Value:      tea.String(value),
	}

	_, err = p.client.AddDomainRecord(request)
	if err != nil {
		return fmt.Errorf("添加DNS记录失败: %w", err)
	}

	log.Printf("[阿里云DNS] 记录已添加")
	return nil
}

// UpdateRecord 更新DNS记录
func (p *DNSProvider) UpdateRecord(ctx context.Context, domain, recordID, rr, recordType, value string) error {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	log.Printf("[阿里云DNS] 更新记录: ID=%s, %s -> %s", recordID, subDomain, value)

	request := &alidns.UpdateDomainRecordRequest{
		RecordId: tea.String(recordID),
		RR:       tea.String(subDomain),
		Type:     tea.String(recordType),
		Value:    tea.String(value),
	}

	_, err := p.client.UpdateDomainRecord(request)
	if err != nil {
		return fmt.Errorf("更新DNS记录失败: %w", err)
	}

	log.Printf("[阿里云DNS] 记录已更新")
	return nil
}

// DeleteRecord 删除DNS记录
func (p *DNSProvider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	log.Printf("[阿里云DNS] 删除记录: ID=%s", recordID)

	request := &alidns.DeleteDomainRecordRequest{
		RecordId: tea.String(recordID),
	}

	_, err := p.client.DeleteDomainRecord(request)
	if err != nil {
		return fmt.Errorf("删除DNS记录失败: %w", err)
	}

	log.Printf("[阿里云DNS] 记录已删除")
	return nil
}

// FindRecord 查找DNS记录
func (p *DNSProvider) FindRecord(ctx context.Context, domain, rr, recordType string) (*provider.DNSRecord, error) {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	request := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(mainDomain),
		RRKeyWord:  tea.String(subDomain),
		Type:       tea.String(recordType),
	}

	response, err := p.client.DescribeDomainRecords(request)
	if err != nil {
		return nil, fmt.Errorf("查询DNS记录失败: %w", err)
	}

	if response.Body != nil && response.Body.DomainRecords != nil {
		for _, record := range response.Body.DomainRecords.Record {
			if tea.StringValue(record.RR) == subDomain && tea.StringValue(record.Type) == recordType {
				return &provider.DNSRecord{
					RecordID: tea.StringValue(record.RecordId),
					Domain:   mainDomain,
					RR:       tea.StringValue(record.RR),
					Type:     tea.StringValue(record.Type),
					Value:    tea.StringValue(record.Value),
					TTL:      int(tea.Int64Value(record.TTL)),
				}, nil
			}
		}
	}

	return nil, nil
}

// ListRecords 列出DNS记录
func (p *DNSProvider) ListRecords(ctx context.Context, domain string) ([]*provider.DNSRecord, error) {
	mainDomain := extractMainDomain(domain)

	request := &alidns.DescribeDomainRecordsRequest{
		DomainName: tea.String(mainDomain),
	}

	response, err := p.client.DescribeDomainRecords(request)
	if err != nil {
		return nil, fmt.Errorf("获取DNS记录列表失败: %w", err)
	}

	var records []*provider.DNSRecord
	if response.Body != nil && response.Body.DomainRecords != nil {
		for _, record := range response.Body.DomainRecords.Record {
			records = append(records, &provider.DNSRecord{
				RecordID: tea.StringValue(record.RecordId),
				Domain:   mainDomain,
				RR:       tea.StringValue(record.RR),
				Type:     tea.StringValue(record.Type),
				Value:    tea.StringValue(record.Value),
				TTL:      int(tea.Int64Value(record.TTL)),
			})
		}
	}

	return records, nil
}

// extractSubDomain 提取子域名部分（用于DNS记录的RR值）
func extractSubDomain(fullRecord, mainDomain string) string {
	if strings.HasSuffix(fullRecord, "."+mainDomain) {
		return strings.TrimSuffix(fullRecord, "."+mainDomain)
	}
	return fullRecord
}
