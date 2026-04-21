package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type strategyReplayEvent struct {
	Time     time.Time
	Type     string
	Price    float64
	Reason   string
	Notional float64
	Balance  float64
}

type paperPlannedOrder struct {
	StrategyVersionID string
	Symbol            string
	Side              string
	Type              string
	Quantity          float64
	Price             float64
	EventTime         time.Time
	Reason            string
	Role              string
	FeeAmount         float64
	FundingPnL        float64
	Metadata          map[string]any
}

// ListPaperSessions 获取所有模拟交易会话。
func (p *Platform) ListPaperSessions() ([]domain.PaperSession, error) {
	return p.store.ListPaperSessions()
}

// CreatePaperSession 创建新的模拟交易会话，并捕获初始净值快照。
func (p *Platform) CreatePaperSession(accountID, strategyID string, startEquity float64, overrides map[string]any) (domain.PaperSession, error) {
	logger := p.logger("service.paper", "account_id", accountID, "strategy_id", strategyID)
	session, err := p.store.CreatePaperSession(accountID, strategyID, startEquity)
	if err != nil {
		logger.Error("create paper session failed", "error", err)
		return domain.PaperSession{}, err
	}
	if len(overrides) > 0 {
		state := cloneMetadata(session.State)
		for key, value := range normalizePaperSessionOverrides(overrides) {
			state[key] = value
		}
		session, err = p.store.UpdatePaperSessionState(session.ID, state)
		if err != nil {
			p.logger("service.paper", "session_id", session.ID).Error("apply paper session overrides failed", "error", err)
			return domain.PaperSession{}, err
		}
	}
	session, err = p.syncPaperSessionRuntime(session)
	if err != nil {
		p.logger("service.paper", "session_id", session.ID).Warn("sync paper session runtime failed", "error", err)
		return domain.PaperSession{}, err
	}
	if err := p.captureAccountSnapshot(accountID); err != nil {
		p.logger("service.paper", "session_id", session.ID).Warn("capture paper account snapshot failed", "error", err)
		return domain.PaperSession{}, err
	}
	p.logger("service.paper",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("paper session created",
		"start_equity", session.StartEquity,
		"override_count", len(overrides),
	)
	return session, nil
}

// StartPaperSession 启动模拟交易会话的后台执行循环。
func (p *Platform) StartPaperSession(sessionID string) (domain.PaperSession, error) {
	logger := p.logger("service.paper", "session_id", sessionID)
	session, err := p.store.GetPaperSession(sessionID)
	if err != nil {
		logger.Warn("load paper session failed", "error", err)
		return domain.PaperSession{}, err
	}
	session, err = p.syncPaperSessionRuntime(session)
	if err != nil {
		return domain.PaperSession{}, err
	}
	session, err = p.ensurePaperSignalRuntimeStarted(session)
	if err != nil {
		return domain.PaperSession{}, err
	}
	if _, _, err := p.ensurePaperExecutionPlan(session); err != nil {
		return domain.PaperSession{}, err
	}

	p.mu.Lock()
	if _, exists := p.run[sessionID]; exists {
		p.mu.Unlock()
		logger.Debug("paper session runner already active")
		return p.store.UpdatePaperSessionStatus(sessionID, "RUNNING")
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.run[sessionID] = cancel
	p.mu.Unlock()

	session, err = p.store.UpdatePaperSessionStatus(sessionID, "RUNNING")
	if err != nil {
		p.mu.Lock()
		delete(p.run, sessionID)
		p.mu.Unlock()
		cancel()
		return domain.PaperSession{}, err
	}

	go p.runPaperSessionLoop(ctx, session)
	p.logger("service.paper",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("paper session started", "tick_interval_seconds", p.tickInterval)
	return session, nil
}

// StopPaperSession 停止模拟交易会话，取消后台执行循环。
func (p *Platform) StopPaperSession(sessionID string) (domain.PaperSession, error) {
	logger := p.logger("service.paper", "session_id", sessionID)
	session, err := p.store.UpdatePaperSessionStatus(sessionID, "STOPPED")
	if err != nil {
		logger.Error("stop paper session failed", "error", err)
		return domain.PaperSession{}, err
	}

	p.mu.Lock()
	cancel, exists := p.run[sessionID]
	if exists {
		delete(p.run, sessionID)
	}
	delete(p.paperPlans, sessionID)
	p.mu.Unlock()

	if exists {
		cancel()
	}
	_, _ = p.stopLinkedSignalRuntime(session)
	p.logger("service.paper",
		"session_id", session.ID,
		"account_id", session.AccountID,
		"strategy_id", session.StrategyID,
	).Info("paper session stopped")
	return session, nil
}

// TickPaperSession 手动触发会话前进一步（处理下一笔策略计划订单）。
func (p *Platform) TickPaperSession(sessionID string) (domain.Order, error) {
	logger := p.logger("service.paper", "session_id", sessionID)
	session, err := p.store.GetPaperSession(sessionID)
	if err != nil {
		logger.Warn("load paper session for manual tick failed", "error", err)
		return domain.Order{}, err
	}
	session, err = p.syncPaperSessionRuntime(session)
	if err != nil {
		logger.Warn("sync paper session runtime before tick failed", "error", err)
		return domain.Order{}, err
	}
	logger.Debug("ticking paper session manually")
	return p.placePaperSessionOrder(session)
}

func (p *Platform) triggerPaperSessionFromSignal(sessionID, runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	session, err := p.store.GetPaperSession(sessionID)
	if err != nil {
		return err
	}
	if session.Status != "RUNNING" {
		return nil
	}

	state := cloneMetadata(session.State)
	state["lastSignalRuntimeEventAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastSignalRuntimeEvent"] = cloneMetadata(summary)
	state["lastSignalRuntimeSessionId"] = runtimeSessionID
	recordStrategyTriggerHealth(state, summary, eventTime)

	throttleSeconds := maxIntValue(state["signalTriggerThrottleSeconds"], 5)
	lastTriggeredAt := parseOptionalRFC3339(stringValue(state["lastSignalDrivenTickAt"]))
	if !lastTriggeredAt.IsZero() && eventTime.Sub(lastTriggeredAt) < time.Duration(throttleSeconds)*time.Second {
		_, err := p.store.UpdatePaperSessionState(session.ID, state)
		return err
	}

	state["lastSignalDrivenTickAt"] = eventTime.UTC().Format(time.RFC3339)
	state["signalTriggerThrottleSeconds"] = throttleSeconds
	updatedSession, err := p.store.UpdatePaperSessionState(session.ID, state)
	if err != nil {
		return err
	}

	_, err = p.evaluatePaperSessionOnSignal(updatedSession, runtimeSessionID, summary, eventTime)
	if err != nil {
		state = cloneMetadata(updatedSession.State)
		state["lastSignalTriggerError"] = err.Error()
		_, _ = p.store.UpdatePaperSessionState(session.ID, state)
	}
	return nil
}

func (p *Platform) evaluatePaperSessionOnSignal(session domain.PaperSession, runtimeSessionID string, summary map[string]any, eventTime time.Time) (domain.Order, error) {
	session, err := p.syncPaperSessionRuntime(session)
	if err != nil {
		return domain.Order{}, err
	}
	session, plan, err := p.ensurePaperExecutionPlan(session)
	if err != nil {
		return domain.Order{}, err
	}

	state := cloneMetadata(session.State)
	state["strategyEvaluationMode"] = "signal-runtime-heartbeat"
	state["lastStrategyEvaluationAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastStrategyEvaluationTrigger"] = cloneMetadata(summary)
	state["lastStrategyEvaluationTriggerSource"] = buildStrategyEvaluationTriggerSource(summary)
	state["lastStrategyEvaluationStatus"] = "evaluated"
	state["lastStrategyEvaluationPlanLength"] = len(plan)
	index := 0
	if value, ok := toFloat64(state["planIndex"]); ok && value >= 0 {
		index = int(value)
	}
	state["lastStrategyEvaluationRemaining"] = maxInt(len(plan)-index, 0)
	var nextPlannedEvent time.Time
	var nextPlannedPrice float64
	var nextPlannedSide string
	var nextPlannedRole string
	var nextPlannedReason string
	if index >= 0 && index < len(plan) {
		nextPlannedEvent = plan[index].EventTime
		nextPlannedPrice = plan[index].Price
		nextPlannedSide = plan[index].Side
		nextPlannedRole = plan[index].Role
		nextPlannedReason = plan[index].Reason
		state["lastStrategyEvaluationNextPlannedEventAt"] = formatOptionalRFC3339(nextPlannedEvent)
		state["lastStrategyEvaluationNextPlannedPrice"] = nextPlannedPrice
		state["lastStrategyEvaluationNextPlannedSide"] = nextPlannedSide
		state["lastStrategyEvaluationNextPlannedRole"] = nextPlannedRole
		state["lastStrategyEvaluationNextPlannedReason"] = nextPlannedReason
	}
	sourceGate := map[string]any{
		"ready":          false,
		"requiredCount":  0,
		"availableCount": 0,
		"freshCount":     0,
		"missing":        []any{},
		"stale":          []any{},
	}
	if runtimeSessionID != "" {
		state["lastSignalRuntimeSessionId"] = runtimeSessionID
	}
	var signalDecision StrategySignalDecision
	signalDecision.Action = "advance-plan"
	signalDecision.Reason = "default"
	var executionContext StrategyExecutionContext
	var sourceStates map[string]any
	var signalBarStates map[string]any
	if runtimeSession, runtimeErr := p.GetSignalRuntimeSession(firstNonEmpty(runtimeSessionID, stringValue(state["signalRuntimeSessionId"]))); runtimeErr == nil {
		state["lastSignalRuntimeStatus"] = runtimeSession.Status
		sourceStates = cloneMetadata(mapValue(runtimeSession.State["sourceStates"]))
		if sourceStates == nil {
			sourceStates = map[string]any{}
		}
		signalBarStates = cloneMetadata(mapValue(runtimeSession.State["signalBarStates"]))
		if signalBarStates == nil {
			signalBarStates = map[string]any{}
		}
		state["lastStrategyEvaluationSourceStates"] = sourceStates
		state["lastStrategyEvaluationSignalBarStates"] = signalBarStates
		state["lastStrategyEvaluationSignalBarStateCount"] = len(signalBarStates)
		state["lastStrategyEvaluationSourceStateCount"] = len(sourceStates)
		state["lastStrategyEvaluationRuntimeSummary"] = cloneMetadata(mapValue(runtimeSession.State["lastEventSummary"]))
		sourceGate = p.evaluateSignalSourceReadiness(session, runtimeSession, eventTime)
		state["lastStrategyEvaluationSourceGate"] = sourceGate
		recordStrategySourceGateHealth(state, sourceGate, eventTime)
	}
	if !boolValue(sourceGate["ready"]) {
		state["lastStrategyEvaluationStatus"] = "waiting-source-states"
		appendTimelineEvent(state, "strategy", eventTime, "waiting-source-states", map[string]any{
			"missing": len(metadataList(sourceGate["missing"])),
			"stale":   len(metadataList(sourceGate["stale"])),
		})
		updatedSession, updateErr := p.store.UpdatePaperSessionState(session.ID, state)
		if updateErr != nil {
			return domain.Order{}, updateErr
		}
		return domain.Order{}, fmt.Errorf("paper session %s is waiting for fresh required signal source states", updatedSession.ID)
	}
	executionContext, signalDecision, err = p.evaluatePaperSignalDecision(session, summary, sourceStates, signalBarStates, eventTime, nextPlannedEvent, nextPlannedPrice, nextPlannedSide, nextPlannedRole, nextPlannedReason)
	if err != nil {
		state["lastStrategyEvaluationStatus"] = "decision-error"
		state["lastStrategyDecision"] = map[string]any{
			"action": "error",
			"reason": err.Error(),
		}
		appendTimelineEvent(state, "strategy", eventTime, "decision-error", map[string]any{
			"error": err.Error(),
		})
		recordStrategyDecisionErrorHealth(state, eventTime, err)
		updatedSession, updateErr := p.store.UpdatePaperSessionState(session.ID, state)
		if updateErr != nil {
			return domain.Order{}, updateErr
		}
		return domain.Order{}, fmt.Errorf("paper session %s signal evaluation failed: %w", updatedSession.ID, err)
	}
	state["lastStrategyDecision"] = map[string]any{
		"action":   signalDecision.Action,
		"reason":   signalDecision.Reason,
		"metadata": cloneMetadata(signalDecision.Metadata),
	}
	recordStrategyDecisionHealth(state, signalDecision, eventTime)
	appendTimelineEvent(state, "strategy", eventTime, "decision", map[string]any{
		"action":        signalDecision.Action,
		"reason":        signalDecision.Reason,
		"decisionState": stringValue(signalDecision.Metadata["decisionState"]),
		"signalKind":    stringValue(signalDecision.Metadata["signalKind"]),
	})
	state["lastStrategyEvaluationContext"] = map[string]any{
		"strategyVersionId":   executionContext.StrategyVersionID,
		"signalTimeframe":     executionContext.SignalTimeframe,
		"executionDataSource": executionContext.ExecutionDataSource,
		"symbol":              executionContext.Symbol,
	}
	if signalDecision.Action != "advance-plan" {
		state["lastStrategyEvaluationStatus"] = "waiting-decision"
		updatedSession, updateErr := p.store.UpdatePaperSessionState(session.ID, state)
		if updateErr != nil {
			return domain.Order{}, updateErr
		}
		return domain.Order{}, fmt.Errorf("paper session %s decision gate blocked: %s", updatedSession.ID, signalDecision.Reason)
	}
	updatedSession, err := p.store.UpdatePaperSessionState(session.ID, state)
	if err != nil {
		return domain.Order{}, err
	}

	order, err := p.placePaperSessionOrder(updatedSession)
	if err != nil {
		latestSession, getErr := p.store.GetPaperSession(session.ID)
		if getErr == nil {
			state = cloneMetadata(latestSession.State)
		} else {
			state = cloneMetadata(updatedSession.State)
		}
		state["lastStrategyEvaluationStatus"] = "no-action"
		appendTimelineEvent(state, "strategy", eventTime, "no-action", map[string]any{
			"reason": "place-paper-order-returned-empty",
		})
		_, _ = p.store.UpdatePaperSessionState(session.ID, state)
		return domain.Order{}, err
	}

	latestSession, getErr := p.store.GetPaperSession(session.ID)
	if getErr == nil {
		state = cloneMetadata(latestSession.State)
	} else {
		state = cloneMetadata(updatedSession.State)
	}
	state["lastStrategyEvaluationStatus"] = "order-dispatched"
	state["lastStrategyEvaluationOrderId"] = order.ID
	recordExecutionDispatchHealth(state, order, eventTime, nil)
	appendTimelineEvent(state, "order", eventTime, "order-dispatched", map[string]any{
		"orderId": order.ID,
		"symbol":  order.Symbol,
		"side":    order.Side,
		"price":   order.Price,
	})
	_, _ = p.store.UpdatePaperSessionState(session.ID, state)
	return order, nil
}

func appendTimelineEvent(state map[string]any, category string, ts time.Time, title string, metadata map[string]any) {
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
	items = append(items, entry)
	if len(items) > 40 {
		items = items[len(items)-40:]
	}
	state["timeline"] = items
}

func (p *Platform) evaluateSignalSourceReadiness(session domain.PaperSession, runtimeSession domain.SignalRuntimeSession, eventTime time.Time) map[string]any {
	return p.evaluateRuntimeSignalSourceReadiness(session.StrategyID, runtimeSession, eventTime)
}

func (p *Platform) evaluateRuntimeSignalSourceReadiness(strategyID string, runtimeSession domain.SignalRuntimeSession, eventTime time.Time) map[string]any {
	result := map[string]any{
		"ready":          true,
		"requiredCount":  0,
		"availableCount": 0,
		"freshCount":     0,
		"missing":        []any{},
		"stale":          []any{},
	}

	requiredBindings, err := p.ListStrategySignalBindings(strategyID)
	if err != nil {
		result["ready"] = false
		result["error"] = err.Error()
		return result
	}
	sourceStates := cloneMetadata(mapValue(runtimeSession.State["sourceStates"]))
	if sourceStates == nil {
		sourceStates = map[string]any{}
	}

	missing := make([]any, 0)
	stale := make([]any, 0)
	requiredCount := 0
	available := 0
	fresh := 0
	for _, binding := range requiredBindings {
		if strings.ToUpper(strings.TrimSpace(binding.Status)) == "DISABLED" {
			continue
		}
		requiredCount++
		entry := resolveRuntimeSourceStateEntry(sourceStates, binding)
		if entry == nil {
			missing = append(missing, map[string]any{
				"sourceKey":  binding.SourceKey,
				"role":       binding.Role,
				"streamType": binding.StreamType,
				"symbol":     binding.Symbol,
			})
			continue
		}
		available++
		lastEventAt := parseOptionalRFC3339(stringValue(entry["lastEventAt"]))
		maxAge := p.signalSourceFreshnessWindowWithOverride(binding, runtimeSession.State)
		if lastEventAt.IsZero() || eventTime.Sub(lastEventAt) > maxAge {
			stale = append(stale, map[string]any{
				"sourceKey":   binding.SourceKey,
				"role":        binding.Role,
				"streamType":  binding.StreamType,
				"symbol":      binding.Symbol,
				"lastEventAt": stringValue(entry["lastEventAt"]),
				"maxAgeSec":   int(maxAge.Seconds()),
			})
			continue
		}
		fresh++
	}

	result["requiredCount"] = requiredCount
	result["availableCount"] = available
	result["freshCount"] = fresh
	result["missing"] = missing
	result["stale"] = stale
	result["ready"] = len(missing) == 0 && len(stale) == 0
	return result
}

func (p *Platform) signalSourceFreshnessWindow(binding domain.AccountSignalBinding) time.Duration {
	return p.signalSourceFreshnessWindowWithOverride(binding, nil)
}

func (p *Platform) signalSourceFreshnessWindowWithOverride(binding domain.AccountSignalBinding, sessionState map[string]any) time.Duration {
	if value, ok := toFloat64(binding.Options["freshnessSeconds"]); ok && value > 0 {
		return time.Duration(value) * time.Second
	}

	// 尝试从 session state 的 freshnessOverride 中读取
	if override := mapValue(sessionState["freshnessOverride"]); override != nil {
		var key string
		switch strings.ToLower(strings.TrimSpace(binding.StreamType)) {
		case "trade_tick":
			key = "tradeTickFreshnessSeconds"
		case "order_book":
			key = "orderBookFreshnessSeconds"
		case "signal_bar":
			key = "signalBarFreshnessSeconds"
		}
		if key != "" {
			if val, ok := toFloat64(override[key]); ok && val > 0 {
				return time.Duration(val) * time.Second
			}
		}
		// 运行时静默覆盖（当没有匹配到具体 streamType 时作为 fallback）
		if val, ok := toFloat64(override["runtimeQuietSeconds"]); ok && val > 0 {
			return time.Duration(val) * time.Second
		}
	}

	switch strings.ToLower(strings.TrimSpace(binding.StreamType)) {
	case "trade_tick":
		return time.Duration(p.runtimePolicy.TradeTickFreshnessSeconds) * time.Second
	case "order_book":
		return time.Duration(p.runtimePolicy.OrderBookFreshnessSeconds) * time.Second
	case "signal_bar":
		return time.Duration(p.runtimePolicy.SignalBarFreshnessSeconds) * time.Second
	default:
		return time.Duration(p.runtimePolicy.RuntimeQuietSeconds) * time.Second
	}
}

func resolveRuntimeSourceStateEntry(sourceStates map[string]any, binding domain.AccountSignalBinding) map[string]any {
	if sourceStates == nil {
		return nil
	}
	if entry := mapValue(sourceStates[signalBindingKey(binding)]); entry != nil {
		return entry
	}
	bindingTimeframe := signalBindingTimeframe(binding.SourceKey, binding.Options)
	for _, raw := range sourceStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		if normalizeSignalSourceKey(stringValue(entry["sourceKey"])) != normalizeSignalSourceKey(binding.SourceKey) {
			continue
		}
		if normalizeSignalSourceRole(stringValue(entry["role"])) != normalizeSignalSourceRole(binding.Role) {
			continue
		}
		if NormalizeSymbol(stringValue(entry["symbol"])) != NormalizeSymbol(binding.Symbol) {
			continue
		}
		if bindingTimeframe != "" {
			entryTimeframe := normalizeSignalBarInterval(firstNonEmpty(
				stringValue(entry["timeframe"]),
				stringValue(mapValue(entry["summary"])["timeframe"]),
			))
			if entryTimeframe != "" && entryTimeframe != bindingTimeframe {
				continue
			}
		}
		return entry
	}
	return nil
}

func (p *Platform) evaluatePaperSignalDecision(session domain.PaperSession, summary map[string]any, sourceStates map[string]any, signalBarStates map[string]any, eventTime time.Time, nextPlannedEvent time.Time, nextPlannedPrice float64, nextPlannedSide, nextPlannedRole, nextPlannedReason string) (StrategyExecutionContext, StrategySignalDecision, error) {
	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, err
	}
	parameters, err := p.resolvePaperSessionParameters(session, version)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, err
	}
	engine, engineKey, err := p.resolveStrategyEngine(version.ID, parameters)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, err
	}
	executionContext := StrategyExecutionContext{
		StrategyEngineKey:   engineKey,
		StrategyVersionID:   version.ID,
		SignalTimeframe:     stringValue(parameters["signalTimeframe"]),
		ExecutionDataSource: stringValue(parameters["executionDataSource"]),
		Symbol:              stringValue(parameters["symbol"]),
		From:                parseOptionalRFC3339(stringValue(parameters["from"])),
		To:                  parseOptionalRFC3339(stringValue(parameters["to"])),
		Parameters:          parameters,
		Semantics:           defaultExecutionSemantics(ExecutionModePaper, parameters),
	}
	evaluator, ok := engine.(SignalEvaluatingStrategyEngine)
	if !ok {
		return executionContext, StrategySignalDecision{
			Action: "advance-plan",
			Reason: "engine-has-no-signal-evaluator",
		}, nil
	}
	currentPosition, _, err := p.resolvePaperSessionPositionSnapshot(session.AccountID, executionContext.Symbol)
	if err != nil {
		return executionContext, StrategySignalDecision{}, err
	}
	decision, err := evaluator.EvaluateSignal(StrategySignalEvaluationContext{
		ExecutionContext:  executionContext,
		PaperSessionID:    session.ID,
		TriggerSummary:    cloneMetadata(summary),
		SourceStates:      cloneMetadata(sourceStates),
		SignalBarStates:   cloneMetadata(signalBarStates),
		CurrentPosition:   currentPosition,
		EventTime:         eventTime.UTC(),
		NextPlannedEvent:  nextPlannedEvent.UTC(),
		NextPlannedPrice:  nextPlannedPrice,
		NextPlannedSide:   nextPlannedSide,
		NextPlannedRole:   nextPlannedRole,
		NextPlannedReason: nextPlannedReason,
	})
	if err != nil {
		return executionContext, StrategySignalDecision{}, err
	}
	if strings.TrimSpace(decision.Action) == "" {
		decision.Action = "wait"
	}
	if strings.TrimSpace(decision.Reason) == "" {
		decision.Reason = "unspecified"
	}
	return executionContext, decision, nil
}

