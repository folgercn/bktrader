package service

import (
	"context"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const liveSessionControlScannerInterval = 5 * time.Second

func (p *Platform) RequestLiveSessionStart(sessionID string) (domain.LiveSession, error) {
	return p.requestLiveSessionDesiredStatus(sessionID, "RUNNING", false)
}

func (p *Platform) RequestLiveSessionStopWithForce(sessionID string, force bool) (domain.LiveSession, error) {
	return p.requestLiveSessionDesiredStatus(sessionID, "STOPPED", force)
}

func (p *Platform) requestLiveSessionDesiredStatus(sessionID, desired string, force bool) (domain.LiveSession, error) {
	session, err := p.store.GetLiveSession(strings.TrimSpace(sessionID))
	if err != nil {
		return domain.LiveSession{}, err
	}
	state := cloneMetadata(session.State)
	now := time.Now().UTC()
	state["desiredStatus"] = desired
	state["actualStatus"] = liveSessionActualStatusFromSession(session)
	state["controlRequestedAt"] = now.Format(time.RFC3339)
	if desired == "STOPPED" {
		state["desiredStopForce"] = force
	} else {
		delete(state, "desiredStopForce")
	}
	delete(state, "lastControlError")
	delete(state, "lastControlErrorAt")
	return p.store.UpdateLiveSessionState(session.ID, state)
}

func (p *Platform) StartLiveSessionControlScanner(ctx context.Context) {
	logger := p.logger("service.live_session_control_scanner")
	logger.Info("live session control scanner started")
	go func() {
		defer logger.Info("live session control scanner stopped")
		p.scanLiveSessionControlRequests(ctx)
		ticker := time.NewTicker(liveSessionControlScannerInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				p.scanLiveSessionControlRequests(ctx)
			}
		}
	}()
}

func (p *Platform) scanLiveSessionControlRequests(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	sessions, err := p.ListLiveSessions()
	if err != nil {
		p.logger("service.live_session_control_scanner").Warn("list live sessions failed", "error", err)
		return
	}
	for _, session := range sessions {
		if err := ctx.Err(); err != nil {
			return
		}
		desired := strings.ToUpper(strings.TrimSpace(stringValue(session.State["desiredStatus"])))
		if desired == "" {
			continue
		}
		if strings.EqualFold(stringValue(session.State["actualStatus"]), "ERROR") {
			continue
		}
		switch desired {
		case "RUNNING":
			if strings.EqualFold(session.Status, "RUNNING") {
				p.markLiveSessionControlActual(session, "RUNNING", nil)
				continue
			}
			p.executeLiveSessionControlStart(session)
		case "STOPPED":
			if strings.EqualFold(session.Status, "STOPPED") {
				p.markLiveSessionControlActual(session, "STOPPED", nil)
				continue
			}
			p.executeLiveSessionControlStop(session)
		}
	}
}

func (p *Platform) executeLiveSessionControlStart(session domain.LiveSession) {
	p.markLiveSessionControlActual(session, "STARTING", nil)
	started, err := p.StartLiveSession(session.ID)
	if err != nil {
		p.markLiveSessionControlActual(session, "ERROR", err)
		return
	}
	p.markLiveSessionControlActual(started, "RUNNING", nil)
}

func (p *Platform) executeLiveSessionControlStop(session domain.LiveSession) {
	force := boolValue(session.State["desiredStopForce"])
	p.markLiveSessionControlActual(session, "STOPPING", nil)
	stopped, err := p.StopLiveSessionWithForce(session.ID, force)
	if err != nil {
		p.markLiveSessionControlActual(session, "ERROR", err)
		return
	}
	p.markLiveSessionControlActual(stopped, "STOPPED", nil)
}

func (p *Platform) markLiveSessionControlActual(session domain.LiveSession, actual string, controlErr error) {
	state := cloneMetadata(session.State)
	now := time.Now().UTC()
	state["actualStatus"] = actual
	state["lastControlUpdateAt"] = now.Format(time.RFC3339)
	if controlErr != nil {
		state["lastControlError"] = controlErr.Error()
		state["lastControlErrorAt"] = now.Format(time.RFC3339)
	} else {
		delete(state, "lastControlError")
		delete(state, "lastControlErrorAt")
	}
	if _, err := p.store.UpdateLiveSessionState(session.ID, state); err != nil {
		p.logger("service.live_session_control_scanner", "session_id", session.ID).Warn("update live session control state failed", "error", err)
	}
}

func liveSessionActualStatusFromSession(session domain.LiveSession) string {
	switch strings.ToUpper(strings.TrimSpace(session.Status)) {
	case "RUNNING":
		return "RUNNING"
	case "STOPPED":
		return "STOPPED"
	default:
		if actual := strings.ToUpper(strings.TrimSpace(stringValue(session.State["actualStatus"]))); actual != "" && actual != "ERROR" {
			return actual
		}
		return "STOPPED"
	}
}
