package core

import (
	"context"
	"fmt"
	"log"
	"time"

	"ssl-manager/internal/config"
	"ssl-manager/internal/provider"
	"ssl-manager/internal/storage"
)

// Manager 证书管理器
type Manager struct {
	config    *config.Config
	factory   *Factory
	storage   *storage.FileStorage
	validator *Validator
	executor  *Executor
}

// NewManager 创建管理器
func NewManager(cfg *config.Config) (*Manager, error) {
	return &Manager{
		config:    cfg,
		factory:   NewFactory(cfg),
		storage:   storage.NewFileStorage(cfg.OutputDir),
		validator: NewValidator(),
		executor:  NewExecutor(),
	}, nil
}

// Run 运行证书管理
func (m *Manager) Run(ctx context.Context) error {
	log.Println("========== 开始检查证书 ==========")

	for _, domainCfg := range m.config.Domains {
		if err := m.ProcessDomain(ctx, domainCfg); err != nil {
			log.Printf("处理域名 %s 失败: %v", domainCfg.Domain, err)
		}
	}

	log.Println("========== 检查完成 ==========")
	return nil
}

// ProcessDomain 处理单个域名
func (m *Manager) ProcessDomain(ctx context.Context, domainCfg config.DomainConfig) error {
	domain := domainCfg.Domain
	renewDays := domainCfg.RenewDays

	certProviderName := domainCfg.GetCertProvider()
	dnsProviderName := domainCfg.GetDNSProvider()

	log.Printf("\n========== 处理域名: %s ==========", domain)
	log.Printf("  证书提供商: %s, DNS提供商: %s", certProviderName, dnsProviderName)

	// 获取提供商
	certProvider, dnsProvider, err := m.factory.GetProvidersForDomain(&domainCfg)
	if err != nil {
		return fmt.Errorf("获取提供商失败: %w", err)
	}

	var certDownloaded bool

	// 1. 检查云平台是否有已签发的有效证书
	log.Printf("检查%s是否有已签发的有效证书...", certProviderName)
	existingCert, err := certProvider.FindValidCertificate(ctx, domain, renewDays)
	if err != nil {
		log.Printf("查询已有证书失败: %v", err)
	}

	if existingCert != nil {
		daysRemaining := int(time.Until(existingCert.NotAfter).Hours() / 24)
		log.Printf("找到有效证书！剩余有效期: %d 天，CertID: %s", daysRemaining, existingCert.CertID)

		// 下载已有证书
		cert, err := certProvider.GetCertificateDetail(ctx, existingCert.CertID)
		if err != nil {
			log.Printf("下载已有证书失败: %v，将尝试申请新证书", err)
		} else {
			if err := m.storage.SaveCertificate(domain, cert); err != nil {
				log.Printf("保存证书失败: %v", err)
			} else {
				log.Printf("域名 %s 已有有效证书，已下载完成！", domain)
				certDownloaded = true
			}
		}
	} else {
		log.Printf("未找到有效期大于 %d 天的已签发证书", renewDays)
	}

	// 2. 如果没有下载到证书，检查是否需要申请新证书
	if !certDownloaded {
		needRenew, expiry, err := m.validator.NeedRenew(domain, renewDays)
		if err != nil {
			log.Printf("检查线上证书失败: %v", err)
		}

		if !needRenew {
			log.Printf("线上证书有效，无需续期")
			return nil
		}

		if !expiry.IsZero() {
			log.Printf("线上证书将在 %s 过期，需要续期", expiry.Format("2006-01-02"))
		}

		// 申请新证书
		orderID, err := certProvider.ApplyCertificate(ctx, domain)
		if err != nil {
			return fmt.Errorf("申请证书失败: %w", err)
		}

		// 等待DNS验证并下载证书
		if err := m.waitForDNSValidation(ctx, certProvider, dnsProvider, domain, orderID); err != nil {
			return fmt.Errorf("域名验证失败: %w", err)
		}

		// 下载证书
		cert, err := certProvider.DownloadCertificate(ctx, orderID)
		if err != nil {
			return fmt.Errorf("下载证书失败: %w", err)
		}

		if err := m.storage.SaveCertificate(domain, cert); err != nil {
			return fmt.Errorf("保存证书失败: %w", err)
		}

		certDownloaded = true
	}

	// 3. 执行后置命令
	if certDownloaded {
		postCommand := domainCfg.PostCommand
		if postCommand == "" {
			postCommand = m.config.PostCommand
		}

		if postCommand != "" {
			vars := m.executor.BuildVars(
				domain,
				m.storage.GetCertDir(domain),
				m.storage.GetCertPath(domain),
				m.storage.GetKeyPath(domain),
				m.storage.GetFullchainPath(domain),
			)
			if err := m.executor.RunPostCommand(postCommand, vars); err != nil {
				log.Printf("执行后置命令失败: %v", err)
			}
		}
	}

	log.Printf("域名 %s 的证书处理完成！", domain)
	return nil
}

