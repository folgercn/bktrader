package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestScanLiveSessionControlRequestsStopsDesiredStoppedSession(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, false); err != nil {
		t.Fatalf("request stop failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	updated, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if updated.Status != "STOPPED" {
		t.Fatalf("expected STOPPED status, got %s", updated.Status)
	}
	if got := stringValue(updated.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected actualStatus STOPPED, got %s", got)
	}
}

func TestScanLiveSessionControlRequestsInitializesLegacyDesiredIntent(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "STOPPED"
	state["actualStatus"] = "RUNNING"
	if _, err := store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("set legacy desired stopped failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	updated, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if updated.Status != "STOPPED" {
		t.Fatalf("expected STOPPED status, got %s", updated.Status)
	}
	if got := stringValue(updated.State["controlRequestId"]); got == "" {
		t.Fatal("expected scanner to initialize controlRequestId for legacy intent")
	}
	if got := liveSessionControlVersion(updated.State); got != 1 {
		t.Fatalf("expected scanner to initialize controlVersion 1 for legacy intent, got %d", got)
	}
}

func TestRequestLiveSessionControlAssignsRequestIdentityAndVersion(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)

	started, err := platform.RequestLiveSessionStart("live-session-main")
	if err != nil {
		t.Fatalf("request start failed: %v", err)
	}
	firstRequestID := stringValue(started.State["controlRequestId"])
	if firstRequestID == "" {
		t.Fatal("expected controlRequestId on start request")
	}
	if got := liveSessionControlVersion(started.State); got != 1 {
		t.Fatalf("expected first controlVersion 1, got %d", got)
	}
	if got := stringValue(started.State["lastControlAction"]); got != "start" {
		t.Fatalf("expected lastControlAction start, got %s", got)
	}

	stopped, err := platform.RequestLiveSessionStopWithForce("live-session-main", true)
	if err != nil {
		t.Fatalf("request stop failed: %v", err)
	}
	if got := stringValue(stopped.State["controlRequestId"]); got == "" || got == firstRequestID {
		t.Fatalf("expected new controlRequestId on stop request, got %s", got)
	}
	if got := liveSessionControlVersion(stopped.State); got != 2 {
		t.Fatalf("expected second controlVersion 2, got %d", got)
	}
	if got := stringValue(stopped.State["lastControlAction"]); got != "stop" {
		t.Fatalf("expected lastControlAction stop, got %s", got)
	}
}

func TestLiveSessionControlStaleRequestCannotOverwriteNewerRequest(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	stopRequest, err := platform.RequestLiveSessionStopWithForce(session.ID, true)
	if err != nil {
		t.Fatalf("request stop failed: %v", err)
	}
	staleRequest, ok := liveSessionControlRequestFromState(stopRequest.State)
	if !ok {
		t.Fatal("expected stop request identity")
	}
	if !platform.markLiveSessionControlActual(stopRequest, staleRequest, "STOPPING", nil) {
		t.Fatal("expected current stop request to mark STOPPING")
	}

	startRequest, err := platform.RequestLiveSessionStart(session.ID)
	if err != nil {
		t.Fatalf("request start failed: %v", err)
	}
	if got := liveSessionControlVersion(startRequest.State); got != staleRequest.Version+1 {
		t.Fatalf("expected newer request version, got %d after %d", got, staleRequest.Version)
	}
	if platform.markLiveSessionControlActual(stopRequest, staleRequest, "STOPPED", nil) {
		t.Fatal("expected stale stop result to be discarded")
	}

	latest, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(latest.State["desiredStatus"]); got != "RUNNING" {
		t.Fatalf("expected newer desiredStatus RUNNING, got %s", got)
	}
	if got := stringValue(latest.State["actualStatus"]); got != "RUNNING" {
		t.Fatalf("expected stale stop not to overwrite actualStatus, got %s", got)
	}
	if got := liveSessionControlVersion(latest.State); got != staleRequest.Version+1 {
		t.Fatalf("expected newer controlVersion to remain, got %d", got)
	}
	if got := stringValue(latest.State["controlRequestId"]); got != stringValue(startRequest.State["controlRequestId"]) {
		t.Fatalf("expected newer controlRequestId to remain, got %s", got)
	}
}

