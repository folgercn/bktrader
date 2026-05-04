package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"net"
	"net/http"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/config"
)

type authContextKey string

const authClaimsKey authContextKey = "authClaims"

func authMiddleware(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/auth/login" {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/api/v1/stream/dashboard" {
			next.ServeHTTP(w, r)
			return
		}
		if isLoopbackRuntimeStatusRequest(cfg, r) {
			next.ServeHTTP(w, r)
			return
		}
		if !cfg.AuthEnabled {
			next.ServeHTTP(w, r)
			return
		}

		var token string
		authorization := strings.TrimSpace(r.Header.Get("Authorization"))
		if authorization != "" {
			if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}
			token = strings.TrimSpace(authorization[len("Bearer "):])
		}

		if token == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		claims, err := parseAuthToken(cfg, token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}

		ctx := context.WithValue(r.Context(), authClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func isLoopbackRuntimeStatusRequest(cfg config.Config, r *http.Request) bool {
	if !cfg.AuthEnabled || len(cfg.SupervisorTargets) == 0 {
		return false
	}
	if r == nil || r.URL == nil || r.URL.Path != "/api/v1/runtime/status" {
		return false
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		host = strings.TrimSpace(r.RemoteAddr)
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func authClaimsFromContext(ctx context.Context) (authClaims, bool) {
	claims, ok := ctx.Value(authClaimsKey).(authClaims)
	return claims, ok
}

func signToken(secret string, payload []byte) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return mac.Sum(nil)
}