func (p *Platform) resolvePaperSessionPositionSnapshot(accountID, symbol string) (map[string]any, bool, error) {
	position, found, err := p.store.FindPosition(accountID, symbol)
	if err != nil {
		return nil, false, err
	}
	if !found || math.Abs(position.Quantity) <= 1e-9 {
		return map[string]any{
			"symbol":   NormalizeSymbol(symbol),
			"found":    false,
			"quantity": 0.0,
		}, false, nil
	}
	return map[string]any{
		"id":         position.ID,
		"symbol":     position.Symbol,
		"side":       position.Side,
		"quantity":   position.Quantity,
		"entryPrice": position.EntryPrice,
		"markPrice":  position.MarkPrice,
		"updatedAt":  position.UpdatedAt.Format(time.RFC3339),
		"found":      true,
	}, true, nil
}

func buildStrategyEvaluationTriggerSource(summary map[string]any) map[string]any {
	return map[string]any{
		"sourceKey":  stringValue(summary["sourceKey"]),
		"role":       stringValue(summary["role"]),
		"streamType": stringValue(summary["streamType"]),
		"channel":    stringValue(summary["channel"]),
		"symbol":     NormalizeSymbol(firstNonEmpty(stringValue(summary["subscriptionSymbol"]), stringValue(summary["symbol"]))),
		"event":      stringValue(summary["event"]),
	}
}

