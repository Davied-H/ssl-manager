package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"ssl-manager/internal/config"
	"ssl-manager/internal/core"
	"ssl-manager/internal/daemon"
)

func printUsage() {
	fmt.Println(`SSL证书自动管理工具 (支持阿里云、腾讯云、华为云)

用法:
  ssl-manager [config.yaml]                                    # 检查并申请证书（单次运行）
  ssl-manager [config.yaml] start                              # 启动守护进程（后台运行）
  ssl-manager [config.yaml] stop                               # 停止守护进程
  ssl-manager [config.yaml] restart                            # 重启守护进程
  ssl-manager [config.yaml] status                             # 查看运行状态
  ssl-manager [config.yaml] daemon                             # 前台守护进程模式（调试用）
  ssl-manager [config.yaml] continue <订单ID> <域名> [提供商]  # 继续处理已有订单

示例:
  ssl-manager                          # 使用默认配置，单次运行
  ssl-manager config.yaml start        # 后台启动守护进程
  ssl-manager config.yaml stop         # 停止守护进程
  ssl-manager config.yaml status       # 查看运行状态

支持的云平台:
  - aliyun   阿里云 (默认)
  - tencent  腾讯云
  - huawei   华为云

配置文件示例:
  providers:
    aliyun:
      access_key_id: "xxx"
      access_key_secret: "xxx"
      region: "cn-hangzhou"
    tencent:
      secret_id: "xxx"
      secret_key: "xxx"
      region: "ap-guangzhou"

  domains:
    - domain: "example.com"
      provider: "aliyun"       # 证书和DNS使用同一平台
      renew_days: 7
    - domain: "mixed.com"
      cert_provider: "tencent" # 证书和DNS使用不同平台
      dns_provider: "aliyun"
      renew_days: 10

  output_dir: "./certs"
  check_interval: 24`)
}

func main() {
	configPath := "config.yaml"
	if len(os.Args) > 1 {
		if os.Args[1] == "-h" || os.Args[1] == "--help" {
			printUsage()
			return
		}
		configPath = os.Args[1]
	}

	// 获取命令
	command := ""
	if len(os.Args) > 2 {
		command = os.Args[2]
	}

	// 处理守护进程命令
	switch command {
	case "start":
		handleStart(configPath)
		return
	case "stop":
		handleStop(configPath)
		return
	case "restart":
		handleRestart(configPath)
		return
	case "status":
		handleStatus(configPath)
		return
	case "daemon":
		runDaemonForeground(configPath)
		return
	case "continue":
		handleContinue(configPath)
		return
	}

	// 默认：单次运行
	runOnce(configPath)
}

func handleStart(configPath string) {
	d := daemon.NewDaemon(configPath)

	// 启动守护进程（如果不是已经后台化的进程，会启动子进程并返回）
	if err := d.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	// 如果是后台化的子进程，继续执行守护逻辑
	if daemon.IsDaemonized() {
		runDaemonBackground(configPath, d)
	}
}

func handleStop(configPath string) {
	d := daemon.NewDaemon(configPath)
	if err := d.Stop(); err != nil {
		log.Fatalf("停止失败: %v", err)
	}
}

func handleRestart(configPath string) {
	d := daemon.NewDaemon(configPath)
	if err := d.Restart(); err != nil {
		log.Fatalf("重启失败: %v", err)
	}
}

func handleStatus(configPath string) {
	d := daemon.NewDaemon(configPath)
	d.Status()
}

func runDaemonBackground(configPath string, d *daemon.Daemon) {
	// 写入 PID
	if err := d.WritePid(); err != nil {
		log.Fatalf("写入PID失败: %v", err)
	}
	defer d.RemovePid()

	// 信号处理
	sigHandler := daemon.NewSignalHandler()
	sigHandler.Start()

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建管理器
	manager, err := core.NewManager(cfg)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	log.Printf("守护进程已启动，PID: %d，检查间隔: %d 小时", os.Getpid(), cfg.CheckInterval)

	ctx := sigHandler.Context()

	// 立即执行一次
	if err := manager.Run(ctx); err != nil {
		log.Printf("运行出错: %v", err)
	}

	// 主循环
	ticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("守护进程正在退出...")
			return
		case <-ticker.C:
			log.Printf("开始定时检查...")
			if err := manager.Run(ctx); err != nil {
				log.Printf("运行出错: %v", err)
			}
		}
	}
}

func runDaemonForeground(configPath string) {
	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建管理器
	manager, err := core.NewManager(cfg)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 信号处理
	sigHandler := daemon.NewSignalHandler()
	sigHandler.Start()

	ctx := sigHandler.Context()

	log.Printf("启动前台守护进程模式，检查间隔: %d 小时", cfg.CheckInterval)

	// 立即执行一次
	if err := manager.Run(ctx); err != nil {
		log.Printf("运行出错: %v", err)
	}

	// 主循环
	ticker := time.NewTicker(time.Duration(cfg.CheckInterval) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("收到退出信号，正在退出...")
			return
		case <-ticker.C:
			log.Printf("开始定时检查...")
			if err := manager.Run(ctx); err != nil {
				log.Printf("运行出错: %v", err)
			}
		}
	}
}

func handleContinue(configPath string) {
	if len(os.Args) < 5 {
		log.Fatalf("用法: ssl-manager [config.yaml] continue <订单ID> <域名> [提供商]")
	}

	orderID := os.Args[3]
	domain := os.Args[4]

	// 默认使用阿里云
	certProvider := "aliyun"
	dnsProvider := "aliyun"
	if len(os.Args) > 5 {
		certProvider = os.Args[5]
		dnsProvider = os.Args[5]
	}

	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建管理器
	manager, err := core.NewManager(cfg)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 信号处理
	sigHandler := daemon.NewSignalHandler()
	sigHandler.Start()

	ctx := sigHandler.Context()

	if err := manager.ContinueOrder(ctx, orderID, domain, certProvider, dnsProvider); err != nil {
		log.Fatalf("处理订单失败: %v", err)
	}
}

func runOnce(configPath string) {
	// 加载配置
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 创建管理器
	manager, err := core.NewManager(cfg)
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	// 信号处理
	sigHandler := daemon.NewSignalHandler()
	sigHandler.Start()

	ctx := sigHandler.Context()

	// 单次运行
	if err := manager.Run(ctx); err != nil {
		log.Fatalf("运行出错: %v", err)
	}
}
