package service

import (
	"slices"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func (p *Platform) HealthSnapshot() (domain.PlatformHealthSnapshot, error) {
	generatedAt := time.Now().UTC()

	accounts, err := p.ListAccounts()
	if err != nil {
		return domain.PlatformHealthSnapshot{}, err
	}
	liveSessions, err := p.ListLiveSessions()
	if err != nil {
		return domain.PlatformHealthSnapshot{}, err
	}
	paperSessions, err := p.ListPaperSessions()
	if err != nil {
		return domain.PlatformHealthSnapshot{}, err
	}
	strategies, err := p.ListStrategies()
	if err != nil {
		return domain.PlatformHealthSnapshot{}, err
	}
	alerts, err := p.ListAlerts()
	if err != nil {
		return domain.PlatformHealthSnapshot{}, err
	}
	runtimeSessions := p.ListSignalRuntimeSessions()

	strategyNameByID := make(map[string]string, len(strategies))
	for _, strategy := range strategies {
		strategyNameByID[stringValue(strategy["id"])] = stringValue(strategy["name"])
	}

	snapshot := domain.PlatformHealthSnapshot{
		GeneratedAt:     generatedAt,
		AlertCounts:     summarizePlatformHealthAlertCounts(alerts),
		RuntimePolicy:   currentRuntimePolicyDomain(p.runtimePolicy),
		LiveAccounts:    make([]domain.PlatformHealthAccountSnapshot, 0),
		RuntimeSessions: make([]domain.PlatformHealthRuntimeSessionSnapshot, 0, len(runtimeSessions)),
		LiveSessions:    make([]domain.PlatformHealthStrategySessionSnapshot, 0, len(liveSessions)),
		PaperSessions:   make([]domain.PlatformHealthStrategySessionSnapshot, 0, len(paperSessions)),
	}
	snapshot.Status = platformHealthStatus(snapshot.AlertCounts)

	runtimeCountByAccount := make(map[string]int)
	runningLiveCountByAccount := make(map[string]int)
	for _, session := range runtimeSessions {
		runtimeCountByAccount[session.AccountID]++
	}
	for _, session := range liveSessions {
		if strings.EqualFold(session.Status, "RUNNING") {
			runningLiveCountByAccount[session.AccountID]++
		}
	}

	for _, account := range accounts {
		if !strings.EqualFold(account.Mode, "LIVE") {
			continue
		}
		stale, ageSeconds := p.liveAccountSyncStale(account, generatedAt)
		accountSync := cloneMetadata(mapValue(mapValue(account.Metadata["healthSummary"])["accountSync"]))
		snapshot.LiveAccounts = append(snapshot.LiveAccounts, domain.PlatformHealthAccountSnapshot{
			ID:                      account.ID,
			Name:                    account.Name,
			Exchange:                account.Exchange,
			Status:                  account.Status,
			LastLiveSyncAt:          stringValue(account.Metadata["lastLiveSyncAt"]),
			SyncAgeSeconds:          ageSeconds,
			SyncStale:               stale,
			RuntimeSessionCount:     runtimeCountByAccount[account.ID],
			RunningLiveSessionCount: runningLiveCountByAccount[account.ID],
			AccountSync:             accountSync,
		})
	}

	for _, runtime := range runtimeSessions {
		state := cloneMetadata(runtime.State)
		healthSummary := cloneMetadata(mapValue(state["healthSummary"]))
		snapshot.RuntimeSessions = append(snapshot.RuntimeSessions, domain.PlatformHealthRuntimeSessionSnapshot{
			ID:              runtime.ID,
			AccountID:       runtime.AccountID,
			StrategyID:      runtime.StrategyID,
			StrategyName:    strategyNameByID[runtime.StrategyID],
			Status:          runtime.Status,
			Transport:       runtime.Transport,
			Health:          firstNonEmpty(stringValue(state["health"]), "unknown"),
			LastEventAt:     stringValue(state["lastEventAt"]),
			LastHeartbeatAt: stringValue(state["lastHeartbeatAt"]),
			Quiet:           p.runtimeSessionQuiet(state),
			TradeTick:       cloneMetadata(mapValue(healthSummary["tradeTick"])),
			OrderBook:       cloneMetadata(mapValue(healthSummary["orderBook"])),
		})
	}

	for _, session := range liveSessions {
		snapshot.LiveSessions = append(snapshot.LiveSessions, p.platformHealthStrategySessionSnapshot("LIVE", session.ID, session.AccountID, session.StrategyID, session.Status, session.State, strategyNameByID[session.StrategyID]))
	}
	for _, session := range paperSessions {
		snapshot.PaperSessions = append(snapshot.PaperSessions, p.platformHealthStrategySessionSnapshot("PAPER", session.ID, session.AccountID, session.StrategyID, session.Status, session.State, strategyNameByID[session.StrategyID]))
	}

	slices.SortFunc(snapshot.LiveAccounts, func(a, b domain.PlatformHealthAccountSnapshot) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		default:
			return 0
		}
	})
	slices.SortFunc(snapshot.RuntimeSessions, func(a, b domain.PlatformHealthRuntimeSessionSnapshot) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	slices.SortFunc(snapshot.LiveSessions, func(a, b domain.PlatformHealthStrategySessionSnapshot) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})
	slices.SortFunc(snapshot.PaperSessions, func(a, b domain.PlatformHealthStrategySessionSnapshot) int {
		switch {
		case a.ID < b.ID:
			return -1
		case a.ID > b.ID:
			return 1
		default:
			return 0
		}
	})

	return snapshot, nil
}

