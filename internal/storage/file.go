package storage

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"ssl-manager/internal/provider"
)

// FileStorage 文件存储
type FileStorage struct {
	baseDir string
}

// NewFileStorage 创建文件存储
func NewFileStorage(baseDir string) *FileStorage {
	return &FileStorage{baseDir: baseDir}
}

// SaveCertificate 保存证书到文件
func (s *FileStorage) SaveCertificate(domain string, cert *provider.Certificate) error {
	outputDir := filepath.Join(s.baseDir, domain)

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 保存证书
	certPath := filepath.Join(outputDir, "cert.pem")
	if err := os.WriteFile(certPath, []byte(cert.Certificate), 0644); err != nil {
		return fmt.Errorf("保存证书失败: %w", err)
	}
	log.Printf("  - 证书文件: %s", certPath)

	// 保存私钥
	if cert.PrivateKey != "" {
		keyPath := filepath.Join(outputDir, "key.pem")
		if err := os.WriteFile(keyPath, []byte(cert.PrivateKey), 0600); err != nil {
			return fmt.Errorf("保存私钥失败: %w", err)
		}
		log.Printf("  - 私钥文件: %s", keyPath)
	} else {
		log.Printf("  - 警告: 私钥不可用")
	}

	// 保存完整证书链
	chain := cert.Chain
	if chain == "" {
		chain = cert.Certificate
	}
	fullchainPath := filepath.Join(outputDir, "fullchain.pem")
	if err := os.WriteFile(fullchainPath, []byte(chain), 0644); err != nil {
		log.Printf("  - 保存证书链失败: %v", err)
	} else {
		log.Printf("  - 证书链文件: %s", fullchainPath)
	}

	log.Printf("证书已保存到: %s", outputDir)
	return nil
}

// GetCertDir 获取证书目录
func (s *FileStorage) GetCertDir(domain string) string {
	return filepath.Join(s.baseDir, domain)
}

// GetCertPath 获取证书路径
func (s *FileStorage) GetCertPath(domain string) string {
	return filepath.Join(s.baseDir, domain, "cert.pem")
}

// GetKeyPath 获取私钥路径
func (s *FileStorage) GetKeyPath(domain string) string {
	return filepath.Join(s.baseDir, domain, "key.pem")
}

// GetFullchainPath 获取完整证书链路径
func (s *FileStorage) GetFullchainPath(domain string) string {
	return filepath.Join(s.baseDir, domain, "fullchain.pem")
}