func TestScanLiveSessionControlRequestsPreservesStopSafetyUntilForceRequested(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, false); err != nil {
		t.Fatalf("request stop failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	blocked, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if blocked.Status != "RUNNING" {
		t.Fatalf("expected session to remain RUNNING after blocked stop, got %s", blocked.Status)
	}
	if got := stringValue(blocked.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected actualStatus ERROR, got %s", got)
	}
	if got := stringValue(blocked.State["lastControlErrorCode"]); got != LiveSessionControlErrorCodeActivePositionsOrOrders {
		t.Fatalf("expected lastControlErrorCode %s, got %s", LiveSessionControlErrorCodeActivePositionsOrOrders, got)
	}
	events := metadataList(blocked.State[liveSessionControlEventStateKey])
	if len(events) != 3 {
		t.Fatalf("expected request, pickup, and failure events, got %d: %#v", len(events), events)
	}
	if got := stringValue(events[2]["phase"]); got != "failed" {
		t.Fatalf("expected failed control event, got %s", got)
	}
	if got := stringValue(events[2]["errorCode"]); got != LiveSessionControlErrorCodeActivePositionsOrOrders {
		t.Fatalf("expected failed event errorCode %s, got %s", LiveSessionControlErrorCodeActivePositionsOrOrders, got)
	}
	page, err := platform.ListLogEvents(UnifiedLogEventQuery{
		Type:          "live-control",
		Level:         "critical",
		LiveSessionID: session.ID,
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list critical live control events: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected one critical live control event, got %d", len(page.Items))
	}

	platform.scanLiveSessionControlRequests(context.Background())
	stillBlocked, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if stillBlocked.Status != "RUNNING" {
		t.Fatalf("expected ERROR state not to auto retry, got %s", stillBlocked.Status)
	}

	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, true); err != nil {
		t.Fatalf("request forced stop failed: %v", err)
	}
	platform.scanLiveSessionControlRequests(context.Background())

	stopped, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if stopped.Status != "STOPPED" {
		t.Fatalf("expected forced stop to stop session, got %s", stopped.Status)
	}
	if got := stringValue(stopped.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected actualStatus STOPPED after force, got %s", got)
	}
}

func TestLiveSessionControlForceStopRecoversFromPreviousError(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	if _, err := store.SavePosition(domain.Position{
		AccountID:         session.AccountID,
		StrategyVersionID: "strategy-version-bk-1d-v010",
		Symbol:            "BTCUSDT",
		Side:              "LONG",
		Quantity:          0.002,
		EntryPrice:        69000,
		MarkPrice:         69100,
	}); err != nil {
		t.Fatalf("save position failed: %v", err)
	}
	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, false); err != nil {
		t.Fatalf("request non-force stop failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	failed, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get failed live session failed: %v", err)
	}
	if failed.Status != "RUNNING" {
		t.Fatalf("expected non-force stop to leave session RUNNING, got %s", failed.Status)
	}
	if got := stringValue(failed.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected actualStatus ERROR after blocked stop, got %s", got)
	}
	if got := stringValue(failed.State["lastControlError"]); got == "" {
		t.Fatal("expected lastControlError after blocked stop")
	}
	if got := stringValue(failed.State["lastControlErrorCode"]); got != LiveSessionControlErrorCodeActivePositionsOrOrders {
		t.Fatalf("expected lastControlErrorCode %s, got %s", LiveSessionControlErrorCodeActivePositionsOrOrders, got)
	}

	if _, err := platform.RequestLiveSessionStopWithForce(session.ID, true); err != nil {
		t.Fatalf("request force stop retry failed: %v", err)
	}
	retry, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get retry live session failed: %v", err)
	}
	if got := stringValue(retry.State["actualStatus"]); got == "ERROR" {
		t.Fatal("expected force stop request to clear previous ERROR actualStatus")
	}
	if got := stringValue(retry.State["lastControlErrorCode"]); got != "" {
		t.Fatalf("expected force stop request to clear previous error code, got %s", got)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	stopped, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get stopped live session failed: %v", err)
	}
	if stopped.Status != "STOPPED" {
		t.Fatalf("expected force stop retry to stop session, got %s", stopped.Status)
	}
	if got := stringValue(stopped.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected actualStatus STOPPED after force stop recovery, got %s", got)
	}
	if got := stringValue(stopped.State["lastControlError"]); got != "" {
		t.Fatalf("expected lastControlError cleared after recovery, got %s", got)
	}
	if got := stringValue(stopped.State["lastControlErrorCode"]); got != "" {
		t.Fatalf("expected lastControlErrorCode cleared after recovery, got %s", got)
	}
}