// SetTickInterval 设置模拟盘后台循环的 Ticker 间隔（秒）。
func (p *Platform) SetTickInterval(seconds int) {
	if seconds > 0 {
		p.tickInterval = seconds
	}
}

// runPaperSessionLoop 后台循环执行策略计划，按 tickInterval 间隔逐步前进。
func (p *Platform) runPaperSessionLoop(ctx context.Context, session domain.PaperSession) {
	interval := p.tickInterval
	if interval <= 0 {
		interval = 15
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	defer p.removeRunner(session.ID)

	if !boolValue(session.State["signalRuntimeRequired"]) {
		_, _ = p.placePaperSessionOrder(session)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			current, err := p.store.GetPaperSession(session.ID)
			if err != nil || current.Status != "RUNNING" {
				return
			}
			if boolValue(current.State["signalRuntimeRequired"]) {
				continue
			}
			_, _ = p.placePaperSessionOrder(current)
		}
	}
}

// placePaperSessionOrder 从统一 StrategyEngine 生成的计划里取下一笔订单并执行。
func (p *Platform) placePaperSessionOrder(session domain.PaperSession) (domain.Order, error) {
	session, plan, err := p.ensurePaperExecutionPlan(session)
	if err != nil {
		return domain.Order{}, err
	}

	state := cloneMetadata(session.State)
	index := 0
	if value, ok := toFloat64(state["planIndex"]); ok && value >= 0 {
		index = int(value)
	}

	if index >= len(plan) {
		state["runner"] = "strategy-engine"
		state["runtimeMode"] = "canonical-strategy-engine"
		state["completedAt"] = time.Now().UTC().Format(time.RFC3339)
		state["planIndex"] = len(plan)
		state["planLength"] = len(plan)
		if _, err := p.store.UpdatePaperSessionState(session.ID, state); err != nil {
			return domain.Order{}, err
		}
		_, _ = p.store.UpdatePaperSessionStatus(session.ID, "STOPPED")
		return domain.Order{}, fmt.Errorf("模拟会话 %s 已完成所有策略计划订单", session.ID)
	}

	step := plan[index]
	state["runner"] = "strategy-engine"
	state["runtimeMode"] = "canonical-strategy-engine"
	state["planIndex"] = index + 1
	state["planLength"] = len(plan)
	state["lastEventTime"] = step.EventTime.UTC().Format(time.RFC3339)
	state["lastEventSide"] = step.Side
	state["lastEventRole"] = step.Role
	state["lastEventReason"] = step.Reason
	updatedSession, err := p.store.UpdatePaperSessionState(session.ID, state)
	if err != nil {
		return domain.Order{}, err
	}
	session = updatedSession

	order := domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: step.StrategyVersionID,
		Symbol:            step.Symbol,
		Side:              step.Side,
		Type:              firstNonEmpty(step.Type, "MARKET"),
		Quantity:          roundQuantity(step.Quantity),
		Price:             step.Price,
		Metadata:          cloneMetadata(step.Metadata),
	}
	created, err := p.CreateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}
	return created, nil
}

