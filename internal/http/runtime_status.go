package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerRuntimeStatusRoutes(mux *http.ServeMux, platform *service.Platform, cfg config.Config) {
	mux.HandleFunc("/api/v1/runtime/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		snapshot, err := platform.RuntimeStatusSnapshot(runtimeStatusServiceName(cfg), time.Now().UTC())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})
}

func runtimeStatusServiceName(cfg config.Config) string {
	if role := strings.TrimSpace(cfg.ProcessRole); role != "" {
		return role
	}
	if app := strings.TrimSpace(cfg.AppName); app != "" {
		return app
	}
	return "platform-api"
}
