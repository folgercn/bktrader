package service

import (
	"testing"
	"time"
)

func TestRestartBackoffUsesFirstThenRepeat(t *testing.T) {
	policy := SupervisorBackoffPolicy{
		First:  time.Minute,
		Repeat: 3 * time.Minute,
	}
	if got := RestartBackoff(policy, 0); got != time.Minute {
		t.Fatalf("expected attempt 0 to use first backoff, got %s", got)
	}
	if got := RestartBackoff(policy, 1); got != time.Minute {
		t.Fatalf("expected attempt 1 to use first backoff, got %s", got)
	}
	if got := RestartBackoff(policy, 2); got != 3*time.Minute {
		t.Fatalf("expected attempt 2 to use repeat backoff, got %s", got)
	}
	if got := RestartBackoff(policy, 3); got != 3*time.Minute {
		t.Fatalf("expected attempt 3 to keep repeat backoff, got %s", got)
	}
}

func TestRestartBackoffCapsAtMax(t *testing.T) {
	policy := SupervisorBackoffPolicy{
		First:  time.Minute,
		Repeat: 5 * time.Minute,
		Max:    2 * time.Minute,
	}
	if got := RestartBackoff(policy, 2); got != 2*time.Minute {
		t.Fatalf("expected repeat backoff capped at max, got %s", got)
	}
}

func TestRestartAttemptParsesLegacyStateTypes(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  int
	}{
		{name: "int", value: 3, want: 3},
		{name: "int64", value: int64(4), want: 4},
		{name: "float64", value: float64(5), want: 5},
		{name: "string", value: " 6 ", want: 6},
		{name: "invalid", value: "later", want: 0},
		{name: "missing", value: nil, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := map[string]any{}
			if tt.value != nil {
				state["restartAttempt"] = tt.value
			}
			if got := RestartAttempt(state, "restartAttempt"); got != tt.want {
				t.Fatalf("expected restart attempt %d, got %d", tt.want, got)
			}
		})
	}
}

func TestParseRestartTimeReturnsRFC3339Only(t *testing.T) {
	want := time.Date(2026, 4, 28, 12, 30, 0, 0, time.UTC)
	state := map[string]any{
		"nextRestartAt": want.Format(time.RFC3339),
	}
	got, ok := ParseRestartTime(state, "nextRestartAt")
	if !ok {
		t.Fatal("expected restart time to parse")
	}
	if !got.Equal(want) {
		t.Fatalf("expected %s, got %s", want, got)
	}
	state["nextRestartAt"] = "not-a-time"
	if _, ok := ParseRestartTime(state, "nextRestartAt"); ok {
		t.Fatal("expected invalid restart time to fail")
	}
}

func TestClearRestartStateDeletesOnlyRequestedKeys(t *testing.T) {
	state := map[string]any{
		"restartAttempt": 2,
		"nextRestartAt":  "2026-04-28T12:30:00Z",
		"health":         "recovering",
	}
	ClearRestartState(state, []string{"restartAttempt", "nextRestartAt"})
	if _, ok := state["restartAttempt"]; ok {
		t.Fatal("expected restartAttempt to be cleared")
	}
	if _, ok := state["nextRestartAt"]; ok {
		t.Fatal("expected nextRestartAt to be cleared")
	}
	if got := stringValue(state["health"]); got != "recovering" {
		t.Fatalf("expected unrelated health field to remain, got %s", got)
	}
}
