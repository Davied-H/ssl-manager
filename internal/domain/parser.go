package domain

import "strings"

// ExtractMainDomain 从完整域名提取主域名
// 例如: www.example.com -> example.com, sub.test.example.com -> example.com
func ExtractMainDomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) >= 2 {
		return parts[len(parts)-2] + "." + parts[len(parts)-1]
	}
	return domain
}

// ExtractSubDomain 提取子域名部分（用于DNS记录的RR值）
// 例如: _dnsauth.www.example.com 中提取 _dnsauth.www
func ExtractSubDomain(fullRecord, mainDomain string) string {
	if strings.HasSuffix(fullRecord, "."+mainDomain) {
		return strings.TrimSuffix(fullRecord, "."+mainDomain)
	}
	return fullRecord
}

// IsSubDomain 检查是否为子域名
func IsSubDomain(domain, mainDomain string) bool {
	return strings.HasSuffix(domain, "."+mainDomain) || domain == mainDomain
}

// MatchDomain 检查域名是否匹配（支持通配符）
func MatchDomain(certDomain, targetDomain string) bool {
	// 完全匹配
	if certDomain == targetDomain {
		return true
	}

	// 通配符匹配
	if strings.HasPrefix(certDomain, "*.") {
		mainDomain := strings.TrimPrefix(certDomain, "*.")
		targetMain := ExtractMainDomain(targetDomain)
		return mainDomain == targetMain
	}

	return false
}
