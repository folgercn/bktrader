package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/wuyaocheng/bktrader/internal/domain"
)

// --- Tiered Disconnect Recovery ---

type disconnectSeverity int

const (
	disconnectTransient disconnectSeverity = iota // L0: safe to reconnect (timeout, EOF, reset)
	disconnectKicked                              // L1: cautious reconnect (close 1006/1008)
	disconnectFatal                               // L2: never reconnect (banned, invalid key)
)

func (s disconnectSeverity) String() string {
	switch s {
	case disconnectTransient:
		return "transient"
	case disconnectKicked:
		return "kicked"
	case disconnectFatal:
		return "fatal"
	default:
		return "unknown"
	}
}

type reconnectPolicy struct {
	maxAttempts int
	backoffs    []time.Duration
}

var (
	transientReconnectPolicy = reconnectPolicy{
		maxAttempts: 3,
		backoffs:    []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second},
	}
	kickedReconnectPolicy = reconnectPolicy{
		maxAttempts: 2,
		backoffs:    []time.Duration{30 * time.Second, 120 * time.Second},
	}
)

func classifyDisconnectSeverity(err error) disconnectSeverity {
	if err == nil {
		return disconnectFatal
	}
	msg := strings.ToLower(err.Error())

	// L2 fatal — never reconnect
	fatalPatterns := []string{
		"invalid api", "banned", "forbidden", "unauthorized",
		"403", "401", "context canceled", "context deadline exceeded",
	}
	for _, pattern := range fatalPatterns {
		if strings.Contains(msg, pattern) {
			return disconnectFatal
		}
	}

	// L1 kicked — cautious reconnect
	kickedPatterns := []string{
		"close 1006", "close 1008", "close 1001",
		"too many requests", "rate limit",
	}
	for _, pattern := range kickedPatterns {
		if strings.Contains(msg, pattern) {
			return disconnectKicked
		}
	}

	// L0 default — safe transient error
	return disconnectTransient
}

const (
	defaultBinanceFuturesWSURL = "wss://fstream.binance.com/ws"
	defaultOKXPublicWSURL      = "wss://ws.okx.com:8443/ws/v5/public"
)

func (p *Platform) runSignalRuntimeLoop(ctx context.Context, sessionID string) {
	session, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return
	}

	var wsURL string
	var subscribeBuilder func([]map[string]any) (map[string]any, error)
	switch session.RuntimeAdapter {
	case "binance-market-ws":
		wsURL = configuredBinanceFuturesWSURL()
		subscribeBuilder = buildBinanceSubscribePayload
	case "okx-market-ws":
		wsURL = configuredOKXPublicWSURL()
		subscribeBuilder = buildOKXSubscribePayload
	default:
		p.setSessionTerminalError(sessionID, fmt.Errorf("unsupported runtime adapter: %s", session.RuntimeAdapter))
		return
	}

	loopErr := p.runExchangeWebsocketLoop(ctx, session, wsURL, subscribeBuilder)
	if loopErr == nil || ctx.Err() != nil {
		p.setSessionStopped(sessionID)
		return
	}
	p.setSessionTerminalError(sessionID, loopErr)
}

