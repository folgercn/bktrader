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
	return NewServerWithRuntimeOptions(cfg, RuntimeOptionsForRole(cfg.ProcessRole))
}

type RuntimeOptions struct {
	WarmLiveMarketData             bool
	StartTelegram                  bool
	RecoverLiveTrading             bool
	StartLiveSync                  bool
	StartDashboard                 bool
	StartRuntimeEventConsumer      bool
	StartSignalRuntimeScanner      bool
	StartLiveSessionControlScanner bool
	StartReadOnlyRuntimeSupervisor bool
}

func RuntimeOptionsForRole(role string) RuntimeOptions {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "api":
		return RuntimeOptions{StartDashboard: true}
	case "live-runner":
		return RuntimeOptions{
			RecoverLiveTrading:             true,
			StartLiveSync:                  true,
			StartRuntimeEventConsumer:      true,
			StartLiveSessionControlScanner: true,
		}
	case "signal-runtime-runner":
		return RuntimeOptions{
			WarmLiveMarketData:        true,
			StartSignalRuntimeScanner: true,
		}
	case "notification-worker":
		return RuntimeOptions{StartTelegram: true}
	case "supervisor":
		return RuntimeOptions{StartReadOnlyRuntimeSupervisor: true}
	default:
		return RuntimeOptions{
			WarmLiveMarketData:             true,
			StartTelegram:                  true,
			RecoverLiveTrading:             true,
			StartLiveSync:                  true,
			StartDashboard:                 true,
			StartRuntimeEventConsumer:      true,
			StartSignalRuntimeScanner:      true,
			StartLiveSessionControlScanner: true,
		}
	}
}

// NewServerWithRuntimeOptions 创建 HTTP 服务，并按进程角色启动后台组件。
func NewServerWithRuntimeOptions(cfg config.Config, runtime RuntimeOptions) (*http.Server, error) {
	logger := slog.Default().With("component", "app.server")
	logger.Info("initializing server",
		"store_backend", cfg.StoreBackend,
		"auto_migrate", cfg.AutoMigrate,
		"paper_tick_interval_seconds", cfg.PaperTickInterval,
		"process_role", cfg.ProcessRole,
	)

	platform, err := NewPlatform(cfg)
	if err != nil {
		return nil, err
	}
	StartRuntimeComponents(context.Background(), platform, cfg, runtime)
	logger.Info("background workers configured",
		"warm_live_market_data", runtime.WarmLiveMarketData,
		"telegram", runtime.StartTelegram,
		"live_recovery", runtime.RecoverLiveTrading,
		"live_sync", runtime.StartLiveSync,
		"dashboard", runtime.StartDashboard,
		"runtime_event_consumer", runtime.StartRuntimeEventConsumer,
		"signal_runtime_scanner", runtime.StartSignalRuntimeScanner,
		"live_session_control_scanner", runtime.StartLiveSessionControlScanner,
		"read_only_runtime_supervisor", runtime.StartReadOnlyRuntimeSupervisor,
	)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: apihttp.NewRouter(cfg, platform),
	}, nil
}

// NewPlatform initializes the shared service facade without starting HTTP.
func NewPlatform(cfg config.Config) (*service.Platform, error) {
	logger := slog.Default().With("component", "app.platform")
	repository, err := buildRepository(cfg)
	if err != nil {
		logger.Error("build repository failed", "error", err)
		return nil, err
	}

	platform := service.NewPlatform(repository)
	platform.SetProcessRole(cfg.ProcessRole)
	platform.SetTickInterval(cfg.PaperTickInterval)
	platform.SetBacktestDataDirs(cfg.MinuteDataDir, cfg.TickDataDir)
	platform.SetTelegramConfig(domain.TelegramConfig{
		Enabled:    cfg.TelegramEnabled,
		BotToken:   cfg.TelegramBotToken,
		ChatID:     cfg.TelegramChatID,
		SendLevels: strings.Split(cfg.TelegramSendLevels, ","),
	})
	if strings.EqualFold(strings.TrimSpace(cfg.RuntimeEventBus), "nats") {
		publisher, err := service.NewNATSRuntimeEventPublisher(cfg.NATSURL)
		if err != nil {
			logger.Warn("runtime event bus unavailable; continuing without JetStream publish", "error", err)
		} else {
			platform.SetRuntimeEventPublisher(publisher)
			logger.Info("runtime event bus publisher configured",
				"stream", service.RuntimeEventStreamName,
				"subject_pattern", service.RuntimeEventSubjectPattern,
			)
		}
	}
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
	return platform, nil
}

func StartRuntimeComponents(ctx context.Context, platform *service.Platform, cfg config.Config, runtime RuntimeOptions) {
	logger := slog.Default().With("component", "app.runtime", "process_role", cfg.ProcessRole)
	if runtime.WarmLiveMarketData {
		warmCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		if err := platform.WarmLiveMarketData(warmCtx); err != nil {
			logger.Warn("warm live market data completed with errors", "error", err)
		} else {
			logger.Info("live market data warmed successfully")
		}
		cancel()
	}
	if runtime.StartTelegram {
		platform.StartTelegramDispatcher(ctx)
	}
	if runtime.RecoverLiveTrading {
		go platform.RecoverLiveTradingOnStartup(ctx)
	}
	if runtime.StartLiveSync {
		go platform.StartLiveSyncDispatcher(ctx)
	}
	if runtime.StartDashboard {
		platform.StartDashboardBroker(ctx, cfg)
	}
	if runtime.StartRuntimeEventConsumer && strings.EqualFold(strings.TrimSpace(cfg.RuntimeEventBus), "nats") {
		consumer, err := service.NewNATSRuntimeEventConsumer(cfg.NATSURL, platform)
		if err != nil {
			logger.Warn("runtime event consumer unavailable; continuing without JetStream consume", "error", err)
			return
		}
		if err := consumer.Start(ctx); err != nil {
			consumer.Close()
			logger.Warn("runtime event consumer failed to start", "error", err)
			return
		}
		platform.SetRuntimeEventConsumerEnabled(true)
	}
	if runtime.StartSignalRuntimeScanner {
		platform.StartSignalRuntimeScanner(ctx)
	}
	if runtime.StartLiveSessionControlScanner {
		platform.StartLiveSessionControlScanner(ctx)
	}
	if runtime.StartReadOnlyRuntimeSupervisor {
		targets := service.ParseRuntimeSupervisorTargets(cfg.SupervisorTargets, cfg.SupervisorBearerToken)
		if len(targets) == 0 {
			logger.Warn("read-only runtime supervisor disabled because SUPERVISOR_TARGETS is empty")
		} else {
			supervisor := service.NewRuntimeSupervisor(targets, &http.Client{Timeout: time.Duration(cfg.SupervisorHTTPTimeoutSeconds) * time.Second})
			platform.SetRuntimeSupervisor(supervisor)
			supervisor.Start(ctx, time.Duration(cfg.SupervisorPollIntervalSeconds)*time.Second)
			logger.Info("read-only runtime supervisor started",
				"target_count", len(supervisor.Targets()),
				"poll_interval_seconds", cfg.SupervisorPollIntervalSeconds,
				"http_timeout_seconds", cfg.SupervisorHTTPTimeoutSeconds,
			)
		}
	}
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
