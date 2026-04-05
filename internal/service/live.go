package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func (p *Platform) ListLiveSessions() ([]domain.LiveSession, error) {
	return p.store.ListLiveSessions()
}

func (p *Platform) CreateLiveSession(accountID, strategyID string, overrides map[string]any) (domain.LiveSession, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.LiveSession{}, fmt.Errorf("live session requires a LIVE account: %s", accountID)
	}

	session, err := p.store.CreateLiveSession(accountID, strategyID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if len(overrides) > 0 {
		state := cloneMetadata(session.State)
		for key, value := range normalizePaperSessionOverrides(overrides) {
			state[key] = value
		}
		session, err = p.store.UpdateLiveSessionState(session.ID, state)
		if err != nil {
			return domain.LiveSession{}, err
		}
	}
	return p.syncLiveSessionRuntime(session)
}

func (p *Platform) StartLiveSession(sessionID string) (domain.LiveSession, error) {
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	account, err := p.store.GetAccount(session.AccountID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	if !strings.EqualFold(account.Mode, "LIVE") {
		return domain.LiveSession{}, fmt.Errorf("live session %s is not bound to a LIVE account", session.ID)
	}
	if account.Status != "CONFIGURED" && account.Status != "READY" {
		return domain.LiveSession{}, fmt.Errorf("live account %s is not configured", account.ID)
	}
	if _, _, err := p.resolveLiveAdapterForAccount(account); err != nil {
		return domain.LiveSession{}, err
	}

	session, err = p.syncLiveSessionRuntime(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	session, err = p.ensureLiveSessionSignalRuntimeStarted(session)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.store.UpdateLiveSessionStatus(sessionID, "RUNNING")
}

func (p *Platform) DispatchLiveSessionIntent(sessionID string) (domain.Order, error) {
	session, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.Order{}, err
	}
	if !strings.EqualFold(session.Status, "RUNNING") && !strings.EqualFold(session.Status, "READY") {
		return domain.Order{}, fmt.Errorf("live session %s is not dispatchable in status %s", session.ID, session.Status)
	}

	intent := cloneMetadata(mapValue(session.State["lastStrategyIntent"]))
	if len(intent) == 0 {
		return domain.Order{}, fmt.Errorf("live session %s has no ready intent", session.ID)
	}
	side := strings.ToUpper(strings.TrimSpace(stringValue(intent["side"])))
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(stringValue(intent["type"]), "MARKET")))
	symbol := NormalizeSymbol(firstNonEmpty(stringValue(intent["symbol"]), stringValue(session.State["symbol"])))
	priceHint := parseFloatValue(intent["priceHint"])
	quantity := firstPositive(parseFloatValue(intent["quantity"]), firstPositive(parseFloatValue(session.State["defaultOrderQuantity"]), 0.001))

	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return domain.Order{}, err
	}
	order := domain.Order{
		AccountID:         session.AccountID,
		StrategyVersionID: version.ID,
		Symbol:            symbol,
		Side:              side,
		Type:              orderType,
		Quantity:          quantity,
		Price:             priceHint,
		Metadata: map[string]any{
			"source":        "live-session-intent",
			"liveSessionId": session.ID,
			"signalKind":    stringValue(intent["signalKind"]),
			"dispatchMode":  stringValue(session.State["dispatchMode"]),
			"intent":        cloneMetadata(intent),
		},
	}
	created, err := p.CreateOrder(order)
	if err != nil {
		return domain.Order{}, err
	}

	state := cloneMetadata(session.State)
	state["lastDispatchedOrderId"] = created.ID
	state["lastDispatchedAt"] = time.Now().UTC().Format(time.RFC3339)
	state["lastDispatchedIntent"] = intent
	delete(state, "lastStrategyIntent")
	appendTimelineEvent(state, "order", time.Now().UTC(), "live-intent-dispatched", map[string]any{
		"orderId": created.ID,
		"side":    created.Side,
		"symbol":  created.Symbol,
		"price":   created.Price,
	})
	_, _ = p.store.UpdateLiveSessionState(session.ID, state)
	return created, nil
}

