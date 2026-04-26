package service

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store"
)

type signalRuntimeRun struct {
	ctx          context.Context
	cancelRunner context.CancelFunc
	starting     bool
	releaseLease func()
	releaseOnce  sync.Once
}

func (r *signalRuntimeRun) releaseRuntimeLease() {
	if r == nil || r.releaseLease == nil {
		return
	}
	r.releaseOnce.Do(r.releaseLease)
}

func (p *Platform) ListSignalRuntimeSessions() []domain.SignalRuntimeSession {
	items, err := p.store.ListSignalRuntimeSessions()
	if err == nil {
		p.mu.Lock()
		seen := make(map[string]struct{}, len(items)+len(p.signalSessions))
		for _, session := range items {
			p.signalSessions[session.ID] = session
			seen[session.ID] = struct{}{}
		}
		for _, session := range p.signalSessions {
			if _, ok := seen[session.ID]; ok {
				continue
			}
			items = append(items, session)
		}
		p.mu.Unlock()
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
	p.logger("service.signal_runtime").Warn("list persisted signal runtime sessions failed", "error", err)
	p.mu.Lock()
	defer p.mu.Unlock()
	items = make([]domain.SignalRuntimeSession, 0, len(p.signalSessions))
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

func (p *Platform) ListSignalRuntimeSessionsSummary() []domain.SignalRuntimeSession {
	items := p.ListSignalRuntimeSessions()
	stripped := make([]domain.SignalRuntimeSession, len(items))
	for i, item := range items {
		newItem := item
		newItem.State = stripHeavyState(item.State)
		stripped[i] = newItem
	}
	return stripped
}

func (p *Platform) GetSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	p.mu.Lock()
	session, ok := p.signalSessions[sessionID]
	p.mu.Unlock()
	if ok {
		return session, nil
	}
	session, err := p.store.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	p.cacheSignalRuntimeSession(session)
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
	persisted, err := p.store.CreateSignalRuntimeSession(session)
	if err != nil {
		logger.Warn("persist signal runtime session failed", "error", err)
		return domain.SignalRuntimeSession{}, err
	}
	p.cacheSignalRuntimeSession(persisted)
	p.logger("service.signal_runtime",
		"session_id", persisted.ID,
		"account_id", persisted.AccountID,
		"strategy_id", persisted.StrategyID,
	).Info("signal runtime session created",
		"subscription_count", persisted.SubscriptionCnt,
		"runtime_adapter", persisted.RuntimeAdapter,
	)
	return persisted, nil
}

// syncSignalRuntimeSessionPlan rebuilds the stored runtime plan/subscription
// state from the current strategy bindings. It does not open or hot-swap live
// transport subscriptions by itself; callers that need actual rebinding must
// restart the runtime afterwards so StartSignalRuntimeSession can prepare the
// new subscriptions from this refreshed plan.
func (p *Platform) syncSignalRuntimeSessionPlan(sessionID string) (domain.SignalRuntimeSession, error) {
	session, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	plan, err := p.BuildSignalRuntimePlan(session.AccountID, session.StrategyID)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	subscriptions := metadataList(plan["subscriptions"])
	adapterKey := ""
	if len(subscriptions) > 0 {
		adapterKey = stringValue(subscriptions[0]["adapterKey"])
	}
	now := time.Now().UTC()

	current, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	state := cloneMetadata(current.State)
	state["plan"] = plan
	state["subscriptions"] = subscriptions
	state["sourceStates"] = p.bootstrapSignalRuntimeSourceStates(subscriptions)
	state["signalBarStates"] = deriveSignalBarStates(mapValue(state["sourceStates"]))
	state["lastEventAt"] = now.Format(time.RFC3339)
	state["lastEventSummary"] = map[string]any{
		"type":              "runtime_plan_refreshed",
		"message":           "signal runtime plan refreshed; new subscriptions apply on next runtime start",
		"subscriptionCount": len(subscriptions),
		"subscriptions":     summarizeSubscriptions(subscriptions),
	}
	current.RuntimeAdapter = adapterKey
	current.Transport = inferSignalRuntimeTransport(subscriptions)
	current.SubscriptionCnt = len(subscriptions)
	current.State = state
	current.UpdatedAt = now
	updated, updateErr := p.store.UpdateSignalRuntimeSession(current)
	if updateErr != nil {
		if isSignalRuntimeSessionNotFoundError(updateErr) {
			p.cacheSignalRuntimeSession(current)
			return current, nil
		}
		return domain.SignalRuntimeSession{}, updateErr
	}
	p.cacheSignalRuntimeSession(updated)
	return updated, nil
}

func (p *Platform) StartSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	return p.startSignalRuntimeSession(context.Background(), sessionID)
}

