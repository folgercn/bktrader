package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
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

func (p *Platform) transientReconnectPolicy() reconnectPolicy {
	backoffs := make([]time.Duration, 0, len(p.runtimePolicy.WSReconnectBackoffs))
	for _, b := range p.runtimePolicy.WSReconnectBackoffs {
		backoffs = append(backoffs, time.Duration(b)*time.Second)
	}
	if len(backoffs) == 0 {
		backoffs = []time.Duration{10 * time.Second, 30 * time.Second, 60 * time.Second}
	}
	return reconnectPolicy{
		maxAttempts: len(backoffs),
		backoffs:    backoffs,
	}
}

func (p *Platform) kickedReconnectPolicy() reconnectPolicy {
	backoffs := make([]time.Duration, 0, len(p.runtimePolicy.WSReconnectRecoveryBackoffs))
	for _, b := range p.runtimePolicy.WSReconnectRecoveryBackoffs {
		backoffs = append(backoffs, time.Duration(b)*time.Second)
	}
	if len(backoffs) == 0 {
		backoffs = []time.Duration{30 * time.Second, 120 * time.Second}
	}
	return reconnectPolicy{
		maxAttempts: len(backoffs),
		backoffs:    backoffs,
	}
}

func classifyDisconnectSeverity(err error) disconnectSeverity {
	if err == nil {
		return disconnectFatal
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return disconnectTransient
	}
	msg := strings.ToLower(err.Error())

	// L2 fatal — never reconnect
	fatalPatterns := []string{
		"invalid api", "banned", "forbidden", "unauthorized",
		"403", "401",
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
	defaultBinanceFuturesWSURL        = "wss://fstream.binance.com/ws"
	defaultBinanceFuturesTestnetWSURL = "wss://stream.binancefuture.com/ws"
	defaultOKXPublicWSURL             = "wss://ws.okx.com:8443/ws/v5/public"
	signalRuntimeWSHandshakeTimeout   = 5 * time.Second
	signalRuntimeWSPingInterval       = 10 * time.Second
	signalRuntimeWSPingWriteTimeout   = 3 * time.Second
	liveSignalBarHistoryLimit         = 260
)

type signalRuntimeWSProfile struct {
	readTimeout      time.Duration
	pingInterval     time.Duration
	pingWriteTimeout time.Duration
}

type signalRuntimeLoopRunner func(context.Context, string) (bool, error)
type signalRuntimeBackoffWaiter func(context.Context, time.Duration) bool

func (p *Platform) runSignalRuntimeLoop(ctx context.Context, sessionID string) {
	_, loopErr := p.runSignalRuntimeLoopOnce(ctx, sessionID)
	if loopErr == nil || signalRuntimeStopRequested(ctx, loopErr) {
		p.setSessionStopped(sessionID)
		return
	}
	p.setSessionTerminalError(sessionID, loopErr)
}

// runSignalRuntimeWithRecovery wraps runSignalRuntimeLoop with tiered disconnect recovery.
// L0 transient errors (timeout, EOF): up to 3 retries with 5s/15s/30s backoff.
// L1 kicked errors (close 1006/1008): up to 2 retries with 20s/60s backoff.
// L2 fatal errors (banned, invalid API key): NO retry, immediate ERROR.
// After reconnect, validates signal bar continuity and marks stale if bars were missed.
func (p *Platform) runSignalRuntimeWithRecovery(ctx context.Context, sessionID string) {
	p.runSignalRuntimeWithRecoveryUsing(ctx, sessionID, p.runSignalRuntimeLoopOnce, waitSignalRuntimeBackoff)
}

func (p *Platform) runSignalRuntimeWithRecoveryUsing(
	ctx context.Context,
	sessionID string,
	runLoop signalRuntimeLoopRunner,
	waitBackoff signalRuntimeBackoffWaiter,
) {
	defer p.removeSignalRuntimeRunner(sessionID)

	disconnectErr := error(nil)
	for {
		if disconnectErr == nil {
			_, loopErr := runLoop(ctx, sessionID)
			if loopErr == nil || signalRuntimeStopRequested(ctx, loopErr) {
				p.setSessionStopped(sessionID)
				return
			}
			severity := classifyDisconnectSeverity(loopErr)
			if severity == disconnectFatal {
				p.setSessionTerminalError(sessionID, loopErr)
				return
			}
			disconnectErr = loopErr
		}

		severity := classifyDisconnectSeverity(disconnectErr)
		policy := p.reconnectPolicyForSeverity(severity)
		p.logger("service.signal_runtime", "session_id", sessionID).Warn(
			"signal runtime disconnected, attempting recovery",
			"severity", severity.String(),
			"max_attempts", policy.maxAttempts,
			"error", disconnectErr.Error(),
		)
		recovered := false
		for attempt := 1; attempt <= policy.maxAttempts; attempt++ {
			backoff := recoveryBackoff(policy, attempt)
			p.setSessionRecovering(sessionID, disconnectErr, attempt, policy.maxAttempts, backoff)

			if !waitBackoff(ctx, backoff) {
				p.setSessionStopped(sessionID)
				return
			}

			connected, retryErr := runLoop(ctx, sessionID)
			if retryErr == nil || signalRuntimeStopRequested(ctx, retryErr) {
				p.setSessionStopped(sessionID)
				return
			}

			retrySeverity := classifyDisconnectSeverity(retryErr)
			if retrySeverity == disconnectFatal {
				p.setSessionTerminalError(sessionID, retryErr)
				return
			}

			if connected {
				// The reconnect succeeded and the runtime ran again before dropping later.
				// Start a fresh recovery cycle so the next disconnect gets a full retry budget.
				p.handleSignalRuntimeReconnect(sessionID, time.Now().UTC())
				disconnectErr = retryErr
				recovered = true
				break
			}

			disconnectErr = retryErr
			severity = retrySeverity
			policy = p.reconnectPolicyForSeverity(severity)
		}

		if recovered {
			continue
		}

		p.setSessionTerminalError(sessionID, fmt.Errorf(
			"reconnect exhausted after %d attempts (severity=%s): %w",
			policy.maxAttempts, severity.String(), disconnectErr))
		return
	}
}

func (p *Platform) handleSignalRuntimeReconnect(sessionID string, eventTime time.Time) {
	runtimeSession, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return
	}
	if _, reconcileErr := p.triggerAuthoritativeLiveAccountReconcile(runtimeSession.AccountID, "ws-reconnect-rest-verify-required", eventTime); reconcileErr != nil {
		p.logger("service.signal_runtime", "session_id", sessionID, "account_id", runtimeSession.AccountID).
			Warn("live account reconcile after websocket reconnect failed", "error", reconcileErr)
	}
}