func (p *Platform) ensurePaperExecutionPlan(session domain.PaperSession) (domain.PaperSession, []paperPlannedOrder, error) {
	p.mu.Lock()
	if plan, ok := p.paperPlans[session.ID]; ok {
		p.mu.Unlock()
		return session, plan, nil
	}
	p.mu.Unlock()

	session, err := p.syncPaperSessionRuntime(session)
	if err != nil {
		return domain.PaperSession{}, nil, err
	}

	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return domain.PaperSession{}, nil, err
	}
	parameters, err := p.resolvePaperSessionParameters(session, version)
	if err != nil {
		return domain.PaperSession{}, nil, err
	}
	engine, engineKey, err := p.resolveStrategyEngine(version.ID, parameters)
	if err != nil {
		return domain.PaperSession{}, nil, err
	}

	if !p.hasExecutionDataset(stringValue(parameters["executionDataSource"]), stringValue(parameters["symbol"])) {
		return domain.PaperSession{}, nil, fmt.Errorf("no %s dataset found for symbol %s", stringValue(parameters["executionDataSource"]), stringValue(parameters["symbol"]))
	}

	from := parseOptionalRFC3339(stringValue(parameters["from"]))
	to := parseOptionalRFC3339(stringValue(parameters["to"]))

	semantics := defaultExecutionSemantics(ExecutionModePaper, parameters)
	result, err := engine.Run(StrategyExecutionContext{
		StrategyEngineKey:   engineKey,
		StrategyVersionID:   version.ID,
		SignalTimeframe:     stringValue(parameters["signalTimeframe"]),
		ExecutionDataSource: stringValue(parameters["executionDataSource"]),
		Symbol:              stringValue(parameters["symbol"]),
		From:                from,
		To:                  to,
		Parameters:          parameters,
		Semantics:           semantics,
	})
	if err != nil {
		return domain.PaperSession{}, nil, err
	}

	trades, err := executionTradesFromResult(result)
	if err != nil {
		return domain.PaperSession{}, nil, err
	}
	plan, err := buildPaperExecutionPlan(session, version, engineKey, semantics, trades)
	if err != nil {
		return domain.PaperSession{}, nil, err
	}

	p.mu.Lock()
	p.paperPlans[session.ID] = plan
	p.mu.Unlock()

	state := cloneMetadata(session.State)
	state["runner"] = "strategy-engine"
	state["runtimeMode"] = "canonical-strategy-engine"
	state["strategyVersionId"] = version.ID
	state["strategyEngine"] = engineKey
	state["signalTimeframe"] = stringValue(parameters["signalTimeframe"])
	state["executionDataSource"] = stringValue(parameters["executionDataSource"])
	state["symbol"] = stringValue(parameters["symbol"])
	state["executionMode"] = string(semantics.Mode)
	state["slippageMode"] = string(semantics.SlippageMode)
	state["tradingFeeBps"] = semantics.TradingFeeBps
	state["fundingRateBps"] = semantics.FundingRateBps
	state["fundingIntervalHours"] = semantics.FundingIntervalHours
	state["planLength"] = len(plan)
	state["planReadyAt"] = time.Now().UTC().Format(time.RFC3339)
	updatedSession, err := p.store.UpdatePaperSessionState(session.ID, state)
	if err != nil {
		return domain.PaperSession{}, nil, err
	}
	return updatedSession, plan, nil
}

