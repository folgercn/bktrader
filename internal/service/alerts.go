package service

import (
	"fmt"
	"math"
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
		if strings.EqualFold(session.Status, "ERROR") || (health != "" && health != "healthy" && health != "idle" && health != "stopped" && health != "recovering" && health != "stale-after-reconnect") {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("runtime-health-%s", session.ID),
				Scope:            "runtime",
				Level:            "critical",
				Title:            "运行时异常",
				Detail:           fmt.Sprintf("会话状态=%s 健康度=%s", session.Status, firstNonEmpty(health, "未知")),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				RuntimeSessionID: session.ID,
				Anchor:           "signals",
				EventTime:        session.UpdatedAt,
			})
		}
		if health == "recovering" {
			appendAlert(domain.PlatformAlert{
				ID:    fmt.Sprintf("runtime-recovering-%s", session.ID),
				Scope: "runtime",
				Level: "warning",
				Title: "运行时恢复中",
				Detail: fmt.Sprintf("尝试次数 %d/%d: %s",
					maxIntValue(state["reconnectAttempt"], 0),
					maxIntValue(state["reconnectMaxAttempts"], 0),
					firstNonEmpty(stringValue(state["lastDisconnectError"]), "未知")),
				AccountID:        session.AccountID,
				AccountName:      account.Name,
				StrategyID:       session.StrategyID,
				StrategyName:     strategyNameByID[session.StrategyID],
				RuntimeSessionID: session.ID,
				Anchor:           "signals",
				EventTime:        session.UpdatedAt,
			})
		}
		if health == "stale-after-reconnect" {
			appendAlert(domain.PlatformAlert{
				ID:               fmt.Sprintf("runtime-stale-reconnect-%s", session.ID),
				Scope:            "runtime",
				Level:            "critical",
				Title:            "重连后信号K线可能过期",
				Detail:           "WebSocket 断开可能导致错过 K 线收盘，请校验数据，必要时手动重启会话",
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
					Title:            "数据源过期",
					Detail:           fmt.Sprintf("%d 个数据源状态已陈旧", sourceSummary.staleCount),
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
				quietSecs := p.runtimePolicy.RuntimeQuietSeconds
				if override := mapValue(state["freshnessOverride"]); override != nil {
					if val, ok := toFloat64(override["runtimeQuietSeconds"]); ok && val > 0 {
						quietSecs = int(val)
					}
				}
				appendAlert(domain.PlatformAlert{
					ID:               fmt.Sprintf("runtime-quiet-%s", session.ID),
					Scope:            "runtime",
					Level:            "warning",
					Title:            "运行时静默",
					Detail:           fmt.Sprintf("过去 %d 秒内未观测到运行时事件", quietSecs),
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
				Title:       "账户未配置",
				Detail:      fmt.Sprintf("当前状态=%s", account.Status),
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
				Title:       "存在持仓但无运行中会话",
				Detail:      fmt.Sprintf("交易所报告有 %d 个持仓，但没有对应的实盘会话在运行 (RUNNING)", openPositionCount),
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
				Title:       "账户同步过期",
				Detail:      fmt.Sprintf("上次成功的账户同步发生在 %d 秒前", ageSeconds),
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
				Title:       "账户同步出错",
				Detail:      fmt.Sprintf("连续错误次数=%d 错误信息=%s", consecutiveErrors, firstNonEmpty(stringValue(accountSyncSummary["lastError"]), "未知")),
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
				Title:       "无运行时会话",
				Detail:      "在开始实盘交易前，请先创建并启动运行时会话 (Signal Runtime)",
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
			alertID := fmt.Sprintf("live-preflight-%s", account.ID)
			if readiness.reason == "runtime-error" {
				alertID = fmt.Sprintf("live-preflight-runtime-error-%s", account.ID)
			}
			appendAlert(domain.PlatformAlert{
				ID:               alertID,
				Scope:            "live",
				Level:            "critical",
				Title:            "实盘预检受阻",
				Detail:           readiness.reason,
				AccountID:        account.ID,
				AccountName:      account.Name,
				StrategyID:       activeRuntime.StrategyID,
				StrategyName:     strategyNameByID[activeRuntime.StrategyID],
				RuntimeSessionID: activeRuntime.ID,
				Anchor:           "live",
				EventTime:        parseOptionalRFC3339(stringValue(activeRuntime.State["lastEventAt"])),
				Metadata: map[string]any{
					"reason": readiness.reason,
				},
			})
		} else if readiness.status == "warning" {
			alertID := fmt.Sprintf("live-warning-%s", account.ID)
			if readiness.reason == "stale-source-states" {
				alertID = fmt.Sprintf("live-warning-stale-source-states-%s", account.ID)
			}
			appendAlert(domain.PlatformAlert{
				ID:               alertID,
				Scope:            "live",
				Level:            "warning",
				Title:            "实盘运行时告警",
				Detail:           readiness.reason,
				AccountID:        account.ID,
				AccountName:      account.Name,
				StrategyID:       activeRuntime.StrategyID,
				StrategyName:     strategyNameByID[activeRuntime.StrategyID],
				RuntimeSessionID: activeRuntime.ID,
				Anchor:           "live",
				EventTime:        parseOptionalRFC3339(stringValue(activeRuntime.State["lastEventAt"])),
				Metadata: map[string]any{
					"reason": readiness.reason,
				},
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
				Title:        "策略评估静默",
				Detail:       fmt.Sprintf("观测到运行时触发但过去 %d 秒内未记录策略评估行为", p.runtimePolicy.StrategyEvaluationQuietSeconds),
				AccountID:    session.AccountID,
				StrategyID:   session.StrategyID,
				StrategyName: strategyNameByID[session.StrategyID],
				Anchor:       "live",
				EventTime:    parseOptionalRFC3339(firstNonEmpty(stringValue(mapValue(mapValue(state["healthSummary"])["strategyIngress"])["lastTriggeredAt"]), stringValue(state["lastSignalRuntimeEventAt"]))),
			})
		}
		recoveryStatus := strings.TrimSpace(stringValue(state["protectionRecoveryStatus"]))
		recoveryError := strings.TrimSpace(stringValue(state["lastRecoveryError"]))
		recoveryAuthoritative := boolValue(state["protectionRecoveryAuthoritative"])
		recoverySource := firstNonEmpty(strings.TrimSpace(stringValue(state["protectionRecoverySource"])), "local-fallback")
		eventTime := parseOptionalRFC3339(firstNonEmpty(stringValue(state["lastProtectionRecoveryAt"]), stringValue(state["lastRecoveryAttemptAt"])))
		if recoveryError != "" {
			appendAlert(domain.PlatformAlert{
				ID:           fmt.Sprintf("live-recovery-error-%s", session.ID),
				Scope:        "live",
				Level:        "critical",
				Title:        "实盘恢复失败",
				Detail:       recoveryError,
				AccountID:    session.AccountID,
				StrategyID:   session.StrategyID,
				StrategyName: strategyNameByID[session.StrategyID],
				Anchor:       "live",
				EventTime:    eventTime,
			})
		}
		if recoveryStatus == "unprotected-open-position" {
			if !recoveryAuthoritative {
				appendAlert(domain.PlatformAlert{
					ID:           fmt.Sprintf("live-recovery-awaiting-authoritative-sync-%s", session.ID),
					Scope:        "live",
					Level:        "warning",
					Title:        "恢复等待权威同步",
					Detail:       fmt.Sprintf("看门狗自动平仓已暂停，因为当前恢复状态来自 %s；请执行账户同步或对账以确认交易所持仓和挂单", recoverySource),
					AccountID:    session.AccountID,
					StrategyID:   session.StrategyID,
					StrategyName: strategyNameByID[session.StrategyID],
					Anchor:       "live",
					EventTime:    eventTime,
					Metadata: map[string]any{
						"protectionRecoverySource":        recoverySource,
						"protectionRecoveryAuthoritative": false,
						"watchdogAutoExitPaused":          true,
					},
				})
				continue
			}
			appendAlert(domain.PlatformAlert{
				ID:           fmt.Sprintf("live-unprotected-position-%s", session.ID),
				Scope:        "live",
				Level:        "critical",
				Title:        "恢复持仓无保护",
				Detail:       "已恢复持仓但未发现对应的止损/止盈保护订单",
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
				Title: "恢复持仓已保护",
				Detail: fmt.Sprintf(
					"成功恢复 %d 个保护订单：止损=%d 止盈=%d",
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
		if alert, ok := p.buildLiveExitDispatchFailureAlert(session, state, accountByID[session.AccountID].Name, strategyNameByID[session.StrategyID]); ok {
			appendAlert(alert)
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

func (p *Platform) buildLiveExitDispatchFailureAlert(session domain.LiveSession, state map[string]any, accountName, strategyName string) (domain.PlatformAlert, bool) {
	dispatchIntent := cloneMetadata(mapValue(state["lastDispatchedIntent"]))
	dispatchSummary := cloneMetadata(mapValue(state["lastExecutionDispatch"]))
	if !liveDispatchRepresentsExit(dispatchIntent, dispatchSummary) {
		return domain.PlatformAlert{}, false
	}
	rejectedStatus := strings.TrimSpace(firstNonEmpty(
		stringValue(state["lastDispatchRejectedStatus"]),
		stringValue(state["lastDispatchedOrderStatus"]),
		stringValue(dispatchSummary["status"]),
	))
	dispatchError := strings.TrimSpace(firstNonEmpty(
		stringValue(state["lastAutoDispatchError"]),
		stringValue(dispatchSummary["error"]),
	))
	if rejectedStatus == "" && dispatchError == "" {
		return domain.PlatformAlert{}, false
	}
	symbol := NormalizeSymbol(firstNonEmpty(
		stringValue(dispatchIntent["symbol"]),
		stringValue(dispatchSummary["symbol"]),
		stringValue(state["symbol"]),
		stringValue(state["lastSymbol"]),
		stringValue(mapValue(state["livePositionState"])["symbol"]),
	))
	if symbol == "" {
		return domain.PlatformAlert{}, false
	}
	positionSnapshot, _, err := p.resolveLiveSessionPositionSnapshot(session, symbol)
	if err != nil {
		return domain.PlatformAlert{}, false
	}
	if boolValue(positionSnapshot["virtual"]) || math.Abs(parseFloatValue(positionSnapshot["quantity"])) <= 1e-9 {
		return domain.PlatformAlert{}, false
	}

	eventTime := parseOptionalRFC3339(firstNonEmpty(
		stringValue(state["lastDispatchRejectedAt"]),
		stringValue(state["lastAutoDispatchAttemptAt"]),
		stringValue(state["lastDispatchedAt"]),
	))
	reason := firstNonEmpty(
		stringValue(dispatchIntent["reason"]),
		stringValue(dispatchSummary["reason"]),
		stringValue(dispatchIntent["signalKind"]),
		stringValue(dispatchSummary["signalKind"]),
	)
	statusLabel := firstNonEmpty(rejectedStatus, "DISPATCH_ERROR")
	detailParts := []string{fmt.Sprintf("自动平仓派单失败，状态=%s", statusLabel)}
	if reason != "" {
		detailParts = append(detailParts, fmt.Sprintf("触发=%s", reason))
	}
	if dispatchError != "" {
		detailParts = append(detailParts, fmt.Sprintf("错误=%s", dispatchError))
	}

	return domain.PlatformAlert{
		ID:           fmt.Sprintf("live-exit-dispatch-failure-%s", session.ID),
		Scope:        "live",
		Level:        "critical",
		Title:        "自动平仓失败",
		Detail:       strings.Join(detailParts, " · "),
		AccountID:    session.AccountID,
		AccountName:  accountName,
		StrategyID:   session.StrategyID,
		StrategyName: strategyName,
		Anchor:       "live",
		EventTime:    eventTime,
		Metadata: map[string]any{
			"liveSessionId":   session.ID,
			"orderId":         stringValue(state["lastDispatchedOrderId"]),
			"orderStatus":     statusLabel,
			"error":           dispatchError,
			"reason":          reason,
			"signalKind":      firstNonEmpty(stringValue(dispatchIntent["signalKind"]), stringValue(dispatchSummary["signalKind"])),
			"symbol":          symbol,
			"positionSide":    stringValue(positionSnapshot["side"]),
			"positionQty":     parseFloatValue(positionSnapshot["quantity"]),
			"dispatchMode":    stringValue(state["dispatchMode"]),
			"reduceOnly":      boolValue(dispatchIntent["reduceOnly"]) || boolValue(dispatchSummary["reduceOnly"]),
			"currentPosition": positionSnapshot,
		},
	}, true
}

func liveDispatchRepresentsExit(dispatchIntent map[string]any, dispatchSummary map[string]any) bool {
	if strings.EqualFold(strings.TrimSpace(stringValue(dispatchIntent["role"])), "exit") {
		return true
	}
	if boolValue(dispatchIntent["reduceOnly"]) || boolValue(dispatchSummary["reduceOnly"]) {
		return true
	}
	executionProfile := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		stringValue(dispatchSummary["executionProfile"]),
		stringValue(dispatchIntent["executionProfile"]),
	)))
	if strings.Contains(executionProfile, "exit") || strings.Contains(executionProfile, "close") {
		return true
	}
	signalKind := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		stringValue(dispatchIntent["signalKind"]),
		stringValue(dispatchSummary["signalKind"]),
	)))
	return strings.Contains(signalKind, "exit") || strings.Contains(signalKind, "watchdog")
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
		maxAge := p.signalSourceFreshnessWindowWithOverride(domain.AccountSignalBinding{
			StreamType: streamType,
			Options:    cloneMetadata(mapValue(entry["options"])),
		}, runtimeSession.State)
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
	threshold := time.Duration(p.runtimePolicy.RuntimeQuietSeconds) * time.Second
	if override := mapValue(runtimeState["freshnessOverride"]); override != nil {
		if val, ok := toFloat64(override["runtimeQuietSeconds"]); ok && val > 0 {
			threshold = time.Duration(val) * time.Second
		}
	}
	return time.Since(lastEventAt) > threshold
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
