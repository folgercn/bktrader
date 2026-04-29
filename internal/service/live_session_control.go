package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	liveSessionControlScannerInterval = 5 * time.Second
	liveSessionControlEventStateKey   = "controlEvents"
	liveSessionControlEventMaxHistory = 50

	LiveSessionControlErrorCodeActivePositionsOrOrders    = "ACTIVE_POSITIONS_OR_ORDERS"
	LiveSessionControlErrorCodeRuntimeLeaseNotAcquired    = "RUNTIME_LEASE_NOT_ACQUIRED"
	LiveSessionControlErrorCodeControlOperationInProgress = "CONTROL_OPERATION_IN_PROGRESS"
	LiveSessionControlErrorCodeAdapterError               = "ADAPTER_ERROR"
	LiveSessionControlErrorCodeConfigError                = "CONFIG_ERROR"
	LiveSessionControlErrorCodeUnknown                    = "UNKNOWN"
)

var (
	ErrLiveControlAdapter = errors.New("live control adapter error")
	ErrLiveControlConfig  = errors.New("live control configuration error")
)

type liveControlClassError struct {
	class error
	cause error
}

func (e liveControlClassError) Error() string {
	if e.cause != nil {
		return e.cause.Error()
	}
	return e.class.Error()
}

func (e liveControlClassError) Unwrap() []error {
	if e.cause == nil {
		return []error{e.class}
	}
	return []error{e.class, e.cause}
}

func liveControlConfigErrorf(format string, args ...any) error {
	return liveControlClassError{
		class: ErrLiveControlConfig,
		cause: fmt.Errorf(format, args...),
	}
}

func wrapLiveControlConfigError(err error) error {
	if err == nil || errors.Is(err, ErrLiveControlConfig) {
		return err
	}
	return liveControlClassError{class: ErrLiveControlConfig, cause: err}
}

func liveControlAdapterErrorf(format string, args ...any) error {
	return liveControlClassError{
		class: ErrLiveControlAdapter,
		cause: fmt.Errorf(format, args...),
	}
}

func wrapLiveControlAdapterError(err error) error {
	if err == nil || errors.Is(err, ErrLiveControlAdapter) {
		return err
	}
	return liveControlClassError{class: ErrLiveControlAdapter, cause: err}
}

type liveSessionControlRequest struct {
	ID      string
	Version int64
}

type liveSessionControlStateCompareAndSwapStore interface {
	UpdateLiveSessionStateIfControlRequest(sessionID, requestID string, version int64, state map[string]any) (domain.LiveSession, bool, error)
}

func (p *Platform) RequestLiveSessionStart(sessionID string) (domain.LiveSession, error) {
	return p.requestLiveSessionDesiredStatus(sessionID, "RUNNING", false)
}

func (p *Platform) RequestLiveSessionStopWithForce(sessionID string, force bool) (domain.LiveSession, error) {
	return p.requestLiveSessionDesiredStatus(sessionID, "STOPPED", force)
}

