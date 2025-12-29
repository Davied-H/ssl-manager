package huawei

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/huaweicloud/huaweicloud-sdk-go-v3/core/auth/basic"
	dns "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2"
	dnsModel "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/model"
	dnsRegion "github.com/huaweicloud/huaweicloud-sdk-go-v3/services/dns/v2/region"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
)

// DNSProvider 华为云DNS提供商
type DNSProvider struct {
	client *dns.DnsClient
}

// NewDNSProvider 创建华为云DNS提供商
func NewDNSProvider(cfg *config.HuaweiConfig) (*DNSProvider, error) {
	auth := basic.NewCredentialsBuilder().
		WithAk(cfg.AccessKey).
		WithSk(cfg.SecretKey).
		Build()

	region := cfg.Region
	if region == "" {
		region = "cn-north-4"
	}

	regionObj, err := dnsRegion.SafeValueOf(region)
	if err != nil {
		return nil, fmt.Errorf("无效的区域: %s", region)
	}

	client := dns.NewDnsClient(
		dns.DnsClientBuilder().
			WithRegion(regionObj).
			WithCredential(auth).
			Build())

	return &DNSProvider{client: client}, nil
}

// Name 返回提供商名称
func (p *DNSProvider) Name() string {
	return "huawei"
}

// getZoneID 获取域名的Zone ID
func (p *DNSProvider) getZoneID(domain string) (string, error) {
	mainDomain := extractMainDomain(domain)

	request := &dnsModel.ListPublicZonesRequest{}

	response, err := p.client.ListPublicZones(request)
	if err != nil {
		return "", fmt.Errorf("获取Zone列表失败: %w", err)
	}

	if response.Zones != nil {
		for _, zone := range *response.Zones {
			if zone.Name != nil {
				zoneName := strings.TrimSuffix(*zone.Name, ".")
				if zoneName == mainDomain {
					return *zone.Id, nil
				}
			}
		}
	}

	return "", fmt.Errorf("未找到域名 %s 的Zone", mainDomain)
}

// AddRecord 添加DNS记录
func (p *DNSProvider) AddRecord(ctx context.Context, domain, rr, recordType, value string) error {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	log.Printf("[华为云DNS] 添加记录: %s.%s -> %s (类型: %s)", subDomain, mainDomain, value, recordType)

	zoneID, err := p.getZoneID(domain)
	if err != nil {
		return err
	}

	// 先检查是否已存在相同记录
	existingRecord, err := p.FindRecord(ctx, domain, subDomain, recordType)
	if err != nil {
		log.Printf("[华为云DNS] 检查现有记录失败: %v", err)
	}

	if existingRecord != nil {
		// 更新现有记录
		return p.UpdateRecord(ctx, domain, existingRecord.RecordID, subDomain, recordType, value)
	}

	// 构建完整记录名
	recordName := subDomain + "." + mainDomain + "."

	// 添加新记录
	request := &dnsModel.CreateRecordSetRequest{
		ZoneId: zoneID,
		Body: &dnsModel.CreateRecordSetRequestBody{
			Name:    recordName,
			Type:    recordType,
			Records: []string{value},
		},
	}

	_, err = p.client.CreateRecordSet(request)
	if err != nil {
		return fmt.Errorf("添加DNS记录失败: %w", err)
	}

	log.Printf("[华为云DNS] 记录已添加")
	return nil
}

// UpdateRecord 更新DNS记录
func (p *DNSProvider) UpdateRecord(ctx context.Context, domain, recordID, rr, recordType, value string) error {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	log.Printf("[华为云DNS] 更新记录: ID=%s, %s -> %s", recordID, subDomain, value)

	zoneID, err := p.getZoneID(domain)
	if err != nil {
		return err
	}

	recordName := subDomain + "." + mainDomain + "."

	request := &dnsModel.UpdateRecordSetRequest{
		ZoneId:      zoneID,
		RecordsetId: recordID,
		Body: &dnsModel.UpdateRecordSetReq{
			Name:    &recordName,
			Type:    &recordType,
			Records: &[]string{value},
		},
	}

	_, err = p.client.UpdateRecordSet(request)
	if err != nil {
		return fmt.Errorf("更新DNS记录失败: %w", err)
	}

	log.Printf("[华为云DNS] 记录已更新")
	return nil
}