func (p *Platform) reconnectPolicyForSeverity(severity disconnectSeverity) reconnectPolicy {
	switch severity {
	case disconnectKicked:
		return p.kickedReconnectPolicy()
	default:
		return p.transientReconnectPolicy()
	}
}

func recoveryBackoff(policy reconnectPolicy, attempt int) time.Duration {
	if len(policy.backoffs) == 0 {
		return 0
	}
	index := attempt - 1
	if index < 0 {
		index = 0
	}
	return policy.backoffs[minInt(index, len(policy.backoffs)-1)]
}

func (p *Platform) signalRuntimeWSProfileForAttempt(attempt int) signalRuntimeWSProfile {
	baseTimeout := time.Duration(p.runtimePolicy.WSReadStaleTimeoutSeconds) * time.Second
	if baseTimeout <= 0 {
		baseTimeout = 20 * time.Second
	}
	pingInterval := time.Duration(p.runtimePolicy.WSPingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = 20 * time.Second
	}

	profile := signalRuntimeWSProfile{
		readTimeout:      baseTimeout,
		pingInterval:     pingInterval,
		pingWriteTimeout: 5 * time.Second,
	}
	switch {
	case attempt >= 2:
		profile.readTimeout = baseTimeout + 25*time.Second
	case attempt == 1:
		profile.readTimeout = baseTimeout + 10*time.Second
	}
	return profile
}

func waitSignalRuntimeBackoff(ctx context.Context, backoff time.Duration) bool {
	if backoff <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func signalRuntimeStopRequested(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (p *Platform) runSignalRuntimeLoopOnce(ctx context.Context, sessionID string) (bool, error) {
	session, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return false, err
	}
	subscriptions := p.signalRuntimeSubscriptions(session)
	if len(subscriptions) > 0 {
		session.State = cloneMetadata(session.State)
		session.State["subscriptions"] = subscriptions
	}

	wsURL, subscribeBuilder, err := signalRuntimeWebsocketConfig(session.RuntimeAdapter, subscriptions)
	if err != nil {
		return false, err
	}

	return p.runExchangeWebsocketLoop(ctx, session, wsURL, subscribeBuilder)
}

func (p *Platform) signalRuntimeSubscriptions(session domain.SignalRuntimeSession) []map[string]any {
	subscriptions := cloneMetadataList(metadataList(session.State["subscriptions"]))
	if len(subscriptions) == 0 {
		return subscriptions
	}
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return subscriptions
	}
	liveBinding := resolveLiveBinding(account)
	for _, subscription := range subscriptions {
		applyLiveBindingToSignalRuntimeSubscription(subscription, liveBinding)
	}
	return subscriptions
}

