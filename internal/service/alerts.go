package service

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type runtimeSourceSummary struct {
	tradeTickCount int
	orderBookCount int
	staleCount     int
	latestEventAt  time.Time
}

func (p *Platform) ListAlerts() ([]domain.PlatformAlert, error) {
	accounts, err := p.ListAccounts()
	if err != nil {
		return nil, err
	}
	paperSessions, err := p.ListPaperSessions()
	if err != nil {
		return nil, err
	}
	liveSessions, err := p.ListLiveSessions()
	if err != nil {
		return nil, err
	}
	strategies, err := p.ListStrategies()
	if err != nil {
		return nil, err
	}
	runtimeSessions := p.ListSignalRuntimeSessions()

	accountByID := make(map[string]domain.Account, len(accounts))
	for _, account := range accounts {
		accountByID[account.ID] = account
	}

	strategyNameByID := make(map[string]string, len(strategies))
	for _, strategy := range strategies {
		strategyNameByID[stringValue(strategy["id"])] = stringValue(strategy["name"])
	}

	runtimeByKey := make(map[string]domain.SignalRuntimeSession, len(runtimeSessions))
	for _, session := range runtimeSessions {
		runtimeByKey[runtimeSessionLookupKey(session.AccountID, session.StrategyID)] = session
	}

	alerts := make([]domain.PlatformAlert, 0, 32)
	seen := make(map[string]struct{})
	appendAlert := func(alert domain.PlatformAlert) {
		key := strings.Join([]string{
			alert.Scope,
			alert.Level,
			alert.Title,
			alert.Detail,
			alert.AccountID,
			alert.StrategyID,
			alert.PaperSessionID,
			alert.RuntimeSessionID,
		}, "|")
		if _, ok := seen[key]; ok {
			return
		}
		if alert.EventTime.IsZero() {
			alert.EventTime = time.Now().UTC()
		}
		seen[key] = struct{}{}
		alerts = append(alerts, alert)
	}

	for _, session := range runtimeSessions {
		account := accountByID[session.AccountID]
		sourceSummary := p.summarizeRuntimeSources(session)
		state := cloneMetadata(session.State)
		health := strings.ToLower(strings.TrimSpace(stringValue(state["health"])))
		if strings.EqualFold(session.Status, "ERROR") || (health != "" && health != "healthy" && health != "idle" && health != "stopped") {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("runtime-health-%s", session.ID),
				Scope:            "runtime",
				Level:            "critical",
				Title:            "Runtime health",
				Detail:           fmt.Sprintf("session=%s health=%s", session.Status, firstNonEmpty(health, "unknown")),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				RuntimeSessionID: session.ID,
				Anchor:           "signals",
				EventTime:        session.UpdatedAt,
			})
		}
		if sourceSummary.staleCount > 0 {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("runtime-stale-%s", session.ID),
				Scope:            "runtime",
				Level:            "warning",
				Title:            "Stale sources",
				Detail:           fmt.Sprintf("%d source state(s) outdated", sourceSummary.staleCount),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				RuntimeSessionID: session.ID,
				Anchor:           "signals",
				EventTime:        sourceSummary.latestEventAt,
			})
		}
		if p.runtimeSessionQuiet(state) {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("runtime-quiet-%s", session.ID),
				Scope:            "runtime",
				Level:            "warning",
				Title:            "Runtime quiet",
				Detail:           fmt.Sprintf("no runtime events observed in the last %ds", p.runtimePolicy.RuntimeQuietSeconds),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				RuntimeSessionID: session.ID,
				Anchor:           "signals",
				EventTime:        parseOptionalRFC3339(stringValue(state["lastEventAt"])),
			})
		}
	}

	for _, session := range paperSessions {
		account := accountByID[session.AccountID]
		state := cloneMetadata(session.State)
		runtimeSessionID := stringValue(state["signalRuntimeSessionId"])
		runtimeSession, hasRuntime := resolveRuntimeForAlert(runtimeSessionID, runtimeByKey, session.AccountID, session.StrategyID)
		var sourceGate map[string]any
		var sourceSummary runtimeSourceSummary
		if hasRuntime {
			sourceGate = p.evaluateSignalSourceReadiness(session, runtimeSession, time.Now().UTC())
			sourceSummary = p.summarizeRuntimeSources(runtimeSession)
		}
		if hasRuntime && !boolValue(sourceGate["ready"]) {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("paper-runtime-%s", session.ID),
				Scope:            "paper",
				Level:            "critical",
				Title:            "Runtime blocked",
				Detail:           summarizeSourceGate(sourceGate),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				PaperSessionID:   session.ID,
				RuntimeSessionID: runtimeSession.ID,
				Anchor:           "paper",
				EventTime:        parseOptionalRFC3339(stringValue(state["signalRuntimeLastCheckedAt"])),
			})
		}
		if hasRuntime && sourceSummary.staleCount > 0 {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("paper-stale-%s", session.ID),
				Scope:            "paper",
				Level:            "warning",
				Title:            "Stale sources",
				Detail:           fmt.Sprintf("%d source state(s) outdated", sourceSummary.staleCount),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				PaperSessionID:   session.ID,
				RuntimeSessionID: runtimeSession.ID,
				Anchor:           "paper",
				EventTime:        sourceSummary.latestEventAt,
			})
		}
		if strings.EqualFold(stringValue(state["lastStrategyEvaluationStatus"]), "decision-error") {
			appendAlert(domain.PlatformAlert{
				ID:             fmt.Sprintf("paper-decision-%s", session.ID),
				Scope:          "paper",
				Level:          "critical",
				Title:          "Decision error",
				Detail:         "latest strategy evaluation returned an error",
				AccountID:      session.AccountID,
				AccountName:    account.Name,
				StrategyID:     session.StrategyID,
				StrategyName:   strategyNameByID[session.StrategyID],
				PaperSessionID: session.ID,
				Anchor:         "paper",
				EventTime:      parseOptionalRFC3339(stringValue(state["lastStrategyEvaluationAt"])),
			})
		}
		if strings.EqualFold(stringValue(state["lastStrategyEvaluationDecisionState"]), "waiting-signal-bars") ||
			strings.EqualFold(stringValue(mapValue(state["lastStrategyEvaluationSignalBarDecision"])["reason"]), "insufficient-signal-bars") {
			appendAlert(domain.PlatformAlert{
				ID:             fmt.Sprintf("paper-signal-bars-%s", session.ID),
				Scope:          "paper",
				Level:          "warning",
				Title:          "Signal bars missing",
				Detail:         "insufficient closed signal bars for MA20 / t-1 / t-2",
				AccountID:      session.AccountID,
				AccountName:    account.Name,
				StrategyID:     session.StrategyID,
				StrategyName:   strategyNameByID[session.StrategyID],
				PaperSessionID: session.ID,
				Anchor:         "paper",
				EventTime:      parseOptionalRFC3339(stringValue(state["lastStrategyEvaluationAt"])),
			})
		}
		if hasRuntime && p.runtimeSessionQuiet(runtimeSession.State) {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("paper-runtime-quiet-%s", session.ID),
				Scope:            "paper",
				Level:            "warning",
				Title:            "Runtime quiet",
				Detail:           fmt.Sprintf("no runtime events observed in the last %ds", p.runtimePolicy.RuntimeQuietSeconds),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				PaperSessionID:   session.ID,
				RuntimeSessionID: runtimeSession.ID,
				Anchor:           "paper",
				EventTime:        parseOptionalRFC3339(stringValue(runtimeSession.State["lastEventAt"])),
			})
		}
	}

	for _, account := range accounts {
		if !strings.EqualFold(account.Mode, "LIVE") {
			continue
		}
		bindings, err := p.ListAccountSignalBindings(account.ID)
		if err != nil {
			continue
		}
		if account.Status != "CONFIGURED" && account.Status != "READY" {
			appendAlert(domain.PlatformAlert{
				ID:          fmt.Sprintf("live-config-%s", account.ID),
				Scope:       "live",
				Level:       "warning",
				Title:       "Account not configured",
				Detail:      fmt.Sprintf("status=%s", account.Status),
				AccountID:   account.ID,
				AccountName: account.Name,
				Anchor:      "live",
				EventTime:   account.CreatedAt,
			})
		}
		if len(bindings) == 0 {
			appendAlert(domain.PlatformAlert{
				ID:          fmt.Sprintf("live-bindings-%s", account.ID),
				Scope:       "live",
				Level:       "warning",
				Title:       "No signal bindings",
				Detail:      "live account has no signal bindings",
				AccountID:   account.ID,
				AccountName: account.Name,
				Anchor:      "live",
				EventTime:   account.CreatedAt,
			})
		}
		runtimeSessionsForAccount := make([]domain.SignalRuntimeSession, 0)
		runningLiveSessionsForAccount := make([]domain.LiveSession, 0)
		for _, session := range runtimeSessions {
			if session.AccountID == account.ID {
				runtimeSessionsForAccount = append(runtimeSessionsForAccount, session)
			}
		}
		for _, session := range liveSessions {
			if session.AccountID == account.ID && strings.EqualFold(session.Status, "RUNNING") {
				runningLiveSessionsForAccount = append(runningLiveSessionsForAccount, session)
			}
		}
		snapshot := cloneMetadata(mapValue(account.Metadata["liveSyncSnapshot"]))
		openPositionCount := maxIntValue(snapshot["positionCount"], 0)
		if openPositionCount > 0 && len(runningLiveSessionsForAccount) == 0 {
			appendAlert(domain.PlatformAlert{
				ID:          fmt.Sprintf("live-position-unmonitored-%s", account.ID),
				Scope:       "live",
				Level:       "critical",
				Title:       "Open position without running session",
				Detail:      fmt.Sprintf("exchange reports %d open position(s) but no live session is RUNNING", openPositionCount),
				AccountID:   account.ID,
				AccountName: account.Name,
				Anchor:      "live",
				EventTime:   parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"])),
			})
		}
		activeRuntime, hasRuntime := pickActiveRuntime(runtimeSessionsForAccount)
		if !hasRuntime {
			appendAlert(domain.PlatformAlert{
				ID:          fmt.Sprintf("live-runtime-%s", account.ID),
				Scope:       "live",
				Level:       "warning",
				Title:       "No runtime session",
				Detail:      "create or start a runtime session before live trading",
				AccountID:   account.ID,
				AccountName: account.Name,
				Anchor:      "live",
				EventTime:   account.CreatedAt,
			})
			continue
		}
		readiness := p.evaluateLiveAccountRuntimeReadiness(account, bindings, activeRuntime)
		if readiness.status == "blocked" {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("live-preflight-%s", account.ID),
				Scope:            "live",
				Level:            "critical",
				Title:            "Live preflight blocked",
				Detail:           readiness.reason,
				AccountID:        account.ID,
				AccountName:      account.Name,
				StrategyID:       activeRuntime.StrategyID,
				StrategyName:     strategyNameByID[activeRuntime.StrategyID],
				RuntimeSessionID: activeRuntime.ID,
				Anchor:           "live",
				EventTime:        parseOptionalRFC3339(stringValue(activeRuntime.State["lastEventAt"])),
			})
		} else if readiness.status == "warning" {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("live-warning-%s", account.ID),
				Scope:            "live",
				Level:            "warning",
				Title:            "Live runtime warning",
				Detail:           readiness.reason,
				AccountID:        account.ID,
				AccountName:      account.Name,
				StrategyID:       activeRuntime.StrategyID,
				StrategyName:     strategyNameByID[activeRuntime.StrategyID],
				RuntimeSessionID: activeRuntime.ID,
				Anchor:           "live",
				EventTime:        parseOptionalRFC3339(stringValue(activeRuntime.State["lastEventAt"])),
			})
		}
	}

	for _, session := range liveSessions {
		state := cloneMetadata(session.State)
		recoveryStatus := strings.TrimSpace(stringValue(state["protectionRecoveryStatus"]))
		recoveryError := strings.TrimSpace(stringValue(state["lastRecoveryError"]))
		eventTime := parseOptionalRFC3339(firstNonEmpty(stringValue(state["lastProtectionRecoveryAt"]), stringValue(state["lastRecoveryAttemptAt"])))
		if recoveryError != "" {
			appendAlert(domain.PlatformAlert{
				ID:           fmt.Sprintf("live-recovery-error-%s", session.ID),
				Scope:        "live",
				Level:        "critical",
				Title:        "Live recovery failed",
				Detail:       recoveryError,
				AccountID:    session.AccountID,
				StrategyID:   session.StrategyID,
				StrategyName: strategyNameByID[session.StrategyID],
				Anchor:       "live",
				EventTime:    eventTime,
			})
		}
		if recoveryStatus == "unprotected-open-position" {
			appendAlert(domain.PlatformAlert{
				ID:           fmt.Sprintf("live-unprotected-position-%s", session.ID),
				Scope:        "live",
				Level:        "critical",
				Title:        "Recovered position has no protection",
				Detail:       "open position was restored but no stop-loss / take-profit protection order was recovered",
				AccountID:    session.AccountID,
				StrategyID:   session.StrategyID,
				StrategyName: strategyNameByID[session.StrategyID],
				Anchor:       "live",
				EventTime:    eventTime,
			})
		} else if recoveryStatus == "protected-open-position" {
			appendAlert(domain.PlatformAlert{
				ID:    fmt.Sprintf("live-protected-position-%s", session.ID),
				Scope: "live",
				Level: "info",
				Title: "Recovered protected position",
				Detail: fmt.Sprintf(
					"recovered %d protection order(s): stop=%d take-profit=%d",
					maxIntValue(state["recoveredProtectionCount"], 0),
					maxIntValue(state["recoveredStopOrderCount"], 0),
					maxIntValue(state["recoveredTakeProfitOrderCount"], 0),
				),
				AccountID:    session.AccountID,
				StrategyID:   session.StrategyID,
				StrategyName: strategyNameByID[session.StrategyID],
				Anchor:       "live",
				EventTime:    eventTime,
			})
		}
	}

	slices.SortFunc(alerts, func(a, b domain.PlatformAlert) int {
		if a.EventTime.Equal(b.EventTime) {
			if a.Level == b.Level {
				switch {
				case a.ID < b.ID:
					return -1
				case a.ID > b.ID:
					return 1
				default:
					return 0
				}
			}
			return compareAlertLevel(a.Level, b.Level)
		}
		if a.EventTime.After(b.EventTime) {
			return -1
		}
		return 1
	})
	return alerts, nil
}