// waitForDNSValidation 等待DNS验证完成
func (m *Manager) waitForDNSValidation(ctx context.Context, certProvider provider.CertProvider, dnsProvider provider.DNSProvider, domain, orderID string) error {
	log.Printf("开始处理域名验证...")

	var dnsRecordAdded bool
	var lastRecordDomain string

	maxRetries := 60 // 最多等待30分钟
	consecutiveErrors := 0

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		status, err := certProvider.GetCertificateStatus(ctx, orderID)
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= 3 {
				return fmt.Errorf("连续获取证书状态失败: %w", err)
			}
			log.Printf("获取状态失败，等待后重试...")
			time.Sleep(10 * time.Second)
			continue
		}
		consecutiveErrors = 0

		log.Printf("当前状态: %s", status.Status)

		switch status.Status {
		case "domain_verify":
			if status.RecordDomain == "" || status.RecordValue == "" {
				log.Printf("等待验证信息...")
				time.Sleep(10 * time.Second)
				continue
			}

			log.Printf("DNS验证信息:")
			log.Printf("  记录名: %s", status.RecordDomain)
			log.Printf("  记录类型: %s", status.RecordType)
			log.Printf("  记录值: %s", status.RecordValue)

			// 只添加一次DNS记录（或记录变化时更新）
			if !dnsRecordAdded || lastRecordDomain != status.RecordDomain {
				if err := dnsProvider.AddRecord(ctx, domain, status.RecordDomain, status.RecordType, status.RecordValue); err != nil {
					log.Printf("添加DNS验证记录失败: %v，将重试...", err)
					time.Sleep(10 * time.Second)
					continue
				}
				dnsRecordAdded = true
				lastRecordDomain = status.RecordDomain
			}

			log.Printf("DNS记录已添加，等待验证...")
			time.Sleep(20 * time.Second)

		case "process":
			log.Printf("证书正在签发中，请等待...")
			time.Sleep(20 * time.Second)

		case "certificate":
			log.Printf("证书已签发成功！")
			return nil

		case "failed":
			return fmt.Errorf("证书申请失败")

		default:
			log.Printf("当前状态: %s，继续等待...", status.Status)
			time.Sleep(15 * time.Second)
		}
	}

	return fmt.Errorf("等待超时，请检查云平台控制台，订单ID: %s", orderID)
}

// ContinueOrder 继续处理已存在的订单
func (m *Manager) ContinueOrder(ctx context.Context, orderID, domain, certProviderName, dnsProviderName string) error {
	log.Printf("\n========== 继续处理订单: %s (域名: %s) ==========", orderID, domain)

	// 获取提供商
	certProvider, err := m.factory.GetCertProvider(certProviderName)
	if err != nil {
		return fmt.Errorf("获取证书提供商失败: %w", err)
	}

	dnsProvider, err := m.factory.GetDNSProvider(dnsProviderName)
	if err != nil {
		return fmt.Errorf("获取DNS提供商失败: %w", err)
	}

	// 检查订单状态
	status, err := certProvider.GetCertificateStatus(ctx, orderID)
	if err != nil {
		return fmt.Errorf("获取订单状态失败: %w", err)
	}

	log.Printf("当前订单状态: %s", status.Status)

	if status.Status == "certificate" {
		// 证书已签发，直接下载
		log.Printf("证书已签发，开始下载...")
		cert, err := certProvider.DownloadCertificate(ctx, orderID)
		if err != nil {
			return fmt.Errorf("下载证书失败: %w", err)
		}
		return m.storage.SaveCertificate(domain, cert)
	}

	// 继续等待验证
	if err := m.waitForDNSValidation(ctx, certProvider, dnsProvider, domain, orderID); err != nil {
		return fmt.Errorf("域名验证失败: %w", err)
	}

	// 下载证书
	cert, err := certProvider.DownloadCertificate(ctx, orderID)
	if err != nil {
		return fmt.Errorf("下载证书失败: %w", err)
	}

	if err := m.storage.SaveCertificate(domain, cert); err != nil {
		return fmt.Errorf("保存证书失败: %w", err)
	}

	log.Printf("订单 %s 处理完成！", orderID)
	return nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *config.Config {
	return m.config
}
