package core

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

// Executor 命令执行器
type Executor struct{}

// NewExecutor 创建执行器
func NewExecutor() *Executor {
	return &Executor{}
}

// RunPostCommand 执行后置命令
func (e *Executor) RunPostCommand(command string, vars map[string]string) error {
	if command == "" {
		return nil
	}

	// 替换命令中的变量
	for key, value := range vars {
		command = strings.ReplaceAll(command, "${"+key+"}", value)
	}

	log.Printf("执行后置命令: %s", command)

	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("执行命令失败: %w", err)
	}

	log.Printf("后置命令执行成功")
	return nil
}

// BuildVars 构建变量映射
func (e *Executor) BuildVars(domain, certDir, certFile, keyFile, fullchainFile string) map[string]string {
	return map[string]string{
		"DOMAIN":         domain,
		"CERT_DIR":       certDir,
		"CERT_FILE":      certFile,
		"KEY_FILE":       keyFile,
		"FULLCHAIN_FILE": fullchainFile,
	}
}