func signalRuntimeWebsocketConfig(runtimeAdapter string, subscriptions []map[string]any) (string, func([]map[string]any) (map[string]any, error), error) {
	switch runtimeAdapter {
	case "binance-market-ws":
		wsURL, err := resolveBinanceSignalRuntimeWSURL(subscriptions)
		if err != nil {
			return "", nil, err
		}
		return wsURL, buildBinanceSubscribePayload, nil
	case "okx-market-ws":
		return configuredOKXPublicWSURL(), buildOKXSubscribePayload, nil
	default:
		return "", nil, fmt.Errorf("unsupported runtime adapter: %s", runtimeAdapter)
	}
}

func resolveBinanceSignalRuntimeWSURL(subscriptions []map[string]any) (string, error) {
	wsURL := ""
	sandboxResolved := false
	sandbox := false
	for _, subscription := range subscriptions {
		if subscription == nil {
			continue
		}
		if candidate := strings.TrimSpace(stringValue(subscription["wsBaseUrl"])); candidate != "" {
			if wsURL == "" {
				wsURL = candidate
			} else if wsURL != candidate {
				return "", fmt.Errorf("binance runtime subscriptions require a single websocket base url: %s vs %s", wsURL, candidate)
			}
		}
		subscriptionSandbox := binanceSignalRuntimeSubscriptionUsesTestnet(subscription)
		if !sandboxResolved {
			sandbox = subscriptionSandbox
			sandboxResolved = true
			continue
		}
		if sandbox != subscriptionSandbox {
			return "", fmt.Errorf("binance runtime subscriptions mix sandbox and live market data environments")
		}
	}
	if wsURL != "" {
		return wsURL, nil
	}
	if sandboxResolved && sandbox {
		return configuredBinanceFuturesWSURLWithDefault(defaultBinanceFuturesTestnetWSURL), nil
	}
	return configuredBinanceFuturesWSURL(), nil
}

func binanceSignalRuntimeSubscriptionUsesTestnet(subscription map[string]any) bool {
	if subscription == nil {
		return false
	}
	if _, exists := subscription["sandbox"]; exists && boolValue(subscription["sandbox"]) {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(stringValue(subscription["restBaseUrl"]))), "testnet.binancefuture.com")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (p *Platform) setSessionRecovering(sessionID string, lastErr error, attempt, maxAttempts int, nextBackoff time.Duration) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		now := time.Now().UTC()
		state := cloneMetadata(session.State)
		state["health"] = "recovering"
		state["lastDisconnectError"] = lastErr.Error()
		if stringValue(state["lastDisconnectAt"]) == "" {
			state["lastDisconnectAt"] = now.Format(time.RFC3339)
		}
		state["reconnectAttempt"] = attempt
		state["reconnectMaxAttempts"] = maxAttempts
		state["reconnectNextBackoff"] = nextBackoff.String()
		state["reconnectSeverity"] = classifyDisconnectSeverity(lastErr).String()
		state["reconnectAttemptStartedAtMs"] = now.UnixMilli()
		appendSignalRuntimeTimeline(state, now, "runtime", "recovering", map[string]any{
			"attempt":            attempt,
			"backoff":            nextBackoff.String(),
			"error":              lastErr.Error(),
			"severity":           classifyDisconnectSeverity(lastErr).String(),
			"attemptStartedAtMs": now.UnixMilli(),
		})
		session.State = state
		session.UpdatedAt = now
	})
}