// runSignalRuntimeWithRecovery wraps runSignalRuntimeLoop with tiered disconnect recovery.
// L0 transient errors (timeout, EOF): up to 3 retries with 10s/30s/60s backoff.
// L1 kicked errors (close 1006/1008): up to 2 retries with 30s/120s backoff.
// L2 fatal errors (banned, invalid API key): NO retry, immediate ERROR.
// After reconnect, validates signal bar continuity and marks stale if bars were missed.
func (p *Platform) runSignalRuntimeWithRecovery(ctx context.Context, sessionID string) {
	defer p.removeSignalRuntimeRunner(sessionID)

	for {
		// Re-read latest session each iteration (subscriptions may have changed)
		session, err := p.GetSignalRuntimeSession(sessionID)
		if err != nil {
			p.setSessionTerminalError(sessionID, err)
			return
		}

		var wsURL string
		var subscribeBuilder func([]map[string]any) (map[string]any, error)
		switch session.RuntimeAdapter {
		case "binance-market-ws":
			wsURL = configuredBinanceFuturesWSURL()
			subscribeBuilder = buildBinanceSubscribePayload
		case "okx-market-ws":
			wsURL = configuredOKXPublicWSURL()
			subscribeBuilder = buildOKXSubscribePayload
		default:
			p.setSessionTerminalError(sessionID, fmt.Errorf("unsupported runtime adapter: %s", session.RuntimeAdapter))
			return
		}

		loopErr := p.runExchangeWebsocketLoop(ctx, session, wsURL, subscribeBuilder)

		// Normal stop
		if loopErr == nil || ctx.Err() != nil {
			p.setSessionStopped(sessionID)
			return
		}

		// Classify error severity
		severity := classifyDisconnectSeverity(loopErr)
		if severity == disconnectFatal {
			p.setSessionTerminalError(sessionID, loopErr)
			return
		}

		// Select recovery policy
		policy := transientReconnectPolicy
		if severity == disconnectKicked {
			policy = kickedReconnectPolicy
		}

		p.logger("service.signal_runtime", "session_id", sessionID).Warn(
			"signal runtime disconnected, attempting recovery",
			"severity", severity.String(),
			"max_attempts", policy.maxAttempts,
			"error", loopErr.Error(),
		)

		// Retry loop
		recovered := false
		for attempt := 0; attempt < policy.maxAttempts; attempt++ {
			backoff := policy.backoffs[minInt(attempt, len(policy.backoffs)-1)]
			p.setSessionRecovering(sessionID, loopErr, attempt+1, policy.maxAttempts, backoff)

			select {
			case <-ctx.Done():
				p.setSessionStopped(sessionID)
				return
			case <-time.After(backoff):
			}

			session, err = p.GetSignalRuntimeSession(sessionID)
			if err != nil {
				continue
			}

			retryErr := p.runExchangeWebsocketLoop(ctx, session, wsURL, subscribeBuilder)
			if retryErr == nil || ctx.Err() != nil {
				p.setSessionStopped(sessionID)
				return
			}

			retrySeverity := classifyDisconnectSeverity(retryErr)
			if retrySeverity == disconnectFatal {
				p.setSessionTerminalError(sessionID, retryErr)
				return
			}

			loopErr = retryErr
		}

		if !recovered {
			p.setSessionTerminalError(sessionID, fmt.Errorf(
				"reconnect exhausted after %d attempts (severity=%s): %w",
				policy.maxAttempts, severity.String(), loopErr))
			return
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *Platform) setSessionRecovering(sessionID string, lastErr error, attempt, maxAttempts int, nextBackoff time.Duration) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		state := cloneMetadata(session.State)
		state["health"] = "recovering"
		state["lastDisconnectError"] = lastErr.Error()
		state["lastDisconnectAt"] = time.Now().UTC().Format(time.RFC3339)
		state["reconnectAttempt"] = attempt
		state["reconnectMaxAttempts"] = maxAttempts
		state["reconnectNextBackoff"] = nextBackoff.String()
		state["reconnectSeverity"] = classifyDisconnectSeverity(lastErr).String()
		appendSignalRuntimeTimeline(state, time.Now().UTC(), "runtime", "recovering", map[string]any{
			"attempt":  attempt,
			"backoff":  nextBackoff.String(),
			"error":    lastErr.Error(),
			"severity": classifyDisconnectSeverity(lastErr).String(),
		})
		session.State = state
		session.UpdatedAt = time.Now().UTC()
	})
}

func (p *Platform) setSessionTerminalError(sessionID string, err error) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		session.Status = "ERROR"
		state := cloneMetadata(session.State)
		state["health"] = "error"
		state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
		state["lastEventSummary"] = map[string]any{
			"type":    "runtime_error",
			"message": err.Error(),
		}
		delete(state, "reconnectAttempt")
		delete(state, "reconnectMaxAttempts")
		delete(state, "reconnectNextBackoff")
		delete(state, "reconnectSeverity")
		appendSignalRuntimeError(state, err.Error())
		appendSignalRuntimeTimeline(state, time.Now().UTC(), "runtime", "error", map[string]any{
			"message": err.Error(),
		})
		session.State = state
		session.UpdatedAt = time.Now().UTC()
	})
}

