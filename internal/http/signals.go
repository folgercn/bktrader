package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerSignalRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/signal-sources", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, platform.SignalSources())
	})
}
