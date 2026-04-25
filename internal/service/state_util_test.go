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
			"status":          "RUNNING",
			"count":           10,
			"sourceStates":    map[string]any{"data": "heavy1"},
			"signalBarStates": map[string]any{"data": "heavy2"},
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
		if len(input) != 4 {
			t.Errorf("expected original map to be unchanged, got len %d", len(input))
		}
	})
}