func (p *Platform) startSignalRuntimeSession(parent context.Context, sessionID string) (domain.SignalRuntimeSession, error) {
	logger := p.logger("service.signal_runtime", "session_id", sessionID)
	p.mu.Lock()
	if run, exists := p.signalRun[sessionID]; exists {
		session, ok := p.signalSessions[sessionID]
		p.mu.Unlock()
		if !ok {
			var err error
			session, err = p.GetSignalRuntimeSession(sessionID)
			if err != nil {
				return domain.SignalRuntimeSession{}, err
			}
		}
		logger.Debug("signal runtime session already running", "starting", run.starting)
		return session, nil
	}
	baseCtx, cancelRunner := context.WithCancel(parent)
	run := &signalRuntimeRun{ctx: baseCtx, cancelRunner: cancelRunner, starting: true}
	p.signalRun[sessionID] = run
	p.mu.Unlock()
	leaseCtx, releaseLease, acquired, err := p.acquireSignalRuntimeSessionLease(baseCtx, sessionID)
	if err != nil {
		cancelRunner()
		p.clearSignalRuntimeRun(sessionID, run)
		logger.Warn("acquire signal runtime session lease failed", "error", err)
		return domain.SignalRuntimeSession{}, err
	}
	if !acquired {
		cancelRunner()
		p.clearSignalRuntimeRun(sessionID, run)
		logger.Debug("signal runtime session lease held by another runner")
		return domain.SignalRuntimeSession{}, ErrRuntimeLeaseNotAcquired
	}
	ctx := leaseCtx
	run.ctx = ctx
	run.releaseLease = releaseLease
	session, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		cancelRunner()
		p.clearSignalRuntimeRun(sessionID, run)
		logger.Warn("signal runtime session not found")
		return domain.SignalRuntimeSession{}, err
	}
	plan, err := p.BuildSignalRuntimePlan(session.AccountID, session.StrategyID)
	if err != nil {
		cancelRunner()
		p.clearSignalRuntimeRun(sessionID, run)
		logger.Warn("build signal runtime plan failed", "error", err)
		return domain.SignalRuntimeSession{}, err
	}
	if err := ctx.Err(); err != nil {
		p.clearSignalRuntimeRun(sessionID, run)
		return domain.SignalRuntimeSession{}, err
	}
	subscriptions := metadataList(plan["subscriptions"])
	adapterKey := ""
	if len(subscriptions) > 0 {
		adapterKey = stringValue(subscriptions[0]["adapterKey"])
	}
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
	state["desiredStatus"] = "RUNNING"
	state["actualStatus"] = "STARTING"
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
	updatedSession, updateErr := p.store.UpdateSignalRuntimeSession(session)
	if updateErr != nil {
		if !isSignalRuntimeSessionNotFoundError(updateErr) {
			cancelRunner()
			p.clearSignalRuntimeRun(sessionID, run)
			logger.Warn("persist signal runtime session start failed", "error", updateErr)
			return domain.SignalRuntimeSession{}, updateErr
		}
		logger.Warn("persist signal runtime session start missed store row; using cache-only session", "error", updateErr)
	} else {
		session = updatedSession
	}
	if err := ctx.Err(); err != nil {
		p.persistSignalRuntimeStoppedAfterStartCancel(session)
		p.clearSignalRuntimeRun(sessionID, run)
		return domain.SignalRuntimeSession{}, err
	}
	p.mu.Lock()
	if current := p.signalRun[sessionID]; current != run {
		p.mu.Unlock()
		cancelRunner()
		run.releaseRuntimeLease()
		return domain.SignalRuntimeSession{}, fmt.Errorf("signal runtime session start superseded: %s", sessionID)
	}
	run.starting = false
	p.signalSessions[session.ID] = session
	p.mu.Unlock()
	go p.runSignalRuntimeWithRecovery(ctx, sessionID)
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

func (p *Platform) StopSignalRuntimeSession(sessionID string) (domain.SignalRuntimeSession, error) {
	return p.StopSignalRuntimeSessionWithForce(sessionID, false)
}