func (p *Platform) setSessionStopped(sessionID string) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		session.Status = "STOPPED"
		state := cloneMetadata(session.State)
		state["health"] = "stopped"
		state["stoppedAt"] = time.Now().UTC().Format(time.RFC3339)
		state["lastEventAt"] = time.Now().UTC().Format(time.RFC3339)
		state["lastEventSummary"] = map[string]any{
			"type":    "runtime_stopped",
			"message": "signal runtime stopped",
		}
		delete(state, "reconnectAttempt")
		delete(state, "reconnectMaxAttempts")
		delete(state, "reconnectNextBackoff")
		delete(state, "reconnectSeverity")
		appendSignalRuntimeTimeline(state, time.Now().UTC(), "runtime", "stopped", nil)
		session.State = state
		session.UpdatedAt = time.Now().UTC()
	})
}

func configuredBinanceFuturesWSURL() string {
	url := strings.TrimSpace(os.Getenv("BINANCE_FUTURES_WS_URL"))
	if url == "" {
		return defaultBinanceFuturesWSURL
	}
	return url
}

func configuredOKXPublicWSURL() string {
	url := strings.TrimSpace(os.Getenv("OKX_PUBLIC_WS_URL"))
	if url == "" {
		return defaultOKXPublicWSURL
	}
	return url
}

// runExchangeWebsocketLoop runs a single WebSocket connection lifecycle.
// Returns error on disconnect (caller decides whether to retry), nil on clean stop.
func (p *Platform) runExchangeWebsocketLoop(
	ctx context.Context,
	session domain.SignalRuntimeSession,
	wsURL string,
	subscribeBuilder func([]map[string]any) (map[string]any, error),
) error {
	subscriptions := metadataList(session.State["subscriptions"])
	if len(subscriptions) == 0 {
		return fmt.Errorf("no subscriptions to start")
	}

	payload, err := subscribeBuilder(subscriptions)
	if err != nil {
		return fmt.Errorf("subscribe payload build failed: %w", err)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial %s failed: %w", wsURL, err)
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
		return fmt.Errorf("subscribe write failed: %w", err)
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
		delete(state, "reconnectAttempt")
		delete(state, "reconnectMaxAttempts")
		delete(state, "reconnectNextBackoff")
		delete(state, "reconnectSeverity")
		appendSignalRuntimeTimeline(state, now, "runtime", "subscribed", map[string]any{
			"subscriptionCount": len(subscriptions),
			"url":               wsURL,
		})
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
			summary := summarizeSignalMessage(session.RuntimeAdapter, payload)
			summary = enrichSignalRuntimeSummary(session, summary)
			_ = p.ingestLiveSignalBarSummary(summary, now)
			_ = conn.SetReadDeadline(now.Add(60 * time.Second))
			_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
				state := cloneMetadata(session.State)
				state["health"] = "healthy"
				state["lastHeartbeatAt"] = now.Format(time.RFC3339)
				state["lastEventAt"] = now.Format(time.RFC3339)
				state["lastEventSummary"] = summary
				state["signalEventCount"] = maxIntValue(state["signalEventCount"], 0) + 1
				state["sourceStates"] = mergeSignalSourceState(state["sourceStates"], summary, now)
				state["signalBarStates"] = deriveSignalBarStates(mapValue(state["sourceStates"]))
				updateRuntimeHealthSummary(state, summary, now)
				appendSignalRuntimeTimeline(state, now, "market", firstNonEmpty(stringValue(summary["event"]), "message"), map[string]any{
					"symbol":    stringValue(summary["symbol"]),
					"timeframe": stringValue(summary["timeframe"]),
					"price":     stringValue(summary["price"]),
				})
				// Validate signal bar continuity after reconnect
				if stringValue(state["lastDisconnectAt"]) != "" {
					p.validateSignalBarContinuityAfterReconnect(state, summary)
				}
				session.State = state
				session.UpdatedAt = now
			})
			_ = p.handleSignalRuntimeMessage(session.ID, summary, now)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session stopped"), time.Now().Add(2*time.Second))
			return nil
		case err := <-done:
			return err
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

