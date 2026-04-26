package service

import (
	"context"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const signalRuntimeScannerInterval = 5 * time.Second

type signalRuntimeSessionStarter func(string) (domain.SignalRuntimeSession, error)

func (p *Platform) StartSignalRuntimeScanner(ctx context.Context) {
	logger := p.logger("service.signal_runtime_scanner")
	logger.Info("signal runtime scanner started")
	go func() {
		defer logger.Info("signal runtime scanner stopped")
		p.scanSignalRuntimeSessions(ctx, p.StartSignalRuntimeSession)
		ticker := time.NewTicker(signalRuntimeScannerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.scanSignalRuntimeSessions(ctx, p.StartSignalRuntimeSession)
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
		if p.signalRuntimeSessionRunningOrStarting(session.ID) {
			continue
		}
		if _, err := starter(session.ID); err != nil {
			p.logger("service.signal_runtime_scanner",
				"session_id", session.ID,
				"account_id", session.AccountID,
				"strategy_id", session.StrategyID,
			).Warn("signal runtime scanner failed to start session", "error", err)
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
