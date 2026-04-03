package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerStrategyRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/strategies", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.ListStrategies())
		case http.MethodPost:
			var payload struct {
				Name        string         `json:"name"`
				Description string         `json:"description"`
				Parameters  map[string]any `json:"parameters"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{"name": payload.Name}, "name"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, platform.CreateStrategy(payload.Name, payload.Description, payload.Parameters))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}