func (p *Platform) requestLiveSessionDesiredStatus(sessionID, desired string, force bool) (domain.LiveSession, error) {
	sessionID = strings.TrimSpace(sessionID)
	for attempt := 0; attempt < 3; attempt++ {
		session, err := p.store.GetLiveSession(sessionID)
		if err != nil {
			return domain.LiveSession{}, err
		}
		state := cloneMetadata(session.State)
		previous := liveSessionControlRequest{
			ID:      strings.TrimSpace(stringValue(state["controlRequestId"])),
			Version: liveSessionControlVersion(state),
		}
		now := time.Now().UTC()
		version := previous.Version + 1
		state["desiredStatus"] = desired
		state["actualStatus"] = liveSessionActualStatusFromSession(session)
		state["controlRequestId"] = uuid.NewString()
		state["controlVersion"] = version
		state["lastControlAction"] = strings.ToLower(desired)
		state["controlRequestedAt"] = now.Format(time.RFC3339)
		if desired == "STOPPED" {
			state["lastControlAction"] = "stop"
			state["desiredStopForce"] = force
		} else {
			state["lastControlAction"] = "start"
			delete(state, "desiredStopForce")
		}
		delete(state, "activeControlRequestId")
		delete(state, "activeControlVersion")
		delete(state, "lastControlError")
		delete(state, "lastControlErrorAt")
		delete(state, "lastControlErrorCode")
		delete(state, "lastControlErrorRequestId")
		delete(state, "lastControlErrorVersion")
		request := liveSessionControlRequest{
			ID:      stringValue(state["controlRequestId"]),
			Version: version,
		}
		eventSession := session
		eventSession.State = state
		appendLiveSessionControlEvent(state, liveSessionControlEvent(eventSession, request, "request_accepted", desired, stringValue(state["actualStatus"]), nil, now))
		updated, ok, err := p.updateLiveSessionControlStateIfPrevious(session.ID, previous, state)
		if err != nil {
			return domain.LiveSession{}, err
		}
		if ok {
			return updated, nil
		}
	}
	return domain.LiveSession{}, fmt.Errorf("live session control request changed concurrently: %s", sessionID)
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
		if liveSessionControlErrorCurrent(session.State) {
			continue
		}
		request, ok := liveSessionControlRequestFromState(session.State)
		if !ok {
			updated, updatedRequest, err := p.initializeLegacyLiveSessionControlRequest(session, desired)
			if err != nil {
				p.logger("service.live_session_control_scanner", "session_id", session.ID).Warn("initialize legacy live session control request failed", "error", err)
				continue
			}
			session = updated
			request = updatedRequest
		}
		switch desired {
		case "RUNNING":
			if strings.EqualFold(session.Status, "RUNNING") {
				p.markLiveSessionControlActual(session, request, "RUNNING", nil)
				continue
			}
			p.executeLiveSessionControlStart(session, request)
		case "STOPPED":
			if strings.EqualFold(session.Status, "STOPPED") {
				p.markLiveSessionControlActual(session, request, "STOPPED", nil)
				continue
			}
			p.executeLiveSessionControlStop(session, request)
		}
	}
}

func (p *Platform) initializeLegacyLiveSessionControlRequest(session domain.LiveSession, desired string) (domain.LiveSession, liveSessionControlRequest, error) {
	state := cloneMetadata(session.State)
	now := time.Now().UTC()
	previous := liveSessionControlRequest{
		ID:      strings.TrimSpace(stringValue(state["controlRequestId"])),
		Version: liveSessionControlVersion(state),
	}
	request := liveSessionControlRequest{
		ID:      uuid.NewString(),
		Version: previous.Version + 1,
	}
	state["controlRequestId"] = request.ID
	state["controlVersion"] = request.Version
	if stringValue(state["controlRequestedAt"]) == "" {
		state["controlRequestedAt"] = now.Format(time.RFC3339)
	}
	if stringValue(state["lastControlAction"]) == "" {
		switch desired {
		case "RUNNING":
			state["lastControlAction"] = "start"
		case "STOPPED":
			state["lastControlAction"] = "stop"
		}
	}
	eventSession := session
	eventSession.State = state
	appendLiveSessionControlEvent(state, liveSessionControlEvent(eventSession, request, "legacy_request_initialized", desired, stringValue(state["actualStatus"]), nil, now))
	updated, ok, err := p.updateLiveSessionControlStateIfPrevious(session.ID, previous, state)
	if err != nil {
		return domain.LiveSession{}, liveSessionControlRequest{}, err
	}
	if !ok {
		return updated, liveSessionControlRequest{}, fmt.Errorf("live session control request changed concurrently: %s", session.ID)
	}
	return updated, request, nil
}

func (p *Platform) executeLiveSessionControlStart(session domain.LiveSession, request liveSessionControlRequest) {
	if !p.markLiveSessionControlActual(session, request, "STARTING", nil) {
		return
	}
	started, err := p.StartLiveSession(session.ID)
	if err != nil {
		p.markLiveSessionControlActual(session, request, "ERROR", err)
		return
	}
	p.markLiveSessionControlActual(started, request, "RUNNING", nil)
}