func executionTradesFromResult(result map[string]any) ([]map[string]any, error) {
	raw, ok := result["executionTrades"]
	if !ok || raw == nil {
		return []map[string]any{}, nil
	}
	switch items := raw.(type) {
	case []map[string]any:
		return items, nil
	case []any:
		trades := make([]map[string]any, 0, len(items))
		for _, item := range items {
			mapped, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid execution trade payload")
			}
			trades = append(trades, mapped)
		}
		return trades, nil
	default:
		return nil, fmt.Errorf("unsupported executionTrades payload type")
	}
}

func buildPaperExecutionPlan(session domain.PaperSession, version domain.StrategyVersion, engineKey string, semantics StrategyExecutionSemantics, trades []map[string]any) ([]paperPlannedOrder, error) {
	plan := make([]paperPlannedOrder, 0, len(trades)*2)
	for _, trade := range trades {
		entryTime := parseOptionalRFC3339(stringValue(trade["entryTime"]))
		if entryTime.IsZero() {
			return nil, fmt.Errorf("invalid execution trade entry time")
		}
		entryPrice := parseFloatValue(trade["entryPrice"])
		quantity := parseFloatValue(trade["quantity"])
		if quantity <= 0 {
			notional := parseFloatValue(trade["notional"])
			if entryPrice > 0 && notional > 0 {
				quantity = notional / entryPrice
			}
		}
		if quantity <= 0 || entryPrice <= 0 {
			continue
		}
		entrySide := normalizePaperPlanSide(stringValue(trade["side"]))
		if entrySide == "" {
			continue
		}
		symbol := normalizeBacktestSymbol(stringValue(trade["symbol"]))
		if symbol == "" {
			symbol = resolvePaperPlanSymbol(version)
		}

		entryReason := firstNonEmpty(stringValue(trade["entryReason"]), "StrategyEntry")
		entryFee := parseFloatValue(trade["entryTradingFee"])
		plan = append(plan, paperPlannedOrder{
			StrategyVersionID: version.ID,
			Symbol:            symbol,
			Side:              entrySide,
			Type:              "MARKET",
			Quantity:          quantity,
			Price:             entryPrice,
			EventTime:         entryTime,
			Reason:            entryReason,
			Role:              "entry",
			FeeAmount:         entryFee,
			Metadata: map[string]any{
				"markPrice":        entryPrice,
				"source":           "paper-session-strategy-engine",
				"paperSession":     session.ID,
				"strategyId":       session.StrategyID,
				"strategyEngine":   engineKey,
				"eventTime":        entryTime.UTC().Format(time.RFC3339),
				"reason":           entryReason,
				"orderRole":        "entry",
				"paperFeeAmount":   entryFee,
				"tradingFeeAmount": entryFee,
				"tradingFeeBps":    semantics.TradingFeeBps,
				"fundingRateBps":   semantics.FundingRateBps,
				"slippageMode":     string(semantics.SlippageMode),
			},
		})

		exitTime := parseOptionalRFC3339(stringValue(trade["exitTime"]))
		if exitTime.IsZero() {
			continue
		}
		exitPrice := parseFloatValue(trade["exitPrice"])
		if exitPrice <= 0 {
			continue
		}
		exitFee := parseFloatValue(trade["exitTradingFee"])
		fundingPnL := parseFloatValue(trade["fundingPnL"])
		exitReason := firstNonEmpty(stringValue(trade["exitType"]), "StrategyExit")
		plan = append(plan, paperPlannedOrder{
			StrategyVersionID: version.ID,
			Symbol:            symbol,
			Side:              oppositePaperPlanSide(entrySide),
			Type:              "MARKET",
			Quantity:          quantity,
			Price:             exitPrice,
			EventTime:         exitTime,
			Reason:            exitReason,
			Role:              "exit",
			FeeAmount:         exitFee - fundingPnL,
			FundingPnL:        fundingPnL,
			Metadata: map[string]any{
				"markPrice":        exitPrice,
				"source":           "paper-session-strategy-engine",
				"paperSession":     session.ID,
				"strategyId":       session.StrategyID,
				"strategyEngine":   engineKey,
				"eventTime":        exitTime.UTC().Format(time.RFC3339),
				"reason":           exitReason,
				"orderRole":        "exit",
				"paperFeeAmount":   exitFee - fundingPnL,
				"tradingFeeAmount": exitFee,
				"fundingPnL":       fundingPnL,
				"tradingFeeBps":    semantics.TradingFeeBps,
				"fundingRateBps":   semantics.FundingRateBps,
				"slippageMode":     string(semantics.SlippageMode),
			},
		})
	}
	return plan, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (p *Platform) syncPaperSessionRuntime(session domain.PaperSession) (domain.PaperSession, error) {
	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return domain.PaperSession{}, err
	}
	parameters, err := p.resolvePaperSessionParameters(session, version)
	if err != nil {
		return domain.PaperSession{}, err
	}
	_, engineKey, err := p.resolveStrategyEngine(version.ID, parameters)
	if err != nil {
		return domain.PaperSession{}, err
	}
	semantics := defaultExecutionSemantics(ExecutionModePaper, parameters)

	state := cloneMetadata(session.State)
	state["runner"] = "strategy-engine"
	state["runtimeMode"] = "canonical-strategy-engine"
	state["strategyVersionId"] = version.ID
	state["strategyEngine"] = engineKey
	state["signalTimeframe"] = stringValue(parameters["signalTimeframe"])
	state["executionDataSource"] = stringValue(parameters["executionDataSource"])
	state["symbol"] = stringValue(parameters["symbol"])
	state["executionMode"] = string(semantics.Mode)
	state["slippageMode"] = string(semantics.SlippageMode)
	state["tradingFeeBps"] = semantics.TradingFeeBps
	state["fundingRateBps"] = semantics.FundingRateBps
	state["fundingIntervalHours"] = semantics.FundingIntervalHours
	if _, ok := state["planIndex"]; !ok {
		state["planIndex"] = 0
	}

	if updatedState, err := p.syncPaperSignalRuntimeState(session, parameters, state); err == nil {
		state = updatedState
	} else {
		return domain.PaperSession{}, err
	}

	updatedSession, err := p.store.UpdatePaperSessionState(session.ID, state)
	if err != nil {
		return domain.PaperSession{}, err
	}
	return updatedSession, nil
}