func TestScanLiveSessionControlRequestsWritesErrorWithoutRetryingStart(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	if _, err := platform.RequestLiveSessionStart("live-session-main"); err != nil {
		t.Fatalf("request start failed: %v", err)
	}

	platform.scanLiveSessionControlRequests(context.Background())

	failed, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if failed.Status == "RUNNING" {
		t.Fatal("expected start to fail for unconfigured live account")
	}
	if got := stringValue(failed.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected actualStatus ERROR, got %s", got)
	}
	if got := stringValue(failed.State["lastControlError"]); got == "" {
		t.Fatal("expected lastControlError")
	}
	if got := stringValue(failed.State["lastControlErrorCode"]); got != LiveSessionControlErrorCodeConfigError {
		t.Fatalf("expected lastControlErrorCode %s, got %s", LiveSessionControlErrorCodeConfigError, got)
	}

	platform.scanLiveSessionControlRequests(context.Background())
	stillFailed, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(stillFailed.State["actualStatus"]); got != "ERROR" {
		t.Fatalf("expected ERROR not to auto retry, got %s", got)
	}
}

func TestLiveSessionControlErrorCodeClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "active exposure", err: activePositionsOrOrdersError{}, want: LiveSessionControlErrorCodeActivePositionsOrOrders},
		{name: "runtime lease", err: fmt.Errorf("wrapped: %w", ErrRuntimeLeaseNotAcquired), want: LiveSessionControlErrorCodeRuntimeLeaseNotAcquired},
		{name: "control operation", err: fmt.Errorf("wrapped: %w", ErrLiveControlOperationInProgress), want: LiveSessionControlErrorCodeControlOperationInProgress},
		{name: "account operation", err: fmt.Errorf("wrapped: %w", ErrLiveAccountOperationInProgress), want: LiveSessionControlErrorCodeControlOperationInProgress},
		{name: "config", err: wrapLiveControlConfigError(fmt.Errorf("live account is not configured")), want: LiveSessionControlErrorCodeConfigError},
		{name: "adapter", err: wrapLiveControlAdapterError(fmt.Errorf("binance request failed: 500 Internal Server Error")), want: LiveSessionControlErrorCodeAdapterError},
		{name: "unknown", err: fmt.Errorf("adapter returned unexpected status"), want: LiveSessionControlErrorCodeUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := liveSessionControlErrorCode(tc.err); got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestLiveSessionControlErrorCurrentAllowsNewerRequest(t *testing.T) {
	now := time.Now().UTC()
	state := map[string]any{
		"actualStatus":       "ERROR",
		"controlRequestedAt": now.Add(-time.Minute).Format(time.RFC3339),
		"lastControlErrorAt": now.Format(time.RFC3339),
	}
	if !liveSessionControlErrorCurrent(state) {
		t.Fatal("expected current control error to block automatic retry")
	}

	state["controlRequestedAt"] = now.Add(time.Minute).Format(time.RFC3339)
	if liveSessionControlErrorCurrent(state) {
		t.Fatal("expected newer control request to wake scanner")
	}
}

func TestDeleteLiveSessionCancelsPendingDesiredControlIntent(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	if _, err := platform.RequestLiveSessionStart("live-session-main"); err != nil {
		t.Fatalf("request start failed: %v", err)
	}

	if err := platform.DeleteLiveSessionWithForce("live-session-main", true); err != nil {
		t.Fatalf("delete live session failed: %v", err)
	}
	platform.scanLiveSessionControlRequests(context.Background())

	deleted, err := store.GetLiveSession("live-session-main")
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if deleted.Status != "DELETED" {
		t.Fatalf("expected DELETED status, got %s", deleted.Status)
	}
	if got := stringValue(deleted.State["desiredStatus"]); got != "STOPPED" {
		t.Fatalf("expected delete to cancel desiredStatus as STOPPED, got %s", got)
	}
	if got := stringValue(deleted.State["actualStatus"]); got != "STOPPED" {
		t.Fatalf("expected delete to mark actualStatus STOPPED, got %s", got)
	}
	if got := stringValue(deleted.State["controlDeletedAt"]); got == "" {
		t.Fatal("expected controlDeletedAt after delete")
	}
	if _, ok := deleted.State["desiredStopForce"]; ok {
		t.Fatalf("expected desiredStopForce to be cleared, got %#v", deleted.State["desiredStopForce"])
	}

	listed, err := store.ListLiveSessions()
	if err != nil {
		t.Fatalf("list live sessions failed: %v", err)
	}
	for _, item := range listed {
		if item.ID == deleted.ID {
			t.Fatalf("expected deleted session to be hidden from scanner list")
		}
	}
}

func TestLiveSessionControlRecordsAuditEvents(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	requested, err := platform.RequestLiveSessionStopWithForce(session.ID, false)
	if err != nil {
		t.Fatalf("request stop failed: %v", err)
	}
	request, ok := liveSessionControlRequestFromState(requested.State)
	if !ok {
		t.Fatal("expected control request identity")
	}

	platform.scanLiveSessionControlRequests(context.Background())

	stopped, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	events := metadataList(stopped.State[liveSessionControlEventStateKey])
	if len(events) != 3 {
		t.Fatalf("expected request, pickup, and success events, got %d: %#v", len(events), events)
	}
	wantPhases := []string{"request_accepted", "runner_picked_up", "succeeded"}
	for i, want := range wantPhases {
		if got := stringValue(events[i]["phase"]); got != want {
			t.Fatalf("event %d expected phase %s, got %s", i, want, got)
		}
		if got := stringValue(events[i]["controlRequestId"]); got != request.ID {
			t.Fatalf("event %d expected request id %s, got %s", i, request.ID, got)
		}
		if got := liveSessionControlVersionKey(events[i], "controlVersion"); got != request.Version {
			t.Fatalf("event %d expected version %d, got %d", i, request.Version, got)
		}
	}
	if got := stringValue(events[0]["action"]); got != "stop" {
		t.Fatalf("expected stop action, got %s", got)
	}
	if got := boolValue(events[0]["force"]); got {
		t.Fatal("expected non-force audit event")
	}

	page, err := platform.ListLogEvents(UnifiedLogEventQuery{
		Type:          "live-control",
		LiveSessionID: session.ID,
		Limit:         10,
	})
	if err != nil {
		t.Fatalf("list live control log events: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("expected 3 unified live control events, got %d", len(page.Items))
	}
	if page.Items[0].Type != "live-control" || page.Items[0].Level != "info" {
		t.Fatalf("unexpected newest live control event: %#v", page.Items[0])
	}
	if got := stringValue(page.Items[0].Metadata["phase"]); got != "succeeded" {
		t.Fatalf("expected newest event to be succeeded, got %s", got)
	}
}

func TestRecoverLiveTradingOnStartupSkipsLiveSessionDesiredStopped(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	session, err := store.UpdateLiveSessionStatus("live-session-main", "RUNNING")
	if err != nil {
		t.Fatalf("set live session running failed: %v", err)
	}
	state := cloneMetadata(session.State)
	state["desiredStatus"] = "STOPPED"
	if _, err := store.UpdateLiveSessionState(session.ID, state); err != nil {
		t.Fatalf("set desired stopped failed: %v", err)
	}

	platform.RecoverLiveTradingOnStartup(context.Background())

	updated, err := store.GetLiveSession(session.ID)
	if err != nil {
		t.Fatalf("get live session failed: %v", err)
	}
	if got := stringValue(updated.State["lastRecoveryStatus"]); got != "skipped-live-desired-stopped" {
		t.Fatalf("expected skipped-live-desired-stopped, got %s", got)
	}
}