func (p *Platform) executeLiveSessionControlStop(session domain.LiveSession, request liveSessionControlRequest) {
	force := boolValue(session.State["desiredStopForce"])
	if !p.markLiveSessionControlActual(session, request, "STOPPING", nil) {
		return
	}
	stopped, err := p.StopLiveSessionWithForce(session.ID, force)
	if err != nil {
		p.markLiveSessionControlActual(session, request, "ERROR", err)
		return
	}
	p.markLiveSessionControlActual(stopped, request, "STOPPED", nil)
}

func (p *Platform) markLiveSessionControlActual(session domain.LiveSession, request liveSessionControlRequest, actual string, controlErr error) bool {
	latest, err := p.store.GetLiveSession(session.ID)
	if err != nil {
		p.logger("service.live_session_control_scanner", "session_id", session.ID).Warn("load live session control state failed", "error", err)
		return false
	}
	if !liveSessionControlRequestMatches(latest.State, request) {
		p.logger("service.live_session_control_scanner", "session_id", session.ID, "request_id", request.ID, "control_version", request.Version).Info("skip stale live session control update")
		p.recordLiveSessionControlStaleDiscard(latest, request, "actual_status_update")
		return false
	}
	state := cloneMetadata(session.State)
	for key, value := range latest.State {
		state[key] = value
	}
	now := time.Now().UTC()
	state["actualStatus"] = actual
	state["lastControlUpdateAt"] = now.Format(time.RFC3339)
	state["activeControlRequestId"] = request.ID
	state["activeControlVersion"] = request.Version
	appendLiveSessionControlEvent(state, liveSessionControlEvent(latest, request, liveSessionControlEventPhase(actual, controlErr), stringValue(state["desiredStatus"]), actual, controlErr, now))
	if controlErr != nil {
		state["lastControlError"] = controlErr.Error()
		state["lastControlErrorCode"] = liveSessionControlErrorCode(controlErr)
		state["lastControlErrorAt"] = now.Format(time.RFC3339)
		state["lastControlErrorRequestId"] = request.ID
		state["lastControlErrorVersion"] = request.Version
	} else {
		delete(state, "lastControlError")
		delete(state, "lastControlErrorCode")
		delete(state, "lastControlErrorAt")
		delete(state, "lastControlErrorRequestId")
		delete(state, "lastControlErrorVersion")
	}
	if actual == "RUNNING" || actual == "STOPPED" || actual == "ERROR" {
		delete(state, "activeControlRequestId")
		delete(state, "activeControlVersion")
		if controlErr == nil {
			state["lastControlSucceededAt"] = now.Format(time.RFC3339)
		}
	}
	updated, ok, err := p.updateLiveSessionControlStateIfCurrent(session.ID, request, state)
	if err != nil {
		p.logger("service.live_session_control_scanner", "session_id", session.ID).Warn("update live session control state failed", "error", err)
		return false
	}
	if !ok {
		p.logger("service.live_session_control_scanner", "session_id", session.ID, "request_id", request.ID, "control_version", request.Version).Info("skip stale live session control update")
		if latest, loadErr := p.store.GetLiveSession(session.ID); loadErr == nil {
			p.recordLiveSessionControlStaleDiscard(latest, request, "actual_status_commit")
		}
		return false
	}
	_ = updated
	return true
}

