package service

import (
	"reflect"
	"testing"
)

func TestStripHeavyState(t *testing.T) {
	t.Run("nil state", func(t *testing.T) {
		if got := stripHeavyState(nil); got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})

	t.Run("state without heavy fields", func(t *testing.T) {
		input := map[string]any{
			"status": "RUNNING",
			"count":  10,
		}
		got := stripHeavyState(input)
		if !reflect.DeepEqual(got, input) {
			t.Errorf("expected %v, got %v", input, got)
		}
		// Modify returned map to ensure it's a copy
		got["status"] = "STOPPED"
		if input["status"] != "RUNNING" {
			t.Errorf("expected original map to be unchanged")
		}
	})

	t.Run("state with heavy fields", func(t *testing.T) {
		input := map[string]any{
			"status":                             "RUNNING",
			"count":                              10,
			"sourceStates":                       map[string]any{"data": "heavy1"},
			"signalBarStates":                    map[string]any{"data": "heavy2"},
			"lastStrategyEvaluationSourceStates": map[string]any{"data": "heavy3"},
			"lastStrategyEvaluationSignalBarStates": map[string]any{
				"data": "heavy4",
			},
		}
		got := stripHeavyState(input)
		expected := map[string]any{
			"status": "RUNNING",
			"count":  10,
		}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
		// Ensure original map is unchanged
		if len(input) != 6 {
			t.Errorf("expected original map to be unchanged, got len %d", len(input))
		}
	})

	t.Run("trims timeline and breakout history", func(t *testing.T) {
		timeline := make([]any, 55)
		for i := range timeline {
			timeline[i] = map[string]any{"idx": i}
		}
		breakoutHistory := make([]any, 15)
		for i := range breakoutHistory {
			breakoutHistory[i] = map[string]any{"idx": i}
		}
		input := map[string]any{
			"timeline":        timeline,
			"breakoutHistory": breakoutHistory,
			"lastBreakoutSignal": map[string]any{
				"idx": "latest",
			},
		}

		got := stripHeavyState(input)

		gotTimeline, ok := got["timeline"].([]any)
		if !ok {
			t.Fatalf("expected timeline []any, got %#v", got["timeline"])
		}
		if len(gotTimeline) != liveSessionSummaryTimelineLimit {
			t.Fatalf("expected timeline limit %d, got %d", liveSessionSummaryTimelineLimit, len(gotTimeline))
		}
		if first := gotTimeline[0].(map[string]any)["idx"]; first != 5 {
			t.Fatalf("expected timeline to keep tail from 5, got %v", first)
		}

		gotBreakoutHistory, ok := got["breakoutHistory"].([]any)
		if !ok {
			t.Fatalf("expected breakoutHistory []any, got %#v", got["breakoutHistory"])
		}
		if len(gotBreakoutHistory) != liveSessionSummaryBreakoutHistoryLimit {
			t.Fatalf("expected breakoutHistory limit %d, got %d", liveSessionSummaryBreakoutHistoryLimit, len(gotBreakoutHistory))
		}
		if first := gotBreakoutHistory[0].(map[string]any)["idx"]; first != 3 {
			t.Fatalf("expected breakoutHistory to keep tail from 3, got %v", first)
		}
		if len(timeline) != 55 || len(breakoutHistory) != 15 {
			t.Fatalf("expected original slices to stay unchanged, got timeline=%d breakoutHistory=%d", len(timeline), len(breakoutHistory))
		}
	})
}
