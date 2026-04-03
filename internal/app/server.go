package app

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/config"
	apihttp "github.com/wuyaocheng/bktrader/internal/http"
)

func NewServer(cfg config.Config) *http.Server {
	return &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: apihttp.NewRouter(cfg),
	}
}
