package service

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func (p *Platform) ListSignalRuntimeSessions() []domain.SignalRuntimeSession {
	p.mu.Lock()
	defer p.mu.Unlock()
	items := make([]domain.SignalRuntimeSession, 0, len(p.signalSessions))
	for _, session := range p.signalSessions {
		items = append(items, session)
	}
	slices.SortFunc(items, func(a, b domain.SignalRuntimeSession) int {
		if a.UpdatedAt.Equal(b.UpdatedAt) {
			switch {
			case a.ID < b.ID:
				return -1
			case a.ID > b.ID:
				return 1
			default:
				return 0
			}
		}
		if a.UpdatedAt.Before(b.UpdatedAt) {
			return 1
		}
		return -1
	})
	return items
}

func (p *Platform) GetSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	session, ok := p.signalSessions[sessionID]
	if !ok {
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session not found: %s", sessionID)
	}
	return session, nil
}

func (p *Platform) CreateSignalRuntimeSession(accountID, strategyID string) (domain.SignalRuntimeSession, error) {
	logger := p.logger("service.signal_runtime", "account_id", accountID, "strategy_id", strategyID)
	plan, err := p.BuildSignalRuntimePlan(accountID, strategyID)
	if err != nil {
		logger.Warn("build signal runtime plan failed", "error", err)
		return domain.SignalRuntimeSession{}, err
	}
	now := time.Now().UTC()
	subscriptions := metadataList(plan["subscriptions"])
	adapterKey := ""
	if len(subscriptions) > 0 {
		adapterKey = stringValue(subscriptions[0]["adapterKey"])
	}
	session := domain.SignalRuntimeSession{
		ID:              fmt.Sprintf("signal-runtime-%d", now.UnixNano()),
		AccountID:       accountID,
		StrategyID:      strategyID,
		Status:          "READY",
		RuntimeAdapter:  adapterKey,
		Transport:       inferSignalRuntimeTransport(subscriptions),
		SubscriptionCnt: len(subscriptions),
		State: map[string]any{
			"plan":             plan,
			"subscriptions":    subscriptions,
			"health":           "idle",
			"signalEventCount": 0,
			"sourceStates":     map[string]any{},
			"lastHeartbeatAt":  "",
			"lastEventAt":      "",
			"lastEventSummary": nil,
			"startedAt":        "",
			"stoppedAt":        "",
			"errors":           []any{},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	p.mu.Lock()
	p.signalSessions[session.ID] = session
	p.mu.Unlock()
	p.logger("service.signal_runtime",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("signal runtime session created",
		"subscription_count", len(subscriptions),
		"runtime_adapter", adapterKey,
	)
	return session, nil
}

func (p *Platform) StartSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	logger := p.logger("service.signal_runtime", "session_id", sessionID)
	p.mu.Lock()
	session, ok := p.signalSessions[sessionID]
	if !ok {
		p.mu.Unlock()
		logger.Warn("signal runtime session not found")
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session not found: %s", sessionID)
	}
	if _, exists := p.signalRun[sessionID]; exists {
		p.mu.Unlock()
		logger.Debug("signal runtime session already running")
		return session, nil
	}
	p.mu.Unlock()
	plan, err := p.BuildSignalRuntimePlan(session.AccountID, session.StrategyID)
	if err != nil {
		logger.Warn("build signal runtime plan failed", "error", err)
		return domain.SignalRuntimeSession{}, err
	}
	subscriptions := metadataList(plan["subscriptions"])
	adapterKey := ""
	if len(subscriptions) > 0 {
		adapterKey = stringValue(subscriptions[0]["adapterKey"])
	}
	p.mu.Lock()
	session, ok = p.signalSessions[sessionID]
	if !ok {
		p.mu.Unlock()
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session not found: %s", sessionID)
	}
	if _, exists := p.signalRun[sessionID]; exists {
		p.mu.Unlock()
		return session, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.signalRun[sessionID] = cancel
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["plan"] = plan
	state["subscriptions"] = subscriptions
	sourceStates := cloneMetadata(mapValue(state["sourceStates"]))
	if len(sourceStates) == 0 {
		sourceStates = p.bootstrapSignalRuntimeSourceStates(subscriptions)
	}
	state["sourceStates"] = sourceStates
	state["signalBarStates"] = deriveSignalBarStates(sourceStates)
	session.RuntimeAdapter = adapterKey
	session.Transport = inferSignalRuntimeTransport(subscriptions)
	session.SubscriptionCnt = len(subscriptions)
	state["health"] = "healthy"
	state["startedAt"] = now.Format(time.RFC3339)
	state["lastHeartbeatAt"] = now.Format(time.RFC3339)
	state["lastEventAt"] = now.Format(time.RFC3339)
	state["lastEventSummary"] = map[string]any{
		"type":              "runtime_started",
		"subscriptionCount": len(subscriptions),
		"subscriptions":     summarizeSubscriptions(subscriptions),
		"message":           "signal runtime subscriptions prepared",
	}
	session.Status = "RUNNING"
	session.State = state
	session.UpdatedAt = now
	p.signalSessions[session.ID] = session
	p.mu.Unlock()
	go p.runSignalRuntimeLoop(ctx, sessionID)
	p.logger("service.signal_runtime",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("signal runtime session started",
		"subscription_count", len(subscriptions),
		"runtime_adapter", adapterKey,
	)
	return session, nil
}

func (p *Platform) StopSignalRuntimeSession(sessionID string, force bool) (domain.SignalRuntimeSession, error) {
	logger := p.logger("service.signal_runtime", "session_id", sessionID)
	p.mu.Lock()
	session, ok := p.signalSessions[sessionID]
	p.mu.Unlock()
	if !ok {
		logger.Warn("signal runtime session not found")
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session not found: %s", sessionID)
	}

	if !force {
		// 检查属于该 Runtime 的所有账户和策略
		if active, reason := p.HasActivePositionsOrOrders(session.AccountID, session.StrategyID); active {
			return domain.SignalRuntimeSession{}, fmt.Errorf("无法停用策略运行会话: %s (请尝试使用 force=true 强制停用)", reason)
		}
	}

	p.mu.Lock()
	session, ok = p.signalSessions[sessionID]
	if !ok {
		p.mu.Unlock()
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session not found: %s", sessionID)
	}
	cancel, running := p.signalRun[sessionID]
	if running {
		delete(p.signalRun, sessionID)
	}
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["health"] = "stopped"
	state["stoppedAt"] = now.Format(time.RFC3339)
	state["lastHeartbeatAt"] = now.Format(time.RFC3339)
	state["lastEventAt"] = now.Format(time.RFC3339)
	state["lastEventSummary"] = map[string]any{
		"type":    "runtime_stopped",
		"message": "signal runtime stopped",
	}
	session.Status = "STOPPED"
	session.State = state
	session.UpdatedAt = now
	p.signalSessions[session.ID] = session
	p.mu.Unlock()
	if running {
		cancel()
	}
	p.logger("service.signal_runtime",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("signal runtime session stopped")
	return session, nil
}

func (p *Platform) bootstrapSignalRuntimeSourceStates(subscriptions []map[string]any) map[string]any {
	out := map[string]any{}
	for _, subscription := range subscriptions {
		if !strings.EqualFold(stringValue(subscription["streamType"]), "signal_bar") {
			continue
		}
		symbol := NormalizeSymbol(stringValue(subscription["symbol"]))
		timeframe := signalBindingTimeframe(stringValue(subscription["sourceKey"]), metadataValue(subscription["options"]))
		if symbol == "" || timeframe == "" {
			continue
		}
		snapshot, err := p.liveMarketSnapshot(symbol)
		if err != nil {
			continue
		}
		bars := snapshot.SignalBars[strings.ToLower(strings.TrimSpace(timeframe))]
		if len(bars) == 0 {
			continue
		}
		key := signalBindingMatchKey(
			stringValue(subscription["sourceKey"]),
			stringValue(subscription["role"]),
			symbol,
			map[string]any{"timeframe": timeframe},
		)
		out[key] = map[string]any{
			"sourceKey":   stringValue(subscription["sourceKey"]),
			"role":        stringValue(subscription["role"]),
			"streamType":  stringValue(subscription["streamType"]),
			"symbol":      symbol,
			"timeframe":   timeframe,
			"event":       "bootstrap",
			"lastEventAt": "",
			"summary": map[string]any{
				"event":      "bootstrap",
				"source":     "market-cache",
				"symbol":     symbol,
				"timeframe":  timeframe,
				"streamType": stringValue(subscription["streamType"]),
			},
			"bars": strategySignalBarsToRuntimeHistory(bars, symbol, timeframe, 200),
		}
	}
	return out
}

func strategySignalBarsToRuntimeHistory(bars []strategySignalBar, symbol, timeframe string, limit int) []any {
	if len(bars) == 0 {
		return nil
	}
	if limit > 0 && len(bars) > limit {
		bars = bars[len(bars)-limit:]
	}
	step := resolutionToDuration(liveSignalResolution(timeframe))
	if step <= 0 {
		return nil
	}
	out := make([]any, 0, len(bars))
	for _, bar := range bars {
		start := bar.Time.UTC()
		out = append(out, map[string]any{
			"timeframe": strings.ToLower(strings.TrimSpace(timeframe)),
			"symbol":    NormalizeSymbol(symbol),
			"barStart":  strconv.FormatInt(start.UnixMilli(), 10),
			"barEnd":    strconv.FormatInt(start.Add(step).UnixMilli(), 10),
			"open":      bar.Open,
			"high":      bar.High,
			"low":       bar.Low,
			"close":     bar.Close,
			"volume":    bar.Volume,
			"isClosed":  true,
			"updatedAt": start.Format(time.RFC3339),
		})
	}
	return out
}

func (p *Platform) DeleteSignalRuntimeSession(sessionID string, force bool) error {
	p.mu.Lock()
	session, sessionExists := p.signalSessions[sessionID]
	p.mu.Unlock()

	if !sessionExists {
		return fmt.Errorf("signal runtime session not found: %s", sessionID)
	}

	if !force {
		if active, reason := p.HasActivePositionsOrOrders(session.AccountID, session.StrategyID); active {
			return fmt.Errorf("无法删除策略运行会话: %s (请尝试使用 force=true 强制删除)", reason)
		}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if cancel, running := p.signalRun[sessionID]; running {
		delete(p.signalRun, sessionID)
		cancel()
	}
	delete(p.signalSessions, sessionID)
	return nil
}

func (p *Platform) updateSignalRuntimeSessionState(sessionID string, updater func(session *domain.SignalRuntimeSession)) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	session, ok := p.signalSessions[sessionID]
	if !ok {
		return fmt.Errorf("signal runtime session not found: %s", sessionID)
	}
	updater(&session)
	p.signalSessions[sessionID] = session
	return nil
}

func (p *Platform) removeSignalRuntimeRunner(sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.signalRun, sessionID)
}

func inferSignalRuntimeTransport(subscriptions []map[string]any) string {
	if len(subscriptions) == 0 {
		return ""
	}
	return stringValue(subscriptions[0]["transport"])
}

func summarizeSubscriptions(subscriptions []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(subscriptions))
	for _, item := range subscriptions {
		out = append(out, map[string]any{
			"sourceKey":  item["sourceKey"],
			"role":       item["role"],
			"symbol":     item["symbol"],
			"channel":    item["channel"],
			"adapterKey": item["adapterKey"],
		})
	}
	return out
}

func metadataList(value any) []map[string]any {
	switch items := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			out = append(out, cloneMetadata(item))
		}
		return out
	case []any:
		out := make([]map[string]any, 0, len(items))
		for _, item := range items {
			if entry, ok := item.(map[string]any); ok {
				out = append(out, cloneMetadata(entry))
			}
		}
		return out
	default:
		return nil
	}
}
