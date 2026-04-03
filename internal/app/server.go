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

func NewServer(cfg config.Config) (*http.Server, error) {
	repository, err := buildRepository(cfg)
	if err != nil {
		return nil, err
	}
	platform := service.NewPlatform(repository)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: apihttp.NewRouter(cfg, platform),
	}, nil
}

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
		return nil, fmt.Errorf("unsupported store backend: %s", cfg.StoreBackend)
	}
}