func (p *Platform) setSessionTerminalError(sessionID string, err error) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		now := time.Now().UTC()
		session.Status = "ERROR"
		state := cloneMetadata(session.State)
		state["health"] = "error"
		state["actualStatus"] = "ERROR"
		state["lastEventAt"] = now.Format(time.RFC3339)
		state["lastEventSummary"] = map[string]any{
			"type":    "runtime_error",
			"message": err.Error(),
		}
		delete(state, "reconnectAttempt")
		delete(state, "reconnectMaxAttempts")
		delete(state, "reconnectNextBackoff")
		delete(state, "reconnectSeverity")
		delete(state, "reconnectAttemptStartedAtMs")
		scheduleSignalRuntimeSupervisorRestartAfterTerminalError(state, err, now)
		appendSignalRuntimeError(state, err.Error())
		appendSignalRuntimeTimeline(state, now, "runtime", "error", map[string]any{
			"message": err.Error(),
		})
		session.State = state
		session.UpdatedAt = now
	})
	p.clearTickEvalThrottleSession(sessionID)
	p.clearRuntimeEventPublishThrottleSession(sessionID)
}

func (p *Platform) setSessionStopped(sessionID string) {
	_ = p.updateSignalRuntimeSessionState(sessionID, func(session *domain.SignalRuntimeSession) {
		session.Status = "STOPPED"
		state := cloneMetadata(session.State)
		state["health"] = "stopped"
		state["actualStatus"] = "STOPPED"
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
		delete(state, "reconnectAttemptStartedAtMs")
		appendSignalRuntimeTimeline(state, time.Now().UTC(), "runtime", "stopped", nil)
		session.State = state
		session.UpdatedAt = time.Now().UTC()
	})
	p.clearTickEvalThrottleSession(sessionID)
	p.clearRuntimeEventPublishThrottleSession(sessionID)
}

func configuredBinanceFuturesWSURL() string {
	return configuredBinanceFuturesWSURLWithDefault(defaultBinanceFuturesWSURL)
}

