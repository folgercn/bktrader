package app

import (
	"fmt"
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/config"
	apihttp "github.com/wuyaocheng/bktrader/internal/http"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
	"github.com/wuyaocheng/bktrader/internal/store/postgres"
)

// NewServer 根据配置创建 HTTP 服务实例，初始化存储层和平台服务。
func NewServer(cfg config.Config) (*http.Server, error) {
	repository, err := buildRepository(cfg)
	if err != nil {
		return nil, err
	}

	platform := service.NewPlatform(repository)
	// 设置模拟盘 Ticker 间隔（来自配置）
	platform.SetTickInterval(cfg.PaperTickInterval)
	platform.SetBacktestDataDirs(cfg.MinuteDataDir, cfg.TickDataDir)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: apihttp.NewRouter(cfg, platform),
	}, nil
}

// buildRepository 根据配置选择并初始化存储后端。
func buildRepository(cfg config.Config) (store.Repository, error) {
	switch cfg.StoreBackend {
	case "postgres":
		if cfg.AutoMigrate {
			if err := postgres.Migrate(cfg.PostgresDSN); err != nil {
				return nil, err
			}
		}
		return postgres.New(cfg.PostgresDSN)
	case "memory", "":
		return memory.NewStore(), nil
	default:
		return nil, fmt.Errorf("不支持的存储后端: %s", cfg.StoreBackend)
	}
}
