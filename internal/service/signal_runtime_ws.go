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

const (
	defaultBinanceFuturesWSURL = "wss://fstream.binance.com/ws"
	defaultOKXPublicWSURL      = "wss://ws.okx.com:8443/ws/v5/public"
)

func (p *Platform) runSignalRuntimeLoop(ctx context.Context, sessionID string) {
	session, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return
	}

	switch session.RuntimeAdapter {
	case "binance-market-ws":
		p.runExchangeWebsocketLoop(ctx, session, configuredBinanceFuturesWSURL(), buildBinanceSubscribePayload)
	case "okx-market-ws":
		p.runExchangeWebsocketLoop(ctx, session, configuredOKXPublicWSURL(), buildOKXSubscribePayload)
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
				appendSignalRuntimeTimeline(state, time.Now().UTC(), "runtime", "stopped", nil)
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
				appendSignalRuntimeTimeline(state, time.Now().UTC(), "runtime", "error", map[string]any{
					"message": err.Error(),
				})
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

func (p *Platform) handleSignalRuntimeMessage(runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
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
		_ = p.triggerLiveSessionFromSignal(session.ID, runtimeSessionID, summary, eventTime)
	}
	return nil
}

func enrichSignalRuntimeSummary(session domain.SignalRuntimeSession, summary map[string]any) map[string]any {
	out := cloneMetadata(summary)
	subscriptions := metadataList(session.State["subscriptions"])
	if len(subscriptions) == 0 {
		return out
	}
	if len(subscriptions) == 1 {
		attachSubscriptionContext(out, subscriptions[0])
		return out
	}

	symbol := NormalizeSymbol(stringValue(out["symbol"]))
	event := strings.ToLower(strings.TrimSpace(stringValue(out["event"])))
	streamType := inferStreamTypeFromEvent(event)
	for _, subscription := range subscriptions {
		if NormalizeSymbol(stringValue(subscription["symbol"])) != symbol {
			continue
		}
		if streamType != "" && strings.TrimSpace(stringValue(subscription["streamType"])) != streamType {
			continue
		}
		attachSubscriptionContext(out, subscription)
		return out
	}
	return out
}

func attachSubscriptionContext(summary map[string]any, subscription map[string]any) {
	summary["sourceKey"] = stringValue(subscription["sourceKey"])
	summary["role"] = stringValue(subscription["role"])
	summary["streamType"] = stringValue(subscription["streamType"])
	summary["channel"] = stringValue(subscription["channel"])
	summary["subscriptionSymbol"] = stringValue(subscription["symbol"])
	if options := metadataValue(subscription["options"]); options != nil {
		if timeframe := strings.TrimSpace(stringValue(options["timeframe"])); timeframe != "" {
			summary["timeframe"] = timeframe
		}
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

func mergeSignalSourceState(existing any, summary map[string]any, eventTime time.Time) map[string]any {
	stateMap := map[string]any{}
	if current := mapValue(existing); current != nil {
		stateMap = cloneMetadata(current)
	}
	key := signalBindingMatchKey(
		stringValue(summary["sourceKey"]),
		stringValue(summary["role"]),
		firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"])),
		map[string]any{"timeframe": stringValue(summary["timeframe"])},
	)
	if strings.Trim(key, "|") == "" {
		key = "unknown"
	}
	stateMap[key] = map[string]any{
		"sourceKey":   stringValue(summary["sourceKey"]),
		"role":        stringValue(summary["role"]),
		"streamType":  stringValue(summary["streamType"]),
		"symbol":      NormalizeSymbol(firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"]))),
		"timeframe":   strings.ToLower(strings.TrimSpace(stringValue(summary["timeframe"]))),
		"event":       stringValue(summary["event"]),
		"lastEventAt": eventTime.UTC().Format(time.RFC3339),
		"summary":     cloneMetadata(summary),
	}
	if strings.EqualFold(stringValue(summary["streamType"]), "signal_bar") {
		entry := cloneMetadata(mapValue(stateMap[key]))
		entry["bars"] = mergeSignalBarHistory(entry["bars"], summary, eventTime, 200)
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
		"timeframe": strings.ToLower(strings.TrimSpace(stringValue(summary["timeframe"]))),
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
		closed := make([]map[string]any, 0, len(bars))
		for _, bar := range bars {
			if boolValue(bar["isClosed"]) {
				closed = append(closed, bar)
			}
		}
		if len(closed) == 0 {
			continue
		}
		closes := make([]float64, 0, len(closed))
		trueRanges := make([]float64, 0, len(closed))
		for i, bar := range closed {
			closePrice := parseFloatValue(bar["close"])
			high := parseFloatValue(bar["high"])
			low := parseFloatValue(bar["low"])
			closes = append(closes, closePrice)
			if i == 0 {
				trueRanges = append(trueRanges, high-low)
				continue
			}
			prevClose := parseFloatValue(closed[i-1]["close"])
			highLow := high - low
			highClose := math.Abs(high - prevClose)
			lowClose := math.Abs(low - prevClose)
			trueRanges = append(trueRanges, math.Max(highLow, math.Max(highClose, lowClose)))
		}

		last := closed[len(closed)-1]
		entry := map[string]any{
			"symbol":    stringValue(last["symbol"]),
			"timeframe": stringValue(last["timeframe"]),
			"barCount":  len(closed),
			"sma5":      rollingMean(closes, len(closed)-1, 5),
			"ma20":      rollingMean(closes, len(closed)-1, 20),
			"atr14":     rollingMean(trueRanges, len(closed)-1, 14),
			"current":   cloneMetadata(last),
		}
		if len(closed) >= 2 {
			entry["prevBar1"] = cloneMetadata(closed[len(closed)-2])
		}
		if len(closed) >= 3 {
			entry["prevBar2"] = cloneMetadata(closed[len(closed)-3])
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
