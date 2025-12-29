package daemon

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

// SignalHandler 信号处理器
type SignalHandler struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSignalHandler 创建信号处理器
func NewSignalHandler() *SignalHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &SignalHandler{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Context 返回可取消的 context
func (h *SignalHandler) Context() context.Context {
	return h.ctx
}

// Start 开始监听信号
func (h *SignalHandler) Start() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		log.Printf("收到信号 %v，正在优雅关闭...", sig)
		h.cancel()
	}()
}
