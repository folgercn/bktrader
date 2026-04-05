package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	binanceFuturesWSURL = "wss://fstream.binance.com/ws"
	okxPublicWSURL      = "wss://ws.okx.com:8443/ws/v5/public"
)

func (p *Platform) runSignalRuntimeLoop(ctx context.Context, sessionID string) {
	session, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return
	}

	switch session.RuntimeAdapter {
	case "binance-market-ws":
		p.runExchangeWebsocketLoop(ctx, session, binanceFuturesWSURL, buildBinanceSubscribePayload)
	case "okx-market-ws":
		p.runExchangeWebsocketLoop(ctx, session, okxPublicWSURL, buildOKXSubscribePayload)
	default:
		_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
			session.Status = "ERROR"
			state := cloneMetadata(session.State)
			state["health"] = "error"
			state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
			state["lastEventSummary"] = map[string]any{
				"type":    "runtime_error",
				"message": "unsupported runtime adapter: " + session.RuntimeAdapter,
			}
			appendSignalRuntimeError(state, fmt.Sprintf("unsupported runtime adapter: %s", session.RuntimeAdapter))
			session.State = state
			session.UpdatedAt = time.Now().UTC()
		})
	}
}

func (p *Platform) runExchangeWebsocketLoop(
	ctx context.Context,
	session domain.SignalRuntimeSession,
	wsURL string,
	subscribeBuilder func([]map[string]any) (map[string]any, error),
) {
	subscriptions := metadataList(session.State["subscriptions"])
	if len(subscriptions) == 0 {
		_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
			session.Status = "ERROR"
			state := cloneMetadata(session.State)
			state["health"] = "error"
			state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
			state["lastEventSummary"] = map[string]any{
				"type":    "runtime_error",
				"message": "no subscriptions to start",
			}
			appendSignalRuntimeError(state, "no subscriptions to start")
			session.State = state
			session.UpdatedAt = time.Now().UTC()
		})
		return
	}

	payload, err := subscribeBuilder(subscriptions)
	if err != nil {
		_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
			session.Status = "ERROR"
			state := cloneMetadata(session.State)
			state["health"] = "error"
			state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
			state["lastEventSummary"] = map[string]any{
				"type":    "runtime_error",
				"message": err.Error(),
			}
			appendSignalRuntimeError(state, err.Error())
			session.State = state
			session.UpdatedAt = time.Now().UTC()
		})
		return
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
			session.Status = "ERROR"
			state := cloneMetadata(session.State)
			state["health"] = "error"
			state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
			state["lastEventSummary"] = map[string]any{
				"type":    "dial_error",
				"message": err.Error(),
				"url":     wsURL,
			}
			appendSignalRuntimeError(state, err.Error())
			session.State = state
			session.UpdatedAt = time.Now().UTC()
		})
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(_ string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		now := time.Now().UTC()
		_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
			state := cloneMetadata(session.State)
			state["health"] = "healthy"
			state["lastHeartbeatAt"] = now.Format(time.RFC3339)
			session.State = state
			session.UpdatedAt = now
		})
		return nil
	})

	if err := conn.WriteJSON(payload); err != nil {
		_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
			session.Status = "ERROR"
			state := cloneMetadata(session.State)
			state["health"] = "error"
			state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
			state["lastEventSummary"] = map[string]any{
				"type":    "subscribe_error",
				"message": err.Error(),
			}
			appendSignalRuntimeError(state, err.Error())
			session.State = state
			session.UpdatedAt = time.Now().UTC()
		})
		return
	}

	now := time.Now().UTC()
	_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
		session.Status = "RUNNING"
		state := cloneMetadata(session.State)
		state["health"] = "healthy"
		state["connectedAt"] = now.Format(time.RFC3339)
		state["wsURL"] = wsURL
		state["lastHeartbeatAt"] = now.Format(time.RFC3339)
		state["lastEventAt"] = now.Format(time.RFC3339)
		state["lastEventSummary"] = map[string]any{
			"type":              "subscribed",
			"message":           "websocket subscribed",
			"subscriptionCount": len(subscriptions),
			"url":               wsURL,
		}
		session.State = state
		session.UpdatedAt = now
	})

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	done := make(chan error, 1)
	go func() {
		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				done <- err
				return
			}
			if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
				continue
			}
			now := time.Now().UTC()
			_ = conn.SetReadDeadline(now.Add(60 * time.Second))
			_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
				state := cloneMetadata(session.State)
				state["health"] = "healthy"
				state["lastHeartbeatAt"] = now.Format(time.RFC3339)
				state["lastEventAt"] = now.Format(time.RFC3339)
				state["lastEventSummary"] = summarizeSignalMessage(session.RuntimeAdapter, payload)
				session.State = state
				session.UpdatedAt = now
			})
		}
	}()

	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session stopped"), time.Now().Add(2*time.Second))
			_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
				session.Status = "STOPPED"
				state := cloneMetadata(session.State)
				state["health"] = "stopped"
				state["stoppedAt"] = time.Now().UTC().Format(time.RFC3339)
				state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
				state["lastEventSummary"] = map[string]any{
					"type":    "runtime_stopped",
					"message": "signal runtime stopped",
				}
				session.State = state
				session.UpdatedAt = time.Now().UTC()
			})
			p.removeSignalRuntimeRunner(session.ID)
			return
		case err := <-done:
			_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
				session.Status = "ERROR"
				state := cloneMetadata(session.State)
				state["health"] = "error"
				state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
				state["lastEventSummary"] = map[string]any{
					"type":    "runtime_error",
					"message": err.Error(),
				}
				appendSignalRuntimeError(state, err.Error())
				session.State = state
				session.UpdatedAt = time.Now().UTC()
			})
			p.removeSignalRuntimeRunner(session.ID)
			return
		case <-ticker.C:
			_ = conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(5*time.Second))
			now := time.Now().UTC()
			_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
				state := cloneMetadata(session.State)
				state["lastHeartbeatAt"] = now.Format(time.RFC3339)
				session.State = state
				session.UpdatedAt = now
			})
		}
	}
}