func configuredBinanceFuturesWSURLWithDefault(fallback string) string {
	url := strings.TrimSpace(os.Getenv("BINANCE_FUTURES_WS_URL"))
	if url == "" {
		return fallback
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
// The boolean reports whether the websocket reached RUNNING/subscribed before exiting.
// The error reports a disconnect/failure; nil means the loop stopped cleanly.
func (p *Platform) runExchangeWebsocketLoop(
	ctx context.Context,
	session domain.SignalRuntimeSession,
	wsURL string,
	subscribeBuilder func([]map[string]any) (map[string]any, error),
) (bool, error) {
	subscriptions := metadataList(session.State["subscriptions"])
	if len(subscriptions) == 0 {
		return false, fmt.Errorf("no subscriptions to start")
	}

	payload, err := subscribeBuilder(subscriptions)
	if err != nil {
		return false, fmt.Errorf("subscribe payload build failed: %w", err)
	}
	subscriptionSummary := summarizeSubscriptions(subscriptions)
	logger := p.logger(
		"service.signal_runtime",
		"session_id", session.ID,
		"runtime_adapter", session.RuntimeAdapter,
		"subscription_count", len(subscriptions),
		"subscriptions", subscriptionSummary,
		"ws_url", wsURL,
	)
	reconnectAttempt := maxIntValue(session.State["reconnectAttempt"], 0)
	wsProfile := p.signalRuntimeWSProfileForAttempt(reconnectAttempt)
	reconnectStartedAt := parseUnixMillisTime(session.State["reconnectAttemptStartedAtMs"])
	reconnecting := !reconnectStartedAt.IsZero()

	handshakeTimeout := p.runtimePolicy.WSHandshakeTimeoutSeconds
	if handshakeTimeout <= 0 {
		handshakeTimeout = 10
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Duration(handshakeTimeout) * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return false, fmt.Errorf("dial %s failed: %w", wsURL, err)
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(wsProfile.readTimeout))
	conn.SetPongHandler(func(_ string) error {
		_ = conn.SetReadDeadline(time.Now().Add(wsProfile.readTimeout))
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
		return false, fmt.Errorf("subscribe write failed: %w", err)
	}

	now := time.Now().UTC()
	reconnectDuration := ""
	reconnectDurationMillis := int64(0)
	if reconnecting {
		reconnectDurationMillis = maxInt64(0, now.Sub(reconnectStartedAt).Milliseconds())
		reconnectDuration = (time.Duration(reconnectDurationMillis) * time.Millisecond).String()
		logger.Info("signal runtime websocket reconnected",
			"reconnect_attempt", reconnectAttempt,
			"reconnect_duration", reconnectDuration,
			"reconnect_duration_ms", reconnectDurationMillis,
			"read_timeout", wsProfile.readTimeout.String(),
			"resubscribe_result", "subscribe_request_sent",
		)
	} else {
		logger.Info("signal runtime websocket connected",
			"read_timeout", wsProfile.readTimeout.String(),
		)
	}
	_ = p.updateSignalRuntimeSessionState(session.ID, func(session *domain.SignalRuntimeSession) {
		session.Status = "RUNNING"
		state := cloneMetadata(session.State)
		state["health"] = "healthy"
		state["actualStatus"] = "RUNNING"
		state["connectedAt"] = now.Format(time.RFC3339)
		state["wsURL"] = wsURL
		state["lastHeartbeatAt"] = now.Format(time.RFC3339)
		state["lastEventAt"] = now.Format(time.RFC3339)
		state["lastEventSummary"] = map[string]any{
			"type":              "subscribed",
			"message":           "websocket subscribed",
			"subscriptionCount": len(subscriptions),
			"subscriptions":     subscriptionSummary,
			"url":               wsURL,
		}
		if reconnecting {
			state["lastReconnectAt"] = now.Format(time.RFC3339)
			state["lastReconnectDuration"] = reconnectDuration
			state["lastReconnectDurationMs"] = reconnectDurationMillis
			state["lastReconnectSubscriptions"] = subscriptionSummary
			state["lastReconnectResult"] = "subscribe_request_sent"
		}
		delete(state, "reconnectAttempt")
		delete(state, "reconnectMaxAttempts")
		delete(state, "reconnectNextBackoff")
		delete(state, "reconnectSeverity")
		delete(state, "reconnectAttemptStartedAtMs")
		ClearRestartState(state, signalRuntimeSupervisorRestartStateKeys)
		appendSignalRuntimeTimeline(state, now, "runtime", "subscribed", map[string]any{
			"subscriptionCount": len(subscriptions),
			"subscriptions":     subscriptionSummary,
			"url":               wsURL,
			"reconnecting":      reconnecting,
			"readTimeout":       wsProfile.readTimeout.String(),
			"reconnectDuration": reconnectDuration,
			"result":            "subscribe_request_sent",
		})
		session.State = state
		session.UpdatedAt = now
	})
	connected := true

	ticker := time.NewTicker(wsProfile.pingInterval)
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
			_ = conn.SetReadDeadline(now.Add(wsProfile.readTimeout))
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
				if stringValue(state["lastDisconnectAt"]) != "" {
					p.validateSignalBarContinuityAfterReconnect(state, summary)
				}
				state["tickEvalThrottledCount"] = p.tickEvalThrottleSkippedCount(session.ID)
				session.State = state
				session.UpdatedAt = now
			})
			p.publishRuntimeSignalEvent(session, summary, now)
			if err := p.handleSignalRuntimeMessageFromWebsocket(session.ID, summary, now); err != nil {
				p.logger("service.signal_runtime",
					"session_id", session.ID,
					"symbol", signalRuntimeSummarySymbol(summary),
					"stream_type", inferSignalRuntimeStreamType(summary),
				).Warn("signal runtime live fanout failed", "error", err)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			passiveTimeout := p.runtimePolicy.WSPassiveCloseTimeoutSeconds
			if passiveTimeout <= 0 {
				passiveTimeout = 2
			}
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session stopped"), time.Now().Add(time.Duration(passiveTimeout)*time.Second))
			return connected, nil
		case err := <-done:
			logger.Warn("signal runtime websocket disconnected",
				"connected", connected,
				"disconnect_severity", classifyDisconnectSeverity(err).String(),
				"error", err,
			)
			return connected, err
		case <-ticker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(wsProfile.pingWriteTimeout)); err != nil {
				logger.Warn("signal runtime websocket ping failed",
					"connected", connected,
					"error", err,
				)
				return connected, fmt.Errorf("websocket ping failed: %w", err)
			}
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
		delete(state, "reconnectAttemptStartedAtMs")
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
	delete(state, "reconnectAttemptStartedAtMs")
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// tickEvalThrottleState maintains the throttle state for strategy evaluation
type tickEvalThrottleState struct {
	mu           sync.Mutex
	lastPrice    string
	lastEvalTime time.Time
	skippedCount int64
}

const minTickEvalInterval = 100 * time.Millisecond

func tickEvalThrottleKey(runtimeSessionID string, targetSymbol string) string {
	return runtimeSessionID + "|" + strings.TrimSpace(targetSymbol)
}

func (p *Platform) tickEvalThrottleSessionState(runtimeSessionID string) *tickEvalThrottleState {
	val, _ := p.tickEvalThrottle.LoadOrStore(runtimeSessionID, &tickEvalThrottleState{})
	return val.(*tickEvalThrottleState)
}

func (p *Platform) tickEvalThrottleSkippedCount(runtimeSessionID string) int64 {
	val, ok := p.tickEvalThrottle.Load(runtimeSessionID)
	if !ok {
		return 0
	}
	state := val.(*tickEvalThrottleState)
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.skippedCount
}

func (p *Platform) incrementTickEvalThrottleSkipped(runtimeSessionID string) {
	state := p.tickEvalThrottleSessionState(runtimeSessionID)
	state.mu.Lock()
	state.skippedCount++
	state.mu.Unlock()
}

func (p *Platform) clearTickEvalThrottleSession(runtimeSessionID string) {
	prefix := runtimeSessionID + "|"
	p.tickEvalThrottle.Range(func(key, _ any) bool {
		keyStr, ok := key.(string)
		if !ok {
			return true
		}
		if keyStr == runtimeSessionID || strings.HasPrefix(keyStr, prefix) {
			p.tickEvalThrottle.Delete(keyStr)
		}
		return true
	})
}

func (p *Platform) shouldThrottleLiveEvaluation(runtimeSessionID string, summary map[string]any, eventTime time.Time, targetSymbol string) bool {
	streamType := inferSignalRuntimeStreamType(summary)
	if streamType != "trade_tick" {
		return false
	}

	price := strings.TrimSpace(stringValue(summary["price"]))
	if price == "" {
		return false
	}
	targetSymbol = strings.TrimSpace(targetSymbol)
	if targetSymbol == "" {
		return false
	}

	throttleKey := tickEvalThrottleKey(runtimeSessionID, targetSymbol)
	val, _ := p.tickEvalThrottle.LoadOrStore(throttleKey, &tickEvalThrottleState{})
	state := val.(*tickEvalThrottleState)

	state.mu.Lock()
	defer state.mu.Unlock()

	if state.lastPrice == price {
		state.skippedCount++
		p.incrementTickEvalThrottleSkipped(runtimeSessionID)
		return true
	}

	if !state.lastEvalTime.IsZero() && eventTime.Sub(state.lastEvalTime) < minTickEvalInterval {
		state.skippedCount++
		p.incrementTickEvalThrottleSkipped(runtimeSessionID)
		return true
	}

	state.lastPrice = price
	state.lastEvalTime = eventTime
	return false
}

func (p *Platform) handleSignalRuntimeMessage(runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	return p.handleSignalRuntimeMessageWithOptions(runtimeSessionID, summary, eventTime, signalRuntimeFanoutOptions{})
}

type signalRuntimeFanoutOptions struct {
	returnTriggerErrors bool
}

func (p *Platform) handleSignalRuntimeMessageWithOptions(runtimeSessionID string, summary map[string]any, eventTime time.Time, options signalRuntimeFanoutOptions) error {
	if !signalRuntimeSummaryShouldTriggerLiveEvaluation(summary) {
		return nil
	}
	targetSymbol := signalRuntimeSummarySymbol(summary)
	// Reject messages with unknown symbol — never broadcast to all sessions
	if targetSymbol == "" {
		return nil
	}
	if p.shouldThrottleLiveEvaluation(runtimeSessionID, summary, eventTime, targetSymbol) {
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
		requiredValue, hasRequired := session.State["signalRuntimeRequired"]
		if !boolValue(requiredValue) && (!hasRequired || requiredValue == nil) {
			refreshed, refreshErr := p.syncLiveSessionRuntime(session)
			if refreshErr != nil {
				p.recordLiveRuntimeFanoutDrop(session, runtimeSessionID, summary, eventTime, "runtime-state-refresh-failed", map[string]any{
					"error": refreshErr.Error(),
				})
				continue
			} else {
				session = refreshed
				p.logger("service.live",
					"session_id", session.ID,
					"runtime_session_id", runtimeSessionID,
					"symbol", targetSymbol,
				).Info("restored missing signalRuntimeRequired before live runtime fanout")
			}
		}
		if !boolValue(session.State["signalRuntimeRequired"]) {
			p.recordLiveRuntimeFanoutDrop(session, runtimeSessionID, summary, eventTime, "runtime-not-required", map[string]any{
				"signalRuntimeRequired": session.State["signalRuntimeRequired"],
			})
			continue
		}
		sessionSymbol := NormalizeSymbol(firstNonEmpty(stringValue(session.State["symbol"]), stringValue(session.State["lastSymbol"])))
		if sessionSymbol == "" {
			p.recordLiveRuntimeFanoutDrop(session, runtimeSessionID, summary, eventTime, "missing-session-symbol", nil)
			continue // session has no symbol — skip
		}
		if sessionSymbol != targetSymbol {
			continue
		}
		if err := p.triggerLiveSessionFromSignal(session.ID, runtimeSessionID, summary, eventTime); err != nil {
			p.logger("service.live",
				"session_id", session.ID,
				"runtime_session_id", runtimeSessionID,
				"symbol", targetSymbol,
			).Warn("trigger live session from signal failed", "error", err)
			if options.returnTriggerErrors {
				return err
			}
		}
	}
	return nil
}

func (p *Platform) recordLiveRuntimeFanoutDrop(
	session domain.LiveSession,
	runtimeSessionID string,
	summary map[string]any,
	eventTime time.Time,
	reason string,
	details map[string]any,
) {
	state := cloneMetadata(session.State)
	lastReason := stringValue(state["lastRuntimeFanoutDropReason"])
	lastRecordedAt := parseOptionalRFC3339(stringValue(state["lastRuntimeFanoutDropAt"]))
	if lastReason == reason && !lastRecordedAt.IsZero() && eventTime.Sub(lastRecordedAt) < 30*time.Second {
		return
	}
	state["lastRuntimeFanoutDropReason"] = reason
	state["lastRuntimeFanoutDropAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastRuntimeFanoutDropRuntimeSessionId"] = runtimeSessionID
	state["lastRuntimeFanoutDropSummary"] = cloneMetadata(summary)
	if len(details) > 0 {
		state["lastRuntimeFanoutDropDetails"] = cloneMetadata(details)
	} else {
		delete(state, "lastRuntimeFanoutDropDetails")
	}
	if updated, err := p.store.UpdateLiveSessionState(session.ID, state); err == nil {
		session = updated
	}
	p.logger("service.live",
		"session_id", session.ID,
		"runtime_session_id", runtimeSessionID,
		"reason", reason,
		"symbol", signalRuntimeSummarySymbol(summary),
	).Warn("runtime event skipped for live session fanout")
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
		"lastEventAt": eventTime.UTC().Format(time.RFC3339Nano),
		"summary":     cloneMetadata(summary),
	}
	if strings.EqualFold(stringValue(summary["streamType"]), "signal_bar") {
		entry := cloneMetadata(mapValue(stateMap[key]))
		entry["bars"] = mergeSignalBarHistory(existingEntry["bars"], summary, eventTime, liveSignalBarHistoryLimit)
		stateMap[key] = entry
	}
	return stateMap
}

func mergeSignalBarHistory(existing any, summary map[string]any, eventTime time.Time, limit int) []any {
	items := normalizeSignalBarEntries(existing)
	bar := normalizeSignalBarEntry(map[string]any{
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
		"updatedAt": eventTime.UTC().Format(time.RFC3339Nano),
	})

	markPriorOpenSignalBarsClosed(items, bar)

	matchIndex := -1
	barKey := signalBarHistoryKey(bar)
	for i, item := range items {
		if signalBarHistoryKey(item) == barKey && barKey != "" {
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

func markPriorOpenSignalBarsClosed(items []map[string]any, current map[string]any) {
	currentStart, ok := signalBarTimestampMillis(current["barStart"])
	if !ok {
		return
	}
	currentSymbol := NormalizeSymbol(stringValue(current["symbol"]))
	currentTimeframe := strings.ToLower(strings.TrimSpace(stringValue(current["timeframe"])))
	if currentSymbol == "" || currentTimeframe == "" {
		return
	}
	for _, item := range items {
		if item == nil || boolValue(item["isClosed"]) {
			continue
		}
		if NormalizeSymbol(stringValue(item["symbol"])) != currentSymbol {
			continue
		}
		if strings.ToLower(strings.TrimSpace(stringValue(item["timeframe"]))) != currentTimeframe {
			continue
		}
		itemStart, ok := signalBarTimestampMillis(item["barStart"])
		if !ok || itemStart >= currentStart {
			continue
		}
		itemEnd, ok := signalBarTimestampMillis(item["barEnd"])
		if ok && itemEnd > currentStart {
			continue
		}
		item["isClosed"] = true
		item["closedByNextBar"] = true
	}
}

func signalBarTimestampMillis(raw any) (int64, bool) {
	if numeric, ok := toFloat64(raw); ok && numeric > 0 {
		return int64(numeric), true
	}
	parsed := parseOptionalRFC3339(stringValue(raw))
	if parsed.IsZero() {
		return 0, false
	}
	return parsed.UTC().UnixMilli(), true
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
			"current":        cloneMetadata(current),
		}
		if sma5 := finiteSignalBarIndicator(rollingMean(closes, len(indicatorBars)-1, 5)); sma5 != nil {
			entry["sma5"] = *sma5
		}
		if ma20 := finiteSignalBarIndicator(rollingMean(closes, len(indicatorBars)-1, 20)); ma20 != nil {
			entry["ma20"] = *ma20
		}
		if atr14 := finiteSignalBarIndicator(rollingMean(trueRanges, len(indicatorBars)-1, 14)); atr14 != nil {
			entry["atr14"] = *atr14
		}
		if atrPercentile := finiteSignalBarIndicator(rollingLastPercentileFromSeries(trueRanges, len(indicatorBars)-1, 14, 240, 50)); atrPercentile != nil {
			entry["atrPercentile"] = *atrPercentile
		}
		if len(previousClosed) >= 1 {
			entry["prevBar1"] = cloneMetadata(previousClosed[len(previousClosed)-1])
		}
		if len(previousClosed) >= 2 {
			entry["prevBar2"] = cloneMetadata(previousClosed[len(previousClosed)-2])
		}
		if len(previousClosed) >= 3 {
			entry["prevBar3"] = cloneMetadata(previousClosed[len(previousClosed)-3])
		}
		out[key] = entry
	}
	return out
}

func finiteSignalBarIndicator(value float64) *float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return nil
	}
	return &value
}