type liveRuntimeReadiness struct {
	status string
	reason string
}

func (p *Platform) evaluateLiveAccountRuntimeReadiness(account domain.Account, bindings []domain.AccountSignalBinding, runtimeSession domain.SignalRuntimeSession) liveRuntimeReadiness {
	if runtimeSession.ID == "" {
		return liveRuntimeReadiness{status: "blocked", reason: "no-runtime-session"}
	}
	if !strings.EqualFold(runtimeSession.Status, "RUNNING") {
		return liveRuntimeReadiness{status: "blocked", reason: "runtime-not-running"}
	}
	health := strings.ToLower(strings.TrimSpace(stringValue(runtimeSession.State["health"])))
	if health != "" && health != "healthy" {
		return liveRuntimeReadiness{status: "blocked", reason: "runtime-error"}
	}

	sourceGate := p.evaluateRuntimeSignalSourceReadiness(runtimeSession.StrategyID, runtimeSession, time.Now().UTC())
	if !boolValue(sourceGate["ready"]) {
		if len(metadataList(sourceGate["missing"])) > 0 {
			if missing, ok := firstBindingWithStreamType(metadataList(sourceGate["missing"]), "trade_tick"); ok {
				return liveRuntimeReadiness{status: "blocked", reason: firstNonEmpty(stringValue(missing["streamType"]), "missing-trade-tick")}
			}
			if missing, ok := firstBindingWithStreamType(metadataList(sourceGate["missing"]), "order_book"); ok {
				return liveRuntimeReadiness{status: "blocked", reason: firstNonEmpty(stringValue(missing["streamType"]), "missing-order-book")}
			}
			return liveRuntimeReadiness{status: "blocked", reason: "missing-source-states"}
		}
		if len(metadataList(sourceGate["stale"])) > 0 {
			return liveRuntimeReadiness{status: "warning", reason: "stale-source-states"}
		}
	}
	if p.runtimeSessionQuiet(runtimeSession.State) {
		return liveRuntimeReadiness{status: "warning", reason: "runtime-quiet"}
	}

	requireTick := false
	requireOrderBook := false
	for _, binding := range bindings {
		if strings.EqualFold(binding.Status, "DISABLED") {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(binding.StreamType)) {
		case "trade_tick":
			requireTick = true
		case "order_book":
			requireOrderBook = true
		}
	}
	if requireTick || requireOrderBook {
		sourceSummary := p.summarizeRuntimeSources(runtimeSession)
		if requireTick && sourceSummary.tradeTickCount == 0 {
			return liveRuntimeReadiness{status: "blocked", reason: "missing-trade-tick"}
		}
		if requireOrderBook && sourceSummary.orderBookCount == 0 {
			return liveRuntimeReadiness{status: "blocked", reason: "missing-order-book"}
		}
	}
	return liveRuntimeReadiness{status: "ready", reason: "runtime-ready"}
}