func (p *Platform) StopSignalRuntimeSessionWithForce(sessionID string, force bool) (domain.SignalRuntimeSession, error) {
	logger := p.logger("service.signal_runtime", "session_id", sessionID)
	existing, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		logger.Warn("signal runtime session not found")
		return domain.SignalRuntimeSession{}, err
	}
	if !force {
		if err := p.ensureNoActivePositionsOrOrders(existing.AccountID, existing.StrategyID); err != nil {
			logger.Warn("stop signal runtime session blocked by active positions or orders", "error", err)
			return domain.SignalRuntimeSession{}, err
		}
	}
	p.mu.Lock()
	run, running := p.signalRun[sessionID]
	p.mu.Unlock()
	session := existing
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["health"] = "stopped"
	state["desiredStatus"] = "STOPPED"
	state["actualStatus"] = "STOPPED"
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
	updatedSession, updateErr := p.store.UpdateSignalRuntimeSession(session)
	if updateErr != nil {
		if !isSignalRuntimeSessionNotFoundError(updateErr) {
			logger.Warn("persist signal runtime session stop failed", "error", updateErr)
			return domain.SignalRuntimeSession{}, updateErr
		}
		logger.Warn("persist signal runtime session stop missed store row; using cache-only session", "error", updateErr)
	} else {
		session = updatedSession
	}
	p.mu.Lock()
	if running {
		delete(p.signalRun, sessionID)
	}
	p.signalSessions[session.ID] = session
	p.mu.Unlock()
	if running {
		run.cancelRunner()
		run.releaseRuntimeLease()
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
		if err != nil || len(snapshot.SignalBars) == 0 {
			if refreshErr := p.refreshLiveMarketSnapshot(symbol); refreshErr != nil {
				continue
			}
			snapshot, err = p.liveMarketSnapshot(symbol)
			if err != nil {
				continue
			}
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

func (p *Platform) DeleteSignalRuntimeSession(sessionID string) error {
	return p.DeleteSignalRuntimeSessionWithForce(sessionID, false)
}

func (p *Platform) DeleteSignalRuntimeSessionWithForce(sessionID string, force bool) error {
	existing, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return err
	}
	if !force {
		if err := p.ensureNoActivePositionsOrOrders(existing.AccountID, existing.StrategyID); err != nil {
			return err
		}
	}
	if err := p.store.DeleteSignalRuntimeSession(sessionID); err != nil && !isSignalRuntimeSessionNotFoundError(err) {
		return err
	}
	p.mu.Lock()
	run, running := p.signalRun[sessionID]
	if running {
		delete(p.signalRun, sessionID)
	}
	delete(p.signalSessions, sessionID)
	p.mu.Unlock()
	if running {
		run.cancelRunner()
		run.releaseRuntimeLease()
	}
	return nil
}

func (p *Platform) updateSignalRuntimeSessionState(sessionID string, updater func(session *domain.SignalRuntimeSession)) error {
	session, err := p.GetSignalRuntimeSession(sessionID)
	if err != nil {
		return err
	}
	updater(&session)
	session.UpdatedAt = time.Now().UTC()
	updated, updateErr := p.store.UpdateSignalRuntimeSession(session)
	if updateErr != nil {
		if isSignalRuntimeSessionNotFoundError(updateErr) {
			p.cacheSignalRuntimeSession(session)
			return nil
		}
		return updateErr
	}
	p.cacheSignalRuntimeSession(updated)
	return nil
}

func (p *Platform) cacheSignalRuntimeSession(session domain.SignalRuntimeSession) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.signalSessions[session.ID] = session
}

func isSignalRuntimeSessionNotFoundError(err error) bool {
	return errors.Is(err, store.ErrSignalRuntimeSessionNotFound)
}

func (p *Platform) clearSignalRuntimeRun(sessionID string, run *signalRuntimeRun) bool {
	p.mu.Lock()
	if current := p.signalRun[sessionID]; current == run {
		delete(p.signalRun, sessionID)
		p.mu.Unlock()
		run.releaseRuntimeLease()
		return true
	}
	p.mu.Unlock()
	return false
}

func (p *Platform) persistSignalRuntimeStoppedAfterStartCancel(session domain.SignalRuntimeSession) {
	now := time.Now().UTC()
	state := cloneMetadata(session.State)
	state["health"] = "stopped"
	state["desiredStatus"] = "STOPPED"
	state["actualStatus"] = "STOPPED"
	state["stoppedAt"] = now.Format(time.RFC3339)
	state["lastHeartbeatAt"] = now.Format(time.RFC3339)
	state["lastEventAt"] = now.Format(time.RFC3339)
	state["lastEventSummary"] = map[string]any{
		"type":    "runtime_start_cancelled",
		"message": "signal runtime start cancelled before runner was launched",
	}
	session.Status = "STOPPED"
	session.State = state
	session.UpdatedAt = now
	if updated, err := p.store.UpdateSignalRuntimeSession(session); err == nil {
		p.cacheSignalRuntimeSession(updated)
		return
	}
	p.cacheSignalRuntimeSession(session)
}

func (p *Platform) removeSignalRuntimeRunner(sessionID string) {
	p.mu.Lock()
	run := p.signalRun[sessionID]
	delete(p.signalRun, sessionID)
	p.mu.Unlock()
	run.releaseRuntimeLease()
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