func canonicalSignalBarTimestamp(raw any) string {
	if numeric, ok := toFloat64(raw); ok && numeric > 0 {
		return strconv.FormatInt(int64(numeric), 10)
	}
	return strings.TrimSpace(stringValue(raw))
}

func normalizeSignalBarEntry(entry map[string]any) map[string]any {
	if entry == nil {
		return nil
	}
	normalized := cloneMetadata(entry)
	normalized["symbol"] = NormalizeSymbol(stringValue(normalized["symbol"]))
	normalized["timeframe"] = strings.ToLower(strings.TrimSpace(stringValue(normalized["timeframe"])))
	normalized["barStart"] = canonicalSignalBarTimestamp(normalized["barStart"])
	normalized["barEnd"] = canonicalSignalBarTimestamp(normalized["barEnd"])
	return normalized
}

func signalBarHistoryKey(entry map[string]any) string {
	barStart := canonicalSignalBarTimestamp(entry["barStart"])
	timeframe := strings.ToLower(strings.TrimSpace(stringValue(entry["timeframe"])))
	if barStart == "" || timeframe == "" {
		return ""
	}
	return strings.Join([]string{
		NormalizeSymbol(stringValue(entry["symbol"])),
		timeframe,
		barStart,
	}, "|")
}

func normalizeSignalBarEntries(value any) []map[string]any {
	out := make([]map[string]any, 0)
	indexByKey := make(map[string]int)
	appendEntry := func(entry map[string]any) {
		normalized := normalizeSignalBarEntry(entry)
		if normalized == nil {
			return
		}
		if key := signalBarHistoryKey(normalized); key != "" {
			if idx, exists := indexByKey[key]; exists {
				out[idx] = normalized
				return
			}
			indexByKey[key] = len(out)
		}
		out = append(out, normalized)
	}
	switch items := value.(type) {
	case []any:
		for _, item := range items {
			if entry := mapValue(item); entry != nil {
				appendEntry(entry)
			}
		}
	case []map[string]any:
		for _, item := range items {
			appendEntry(item)
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
			summary["barStart"] = canonicalSignalBarTimestamp(kline["t"])
			summary["barEnd"] = canonicalSignalBarTimestamp(kline["T"])
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
// Entries without a symbol tag are kept for backward compatibility with pre-isolation
// state snapshots. Once we have telemetry on blank-symbol entries, this fallback can tighten.
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
// Blank-symbol entries are preserved only for backward compatibility with older state data.
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