// validateSignalBarContinuityAfterReconnect checks if a signal bar close was missed
// during the disconnect period. If so, marks health as stale-after-reconnect.
func (p *Platform) validateSignalBarContinuityAfterReconnect(state map[string]any, summary map[string]any) {
	lastDisconnectAt := parseOptionalRFC3339(stringValue(state["lastDisconnectAt"]))
	if lastDisconnectAt.IsZero() {
		return
	}

	timeframe := stringValue(summary["timeframe"])
	if timeframe == "" {
		// Not a signal bar message — just clear recovery state
		delete(state, "lastDisconnectAt")
		delete(state, "lastDisconnectError")
		delete(state, "reconnectAttempt")
		delete(state, "reconnectMaxAttempts")
		delete(state, "reconnectNextBackoff")
		delete(state, "reconnectSeverity")
		return
	}

	barDuration := resolutionToDuration(liveSignalResolution(timeframe))
	disconnectDuration := time.Since(lastDisconnectAt)

	if barDuration > 0 && disconnectDuration > barDuration {
		// Disconnect spanned a full bar period — may have missed a close
		state["signalBarContinuityWarning"] = map[string]any{
			"disconnectDuration": disconnectDuration.String(),
			"barDuration":        barDuration.String(),
			"timeframe":          timeframe,
			"possibleMissedBars": int(disconnectDuration / barDuration),
			"detectedAt":         time.Now().UTC().Format(time.RFC3339),
		}
		state["health"] = "stale-after-reconnect"
		appendSignalRuntimeTimeline(state, time.Now().UTC(), "runtime", "stale-after-reconnect", map[string]any{
			"disconnectDuration": disconnectDuration.String(),
			"possibleMissedBars": int(disconnectDuration / barDuration),
		})
	} else {
		// Short disconnect — safe recovery
		delete(state, "signalBarContinuityWarning")
	}

	// Clear recovery tracking state
	delete(state, "lastDisconnectAt")
	delete(state, "lastDisconnectError")
	delete(state, "reconnectAttempt")
	delete(state, "reconnectMaxAttempts")
	delete(state, "reconnectNextBackoff")
	delete(state, "reconnectSeverity")
}

func (p *Platform) handleSignalRuntimeMessage(runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	if !signalRuntimeSummaryShouldTriggerLiveEvaluation(summary) {
		return nil
	}
	targetSymbol := signalRuntimeSummarySymbol(summary)
	// Reject messages with unknown symbol — never broadcast to all sessions
	if targetSymbol == "" {
		return nil
	}
	liveSessions, err := p.store.ListLiveSessions()
	if err != nil {
		return err
	}
	for _, session := range liveSessions {
		if stringValue(session.State["signalRuntimeSessionId"]) != runtimeSessionID {
			continue
		}
		if session.Status != "RUNNING" {
			continue
		}
		if !boolValue(session.State["signalRuntimeRequired"]) {
			continue
		}
		sessionSymbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
		if sessionSymbol == "" {
			continue // session has no symbol — skip
		}
		if sessionSymbol != targetSymbol {
			continue
		}
		_ = p.triggerLiveSessionFromSignal(session.ID, runtimeSessionID, summary, eventTime)
	}
	return nil
}

func signalRuntimeSummaryShouldTriggerLiveEvaluation(summary map[string]any) bool {
	role := strings.TrimSpace(stringValue(summary["role"]))
	streamType := inferSignalRuntimeStreamType(summary)
	if role != "" {
		return strings.EqualFold(normalizeSignalSourceRole(role), "trigger") &&
			(streamType == "" || streamType == "trade_tick" || streamType == "replay_tick")
	}
	return streamType == "trade_tick" || streamType == "replay_tick"
}