func (p *Platform) StopLiveSession(sessionID string) (domain.LiveSession, error) {
	session, err := p.store.UpdateLiveSessionStatus(sessionID, "STOPPED")
	if err != nil {
		return domain.LiveSession{}, err
	}
	_, _ = p.stopLinkedLiveSignalRuntime(session)
	return session, nil
}

func (p *Platform) triggerLiveSessionFromSignal(sessionID, runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	session, err := p.store.GetLiveSession(sessionID)
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
	updatedSession, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return err
	}
	return p.evaluateLiveSessionOnSignal(updatedSession, runtimeSessionID, summary, eventTime)
}

func (p *Platform) evaluateLiveSessionOnSignal(session domain.LiveSession, runtimeSessionID string, summary map[string]any, eventTime time.Time) error {
	session, err := p.syncLiveSessionRuntime(session)
	if err != nil {
		return err
	}

	state := cloneMetadata(session.State)
	state["strategyEvaluationMode"] = "signal-runtime-heartbeat"
	state["lastStrategyEvaluationAt"] = eventTime.UTC().Format(time.RFC3339)
	state["lastStrategyEvaluationTrigger"] = cloneMetadata(summary)
	state["lastStrategyEvaluationTriggerSource"] = buildStrategyEvaluationTriggerSource(summary)
	state["lastStrategyEvaluationStatus"] = "evaluated"

	sourceGate := map[string]any{
		"ready":   false,
		"missing": []any{},
		"stale":   []any{},
	}
	sourceStates := map[string]any{}
	signalBarStates := map[string]any{}
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
		sourceGate = p.evaluateRuntimeSignalSourceReadiness(session.StrategyID, runtimeSession, eventTime)
		state["lastStrategyEvaluationSourceGate"] = sourceGate
	}
	if !boolValue(sourceGate["ready"]) {
		state["lastStrategyEvaluationStatus"] = "waiting-source-states"
		appendTimelineEvent(state, "strategy", eventTime, "waiting-source-states", map[string]any{
			"missing": len(metadataList(sourceGate["missing"])),
			"stale":   len(metadataList(sourceGate["stale"])),
		})
		_, err := p.store.UpdateLiveSessionState(session.ID, state)
		return err
	}

	executionContext, decision, err := p.evaluateLiveSignalDecision(session, summary, sourceStates, signalBarStates, eventTime)
	if err != nil {
		state["lastStrategyEvaluationStatus"] = "decision-error"
		state["lastStrategyDecision"] = map[string]any{
			"action": "error",
			"reason": err.Error(),
		}
		appendTimelineEvent(state, "strategy", eventTime, "decision-error", map[string]any{"error": err.Error()})
		_, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
		if updateErr != nil {
			return updateErr
		}
		return err
	}

	intent := deriveLiveSessionIntent(decision, executionContext.Symbol)
	state["lastStrategyDecision"] = map[string]any{
		"action":   decision.Action,
		"reason":   decision.Reason,
		"metadata": cloneMetadata(decision.Metadata),
	}
	state["lastStrategyEvaluationContext"] = map[string]any{
		"strategyVersionId":   executionContext.StrategyVersionID,
		"signalTimeframe":     executionContext.SignalTimeframe,
		"executionDataSource": executionContext.ExecutionDataSource,
		"symbol":              executionContext.Symbol,
	}
	if intent != nil {
		state["lastStrategyIntent"] = intent
	} else {
		delete(state, "lastStrategyIntent")
	}
	appendTimelineEvent(state, "strategy", eventTime, "decision", map[string]any{
		"action":        decision.Action,
		"reason":        decision.Reason,
		"decisionState": stringValue(decision.Metadata["decisionState"]),
		"signalKind":    stringValue(decision.Metadata["signalKind"]),
		"intent":        cloneMetadata(intent),
	})
	if intent != nil {
		state["lastStrategyEvaluationStatus"] = "intent-ready"
	} else if decision.Action == "advance-plan" {
		state["lastStrategyEvaluationStatus"] = "monitoring"
	} else {
		state["lastStrategyEvaluationStatus"] = "waiting-decision"
	}
	_, err = p.store.UpdateLiveSessionState(session.ID, state)
	return err
}

