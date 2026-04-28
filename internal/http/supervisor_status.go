package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerSupervisorStatusRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/supervisor/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		snapshot, ok := platform.RuntimeSupervisorSnapshot()
		if !ok {
			writeError(w, http.StatusNotFound, "runtime supervisor is not configured")
			return
		}
		writeJSON(w, http.StatusOK, snapshot)
	})
}
