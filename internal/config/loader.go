package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 向后兼容：旧版 aliyun 配置迁移到 providers.aliyun
	if config.Aliyun != nil && config.Providers.Aliyun == nil {
		config.Providers.Aliyun = config.Aliyun
	}

	// 设置默认值
	if config.OutputDir == "" {
		config.OutputDir = "./certs"
	}
	if config.CheckInterval == 0 {
		config.CheckInterval = 24
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 1 // 默认并发数为1，保持向后兼容
	}

	// 验证配置
	if err := validate(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// validate 验证配置
func validate(config *Config) error {
	if len(config.Domains) == 0 {
		return fmt.Errorf("未配置任何域名")
	}

	// 检查每个域名配置的提供商凭证是否存在
	for _, domain := range config.Domains {
		certProvider := domain.GetCertProvider()
		dnsProvider := domain.GetDNSProvider()

		if err := validateProviderConfig(config, certProvider, "证书"); err != nil {
			return fmt.Errorf("域名 %s: %w", domain.Domain, err)
		}

		if err := validateProviderConfig(config, dnsProvider, "DNS"); err != nil {
			return fmt.Errorf("域名 %s: %w", domain.Domain, err)
		}

		if domain.RenewDays <= 0 {
			return fmt.Errorf("域名 %s: renew_days 必须大于 0", domain.Domain)
		}
	}

	return nil
}

// validateProviderConfig 验证提供商配置是否存在
func validateProviderConfig(config *Config, providerName, providerType string) error {
	switch providerName {
	case "aliyun":
		if config.Providers.Aliyun == nil {
			return fmt.Errorf("%s提供商 aliyun 未配置凭证", providerType)
		}
		if config.Providers.Aliyun.AccessKeyID == "" || config.Providers.Aliyun.AccessKeySecret == "" {
			return fmt.Errorf("aliyun 凭证不完整")
		}
	case "tencent":
		if config.Providers.Tencent == nil {
			return fmt.Errorf("%s提供商 tencent 未配置凭证", providerType)
		}
		if config.Providers.Tencent.SecretID == "" || config.Providers.Tencent.SecretKey == "" {
			return fmt.Errorf("tencent 凭证不完整")
		}
	case "huawei":
		if config.Providers.Huawei == nil {
			return fmt.Errorf("%s提供商 huawei 未配置凭证", providerType)
		}
		if config.Providers.Huawei.AccessKey == "" || config.Providers.Huawei.SecretKey == "" {
			return fmt.Errorf("huawei 凭证不完整")
		}
	default:
		return fmt.Errorf("不支持的%s提供商: %s", providerType, providerName)
	}
	return nil
}