func (p *Platform) summarizeRuntimeSources(runtimeSession domain.SignalRuntimeSession) runtimeSourceSummary {
	sourceStates := cloneMetadata(mapValue(runtimeSession.State["sourceStates"]))
	if sourceStates == nil {
		return runtimeSourceSummary{}
	}
	now := time.Now().UTC()
	summary := runtimeSourceSummary{}
	for _, raw := range sourceStates {
		entry := mapValue(raw)
		if entry == nil {
			continue
		}
		streamType := strings.ToLower(strings.TrimSpace(stringValue(entry["streamType"])))
		switch streamType {
		case "trade_tick":
			summary.tradeTickCount++
		case "order_book":
			summary.orderBookCount++
		}
		lastEventAt := parseOptionalRFC3339(stringValue(entry["lastEventAt"]))
		if !lastEventAt.IsZero() && lastEventAt.After(summary.latestEventAt) {
			summary.latestEventAt = lastEventAt
		}
		maxAge := p.signalSourceFreshnessWindow(domain.AccountSignalBinding{
			StreamType: streamType,
			Options:    cloneMetadata(mapValue(entry["options"])),
		})
		if lastEventAt.IsZero() || now.Sub(lastEventAt) > maxAge {
			summary.staleCount++
		}
	}
	return summary
}