func buildBinanceSubscribePayload(subscriptions []map[string]any) (map[string]any, error) {
	params := make([]string, 0, len(subscriptions))
	for _, item := range subscriptions {
		channel := strings.TrimSpace(stringValue(item["channel"]))
		if channel == "" {
			return nil, fmt.Errorf("binance subscription channel is required")
		}
		params = append(params, channel)
	}
	return map[string]any{
		"method": "SUBSCRIBE",
		"params": params,
		"id":     time.Now().UnixNano(),
	}, nil
}

func buildOKXSubscribePayload(subscriptions []map[string]any) (map[string]any, error) {
	args := make([]map[string]any, 0, len(subscriptions))
	for _, item := range subscriptions {
		channel := strings.TrimSpace(stringValue(item["channel"]))
		symbol := strings.TrimSpace(stringValue(item["symbol"]))
		if channel == "" || symbol == "" {
			return nil, fmt.Errorf("okx subscription requires channel and symbol")
		}
		channelName := channel
		instID := symbol
		if idx := strings.Index(channel, ":"); idx > 0 {
			channelName = channel[:idx]
			instID = channel[idx+1:]
		}
		args = append(args, map[string]any{
			"channel": channelName,
			"instId":  instID,
		})
	}
	return map[string]any{
		"op":   "subscribe",
		"args": args,
	}, nil
}

func summarizeSignalMessage(adapterKey string, payload []byte) map[string]any {
	summary := map[string]any{
		"type":    "message",
		"adapter": adapterKey,
		"size":    len(payload),
	}
	var body map[string]any
	if err := json.Unmarshal(payload, &body); err != nil {
		summary["message"] = truncateSignalMessage(payload)
		return summary
	}

	switch adapterKey {
	case "binance-market-ws":
		summary["event"] = firstNonEmpty(stringValue(body["e"]), stringValue(body["result"]), "message")
		summary["symbol"] = stringValue(body["s"])
		summary["price"] = firstNonEmpty(stringValue(body["p"]), stringValue(body["a"]), stringValue(body["b"]))
	case "okx-market-ws":
		if arg := metadataValue(body["arg"]); arg != nil {
			summary["channel"] = stringValue(arg["channel"])
			summary["symbol"] = stringValue(arg["instId"])
		}
		if data, ok := body["data"].([]any); ok && len(data) > 0 {
			if first, ok := data[0].(map[string]any); ok {
				summary["price"] = firstNonEmpty(stringValue(first["px"]), stringValue(first["askPx"]), stringValue(first["bidPx"]))
			}
		}
		summary["event"] = firstNonEmpty(stringValue(body["event"]), stringValue(body["op"]), "message")
	default:
		summary["message"] = truncateSignalMessage(payload)
	}
	return summary
}

func truncateSignalMessage(payload []byte) string {
	text := strings.TrimSpace(string(payload))
	if len(text) <= 180 {
		return text
	}
	return text[:180] + "..."
}

func appendSignalRuntimeError(state map[string]any, message string) {
	errors := make([]any, 0)
	switch items := state["errors"].(type) {
	case []any:
		errors = append(errors, items...)
	}
	errors = append(errors, map[string]any{
		"time":    time.Now().UTC().Format(time.RFC3339),
		"message": message,
	})
	if len(errors) > 20 {
		errors = errors[len(errors)-20:]
	}
	state["errors"] = errors
}
