package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestRuntimeStatusFromLiveSessionUsesFreshestStateTimestamp(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	lastEventAt := createdAt.Add(3 * time.Minute)
	lastEvaluationAt := createdAt.Add(9 * time.Minute)
	session := domain.LiveSession{
		ID:         "live-session-1",
		AccountID:  "live-main",
		StrategyID: "strategy-1",
		Status:     "RUNNING",
		CreatedAt:  createdAt,
		State: map[string]any{
			"lastEventAt":              lastEventAt.Format(time.RFC3339),
			"lastStrategyEvaluationAt": lastEvaluationAt.Format(time.RFC3339),
		},
	}

	status := runtimeStatusFromLiveSession("platform-api", createdAt.Add(time.Hour), session)
	if status.UpdatedAt == nil {
		t.Fatal("expected live runtime status updatedAt from runtime state")
	}
	if !status.UpdatedAt.Equal(lastEvaluationAt) {
		t.Fatalf("expected updatedAt %s, got %s", lastEvaluationAt.Format(time.RFC3339), status.UpdatedAt.Format(time.RFC3339))
	}
}

func TestRuntimeStatusFromLiveSessionOmitsUpdatedAtWhenStateHasNoFreshnessTime(t *testing.T) {
	createdAt := time.Date(2026, 4, 28, 8, 0, 0, 0, time.UTC)
	session := domain.LiveSession{
		ID:         "live-session-1",
		AccountID:  "live-main",
		StrategyID: "strategy-1",
		Status:     "RUNNING",
		CreatedAt:  createdAt,
		State:      map[string]any{},
	}

	status := runtimeStatusFromLiveSession("platform-api", createdAt.Add(time.Hour), session)
	if status.UpdatedAt != nil {
		t.Fatalf("expected live runtime status to avoid createdAt fallback, got %s", status.UpdatedAt.Format(time.RFC3339))
	}
}