func (p *Platform) evaluateLiveSignalDecision(session domain.LiveSession, summary map[string]any, sourceStates map[string]any, signalBarStates map[string]any, eventTime time.Time) (StrategyExecutionContext, StrategySignalDecision, error) {
	version, err := p.resolveCurrentStrategyVersion(session.StrategyID)
	if err != nil {
		return StrategyExecutionContext{}, StrategySignalDecision{}, err
	}
	parameters, err := p.resolveLiveSessionParameters(session, version)
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
		Semantics:           defaultExecutionSemantics(ExecutionModeLive, parameters),
	}
	evaluator, ok := engine.(SignalEvaluatingStrategyEngine)
	if !ok {
		return executionContext, StrategySignalDecision{
			Action: "wait",
			Reason: "engine-has-no-signal-evaluator",
		}, nil
	}
	currentPosition, _, err := p.resolvePaperSessionPositionSnapshot(session.AccountID, executionContext.Symbol)
	if err != nil {
		return executionContext, StrategySignalDecision{}, err
	}
	decision, err := evaluator.EvaluateSignal(StrategySignalEvaluationContext{
		ExecutionContext: executionContext,
		TriggerSummary:   cloneMetadata(summary),
		SourceStates:     cloneMetadata(sourceStates),
		SignalBarStates:  cloneMetadata(signalBarStates),
		CurrentPosition:  currentPosition,
		EventTime:        eventTime.UTC(),
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

func (p *Platform) syncLiveSessionRuntime(session domain.LiveSession) (domain.LiveSession, error) {
	state := cloneMetadata(session.State)
	plan, err := p.BuildSignalRuntimePlan(session.AccountID, session.StrategyID)
	if err != nil {
		state["signalRuntimeMode"] = "detached"
		state["signalRuntimeRequired"] = false
		state["signalRuntimeStatus"] = "ERROR"
		state["signalRuntimeError"] = err.Error()
		updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
		if updateErr != nil {
			return domain.LiveSession{}, updateErr
		}
		return updated, err
	}

	required := len(metadataList(plan["requiredBindings"])) > 0
	state["signalRuntimePlan"] = plan
	state["signalRuntimeMode"] = "linked"
	state["signalRuntimeRequired"] = required
	state["signalRuntimeReady"] = boolValue(plan["ready"])
	state["dispatchMode"] = firstNonEmpty(stringValue(state["dispatchMode"]), "manual-review")

	runtimeSessionID := stringValue(state["signalRuntimeSessionId"])
	if runtimeSessionID == "" && required {
		runtimeSession, createErr := p.CreateSignalRuntimeSession(session.AccountID, session.StrategyID)
		if createErr != nil {
			state["signalRuntimeStatus"] = "ERROR"
			state["signalRuntimeError"] = createErr.Error()
			updated, updateErr := p.store.UpdateLiveSessionState(session.ID, state)
			if updateErr != nil {
				return domain.LiveSession{}, updateErr
			}
			return updated, createErr
		}
		runtimeSessionID = runtimeSession.ID
		state["signalRuntimeSessionId"] = runtimeSession.ID
		state["signalRuntimeStatus"] = runtimeSession.Status
	} else if runtimeSessionID != "" {
		runtimeSession, getErr := p.GetSignalRuntimeSession(runtimeSessionID)
		if getErr == nil {
			state["signalRuntimeStatus"] = runtimeSession.Status
		}
	}

	updated, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return updated, nil
}

func (p *Platform) ensureLiveSessionSignalRuntimeStarted(session domain.LiveSession) (domain.LiveSession, error) {
	if !boolValue(session.State["signalRuntimeRequired"]) {
		return session, nil
	}
	if !boolValue(session.State["signalRuntimeReady"]) {
		return session, fmt.Errorf("live session %s signal runtime plan is not ready", session.ID)
	}
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		return domain.LiveSession{}, fmt.Errorf("live session %s has no linked signal runtime session", session.ID)
	}
	runtimeSession, err := p.StartSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		return domain.LiveSession{}, err
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeStatus"] = runtimeSession.Status
	state["signalRuntimeSessionId"] = runtimeSession.ID
	session, err = p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return p.awaitLiveSignalRuntimeReadiness(session, runtimeSession.ID, time.Duration(p.runtimePolicy.PaperStartReadinessTimeoutSecs)*time.Second)
}

func (p *Platform) awaitLiveSignalRuntimeReadiness(session domain.LiveSession, runtimeSessionID string, timeout time.Duration) (domain.LiveSession, error) {
	deadline := time.Now().Add(timeout)
	lastGate := map[string]any{}
	for time.Now().Before(deadline) || time.Now().Equal(deadline) {
		runtimeSession, err := p.GetSignalRuntimeSession(runtimeSessionID)
		if err != nil {
			return domain.LiveSession{}, err
		}
		lastGate = p.evaluateRuntimeSignalSourceReadiness(session.StrategyID, runtimeSession, time.Now().UTC())
		if boolValue(lastGate["ready"]) {
			state := cloneMetadata(session.State)
			state["signalRuntimeStatus"] = runtimeSession.Status
			state["signalRuntimeStartReadiness"] = lastGate
			state["signalRuntimeLastCheckedAt"] = time.Now().UTC().Format(time.RFC3339)
			return p.store.UpdateLiveSessionState(session.ID, state)
		}
		time.Sleep(250 * time.Millisecond)
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeStartReadiness"] = lastGate
	state["signalRuntimeLastCheckedAt"] = time.Now().UTC().Format(time.RFC3339)
	updated, err := p.store.UpdateLiveSessionState(session.ID, state)
	if err != nil {
		return domain.LiveSession{}, err
	}
	return updated, fmt.Errorf("live session %s runtime readiness timed out", session.ID)
}

func (p *Platform) stopLinkedLiveSignalRuntime(session domain.LiveSession) (domain.SignalRuntimeSession, error) {
	runtimeSessionID := stringValue(session.State["signalRuntimeSessionId"])
	if runtimeSessionID == "" {
		return domain.SignalRuntimeSession{}, fmt.Errorf("live session %s has no linked signal runtime session", session.ID)
	}
	runtimeSession, err := p.StopSignalRuntimeSession(runtimeSessionID)
	if err != nil {
		return domain.SignalRuntimeSession{}, err
	}
	state := cloneMetadata(session.State)
	state["signalRuntimeStatus"] = runtimeSession.Status
	_, _ = p.store.UpdateLiveSessionState(session.ID, state)
	return runtimeSession, nil
}

func (p *Platform) resolveLiveSessionParameters(session domain.LiveSession, version domain.StrategyVersion) (map[string]any, error) {
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
		"fixed_slippage",
	} {
		if value, ok := state[key]; ok {
			parameters[key] = value
		}
	}
	return NormalizeBacktestParameters(parameters)
}

func deriveLiveSessionIntent(decision StrategySignalDecision, symbol string) map[string]any {
	meta := cloneMetadata(decision.Metadata)
	signalBarDecision := mapValue(meta["signalBarDecision"])
	if strings.TrimSpace(decision.Action) != "advance-plan" || signalBarDecision == nil {
		return nil
	}
	longReady := boolValue(signalBarDecision["longReady"])
	shortReady := boolValue(signalBarDecision["shortReady"])
	marketPrice := parseFloatValue(meta["marketPrice"])
	signalKind := stringValue(meta["signalKind"])
	switch {
	case longReady && !shortReady:
		return map[string]any{
			"action":     "entry",
			"side":       "BUY",
			"type":       "MARKET",
			"symbol":     NormalizeSymbol(symbol),
			"priceHint":  marketPrice,
			"signalKind": signalKind,
		}
	case shortReady && !longReady:
		return map[string]any{
			"action":     "entry",
			"side":       "SELL",
			"type":       "MARKET",
			"symbol":     NormalizeSymbol(symbol),
			"priceHint":  marketPrice,
			"signalKind": signalKind,
		}
	default:
		return nil
	}
}
