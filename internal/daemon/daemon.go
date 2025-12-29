package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	// EnvDaemonized 标记进程是否已后台化
	EnvDaemonized = "SSL_MANAGER_DAEMONIZED"
)

// Daemon 守护进程管理器
type Daemon struct {
	PidFile    string
	LogFile    string
	ConfigPath string
}

// NewDaemon 创建守护进程管理器
func NewDaemon(configPath string) *Daemon {
	dir := filepath.Dir(configPath)
	if dir == "." {
		dir, _ = os.Getwd()
	}

	return &Daemon{
		PidFile:    filepath.Join(dir, "ssl-manager.pid"),
		LogFile:    filepath.Join(dir, "ssl-manager.log"),
		ConfigPath: configPath,
	}
}

// Start 启动守护进程
func (d *Daemon) Start() error {
	// 检查是否已经在运行
	if pid, running := d.IsRunning(); running {
		return fmt.Errorf("守护进程已在运行，PID: %d", pid)
	}

	// 如果当前进程已经是守护进程，返回 nil 让调用方继续执行业务逻辑
	if IsDaemonized() {
		return nil
	}

	// 否则，启动子进程并退出
	return d.daemonize()
}

// daemonize 将进程后台化
func (d *Daemon) daemonize() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取可执行文件路径失败: %w", err)
	}

	// 打开日志文件
	logFile, err := os.OpenFile(d.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("无法打开日志文件 %s: %w", d.LogFile, err)
	}
	defer logFile.Close()

	// 构建子进程命令
	args := []string{d.ConfigPath, "start"}
	cmd := exec.Command(executable, args...)
	cmd.Env = append(os.Environ(), EnvDaemonized+"=1")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Stdin = nil

	// 设置进程属性，创建新会话
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动守护进程失败: %w", err)
	}

	fmt.Printf("守护进程已启动，PID: %d\n", cmd.Process.Pid)
	fmt.Printf("日志文件: %s\n", d.LogFile)
	fmt.Printf("PID文件: %s\n", d.PidFile)

	return nil
}

// Stop 停止守护进程
func (d *Daemon) Stop() error {
	pid, running := d.IsRunning()
	if !running {
		return fmt.Errorf("守护进程未运行")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("找不到进程 %d: %w", pid, err)
	}

	// 发送 SIGTERM 信号
	if err := process.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("发送停止信号失败: %w", err)
	}

	fmt.Printf("已发送停止信号到进程 %d\n", pid)

	// 等待进程退出
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if _, running := d.IsRunning(); !running {
			fmt.Println("守护进程已停止")
			return nil
		}
	}

	// 进程未退出，尝试强制终止
	fmt.Println("进程未响应，尝试强制终止...")
	if err := process.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("强制终止失败: %w", err)
	}

	d.RemovePid()
	fmt.Println("守护进程已强制停止")
	return nil
}

// Restart 重启守护进程
func (d *Daemon) Restart() error {
	if _, running := d.IsRunning(); running {
		if err := d.Stop(); err != nil {
			return fmt.Errorf("停止守护进程失败: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return d.Start()
}

// Status 显示守护进程状态
func (d *Daemon) Status() {
	pid, running := d.IsRunning()
	if running {
		fmt.Printf("守护进程运行中，PID: %d\n", pid)
		fmt.Printf("PID文件: %s\n", d.PidFile)
		fmt.Printf("日志文件: %s\n", d.LogFile)
	} else {
		fmt.Println("守护进程未运行")
	}
}

// IsRunning 检查守护进程是否运行
func (d *Daemon) IsRunning() (int, bool) {
	data, err := os.ReadFile(d.PidFile)
	if err != nil {
		return 0, false
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, false
	}

	// 发送信号 0 检查进程是否存在
	err = process.Signal(syscall.Signal(0))
	return pid, err == nil
}

// WritePid 写入 PID 文件
func (d *Daemon) WritePid() error {
	return os.WriteFile(d.PidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

// RemovePid 删除 PID 文件
func (d *Daemon) RemovePid() {
	os.Remove(d.PidFile)
}

// IsDaemonized 检查当前进程是否是守护进程
func IsDaemonized() bool {
	return os.Getenv(EnvDaemonized) == "1"
}
