package app

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/config"
	apihttp "github.com/wuyaocheng/bktrader/internal/http"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func NewServer(cfg config.Config) *http.Server {
	store := memory.NewStore()
	platform := service.NewPlatform(store)

	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: apihttp.NewRouter(cfg, platform),
	}
}
