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
	})

	t.Run("state with heavy fields", func(t *testing.T) {
		input := map[string]any{
			"status":                             "RUNNING",
			"count":                              10,
			"sourceStates":                       map[string]any{"key1": map[string]any{"bars": []any{1, 2, 3}, "streamType": "trade_tick", "extra": "omit"}},
			"signalBarStates":                    map[string]any{"key2": map[string]any{"sma5": 123.45, "extra": "omit"}},
			"lastStrategyEvaluationSourceStates": map[string]any{"data": "heavy3"},
			"lastStrategyEvaluationSignalBarStates": map[string]any{
				"data": "heavy4",
			},
		}
		got := stripHeavyState(input)
		expected := map[string]any{
			"status":          "RUNNING",
			"count":           10,
			"sourceStates":    map[string]any{"key1": map[string]any{"streamType": "trade_tick"}},
			"signalBarStates": map[string]any{"key2": map[string]any{"sma5": 123.45}},
		}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected %v, got %v", expected, got)
		}
	})
}

func TestStripHeavyState_Whitelist(t *testing.T) {
	input := map[string]any{
		"sourceStates": map[string]any{
			"s1": map[string]any{
				"streamType":  "trade_tick",
				"lastEventAt": "2024-01-01T00:00:00Z",
				"bars":        []any{1, 2, 3},
				"price":       "100",
				"summary": map[string]any{
					"price":   "100",
					"bestBid": "99",
					"bestAsk": "101",
					"volume":  "1000",
				},
			},
			"s2": map[string]any{
				"streamType":  "order_book",
				"lastEventAt": "2024-01-01T00:00:01Z",
				"bestBid":     "99",
				"bestAsk":     "101",
				"summary": map[string]any{
					"bestBid": "99",
					"bestAsk": "101",
					"bids":    []any{1, 2, 3},
				},
			},
		},
		"signalBarStates": map[string]any{
			"b1": map[string]any{
				"sma5":    123.45,
				"current": map[string]any{"o": "100"},
				"junk":    "data",
			},
		},
	}

	output := stripHeavyState(input)

	s1 := output["sourceStates"].(map[string]any)["s1"].(map[string]any)
	if s1["streamType"] != "trade_tick" || s1["lastEventAt"] != "2024-01-01T00:00:00Z" {
		t.Error("missing allowed source metadata")
	}
	if s1["price"] != "100" {
		t.Error("missing allowed trade price")
	}
	s1Summary := s1["summary"].(map[string]any)
	if s1Summary["price"] != "100" || s1Summary["bestBid"] != "99" || s1Summary["bestAsk"] != "101" {
		t.Error("missing allowed source summary market fields")
	}
	if s1["bars"] != nil || s1Summary["volume"] != nil {
		t.Error("failed to strip disallowed source metadata")
	}
	s2 := output["sourceStates"].(map[string]any)["s2"].(map[string]any)
	s2Summary := s2["summary"].(map[string]any)
	if s2["bestBid"] != "99" || s2["bestAsk"] != "101" || s2Summary["bestBid"] != "99" || s2Summary["bestAsk"] != "101" {
		t.Error("missing allowed order book fields")
	}
	if s2Summary["bids"] != nil {
		t.Error("failed to strip heavy order book summary fields")
	}

	b1 := output["signalBarStates"].(map[string]any)["b1"].(map[string]any)
	if b1["sma5"] != 123.45 || b1["current"] == nil {
		t.Error("missing allowed signal bar metadata")
	}
	if b1["junk"] != nil {
		t.Error("failed to strip disallowed signal bar metadata")
	}
}

func TestStripHeavyState_Timeline(t *testing.T) {
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
		}

		got := stripHeavyState(input)

		gotTimeline, ok := got["timeline"].([]any)
		if !ok || len(gotTimeline) != 50 {
			t.Fatalf("expected timeline limit 50, got %d", len(gotTimeline))
		}

		gotBreakoutHistory, ok := got["breakoutHistory"].([]any)
		if !ok || len(gotBreakoutHistory) != 12 {
			t.Fatalf("expected breakoutHistory limit 12, got %d", len(gotBreakoutHistory))
		}
	})
}
