package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const signalRuntimeScannerInterval = 5 * time.Second

var signalRuntimeSupervisorRestartBackoffs = []time.Duration{
	time.Minute,
	3 * time.Minute,
}

type signalRuntimeSessionStarter func(context.Context, string) (domain.SignalRuntimeSession, error)

func (p *Platform) StartSignalRuntimeScanner(ctx context.Context) {
	logger := p.logger("service.signal_runtime_scanner")
	logger.Info("signal runtime scanner started")
	go func() {
		defer logger.Info("signal runtime scanner stopped")
		p.scanSignalRuntimeSessions(ctx, p.startSignalRuntimeSession)
		ticker := time.NewTicker(signalRuntimeScannerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.scanSignalRuntimeSessions(ctx, p.startSignalRuntimeSession)
			}
		}
	}()
}

func (p *Platform) scanSignalRuntimeSessions(ctx context.Context, starter signalRuntimeSessionStarter) {
	if err := ctx.Err(); err != nil {
		return
	}
	sessions := p.ListSignalRuntimeSessions()
	for _, session := range sessions {
		if err := ctx.Err(); err != nil {
			return
		}
		if !signalRuntimeSessionDesiredRunning(session) {
			continue
		}
		if p.signalRuntimeSupervisorRestartDeferred(session, time.Now().UTC()) {
			continue
		}
		if p.stopSignalRuntimeLinkedToStoppedLiveSession(session) {
			continue
		}
		if p.signalRuntimeSessionRunningOrStarting(session.ID) {
			continue
		}
		if _, err := starter(ctx, session.ID); err != nil {
			if errors.Is(err, ErrRuntimeLeaseNotAcquired) {
				continue
			}
			p.logger("service.signal_runtime_scanner",
				"session_id", session.ID,
				"account_id", session.AccountID,
				"strategy_id", session.StrategyID,
			).Warn("signal runtime scanner failed to start session", "error", err)
			continue
		}
	}
}

func signalRuntimeSessionDesiredRunning(session domain.SignalRuntimeSession) bool {
	desired := stringValue(session.State["desiredStatus"])
	if desired != "" {
		return desired == "RUNNING"
	}
	if session.Status == "ERROR" || stringValue(session.State["actualStatus"]) == "ERROR" {
		return false
	}
	return session.Status == "RUNNING"
}

func (p *Platform) signalRuntimeSupervisorRestartDeferred(session domain.SignalRuntimeSession, now time.Time) bool {
	if !strings.EqualFold(session.Status, "ERROR") && !strings.EqualFold(stringValue(session.State["actualStatus"]), "ERROR") {
		return false
	}
	state := cloneMetadata(session.State)
	if boolValue(state["autoRestartSuppressed"]) {
		return true
	}
	if strings.EqualFold(stringValue(state["supervisorRestartSeverity"]), disconnectFatal.String()) {
		return true
	}
	nextAt, ok := parseSignalRuntimeSupervisorTime(state["nextAutoRestartAt"])
	if !ok {
		p.scheduleSignalRuntimeSupervisorRestart(session, now)
		return true
	}
	return now.Before(nextAt)
}

func (p *Platform) scheduleSignalRuntimeSupervisorRestart(session domain.SignalRuntimeSession, now time.Time) {
	_ = p.updateSignalRuntimeSessionState(session.ID, func(updated *domain.SignalRuntimeSession) {
		state := cloneMetadata(updated.State)
		attempt := signalRuntimeSupervisorRestartAttempt(state)
		if attempt <= 0 {
			attempt = 1
			state["supervisorRestartAttempt"] = attempt
		}
		backoff := signalRuntimeSupervisorRestartBackoff(attempt)
		state["nextAutoRestartAt"] = now.Add(backoff).UTC().Format(time.RFC3339)
		state["supervisorRestartBackoff"] = backoff.String()
		state["supervisorRestartReason"] = "runtime-error"
		updated.State = state
		updated.UpdatedAt = now
	})
}

func scheduleSignalRuntimeSupervisorRestartAfterTerminalError(state map[string]any, err error, now time.Time) {
	severity := classifyDisconnectSeverity(err)
	state["supervisorRestartSeverity"] = severity.String()
	if severity == disconnectFatal {
		state["autoRestartSuppressed"] = true
		delete(state, "nextAutoRestartAt")
		delete(state, "supervisorRestartBackoff")
		return
	}
	if !strings.EqualFold(stringValue(state["desiredStatus"]), "RUNNING") {
		delete(state, "nextAutoRestartAt")
		delete(state, "supervisorRestartBackoff")
		return
	}
	attempt := signalRuntimeSupervisorRestartAttempt(state) + 1
	state["supervisorRestartAttempt"] = attempt
	backoff := signalRuntimeSupervisorRestartBackoff(attempt)
	state["nextAutoRestartAt"] = now.Add(backoff).UTC().Format(time.RFC3339)
	state["supervisorRestartBackoff"] = backoff.String()
	state["lastSupervisorError"] = err.Error()
	delete(state, "autoRestartSuppressed")
}

func signalRuntimeSupervisorRestartBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return signalRuntimeSupervisorRestartBackoffs[0]
	}
	return signalRuntimeSupervisorRestartBackoffs[len(signalRuntimeSupervisorRestartBackoffs)-1]
}

func signalRuntimeSupervisorRestartAttempt(state map[string]any) int {
	switch value := state["supervisorRestartAttempt"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}

func parseSignalRuntimeSupervisorTime(value any) (time.Time, bool) {
	raw := stringValue(value)
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func (p *Platform) signalRuntimeSessionRunningOrStarting(sessionID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, exists := p.signalRun[sessionID]
	return exists
}

func (p *Platform) stopSignalRuntimeLinkedToStoppedLiveSession(runtimeSession domain.SignalRuntimeSession) bool {
	liveSession, found := p.findStoppedLiveSessionLinkedToRuntime(runtimeSession.ID)
	if !found {
		return false
	}
	requested := liveControlOperationInfo{
		Operation:        liveControlOperationScannerStop,
		AccountID:        runtimeSession.AccountID,
		StrategyID:       runtimeSession.StrategyID,
		LiveSessionID:    liveSession.ID,
		RuntimeSessionID: runtimeSession.ID,
	}
	release, acquired, current, lockErr := p.tryStartLiveControlOperation(requested)
	if lockErr != nil {
		p.logger("service.signal_runtime_scanner",
			"session_id", runtimeSession.ID,
			"live_session_id", liveSession.ID,
			"account_id", runtimeSession.AccountID,
			"strategy_id", runtimeSession.StrategyID,
		).Warn("skip stopped-live runtime cleanup because control operation key is invalid", "error", lockErr)
		return true
	}
	if !acquired {
		p.logger("service.signal_runtime_scanner",
			"session_id", runtimeSession.ID,
			"live_session_id", liveSession.ID,
			"account_id", runtimeSession.AccountID,
			"strategy_id", runtimeSession.StrategyID,
		).Warn("skip stopped-live runtime cleanup because control operation is already in progress", "error", liveControlOperationInProgressError(requested, current))
		return true
	}
	defer release()
	if _, err := p.stopLinkedLiveSignalRuntime(liveSession); err != nil {
		p.logger("service.signal_runtime_scanner",
			"session_id", runtimeSession.ID,
			"live_session_id", liveSession.ID,
			"account_id", runtimeSession.AccountID,
			"strategy_id", runtimeSession.StrategyID,
		).Warn("failed to stop signal runtime linked to stopped live session", "error", err)
		return true
	}
	p.logger("service.signal_runtime_scanner",
		"session_id", runtimeSession.ID,
		"live_session_id", liveSession.ID,
		"account_id", runtimeSession.AccountID,
		"strategy_id", runtimeSession.StrategyID,
	).Warn("stopped signal runtime linked to stopped live session")
	return true
}

func (p *Platform) findStoppedLiveSessionLinkedToRuntime(runtimeSessionID string) (domain.LiveSession, bool) {
	runtimeSessionID = strings.TrimSpace(runtimeSessionID)
	if runtimeSessionID == "" {
		return domain.LiveSession{}, false
	}
	sessions, err := p.ListLiveSessions()
	if err != nil {
		return domain.LiveSession{}, false
	}
	var stopped domain.LiveSession
	for _, session := range sessions {
		if !liveSessionReferencesRuntime(session, runtimeSessionID) {
			continue
		}
		if strings.EqualFold(session.Status, "RUNNING") {
			return domain.LiveSession{}, false
		}
		if strings.EqualFold(session.Status, "STOPPED") && stopped.ID == "" {
			stopped = session
		}
	}
	return stopped, stopped.ID != ""
}

func liveSessionReferencesRuntime(session domain.LiveSession, runtimeSessionID string) bool {
	runtimeSessionID = strings.TrimSpace(runtimeSessionID)
	if runtimeSessionID == "" {
		return false
	}
	return strings.TrimSpace(stringValue(session.State["signalRuntimeSessionId"])) == runtimeSessionID ||
		strings.TrimSpace(stringValue(session.State["lastSignalRuntimeSessionId"])) == runtimeSessionID
}
