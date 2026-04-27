package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const signalRuntimeScannerInterval = 5 * time.Second

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
	if session.Status == "ERROR" || stringValue(session.State["actualStatus"]) == "ERROR" {
		return false
	}
	desired := stringValue(session.State["desiredStatus"])
	if desired != "" {
		return desired == "RUNNING"
	}
	return session.Status == "RUNNING"
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
	release, acquired, current := p.tryStartLiveControlOperation(requested)
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