// DeleteRecord 删除DNS记录
func (p *DNSProvider) DeleteRecord(ctx context.Context, domain, recordID string) error {
	log.Printf("[华为云DNS] 删除记录: ID=%s", recordID)

	zoneID, err := p.getZoneID(domain)
	if err != nil {
		return err
	}

	request := &dnsModel.DeleteRecordSetRequest{
		ZoneId:      zoneID,
		RecordsetId: recordID,
	}

	_, err = p.client.DeleteRecordSet(request)
	if err != nil {
		return fmt.Errorf("删除DNS记录失败: %w", err)
	}

	log.Printf("[华为云DNS] 记录已删除")
	return nil
}

// FindRecord 查找DNS记录
func (p *DNSProvider) FindRecord(ctx context.Context, domain, rr, recordType string) (*provider.DNSRecord, error) {
	mainDomain := extractMainDomain(domain)
	subDomain := extractSubDomain(rr, mainDomain)

	zoneID, err := p.getZoneID(domain)
	if err != nil {
		return nil, err
	}

	recordName := subDomain + "." + mainDomain + "."

	request := &dnsModel.ListRecordSetsByZoneRequest{
		ZoneId: zoneID,
		Name:   &recordName,
		Type:   &recordType,
	}

	response, err := p.client.ListRecordSetsByZone(request)
	if err != nil {
		return nil, fmt.Errorf("查询DNS记录失败: %w", err)
	}

	if response.Recordsets != nil {
		for _, recordSet := range *response.Recordsets {
			if recordSet.Name != nil && *recordSet.Name == recordName &&
				recordSet.Type != nil && *recordSet.Type == recordType {
				var value string
				if recordSet.Records != nil && len(*recordSet.Records) > 0 {
					value = (*recordSet.Records)[0]
				}
				var ttl int
				if recordSet.Ttl != nil {
					ttl = int(*recordSet.Ttl)
				}
				return &provider.DNSRecord{
					RecordID: *recordSet.Id,
					Domain:   mainDomain,
					RR:       subDomain,
					Type:     *recordSet.Type,
					Value:    value,
					TTL:      ttl,
				}, nil
			}
		}
	}

	return nil, nil
}

// ListRecords 列出DNS记录
func (p *DNSProvider) ListRecords(ctx context.Context, domain string) ([]*provider.DNSRecord, error) {
	mainDomain := extractMainDomain(domain)

	zoneID, err := p.getZoneID(domain)
	if err != nil {
		return nil, err
	}

	request := &dnsModel.ListRecordSetsByZoneRequest{
		ZoneId: zoneID,
	}

	response, err := p.client.ListRecordSetsByZone(request)
	if err != nil {
		return nil, fmt.Errorf("获取DNS记录列表失败: %w", err)
	}

	var records []*provider.DNSRecord
	if response.Recordsets != nil {
		for _, recordSet := range *response.Recordsets {
			var value string
			if recordSet.Records != nil && len(*recordSet.Records) > 0 {
				value = (*recordSet.Records)[0]
			}

			rr := ""
			if recordSet.Name != nil {
				rr = strings.TrimSuffix(*recordSet.Name, "."+mainDomain+".")
			}

			var ttl int
			if recordSet.Ttl != nil {
				ttl = int(*recordSet.Ttl)
			}

			recordType := ""
			if recordSet.Type != nil {
				recordType = *recordSet.Type
			}

			records = append(records, &provider.DNSRecord{
				RecordID: *recordSet.Id,
				Domain:   mainDomain,
				RR:       rr,
				Type:     recordType,
				Value:    value,
				TTL:      ttl,
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
