package http

import (
	"net/http"

	"github.com/wuyaocheng/bktrader/internal/service"
)

func registerAccountRoutes(mux *http.ServeMux, platform *service.Platform) {
	mux.HandleFunc("/api/v1/accounts", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, platform.ListAccounts())
		case http.MethodPost:
			var payload struct {
				Name     string `json:"name"`
				Mode     string `json:"mode"`
				Exchange string `json:"exchange"`
			}
			if err := decodeJSON(r, &payload); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			if err := service.ValidateRequired(map[string]string{
				"name":     payload.Name,
				"mode":     payload.Mode,
				"exchange": payload.Exchange,
			}, "name", "mode", "exchange"); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, platform.CreateAccount(payload.Name, payload.Mode, payload.Exchange))
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/positions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, platform.ListPositions())
	})
}
