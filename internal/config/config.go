package config

// Config 配置结构
type Config struct {
	// 云平台凭证配置
	Providers ProvidersConfig `yaml:"providers"`

	// 域名配置
	Domains []DomainConfig `yaml:"domains"`

	// 全局配置
	OutputDir     string `yaml:"output_dir"`
	CheckInterval int    `yaml:"check_interval"` // 检查间隔（小时）
	PostCommand   string `yaml:"post_command"`   // 全局后置命令

	// 向后兼容：旧版阿里云配置
	Aliyun *AliyunConfig `yaml:"aliyun,omitempty"`
}

// ProvidersConfig 云平台凭证配置
type ProvidersConfig struct {
	Aliyun  *AliyunConfig  `yaml:"aliyun,omitempty"`
	Tencent *TencentConfig `yaml:"tencent,omitempty"`
	Huawei  *HuaweiConfig  `yaml:"huawei,omitempty"`
}

// AliyunConfig 阿里云配置
type AliyunConfig struct {
	AccessKeyID     string `yaml:"access_key_id"`
	AccessKeySecret string `yaml:"access_key_secret"`
	Region          string `yaml:"region"`
}

// TencentConfig 腾讯云配置
type TencentConfig struct {
	SecretID  string `yaml:"secret_id"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
}

// HuaweiConfig 华为云配置
type HuaweiConfig struct {
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Region    string `yaml:"region"`
	ProjectID string `yaml:"project_id"`
}

// DomainConfig 域名配置
type DomainConfig struct {
	Domain string `yaml:"domain"`

	// 简单模式：证书和DNS使用同一平台
	Provider string `yaml:"provider,omitempty"` // aliyun, tencent, huawei

	// 混合模式：证书和DNS使用不同平台
	CertProvider string `yaml:"cert_provider,omitempty"`
	DNSProvider  string `yaml:"dns_provider,omitempty"`

	RenewDays   int    `yaml:"renew_days"`
	PostCommand string `yaml:"post_command,omitempty"`
}

// GetCertProvider 获取证书提供商名称
func (d *DomainConfig) GetCertProvider() string {
	if d.CertProvider != "" {
		return d.CertProvider
	}
	if d.Provider != "" {
		return d.Provider
	}
	return "aliyun" // 默认使用阿里云
}

// GetDNSProvider 获取DNS提供商名称
func (d *DomainConfig) GetDNSProvider() string {
	if d.DNSProvider != "" {
		return d.DNSProvider
	}
	if d.Provider != "" {
		return d.Provider
	}
	return "aliyun" // 默认使用阿里云
}