func signalRuntimeSummarySymbol(summary map[string]any) string {
	return NormalizeSymbol(firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"])))
}

func enrichSignalRuntimeSummary(session domain.SignalRuntimeSession, summary map[string]any) map[string]any {
	out := cloneMetadata(summary)
	subscriptions := metadataList(session.State["subscriptions"])
	if len(subscriptions) == 0 {
		return out
	}
	if len(subscriptions) == 1 {
		sub := subscriptions[0]
		subSymbol := NormalizeSymbol(stringValue(sub["symbol"]))
		msgSymbol := NormalizeSymbol(stringValue(out["symbol"]))
		// Only attach if message symbol is missing or matches subscription symbol
		if msgSymbol == "" || subSymbol == "" || msgSymbol == subSymbol {
			attachSubscriptionContext(out, sub)
		}
		return out
	}

	symbol := NormalizeSymbol(stringValue(out["symbol"]))
	streamType := inferSignalRuntimeStreamType(out)
	for _, subscription := range subscriptions {
		if !signalRuntimeSubscriptionMatchesSummary(subscription, out, symbol, streamType) {
			continue
		}
		attachSubscriptionContext(out, subscription)
		return out
	}
	return out
}

func signalRuntimeSubscriptionMatchesSummary(subscription, summary map[string]any, symbol, streamType string) bool {
	if NormalizeSymbol(stringValue(subscription["symbol"])) != symbol {
		return false
	}
	if streamType != "" && strings.TrimSpace(stringValue(subscription["streamType"])) != streamType {
		return false
	}
	if !strings.EqualFold(streamType, "signal_bar") {
		return true
	}
	eventTimeframe := normalizeSignalBarInterval(strings.TrimSpace(stringValue(summary["timeframe"])))
	if eventTimeframe == "" {
		return true
	}
	subscriptionTimeframe := signalBindingTimeframe(stringValue(subscription["sourceKey"]), metadataValue(subscription["options"]))
	return strings.EqualFold(subscriptionTimeframe, eventTimeframe)
}

func attachSubscriptionContext(summary map[string]any, subscription map[string]any) {
	summary["sourceKey"] = stringValue(subscription["sourceKey"])
	summary["role"] = stringValue(subscription["role"])
	summary["streamType"] = stringValue(subscription["streamType"])
	summary["channel"] = stringValue(subscription["channel"])
	summary["subscriptionSymbol"] = stringValue(subscription["symbol"])
	if timeframe := signalBindingTimeframe(stringValue(subscription["sourceKey"]), metadataValue(subscription["options"])); timeframe != "" {
		summary["timeframe"] = timeframe
	}
}

func inferStreamTypeFromEvent(event string) string {
	switch event {
	case "trade", "aggtrade":
		return "trade_tick"
	case "depthupdate":
		return "order_book"
	case "kline":
		return "signal_bar"
	default:
		return ""
	}
}

func inferSignalRuntimeStreamType(summary map[string]any) string {
	streamType := strings.ToLower(strings.TrimSpace(stringValue(summary["streamType"])))
	if streamType != "" {
		return streamType
	}
	if inferred := inferStreamTypeFromEvent(strings.ToLower(strings.TrimSpace(stringValue(summary["event"])))); inferred != "" {
		return inferred
	}
	channel := strings.ToLower(strings.TrimSpace(stringValue(summary["channel"])))
	switch {
	case channel == "", channel == "message":
		return ""
	case strings.HasPrefix(channel, "trades"):
		return "trade_tick"
	case strings.HasPrefix(channel, "books"), strings.HasPrefix(channel, "book"):
		return "order_book"
	case strings.HasPrefix(channel, "candle"), strings.HasPrefix(channel, "kline"):
		return "signal_bar"
	default:
		return ""
	}
}

func mergeSignalSourceState(existing any, summary map[string]any, eventTime time.Time) map[string]any {
	stateMap := map[string]any{}
	if current := mapValue(existing); current != nil {
		stateMap = cloneMetadata(current)
	}
	timeframe := signalBindingTimeframe(stringValue(summary["sourceKey"]), map[string]any{
		"timeframe": stringValue(summary["timeframe"]),
	})
	key := signalBindingMatchKey(
		stringValue(summary["sourceKey"]),
		stringValue(summary["role"]),
		firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"])),
		map[string]any{"timeframe": timeframe},
	)
	if strings.Trim(key, "|") == "" {
		key = "unknown"
	}
	existingEntry := cloneMetadata(mapValue(stateMap[key]))
	stateMap[key] = map[string]any{
		"sourceKey":   stringValue(summary["sourceKey"]),
		"role":        stringValue(summary["role"]),
		"streamType":  stringValue(summary["streamType"]),
		"symbol":      NormalizeSymbol(firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"]))),
		"timeframe":   timeframe,
		"event":       stringValue(summary["event"]),
		"lastEventAt": eventTime.UTC().Format(time.RFC3339),
		"summary":     cloneMetadata(summary),
	}
	if strings.EqualFold(stringValue(summary["streamType"]), "signal_bar") {
		entry := cloneMetadata(mapValue(stateMap[key]))
		entry["bars"] = mergeSignalBarHistory(existingEntry["bars"], summary, eventTime, 200)
		stateMap[key] = entry
	}
	return stateMap
}

func mergeSignalBarHistory(existing any, summary map[string]any, eventTime time.Time, limit int) []any {
	items := make([]map[string]any, 0)
	switch current := existing.(type) {
	case []any:
		for _, item := range current {
			if entry := mapValue(item); entry != nil {
				items = append(items, cloneMetadata(entry))
			}
		}
	case []map[string]any:
		for _, item := range current {
			items = append(items, cloneMetadata(item))
		}
	}

	bar := map[string]any{
		"timeframe": signalBindingTimeframe(stringValue(summary["sourceKey"]), map[string]any{
			"timeframe": stringValue(summary["timeframe"]),
		}),
		"symbol":    NormalizeSymbol(firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"]))),
		"barStart":  stringValue(summary["barStart"]),
		"barEnd":    stringValue(summary["barEnd"]),
		"open":      stringValue(summary["open"]),
		"high":      stringValue(summary["high"]),
		"low":       stringValue(summary["low"]),
		"close":     stringValue(summary["close"]),
		"volume":    stringValue(summary["volume"]),
		"isClosed":  summary["isClosed"],
		"updatedAt": eventTime.UTC().Format(time.RFC3339),
	}

	matchIndex := -1
	barStart := stringValue(bar["barStart"])
	timeframe := stringValue(bar["timeframe"])
	for i, item := range items {
		if stringValue(item["barStart"]) == barStart && stringValue(item["timeframe"]) == timeframe {
			matchIndex = i
			break
		}
	}
	if matchIndex >= 0 {
		items[matchIndex] = bar
	} else {
		items = append(items, bar)
	}
	if len(items) > limit {
		items = items[len(items)-limit:]
	}

	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, item)
	}
	return out
}

func deriveSignalBarStates(sourceStates map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range sourceStates {
		state := mapValue(value)
		if state == nil || !strings.EqualFold(stringValue(state["streamType"]), "signal_bar") {
			continue
		}
		bars := normalizeSignalBarEntries(state["bars"])
		if len(bars) == 0 {
			continue
		}
		current := bars[len(bars)-1]
		currentClosed := boolValue(current["isClosed"])
		closed := make([]map[string]any, 0, len(bars))
		for _, bar := range bars {
			if boolValue(bar["isClosed"]) {
				closed = append(closed, bar)
			}
		}
		indicatorBars := closed
		if !currentClosed {
			indicatorBars = append(indicatorBars, current)
		}
		if len(indicatorBars) == 0 {
			continue
		}
		closes := make([]float64, 0, len(indicatorBars))
		trueRanges := make([]float64, 0, len(indicatorBars))
		for i, bar := range indicatorBars {
			closePrice := parseFloatValue(bar["close"])
			high := parseFloatValue(bar["high"])
			low := parseFloatValue(bar["low"])
			closes = append(closes, closePrice)
			if i == 0 {
				trueRanges = append(trueRanges, high-low)
				continue
			}
			prevClose := parseFloatValue(indicatorBars[i-1]["close"])
			highLow := high - low
			highClose := math.Abs(high - prevClose)
			lowClose := math.Abs(low - prevClose)
			trueRanges = append(trueRanges, math.Max(highLow, math.Max(highClose, lowClose)))
		}

		previousClosed := closed
		if currentClosed && len(previousClosed) > 0 {
			previousClosed = previousClosed[:len(previousClosed)-1]
		}
		entry := map[string]any{
			"symbol":         stringValue(current["symbol"]),
			"timeframe":      stringValue(current["timeframe"]),
			"barCount":       len(indicatorBars),
			"closedBarCount": len(closed),
			"currentClosed":  currentClosed,
			"sma5":           rollingMean(closes, len(indicatorBars)-1, 5),
			"ma20":           rollingMean(closes, len(indicatorBars)-1, 20),
			"atr14":          rollingMean(trueRanges, len(indicatorBars)-1, 14),
			"current":        cloneMetadata(current),
		}
		if len(previousClosed) >= 1 {
			entry["prevBar1"] = cloneMetadata(previousClosed[len(previousClosed)-1])
		}
		if len(previousClosed) >= 2 {
			entry["prevBar2"] = cloneMetadata(previousClosed[len(previousClosed)-2])
		}
		out[key] = entry
	}
	return out
}

func normalizeSignalBarEntries(value any) []map[string]any {
	out := make([]map[string]any, 0)
	switch items := value.(type) {
	case []any:
		for _, item := range items {
			if entry := mapValue(item); entry != nil {
				out = append(out, cloneMetadata(entry))
			}
		}
	case []map[string]any:
		for _, item := range items {
			out = append(out, cloneMetadata(item))
		}
	}
	return out
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
		if kline := metadataValue(body["k"]); kline != nil {
			summary["event"] = "kline"
			summary["symbol"] = firstNonEmpty(stringValue(kline["s"]), stringValue(body["s"]))
			summary["timeframe"] = strings.ToLower(strings.TrimSpace(stringValue(kline["i"])))
			summary["barStart"] = stringValue(kline["t"])
			summary["barEnd"] = stringValue(kline["T"])
			summary["open"] = stringValue(kline["o"])
			summary["high"] = stringValue(kline["h"])
			summary["low"] = stringValue(kline["l"])
			summary["close"] = stringValue(kline["c"])
			summary["volume"] = stringValue(kline["v"])
			summary["isClosed"] = kline["x"]
			summary["price"] = firstNonEmpty(stringValue(kline["c"]), stringValue(summary["price"]))
		}
		if bids, ok := body["b"].([]any); ok && len(bids) > 0 {
			if first, ok := bids[0].([]any); ok && len(first) >= 2 {
				summary["bestBid"] = stringValue(first[0])
				summary["bestBidQty"] = stringValue(first[1])
				summary["event"] = firstNonEmpty(stringValue(body["e"]), "depthUpdate")
			}
		}
		if asks, ok := body["a"].([]any); ok && len(asks) > 0 {
			if first, ok := asks[0].([]any); ok && len(first) >= 2 {
				summary["bestAsk"] = stringValue(first[0])
				summary["bestAskQty"] = stringValue(first[1])
				summary["event"] = firstNonEmpty(stringValue(body["e"]), "depthUpdate")
			}
		}
		if summary["bestBid"] != nil || summary["bestAsk"] != nil {
			summary["price"] = firstNonEmpty(stringValue(summary["bestAsk"]), stringValue(summary["bestBid"]), stringValue(summary["price"]))
		}
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

func appendSignalRuntimeTimeline(state map[string]any, ts time.Time, category, title string, metadata map[string]any) {
	items := make([]any, 0)
	switch current := state["timeline"].(type) {
	case []any:
		items = append(items, current...)
	}

	entry := map[string]any{
		"time":     ts.UTC().Format(time.RFC3339),
		"category": category,
		"title":    title,
		"metadata": cloneMetadata(metadata),
	}

	if len(items) > 0 {
		if last, ok := items[len(items)-1].(map[string]any); ok {
			if stringValue(last["category"]) == category && stringValue(last["title"]) == title {
				lastMeta := mapValue(last["metadata"])
				newMeta := metadata
				if stringValue(lastMeta["symbol"]) == stringValue(newMeta["symbol"]) &&
					stringValue(lastMeta["timeframe"]) == stringValue(newMeta["timeframe"]) &&
					stringValue(lastMeta["price"]) == stringValue(newMeta["price"]) {
					return
				}
			}
		}
	}

	items = append(items, entry)
	if len(items) > 60 {
		items = items[len(items)-60:]
	}
	state["timeline"] = items
}

// --- Symbol Isolation Helpers ---

// filterSourceStatesBySymbol returns only source state entries matching the target symbol.
// Entries without a symbol tag are kept for backward compatibility.
func filterSourceStatesBySymbol(sourceStates map[string]any, targetSymbol string) map[string]any {
	if targetSymbol == "" || len(sourceStates) == 0 {
		return sourceStates
	}
	filtered := make(map[string]any, len(sourceStates))
	for key, raw := range sourceStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		entrySymbol := NormalizeSymbol(stringValue(entry["symbol"]))
		if entrySymbol == "" || entrySymbol == targetSymbol {
			filtered[key] = raw
		}
	}
	return filtered
}

// filterSignalBarStatesBySymbol returns only signal bar state entries matching the target symbol.
func filterSignalBarStatesBySymbol(signalBarStates map[string]any, targetSymbol string) map[string]any {
	if targetSymbol == "" || len(signalBarStates) == 0 {
		return signalBarStates
	}
	filtered := make(map[string]any, len(signalBarStates))
	for key, raw := range signalBarStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		entrySymbol := NormalizeSymbol(stringValue(entry["symbol"]))
		if entrySymbol == "" || entrySymbol == targetSymbol {
			filtered[key] = raw
		}
	}
	return filtered
}