func (p *Platform) runtimeSessionQuiet(runtimeState map[string]any) bool {
	lastEventAt := parseOptionalRFC3339(stringValue(runtimeState["lastEventAt"]))
	if lastEventAt.IsZero() {
		return false
	}
	return time.Since(lastEventAt) > time.Duration(p.runtimePolicy.RuntimeQuietSeconds)*time.Second
}

func summarizeSourceGate(sourceGate map[string]any) string {
	missing := len(metadataList(sourceGate["missing"]))
	stale := len(metadataList(sourceGate["stale"]))
	switch {
	case missing > 0 && stale > 0:
		return fmt.Sprintf("missing=%d stale=%d", missing, stale)
	case missing > 0:
		return fmt.Sprintf("missing=%d", missing)
	case stale > 0:
		return fmt.Sprintf("stale=%d", stale)
	default:
		return "waiting-for-sources"
	}
}

func resolveRuntimeForAlert(runtimeSessionID string, runtimeByKey map[string]domain.SignalRuntimeSession, accountID, strategyID string) (domain.SignalRuntimeSession, bool) {
	if strings.TrimSpace(runtimeSessionID) != "" {
		for _, item := range runtimeByKey {
			if item.ID == runtimeSessionID {
				return item, true
			}
		}
	}
	runtimeSession, ok := runtimeByKey[runtimeSessionLookupKey(accountID, strategyID)]
	return runtimeSession, ok
}

func runtimeSessionLookupKey(accountID, strategyID string) string {
	return fmt.Sprintf("%s::%s", accountID, strategyID)
}

func pickActiveRuntime(sessions []domain.SignalRuntimeSession) (domain.SignalRuntimeSession, bool) {
	for _, session := range sessions {
		if strings.EqualFold(session.Status, "RUNNING") {
			return session, true
		}
	}
	if len(sessions) == 0 {
		return domain.SignalRuntimeSession{}, false
	}
	return sessions[0], true
}

func firstBindingWithStreamType(items []map[string]any, streamType string) (map[string]any, bool) {
	for _, item := range items {
		if strings.EqualFold(stringValue(item["streamType"]), streamType) {
			return item, true
		}
	}
	return nil, false
}

func compareAlertLevel(a, b string) int {
	weight := func(level string) int {
		switch strings.ToLower(strings.TrimSpace(level)) {
		case "critical":
			return 0
		case "warning":
			return 1
		case "info":
			return 2
		default:
			return 3
		}
	}
	aw := weight(a)
	bw := weight(b)
	switch {
	case aw < bw:
		return -1
	case aw > bw:
		return 1
	default:
		return 0
	}
}
