package http

import (
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
)

type authClaims struct {
	Username string `json:"username"`
	Scope    string `json:"scope,omitempty"`
	Jti      string `json:"jti,omitempty"`
	IssuedAt int64  `json:"iat"`
	Expiry   int64  `json:"exp"`
}

func registerAuthRoutes(mux *http.ServeMux, cfg config.Config) {
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := decodeJSON(r, &payload); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := validateCredentials(cfg, payload.Username, payload.Password); err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		token, expiresAt, err := issueToken(cfg, strings.TrimSpace(payload.Username), "api", "", time.Duration(cfg.AuthTokenTTLMinutes)*time.Minute)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"token":       token,
			"tokenType":   "Bearer",
			"expiresAt":   expiresAt.UTC().Format(time.RFC3339),
			"username":    strings.TrimSpace(payload.Username),
			"environment": cfg.Environment,
		})
	})

	mux.HandleFunc("/api/v1/auth/stream-token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		claims, ok := authClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		jti := fmt.Sprintf("%d-%s", time.Now().UnixNano(), claims.Username)
		token, expiresAt, err := issueToken(cfg, claims.Username, "dashboard_stream", jti, 1*time.Minute)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"token":     token,
			"tokenType": "Bearer",
			"expiresAt": expiresAt.UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		claims, ok := authClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"username":  claims.Username,
			"expiresAt": time.Unix(claims.Expiry, 0).UTC().Format(time.RFC3339),
		})
	})
}

func validateCredentials(cfg config.Config, username, password string) error {
	if strings.TrimSpace(username) == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}
	if cfg.AuthUsername == "" || cfg.AuthPassword == "" {
		return fmt.Errorf("server auth is not configured")
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(username)), []byte(cfg.AuthUsername)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(cfg.AuthPassword)) != 1 {
		return fmt.Errorf("invalid username or password")
	}
	return nil
}

func issueToken(cfg config.Config, username, scope, jti string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(ttl)
	claims := authClaims{
		Username: username,
		Scope:    scope,
		Jti:      jti,
		IssuedAt: now.Unix(),
		Expiry:   expiresAt.Unix(),
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}
	signature := signToken(cfg.AuthSecret, payload)
	token := base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
	return token, expiresAt, nil
}

func parseAuthToken(cfg config.Config, token string) (authClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return authClaims{}, fmt.Errorf("invalid token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return authClaims{}, fmt.Errorf("invalid token")
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return authClaims{}, fmt.Errorf("invalid token")
	}
	expected := signToken(cfg.AuthSecret, payload)
	if subtle.ConstantTimeCompare(signature, expected) != 1 {
		return authClaims{}, fmt.Errorf("invalid token")
	}
	var claims authClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return authClaims{}, fmt.Errorf("invalid token")
	}
	if claims.Username == "" || claims.Expiry <= 0 {
		return authClaims{}, fmt.Errorf("invalid token")
	}
	if time.Now().UTC().Unix() > claims.Expiry {
		return authClaims{}, fmt.Errorf("token expired")
	}
	return claims, nil
}
