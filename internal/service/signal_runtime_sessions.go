package service

import (
	"context"
	"fmt"
	"slices"
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
	plan, err := p.BuildSignalRuntimePlan(accountID, strategyID)
	if err != nil {
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
			"plan":                   plan,
			"subscriptions":          subscriptions,
			"health":                 "idle",
			"signalEventCount":       0,
			"strategySignalCount":    0,
			"sourceStates":           map[string]any{},
			"strategySignalsByScope": map[string]any{},
			"lastHeartbeatAt":        "",
			"lastEventAt":            "",
			"lastEventSummary":       nil,
			"lastStrategySignalAt":   "",
			"lastStrategySignal":     nil,
			"startedAt":              "",
			"stoppedAt":              "",
			"errors":                 []any{},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
	p.mu.Lock()
	p.signalSessions[session.ID] = session
	p.mu.Unlock()
	return session, nil
}

func (p *Platform) StartSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	p.mu.Lock()
	session, ok := p.signalSessions[sessionID]
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
	state["health"] = "healthy"
	state["startedAt"] = now.Format(time.RFC3339)
	state["lastHeartbeatAt"] = now.Format(time.RFC3339)
	state["lastEventAt"] = now.Format(time.RFC3339)
	subscriptions := metadataList(state["subscriptions"])
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
	return session, nil
}

func (p *Platform) StopSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	p.mu.Lock()
	session, ok := p.signalSessions[sessionID]
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
	return session, nil
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

func (p *Platform) persistRuntimeStrategySignal(runtimeSessionID string, eventTime time.Time, snapshot map[string]any) error {
	if strings.TrimSpace(runtimeSessionID) == "" || len(snapshot) == 0 {
		return nil
	}
	return p.updateSignalRuntimeSessionState(runtimeSessionID, func(session *domain.SignalRuntimeSession) {
		state := cloneMetadata(session.State)
		if state == nil {
			state = map[string]any{}
		}

		scope := strings.TrimSpace(stringValue(snapshot["scope"]))
		scopeID := strings.TrimSpace(stringValue(snapshot["scopeId"]))
		signalKey := firstNonEmpty(scope+"|"+scopeID, scopeID, scope, "runtime")

		signalsByScope := cloneMetadata(mapValue(state["strategySignalsByScope"]))
		if signalsByScope == nil {
			signalsByScope = map[string]any{}
		}
		signalsByScope[signalKey] = cloneMetadata(snapshot)

		state["strategySignalsByScope"] = signalsByScope
		state["lastStrategySignalAt"] = eventTime.UTC().Format(time.RFC3339)
		state["lastStrategySignal"] = cloneMetadata(snapshot)
		state["strategySignalCount"] = maxIntValue(state["strategySignalCount"], 0) + 1

		appendSignalRuntimeTimeline(state, eventTime, "strategy", firstNonEmpty(stringValue(snapshot["status"]), "evaluated"), map[string]any{
			"scope":          scope,
			"scopeId":        scopeID,
			"decisionAction": stringValue(mapValue(snapshot["decision"])["action"]),
			"decisionReason": stringValue(mapValue(snapshot["decision"])["reason"]),
			"intentSide":     stringValue(mapValue(snapshot["intent"])["side"]),
			"orderId":        stringValue(mapValue(snapshot["order"])["id"]),
		})

		session.State = state
		session.UpdatedAt = eventTime.UTC()
	})
}

func buildRuntimeStrategySignalSnapshot(
	scope string,
	scopeID string,
	status string,
	eventTime time.Time,
	decision StrategySignalDecision,
	intent map[string]any,
	order map[string]any,
	executionContext StrategyExecutionContext,
	nextPlannedEvent time.Time,
	nextPlannedPrice float64,
	nextPlannedSide string,
	nextPlannedRole string,
	nextPlannedReason string,
	sourceGate map[string]any,
) map[string]any {
	nextEvent := ""
	if !nextPlannedEvent.IsZero() {
		nextEvent = nextPlannedEvent.UTC().Format(time.RFC3339)
	}
	return map[string]any{
		"scope":     strings.TrimSpace(scope),
		"scopeId":   strings.TrimSpace(scopeID),
		"status":    strings.TrimSpace(status),
		"eventTime": eventTime.UTC().Format(time.RFC3339),
		"decision": map[string]any{
			"action":   decision.Action,
			"reason":   decision.Reason,
			"metadata": cloneMetadata(decision.Metadata),
		},
		"intent": cloneMetadata(intent),
		"order":  cloneMetadata(order),
		"context": map[string]any{
			"strategyVersionId":   executionContext.StrategyVersionID,
			"strategyEngine":      executionContext.StrategyEngineKey,
			"signalTimeframe":     executionContext.SignalTimeframe,
			"executionDataSource": executionContext.ExecutionDataSource,
			"symbol":              executionContext.Symbol,
		},
		"nextPlan": map[string]any{
			"eventAt": nextEvent,
			"price":   nextPlannedPrice,
			"side":    nextPlannedSide,
			"role":    nextPlannedRole,
			"reason":  nextPlannedReason,
		},
		"sourceGate": cloneMetadata(sourceGate),
	}
}