func currentRuntimePolicyDomain(policy RuntimePolicy) domain.RuntimePolicy {
	return domain.RuntimePolicy{
		TradeTickFreshnessSeconds:      policy.TradeTickFreshnessSeconds,
		OrderBookFreshnessSeconds:      policy.OrderBookFreshnessSeconds,
		SignalBarFreshnessSeconds:      policy.SignalBarFreshnessSeconds,
		RuntimeQuietSeconds:            policy.RuntimeQuietSeconds,
		StrategyEvaluationQuietSeconds: policy.StrategyEvaluationQuietSeconds,
		LiveAccountSyncFreshnessSecs:   policy.LiveAccountSyncFreshnessSecs,
		PaperStartReadinessTimeoutSecs: policy.PaperStartReadinessTimeoutSecs,
	}
}

func summarizePlatformHealthAlertCounts(alerts []domain.PlatformAlert) domain.PlatformHealthAlertCounts {
	counts := domain.PlatformHealthAlertCounts{
		Total: len(alerts),
	}
	for _, alert := range alerts {
		switch strings.ToLower(strings.TrimSpace(alert.Level)) {
		case "critical":
			counts.Critical++
		case "warning":
			counts.Warning++
		case "info":
			counts.Info++
		}
	}
	return counts
}

func platformHealthStatus(counts domain.PlatformHealthAlertCounts) string {
	switch {
	case counts.Critical > 0:
		return "critical"
	case counts.Warning > 0:
		return "warning"
	default:
		return "healthy"
	}
}

func (p *Platform) platformHealthStrategySessionSnapshot(mode, sessionID, accountID, strategyID, status string, state map[string]any, strategyName string) domain.PlatformHealthStrategySessionSnapshot {
	sectionRoot := cloneMetadata(mapValue(state["healthSummary"]))
	return domain.PlatformHealthStrategySessionSnapshot{
		ID:                           sessionID,
		Mode:                         mode,
		AccountID:                    accountID,
		StrategyID:                   strategyID,
		StrategyName:                 strategyName,
		Status:                       status,
		RuntimeSessionID:             firstNonEmpty(stringValue(state["signalRuntimeSessionId"]), stringValue(state["lastSignalRuntimeSessionId"])),
		LastSignalRuntimeEventAt:     stringValue(state["lastSignalRuntimeEventAt"]),
		LastStrategyEvaluationAt:     stringValue(state["lastStrategyEvaluationAt"]),
		LastStrategyEvaluationStatus: stringValue(state["lastStrategyEvaluationStatus"]),
		LastSyncedOrderStatus:        firstNonEmpty(stringValue(state["lastSyncedOrderStatus"]), stringValue(state["lastDispatchedOrderStatus"])),
		EvaluationQuiet:              strings.EqualFold(mode, "LIVE") && p.strategyEvaluationQuiet(state),
		StrategyIngress:              cloneMetadata(mapValue(sectionRoot["strategyIngress"])),
		Execution:                    cloneMetadata(mapValue(sectionRoot["execution"])),
		SourceGate:                   cloneMetadata(mapValue(state["lastStrategyEvaluationSourceGate"])),
	}
}
