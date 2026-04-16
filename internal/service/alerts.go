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
		if !strings.EqualFold(session.Status, "ERROR") && !strings.EqualFold(session.Status, "STOPPED") && health != "stopped" {
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
	}

	for _, account := range accounts {
		if !strings.EqualFold(account.Mode, "LIVE") {
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
		accountSyncSummary := cloneMetadata(mapValue(mapValue(account.Metadata["healthSummary"])["accountSync"]))
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
		if stale, ageSeconds := p.liveAccountSyncStale(account, time.Now().UTC()); stale {
			level := "warning"
			if openPositionCount > 0 || len(runningLiveSessionsForAccount) > 0 {
				level = "critical"
			}
			appendAlert(domain.PlatformAlert{
				ID:          fmt.Sprintf("live-account-sync-stale-%s", account.ID),
				Scope:       "live",
				Level:       level,
				Title:       "Live account sync stale",
				Detail:      fmt.Sprintf("last successful account sync was %ds ago", ageSeconds),
				AccountID:   account.ID,
				AccountName: account.Name,
				Anchor:      "live",
				EventTime:   parseOptionalRFC3339(stringValue(account.Metadata["lastLiveSyncAt"])),
			})
		}
		if consecutiveErrors := maxIntValue(accountSyncSummary["consecutiveErrorCount"], 0); consecutiveErrors > 0 {
			level := "warning"
			if consecutiveErrors >= 3 {
				level = "critical"
			}
			appendAlert(domain.PlatformAlert{
				ID:          fmt.Sprintf("live-account-sync-error-%s", account.ID),
				Scope:       "live",
				Level:       level,
				Title:       "Live account sync errors",
				Detail:      fmt.Sprintf("consecutive_errors=%d last_error=%s", consecutiveErrors, firstNonEmpty(stringValue(accountSyncSummary["lastError"]), "unknown")),
				AccountID:   account.ID,
				AccountName: account.Name,
				Anchor:      "live",
				EventTime:   parseOptionalRFC3339(stringValue(accountSyncSummary["lastErrorAt"])),
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
		strategyBindings, err := p.ListStrategySignalBindings(activeRuntime.StrategyID)
		if err != nil {
			continue
		}
		readiness := p.evaluateLiveRuntimeReadiness(strategyBindings, activeRuntime)
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
		if p.liveSessionEvaluationQuiet("LIVE", session.Status, state) {
			appendAlert(domain.PlatformAlert{
				ID:           fmt.Sprintf("live-strategy-eval-quiet-%s", session.ID),
				Scope:        "live",
				Level:        "warning",
				Title:        "Strategy evaluation quiet",
				Detail:       fmt.Sprintf("runtime triggers observed but no strategy evaluation recorded in the last %ds", p.runtimePolicy.StrategyEvaluationQuietSeconds),
				AccountID:    session.AccountID,
				StrategyID:   session.StrategyID,
				StrategyName: strategyNameByID[session.StrategyID],
				Anchor:       "live",
				EventTime:    parseOptionalRFC3339(firstNonEmpty(stringValue(mapValue(mapValue(state["healthSummary"])["strategyIngress"])["lastTriggeredAt"]), stringValue(state["lastSignalRuntimeEventAt"]))),
			})
		}
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

func (p *Platform) evaluateLiveRuntimeReadiness(bindings []domain.AccountSignalBinding, runtimeSession domain.SignalRuntimeSession) liveRuntimeReadiness {
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

func (p *Platform) strategyEvaluationQuiet(sessionState map[string]any) bool {
	lastTriggeredAt := parseOptionalRFC3339(firstNonEmpty(
		stringValue(mapValue(mapValue(sessionState["healthSummary"])["strategyIngress"])["lastTriggeredAt"]),
		stringValue(sessionState["lastSignalRuntimeEventAt"]),
	))
	if lastTriggeredAt.IsZero() {
		return false
	}
	lastEvaluationAt := parseOptionalRFC3339(stringValue(mapValue(mapValue(sessionState["healthSummary"])["strategyIngress"])["lastEvaluationAt"]))
	if stateEvaluationAt := parseOptionalRFC3339(stringValue(sessionState["lastStrategyEvaluationAt"])); stateEvaluationAt.After(lastEvaluationAt) {
		lastEvaluationAt = stateEvaluationAt
	}
	if !lastEvaluationAt.IsZero() && !lastTriggeredAt.After(lastEvaluationAt) {
		return false
	}
	threshold := time.Duration(p.runtimePolicy.StrategyEvaluationQuietSeconds) * time.Second
	if threshold <= 0 {
		return false
	}
	return time.Since(lastTriggeredAt) > threshold
}

func (p *Platform) liveSessionEvaluationQuiet(mode, status string, sessionState map[string]any) bool {
	return strings.EqualFold(mode, "LIVE") &&
		strings.EqualFold(status, "RUNNING") &&
		p.strategyEvaluationQuiet(sessionState)
}

func (p *Platform) liveAccountSyncStale(account domain.Account, referenceTime time.Time) (bool, int) {
	threshold := time.Duration(p.runtimePolicy.LiveAccountSyncFreshnessSecs) * time.Second
	if threshold <= 0 {
		return false, 0
	}
	lastSuccessAt := parseOptionalRFC3339(firstNonEmpty(
		stringValue(mapValue(mapValue(account.Metadata["healthSummary"])["accountSync"])["lastSuccessAt"]),
		stringValue(account.Metadata["lastLiveSyncAt"]),
	))
	if lastSuccessAt.IsZero() {
		age := 0
		if !account.CreatedAt.IsZero() {
			age = int(referenceTime.Sub(account.CreatedAt).Seconds())
		}
		return true, age
	}
	age := int(referenceTime.Sub(lastSuccessAt).Seconds())
	return referenceTime.Sub(lastSuccessAt) > threshold, age
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
