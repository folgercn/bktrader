package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/wuyaocheng/bktrader/internal/app"
	"github.com/wuyaocheng/bktrader/internal/config"
)

func main() {
	// 加载并验证配置
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	// 创建 HTTP 服务实例
	server, err := app.NewServer(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// 启动 HTTP 服务（非阻塞）
	go func() {
		log.Printf("platform-api 正在监听 %s", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
	}()

	// 优雅关闭：监听系统信号（SIGINT / SIGTERM）
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("收到信号 %v，正在优雅关闭...", sig)

	// 给予 10 秒超时让正在处理的请求完成
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("优雅关闭失败: %v", err)
	}
	log.Println("服务已关闭")
}
