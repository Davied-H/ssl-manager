package core

import (
	"fmt"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
	"ssl-manager/internal/provider/aliyun"
	"ssl-manager/internal/provider/huawei"
	"ssl-manager/internal/provider/tencent"
)

// Factory 提供商工厂
type Factory struct {
	config *config.Config

	// 缓存已创建的提供商实例
	certProviders map[string]provider.CertProvider
	dnsProviders  map[string]provider.DNSProvider
}

// NewFactory 创建工厂
func NewFactory(cfg *config.Config) *Factory {
	return &Factory{
		config:        cfg,
		certProviders: make(map[string]provider.CertProvider),
		dnsProviders:  make(map[string]provider.DNSProvider),
	}
}

// GetCertProvider 获取证书提供商
func (f *Factory) GetCertProvider(name string) (provider.CertProvider, error) {
	// 检查缓存
	if p, ok := f.certProviders[name]; ok {
		return p, nil
	}

	// 创建新实例
	var p provider.CertProvider
	var err error

	switch name {
	case "aliyun":
		if f.config.Providers.Aliyun == nil {
			return nil, fmt.Errorf("阿里云证书提供商未配置")
		}
		p, err = aliyun.NewCertProvider(f.config.Providers.Aliyun)

	case "tencent":
		if f.config.Providers.Tencent == nil {
			return nil, fmt.Errorf("腾讯云证书提供商未配置")
		}
		p, err = tencent.NewCertProvider(f.config.Providers.Tencent)

	case "huawei":
		if f.config.Providers.Huawei == nil {
			return nil, fmt.Errorf("华为云证书提供商未配置")
		}
		p, err = huawei.NewCertProvider(f.config.Providers.Huawei)

	default:
		return nil, fmt.Errorf("不支持的证书提供商: %s", name)
	}

	if err != nil {
		return nil, err
	}

	// 缓存实例
	f.certProviders[name] = p
	return p, nil
}

// GetDNSProvider 获取DNS提供商
func (f *Factory) GetDNSProvider(name string) (provider.DNSProvider, error) {
	// 检查缓存
	if p, ok := f.dnsProviders[name]; ok {
		return p, nil
	}

	// 创建新实例
	var p provider.DNSProvider
	var err error

	switch name {
	case "aliyun":
		if f.config.Providers.Aliyun == nil {
			return nil, fmt.Errorf("阿里云DNS提供商未配置")
		}
		p, err = aliyun.NewDNSProvider(f.config.Providers.Aliyun)

	case "tencent":
		if f.config.Providers.Tencent == nil {
			return nil, fmt.Errorf("腾讯云DNS提供商未配置")
		}
		p, err = tencent.NewDNSProvider(f.config.Providers.Tencent)

	case "huawei":
		if f.config.Providers.Huawei == nil {
			return nil, fmt.Errorf("华为云DNS提供商未配置")
		}
		p, err = huawei.NewDNSProvider(f.config.Providers.Huawei)

	default:
		return nil, fmt.Errorf("不支持的DNS提供商: %s", name)
	}

	if err != nil {
		return nil, err
	}

	// 缓存实例
	f.dnsProviders[name] = p
	return p, nil
}

// GetProvidersForDomain 获取域名的证书和DNS提供商
func (f *Factory) GetProvidersForDomain(domainCfg *config.DomainConfig) (provider.CertProvider, provider.DNSProvider, error) {
	certProvider, err := f.GetCertProvider(domainCfg.GetCertProvider())
	if err != nil {
		return nil, nil, fmt.Errorf("获取证书提供商失败: %w", err)
	}

	dnsProvider, err := f.GetDNSProvider(domainCfg.GetDNSProvider())
	if err != nil {
		return nil, nil, fmt.Errorf("获取DNS提供商失败: %w", err)
	}

	return certProvider, dnsProvider, nil
}
