package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/domain"
	apihttp "github.com/wuyaocheng/bktrader/internal/http"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
	"github.com/wuyaocheng/bktrader/internal/store/postgres"
)

// NewServer 根据配置创建 HTTP 服务实例，初始化存储层和平台服务。
func NewServer(cfg config.Config) (*http.Server, error) {
	logger := slog.Default().With("component", "app.server")
	logger.Info("initializing server",
		"store_backend", cfg.StoreBackend,
		"auto_migrate", cfg.AutoMigrate,
		"paper_tick_interval_seconds", cfg.PaperTickInterval,
	)

	repository, err := buildRepository(cfg)
	if err != nil {
		logger.Error("build repository failed", "error", err)
		return nil, err
	}

	platform := service.NewPlatform(repository)
	platform.SetTickInterval(cfg.PaperTickInterval)
	platform.SetBacktestDataDirs(cfg.MinuteDataDir, cfg.TickDataDir)
	platform.SetTelegramConfig(domain.TelegramConfig{
		Enabled:    cfg.TelegramEnabled,
		BotToken:   cfg.TelegramBotToken,
		ChatID:     cfg.TelegramChatID,
		SendLevels: strings.Split(cfg.TelegramSendLevels, ","),
	})
	if err := platform.LoadPersistedRuntimePolicy(); err != nil {
		logger.Error("load persisted runtime policy failed", "error", err)
		return nil, err
	}
	if err := platform.LoadPersistedTelegramConfig(); err != nil {
		logger.Error("load persisted telegram config failed", "error", err)
		return nil, err
	}
	// 关键：在加载持久化配置后，应用环境变量配置。
	// ApplyRuntimeConfigOverrides 会根据字段业务语义进行严格校验 (>0 vs >=0)，
	// 并且仅当环境变量明确设置时（指针非 nil）才执行覆盖。
	platform.ApplyRuntimeConfigOverrides(cfg)
	warmCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	if err := platform.WarmLiveMarketData(warmCtx); err != nil {
		logger.Warn("warm live market data completed with errors", "error", err)
	} else {
		logger.Info("live market data warmed successfully")
	}
	cancel()
	platform.StartTelegramDispatcher(context.Background())
	go platform.RecoverLiveTradingOnStartup(context.Background())
	go platform.StartLiveSyncDispatcher(context.Background())
	platform.StartDashboardBroker(context.Background())
	logger.Info("background workers started")

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: apihttp.NewRouter(cfg, platform),
	}, nil
}

// buildRepository 根据配置选择并初始化存储后端。
func buildRepository(cfg config.Config) (store.Repository, error) {
	logger := slog.Default().With("component", "app.repository", "store_backend", cfg.StoreBackend)

	switch cfg.StoreBackend {
	case "postgres":
		logger.Info("using postgres repository", "auto_migrate", cfg.AutoMigrate)
		if cfg.AutoMigrate {
			logger.Info("running startup migrations")
			if err := postgres.Migrate(cfg.PostgresDSN); err != nil {
				logger.Error("startup migrations failed", "error", err)
				return nil, err
			}
		}
		return postgres.New(cfg.PostgresDSN)
	case "memory", "":
		logger.Warn("using in-memory repository")
		return memory.NewStore(), nil
	default:
		return nil, fmt.Errorf("不支持的存储后端: %s", cfg.StoreBackend)
	}
}
