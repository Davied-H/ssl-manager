package tencent

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
)

// DNSProvider 腾讯云DNS提供商 (DNSPod)
type DNSProvider struct {
	client *dnspod.Client
}

// NewDNSProvider 创建腾讯云DNS提供商
func NewDNSProvider(cfg *config.TencentConfig) (*DNSProvider, error) {
	credential := common.NewCredential(cfg.SecretID, cfg.SecretKey)
	cpf := profile.NewClientProfile()
	cpf.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"

	client, err := dnspod.NewClient(credential, "", cpf)
	if err != nil {
		return nil, fmt.Errorf("创建腾讯云DNSPod客户端失败: %w", err)
	}

	return &DNSProvider{client: client}, nil
}

// Name 返回提供商名称
func (p *DNSProvider) Name() string {
	return "tencent"
}

// AddRecord 添加DNS记录
func (p *DNSProvider) AddRecord(ctx context.Context, domain, rr, recordType, value string) error {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	log.Printf("[腾讯云DNS] 添加记录: %s.%s -> %s (类型: %s)", subDomain, mainDomain, value, recordType)

	// 先检查是否已存在相同记录
	existingRecord, err := p.FindRecord(ctx, domain, subDomain, recordType)
	if err != nil {
		log.Printf("[腾讯云DNS] 检查现有记录失败: %v", err)
	}

	if existingRecord != nil {
		// 更新现有记录
		return p.UpdateRecord(ctx, domain, existingRecord.RecordID, subDomain, recordType, value)
	}

	// 添加新记录
	request := dnspod.NewCreateRecordRequest()
	request.Domain = common.StringPtr(mainDomain)
	request.SubDomain = common.StringPtr(subDomain)
	request.RecordType = common.StringPtr(recordType)
	request.RecordLine = common.StringPtr("默认")
	request.Value = common.StringPtr(value)

	_, err = p.client.CreateRecord(request)
	if err != nil {
		return fmt.Errorf("添加DNS记录失败: %w", err)
	}

	log.Printf("[腾讯云DNS] 记录已添加")
	return nil
}

// UpdateRecord 更新DNS记录
func (p *DNSProvider) UpdateRecord(ctx context.Context, domain, recordID, rr, recordType, value string) error {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	log.Printf("[腾讯云DNS] 更新记录: ID=%s, %s -> %s", recordID, subDomain, value)

	var recordIdUint uint64
	fmt.Sscanf(recordID, "%d", &recordIdUint)

	request := dnspod.NewModifyRecordRequest()
	request.Domain = common.StringPtr(mainDomain)
	request.RecordId = common.Uint64Ptr(recordIdUint)
	request.SubDomain = common.StringPtr(subDomain)
	request.RecordType = common.StringPtr(recordType)
	request.RecordLine = common.StringPtr("默认")
	request.Value = common.StringPtr(value)

	_, err := p.client.ModifyRecord(request)
	if err != nil {
		return fmt.Errorf("更新DNS记录失败: %w", err)
	}

	log.Printf("[腾讯云DNS] 记录已更新")
	return nil
}

// DeleteRecord 删除DNS记录
func (p *DNSProvider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	mainDomain := extractMainDomain(domain)

	log.Printf("[腾讯云DNS] 删除记录: ID=%s", recordID)

	var recordIdUint uint64
	fmt.Sscanf(recordID, "%d", &recordIdUint)

	request := dnspod.NewDeleteRecordRequest()
	request.Domain = common.StringPtr(mainDomain)
	request.RecordId = common.Uint64Ptr(recordIdUint)

	_, err := p.client.DeleteRecord(request)
	if err != nil {
		return fmt.Errorf("删除DNS记录失败: %w", err)
	}

	log.Printf("[腾讯云DNS] 记录已删除")
	return nil
}

// FindRecord 查找DNS记录
func (p *DNSProvider) FindRecord(ctx context.Context, domain, rr, recordType string) (*provider.DNSRecord, error) {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	request := dnspod.NewDescribeRecordListRequest()
	request.Domain = common.StringPtr(mainDomain)
	request.Subdomain = common.StringPtr(subDomain)
	request.RecordType = common.StringPtr(recordType)

	response, err := p.client.DescribeRecordList(request)
	if err != nil {
		// 如果没有记录，腾讯云会返回错误
		if strings.Contains(err.Error(), "NoRecord") || strings.Contains(err.Error(), "记录列表为空") {
			return nil, nil
		}
		return nil, fmt.Errorf("查询DNS记录失败: %w", err)
	}

	if response.Response != nil && response.Response.RecordList != nil {
		for _, record := range response.Response.RecordList {
			if record.Name != nil && *record.Name == subDomain &&
				record.Type != nil && *record.Type == recordType {
				return &provider.DNSRecord{
					RecordID: fmt.Sprintf("%d", *record.RecordId),
					Domain:   mainDomain,
					RR:       *record.Name,
					Type:     *record.Type,
					Value:    *record.Value,
					TTL:      int(*record.TTL),
				}, nil
			}
		}
	}

	return nil, nil
}

// ListRecords 列出DNS记录
func (p *DNSProvider) ListRecords(ctx context.Context, domain string) ([]*provider.DNSRecord, error) {
	mainDomain := extractMainDomain(domain)

	request := dnspod.NewDescribeRecordListRequest()
	request.Domain = common.StringPtr(mainDomain)

	response, err := p.client.DescribeRecordList(request)
	if err != nil {
		if strings.Contains(err.Error(), "NoRecord") {
			return nil, nil
		}
		return nil, fmt.Errorf("获取DNS记录列表失败: %w", err)
	}

	var records []*provider.DNSRecord
	if response.Response != nil && response.Response.RecordList != nil {
		for _, record := range response.Response.RecordList {
			records = append(records, &provider.DNSRecord{
				RecordID: fmt.Sprintf("%d", *record.RecordId),
				Domain:   mainDomain,
				RR:       *record.Name,
				Type:     *record.Type,
				Value:    *record.Value,
				TTL:      int(*record.TTL),
			})
		}
	}

	return records, nil
}

// extractSubDomain 提取子域名部分
func extractSubDomain(fullRecord, mainDomain string) string {
	if strings.HasSuffix(fullRecord, "."+mainDomain) {
		return strings.TrimSuffix(fullRecord, "."+mainDomain)
	}
	return fullRecord
}