func (p *Platform) recordLiveSessionControlStaleDiscard(session domain.LiveSession, request liveSessionControlRequest, reason string) {
	state := cloneMetadata(session.State)
	current, ok := liveSessionControlRequestFromState(state)
	if !ok {
		return
	}
	now := time.Now().UTC()
	event := liveSessionControlEvent(session, request, "stale_update_discarded", stringValue(state["desiredStatus"]), stringValue(state["actualStatus"]), nil, now)
	event["staleReason"] = reason
	event["currentControlRequestId"] = stringValue(state["controlRequestId"])
	event["currentControlVersion"] = liveSessionControlVersion(state)
	appendLiveSessionControlEvent(state, event)
	if _, updated, err := p.updateLiveSessionControlStateIfPrevious(session.ID, current, state); err != nil {
		p.logger("service.live_session_control_scanner", "session_id", session.ID, "request_id", request.ID, "control_version", request.Version).Warn("record stale live session control event failed", "error", err)
	} else if !updated {
		p.logger("service.live_session_control_scanner", "session_id", session.ID, "request_id", request.ID, "control_version", request.Version).Info("skip stale live session control event update")
	}
}

func (p *Platform) updateLiveSessionControlStateIfCurrent(sessionID string, request liveSessionControlRequest, state map[string]any) (domain.LiveSession, bool, error) {
	return p.updateLiveSessionControlStateIfPrevious(sessionID, request, state)
}

func (p *Platform) updateLiveSessionControlStateIfPrevious(sessionID string, previous liveSessionControlRequest, state map[string]any) (domain.LiveSession, bool, error) {
	if store, ok := p.store.(liveSessionControlStateCompareAndSwapStore); ok {
		return store.UpdateLiveSessionStateIfControlRequest(sessionID, previous.ID, previous.Version, state)
	}
	latest, err := p.store.GetLiveSession(sessionID)
	if err != nil {
		return domain.LiveSession{}, false, err
	}
	if stringValue(latest.State["controlRequestId"]) != previous.ID || liveSessionControlVersion(latest.State) != previous.Version {
		return latest, false, nil
	}
	updated, err := p.store.UpdateLiveSessionState(sessionID, state)
	return updated, err == nil, err
}

func liveSessionControlErrorCode(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrActivePositionsOrOrders):
		return LiveSessionControlErrorCodeActivePositionsOrOrders
	case errors.Is(err, ErrRuntimeLeaseNotAcquired):
		return LiveSessionControlErrorCodeRuntimeLeaseNotAcquired
	case errors.Is(err, ErrLiveControlOperationInProgress), errors.Is(err, ErrLiveAccountOperationInProgress):
		return LiveSessionControlErrorCodeControlOperationInProgress
	case errors.Is(err, ErrLiveControlConfig):
		return LiveSessionControlErrorCodeConfigError
	case errors.Is(err, ErrLiveControlAdapter):
		return LiveSessionControlErrorCodeAdapterError
	default:
		return LiveSessionControlErrorCodeUnknown
	}
}

func appendLiveSessionControlEvent(state map[string]any, event map[string]any) {
	if state == nil || event == nil {
		return
	}
	events := metadataList(state[liveSessionControlEventStateKey])
	events = append(events, cloneMetadata(event))
	if len(events) > liveSessionControlEventMaxHistory {
		events = events[len(events)-liveSessionControlEventMaxHistory:]
	}
	state[liveSessionControlEventStateKey] = events
}

func liveSessionControlEvent(session domain.LiveSession, request liveSessionControlRequest, phase, desired, actual string, controlErr error, eventTime time.Time) map[string]any {
	if eventTime.IsZero() {
		eventTime = time.Now().UTC()
	}
	state := cloneMetadata(session.State)
	event := map[string]any{
		"id":               uuid.NewString(),
		"type":             "live-control",
		"phase":            phase,
		"liveSessionId":    session.ID,
		"accountId":        session.AccountID,
		"strategyId":       session.StrategyID,
		"sessionStatus":    session.Status,
		"desiredStatus":    strings.ToUpper(strings.TrimSpace(desired)),
		"actualStatus":     strings.ToUpper(strings.TrimSpace(actual)),
		"controlRequestId": request.ID,
		"controlVersion":   request.Version,
		"action":           firstNonEmpty(stringValue(state["lastControlAction"]), liveSessionControlActionFromDesired(desired)),
		"eventTime":        eventTime.UTC().Format(time.RFC3339Nano),
		"recordedAt":       eventTime.UTC().Format(time.RFC3339Nano),
	}
	if force, ok := state["desiredStopForce"]; ok {
		event["force"] = boolValue(force)
	}
	if requestedAt, ok := parseLiveSessionControlTime(state["controlRequestedAt"]); ok {
		event["controlRequestedAt"] = requestedAt.UTC().Format(time.RFC3339Nano)
		event["latencyMs"] = eventTime.UTC().Sub(requestedAt.UTC()).Milliseconds()
	}
	if controlErr != nil {
		event["error"] = controlErr.Error()
		event["errorCode"] = liveSessionControlErrorCode(controlErr)
	}
	return event
}