func (p *Platform) syncPaperSignalRuntimeState(session domain.PaperSession, parameters map[string]any, state map[string]any) (map[string]any, error) {
	state = cloneMetadata(state)
	executionDataSource := stringValue(parameters["executionDataSource"])
	strategyBindings, err := p.ListStrategySignalBindings(session.StrategyID)
	if err != nil {
		return nil, err
	}
	hasBindings := len(strategyBindings) > 0
	state["signalRuntimeMode"] = "detached"
	state["signalRuntimeRequired"] = false
	if !hasBindings {
		delete(state, "signalRuntimePlan")
		delete(state, "signalRuntimeSessionId")
		delete(state, "signalRuntimeStatus")
		return state, nil
	}

	plan, err := p.BuildSignalRuntimePlan(session.AccountID, session.StrategyID)
	if err != nil {
		return nil, err
	}
	state["signalRuntimePlan"] = plan
	state["signalRuntimeMode"] = "linked"
	required := executionDataSource == "tick"
	state["signalRuntimeRequired"] = required
	state["signalRuntimeReady"] = boolValue(plan["ready"])

	runtimeSessionID := stringValue(state["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		runtimeSession, err := p.CreateSignalRuntimeSession(session.AccountID, session.StrategyID)
		if err != nil {
			if required {
				return nil, err
			}
			state["signalRuntimeStatus"] = "ERROR"
			state["signalRuntimeError"] = err.Error()
			return state, nil
		}
		runtimeSessionID = runtimeSession.ID
		state["signalRuntimeSessionId"] = runtimeSession.ID
		state["signalRuntimeStatus"] = runtimeSession.Status
	} else {
		runtimeSession, err := p.GetSignalRuntimeSession(runtimeSessionID)
		if err == nil {
			state["signalRuntimeStatus"] = runtimeSession.Status
		}
	}

	if required && !boolValue(plan["ready"]) {
		state["signalRuntimeStatus"] = "BLOCKED"
	}
	return state, nil
}

func (p *Platform) ensurePaperSignalRuntimeStarted(session domain.PaperSession) (domain.PaperSession, error) {
	state := cloneMetadata(session.State)
	if !boolValue(state["signalRuntimeRequired"]) {
		return session, nil
	}
	if !boolValue(state["signalRuntimeReady"]) {
		return domain.PaperSession{}, fmt.Errorf("paper session %s requires a ready signal runtime plan before start", session.ID)
	}
	runtimeSessionID := stringValue(state["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		return domain.PaperSession{}, fmt.Errorf("paper session %s has no linked signal runtime session", session.ID)
	}
	runtimeSession, err := p.StartSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		return domain.PaperSession{}, err
	}
	state["signalRuntimeStatus"] = runtimeSession.Status
	state["signalRuntimeSessionId"] = runtimeSession.ID
	session, err = p.store.UpdatePaperSessionState(session.ID, state)
	if err != nil {
		return domain.PaperSession{}, err
	}
	session, err = p.awaitPaperSignalRuntimeReadiness(session, runtimeSession.ID, time.Duration(p.runtimePolicy.PaperStartReadinessTimeoutSecs)*time.Second)
	if err != nil {
		return domain.PaperSession{}, err
	}
	return session, nil
}

func (p *Platform) awaitPaperSignalRuntimeReadiness(session domain.PaperSession, runtimeSessionID string, timeout time.Duration) (domain.PaperSession, error) {
	if !boolValue(session.State["signalRuntimeRequired"]) {
		return session, nil
	}
	deadline := time.Now().Add(timeout)
	var lastGate map[string]any
	for {
		runtimeSession, err := p.GetSignalRuntimeSession(runtimeSessionID)
		if err != nil {
			return domain.PaperSession{}, err
		}

		lastGate = p.evaluateSignalSourceReadiness(session, runtimeSession, time.Now().UTC())
		state := cloneMetadata(session.State)
		state["signalRuntimeStatus"] = runtimeSession.Status
		state["signalRuntimeStartReadiness"] = lastGate
		state["signalRuntimeLastCheckedAt"] = time.Now().UTC().Format(time.RFC3339)
		session, err = p.store.UpdatePaperSessionState(session.ID, state)
		if err != nil {
			return domain.PaperSession{}, err
		}

		if boolValue(lastGate["ready"]) {
			return session, nil
		}
		if strings.EqualFold(runtimeSession.Status, "ERROR") {
			return domain.PaperSession{}, fmt.Errorf("paper session %s linked signal runtime entered error state during start", session.ID)
		}
		if time.Now().After(deadline) {
			return domain.PaperSession{}, fmt.Errorf(
				"paper session %s linked signal runtime not ready before start: missing=%d stale=%d",
				session.ID,
				len(metadataList(lastGate["missing"])),
				len(metadataList(lastGate["stale"])),
			)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func (p *Platform) stopLinkedSignalRuntime(session domain.PaperSession) (domain.SignalRuntimeSession, error) {
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		return domain.SignalRuntimeSession{}, fmt.Errorf("paper session %s has no linked signal runtime session", session.ID)
	}
	runtimeSession, err := p.StopSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeStatus"] = runtimeSession.Status
	_, _ = p.store.UpdatePaperSessionState(session.ID, state)
	return runtimeSession, nil
}

func (p *Platform) resolveCurrentStrategyVersion(strategyID string) (domain.StrategyVersion, error) {
	items, err := p.ListStrategies()
	if err != nil {
		return domain.StrategyVersion{}, err
	}
	for _, item := range items {
		if stringValue(item["id"]) != strategyID {
			continue
		}
		switch currentVersion := item["currentVersion"].(type) {
		case domain.StrategyVersion:
			return currentVersion, nil
		case map[string]any:
			return domain.StrategyVersion{
				ID:                 stringValue(currentVersion["id"]),
				StrategyID:         strategyID,
				Version:            stringValue(currentVersion["version"]),
				SignalTimeframe:    stringValue(currentVersion["signalTimeframe"]),
				ExecutionTimeframe: stringValue(currentVersion["executionTimeframe"]),
				Parameters:         cloneMetadata(mapValue(currentVersion["parameters"])),
			}, nil
		}
	}
	return domain.StrategyVersion{}, fmt.Errorf("strategy version not found for strategy %s", strategyID)
}

func (p *Platform) resolvePaperSessionParameters(session domain.PaperSession, version domain.StrategyVersion) (map[string]any, error) {
	parameters := cloneMetadata(version.Parameters)
	if parameters == nil {
		parameters = map[string]any{}
	}
	if stringValue(parameters["signalTimeframe"]) == "" {
		parameters["signalTimeframe"] = normalizePaperSignalTimeframe(version.SignalTimeframe)
	}
	if stringValue(parameters["executionDataSource"]) == "" {
		parameters["executionDataSource"] = normalizePaperExecutionSource(version.ExecutionTimeframe)
	}
	if stringValue(parameters["symbol"]) == "" {
		parameters["symbol"] = resolvePaperPlanSymbol(version)
	}
	state := cloneMetadata(session.State)
	for _, key := range []string{
		"signalTimeframe",
		"executionDataSource",
		"symbol",
		"from",
		"to",
		"strategyEngine",
		"tradingFeeBps",
		"fundingRateBps",
		"fundingIntervalHours",
		"stop_mode",
		"stop_loss_atr",
		"profit_protect_atr",
		"long_reentry_atr",
		"short_reentry_atr",
		"reentry_size_schedule",
		"trailing_stop_atr",
		"delayed_trailing_activation_atr",
		"reentry_decay_factor",
		"max_trades_per_bar",
		"fixed_slippage",
		"dir2_zero_initial",
		"zero_initial_mode",
	} {
		if value, ok := state[key]; ok {
			parameters[key] = value
		}
	}
	return NormalizeBacktestParameters(parameters)
}

func normalizePaperSignalTimeframe(value string) string {
	normalized := normalizeSignalBarInterval(value)
	switch normalized {
	case "5m", "4h", "1d":
		return normalized
	default:
		return "1d"
	}
}

func normalizePaperExecutionSource(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "tick", "1min":
		return value
	case "1m", "1":
		return "1min"
	default:
		return "1min"
	}
}

func normalizePaperSessionOverrides(overrides map[string]any) map[string]any {
	normalized := map[string]any{}
	for key, value := range overrides {
		switch key {
		case "signalTimeframe":
			if timeframe := normalizePaperSignalTimeframe(stringValue(value)); timeframe != "" {
				normalized[key] = timeframe
			}
		case "executionDataSource":
			if source := normalizePaperExecutionSource(stringValue(value)); source != "" {
				normalized[key] = source
			}
		case "symbol":
			if symbol := normalizeBacktestSymbol(stringValue(value)); symbol != "" {
				normalized[key] = symbol
			}
		case "from", "to":
			if parsed := parseOptionalRFC3339(stringValue(value)); !parsed.IsZero() {
				normalized[key] = parsed.Format(time.RFC3339)
			}
		case "strategyEngine":
			normalized[key] = normalizeStrategyEngineKey(stringValue(value))
		case "tradingFeeBps", "fundingRateBps", "fixed_slippage", "stop_loss_atr", "profit_protect_atr",
			"long_reentry_atr", "short_reentry_atr", "trailing_stop_atr", "delayed_trailing_activation_atr",
			"reentry_decay_factor":
			normalized[key] = parseFloatValue(value)
		case "fundingIntervalHours", "max_trades_per_bar":
			normalized[key] = maxIntValue(value, 0)
		case "dir2_zero_initial":
			normalized[key] = boolValue(value)
		case "stop_mode":
			mode := strings.ToLower(strings.TrimSpace(stringValue(value)))
			if mode != "" {
				normalized[key] = mode
			}
		case "zero_initial_mode":
			if mode := normalizeStrategyZeroInitialMode(stringValue(value)); mode != "" {
				normalized[key] = mode
			}
		case "reentry_size_schedule":
			switch schedule := value.(type) {
			case []float64:
				normalized[key] = append([]float64(nil), schedule...)
			case []any:
				normalized[key] = append([]any(nil), schedule...)
			case []string:
				normalized[key] = append([]string(nil), schedule...)
			default:
				normalized[key] = value
			}
		}
	}
	return normalized
}

func normalizePaperPlanSide(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "BUY", "LONG":
		return "BUY"
	case "SHORT", "SELL":
		return "SELL"
	default:
		return ""
	}
}

func oppositePaperPlanSide(value string) string {
	if strings.EqualFold(value, "BUY") {
		return "SELL"
	}
	return "BUY"
}

func resolvePaperPlanSymbol(version domain.StrategyVersion) string {
	if symbol := normalizeBacktestSymbol(stringValue(version.Parameters["symbol"])); symbol != "" {
		return symbol
	}
	return "BTCUSDT"
}

func mapValue(value any) map[string]any {
	if value == nil {
		return nil
	}
	switch mapped := value.(type) {
	case map[string]any:
		return mapped
	default:
		return nil
	}
}

// loadReplayLedger 继续保留给图表与旧审计能力使用。
func (p *Platform) loadReplayLedger() ([]strategyReplayEvent, error) {
	p.once.Do(func() {
		p.ledger, p.ledgerErr = readStrategyReplayLedger("FINAL_1D_LEDGER_BEST_SL.csv")
	})
	return p.ledger, p.ledgerErr
}

func readStrategyReplayLedger(path string) ([]strategyReplayEvent, error) {
	resolved := path
	if !filepath.IsAbs(path) {
		_, currentFile, _, _ := runtime.Caller(0)
		resolved = filepath.Join(filepath.Dir(currentFile), "..", "..", path)
	}

	file, err := os.Open(filepath.Clean(resolved))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, fmt.Errorf("策略回放账本为空: %s", resolved)
	}

	events := make([]strategyReplayEvent, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}
		eventTime, err := time.Parse("2006-01-02 15:04:05Z07:00", row[0])
		if err != nil {
			return nil, fmt.Errorf("解析回放时间 %q: %w", row[0], err)
		}
		price, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, fmt.Errorf("解析回放价格 %q: %w", row[2], err)
		}
		notional, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, fmt.Errorf("解析回放名义金额 %q: %w", row[4], err)
		}
		balance, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, fmt.Errorf("解析回放余额 %q: %w", row[5], err)
		}
		events = append(events, strategyReplayEvent{
			Time:     eventTime.UTC(),
			Type:     strings.ToUpper(strings.TrimSpace(row[1])),
			Price:    price,
			Reason:   strings.TrimSpace(row[3]),
			Notional: notional,
			Balance:  balance,
		})
	}

	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
	return events, nil
}

// resolveStrategyVersionID 从策略 ID 查找当前版本 ID。
func (p *Platform) resolveStrategyVersionID(strategyID string) (string, error) {
	version, err := p.resolveCurrentStrategyVersion(strategyID)
	if err != nil {
		return "", err
	}
	return version.ID, nil
}

// removeRunner 从运行中列表移除指定会话。
func (p *Platform) removeRunner(sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.run, sessionID)
}

// roundQuantity 将数量精确到小数点后 6 位。
func roundQuantity(quantity float64) float64 {
	return math.Round(quantity*1_000_000) / 1_000_000
}