func liveSessionControlActionFromDesired(desired string) string {
	switch strings.ToUpper(strings.TrimSpace(desired)) {
	case "RUNNING":
		return "start"
	case "STOPPED":
		return "stop"
	default:
		return strings.ToLower(strings.TrimSpace(desired))
	}
}

func liveSessionControlEventPhase(actual string, controlErr error) string {
	if controlErr != nil || strings.EqualFold(actual, "ERROR") {
		return "failed"
	}
	switch strings.ToUpper(strings.TrimSpace(actual)) {
	case "STARTING", "STOPPING":
		return "runner_picked_up"
	case "RUNNING", "STOPPED":
		return "succeeded"
	default:
		return "actual_status_changed"
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

func liveSessionControlErrorCurrent(state map[string]any) bool {
	if requestID := stringValue(state["controlRequestId"]); requestID != "" {
		if errorRequestID := stringValue(state["lastControlErrorRequestId"]); errorRequestID != "" && errorRequestID != requestID {
			return false
		}
	}
	if version := liveSessionControlVersion(state); version > 0 {
		if errorVersion := liveSessionControlVersionKey(state, "lastControlErrorVersion"); errorVersion > 0 && errorVersion != version {
			return false
		}
	}
	if !strings.EqualFold(stringValue(state["actualStatus"]), "ERROR") {
		return false
	}
	requestedAt, requestedOK := parseLiveSessionControlTime(state["controlRequestedAt"])
	errorAt, errorOK := parseLiveSessionControlTime(state["lastControlErrorAt"])
	if !requestedOK || !errorOK {
		return true
	}
	return !requestedAt.After(errorAt)
}

func liveSessionControlRequestFromState(state map[string]any) (liveSessionControlRequest, bool) {
	request := liveSessionControlRequest{
		ID:      strings.TrimSpace(stringValue(state["controlRequestId"])),
		Version: liveSessionControlVersion(state),
	}
	return request, request.ID != "" && request.Version > 0
}

func liveSessionControlRequestMatches(state map[string]any, request liveSessionControlRequest) bool {
	current, ok := liveSessionControlRequestFromState(state)
	return ok && current.ID == request.ID && current.Version == request.Version
}

func liveSessionControlVersion(state map[string]any) int64 {
	return liveSessionControlVersionKey(state, "controlVersion")
}

func liveSessionControlVersionKey(state map[string]any, key string) int64 {
	if state == nil {
		return 0
	}
	switch value := state[key].(type) {
	case int:
		return int64(value)
	case int8:
		return int64(value)
	case int16:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case uint:
		if uint64(value) > math.MaxInt64 {
			return 0
		}
		return int64(value)
	case uint8:
		return int64(value)
	case uint16:
		return int64(value)
	case uint32:
		return int64(value)
	case uint64:
		if value > math.MaxInt64 {
			return 0
		}
		return int64(value)
	case float32:
		return int64(value)
	case float64:
		return int64(value)
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err == nil {
			return parsed
		}
		parsedFloat, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return int64(parsedFloat)
		}
	case fmt.Stringer:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value.String()), 10, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func parseLiveSessionControlTime(value any) (time.Time, bool) {
	raw := strings.TrimSpace(stringValue(value))
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err == nil {
		return parsed, true
	}
	parsed, err = time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
