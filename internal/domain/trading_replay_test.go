package domain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTradingReplayGoldenCases(t *testing.T) {
	casePaths, err := filepath.Glob(filepath.Join("testdata", "trading-replay", "*.input.json"))
	if err != nil {
		t.Fatalf("glob trading replay cases: %v", err)
	}
	if len(casePaths) == 0 {
		t.Fatal("expected trading replay golden cases")
	}

	for _, inputPath := range casePaths {
		name := strings.TrimSuffix(filepath.Base(inputPath), ".input.json")
		t.Run(name, func(t *testing.T) {
			inputRaw, err := os.ReadFile(inputPath)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			var orders []TradingReplayOrder
			if err := json.Unmarshal(inputRaw, &orders); err != nil {
				t.Fatalf("decode input: %v", err)
			}

			got := ReplayTradingOrders(orders)

			expectedPath := strings.TrimSuffix(inputPath, ".input.json") + ".expected.json"
			expectedRaw, err := os.ReadFile(expectedPath)
			if err != nil {
				t.Fatalf("read expected snapshot: %v", err)
			}
			var expected TradingReplayResult
			if err := json.Unmarshal(expectedRaw, &expected); err != nil {
				t.Fatalf("decode expected snapshot: %v", err)
			}

			if !reflect.DeepEqual(got, expected) {
				gotJSON, _ := json.MarshalIndent(got, "", "  ")
				expectedJSON, _ := json.MarshalIndent(expected, "", "  ")
				t.Fatalf("replay snapshot mismatch\nexpected:\n%s\n\ngot:\n%s", expectedJSON, gotJSON)
			}
		})
	}
}

func TestTradingReplayOrderEffectiveSignalKindPrefersTopLevel(t *testing.T) {
	order := TradingReplayOrder{
		SignalKind: " risk-exit ",
		Metadata:   map[string]any{"signalKind": "zero-initial-reentry"},
	}
	if got := order.EffectiveSignalKind(); got != "risk-exit" {
		t.Fatalf("EffectiveSignalKind() = %q, want risk-exit", got)
	}

	domainOrder := order.ToDomainOrder()
	if got := domainOrder.Metadata["signalKind"]; got != "risk-exit" {
		t.Fatalf("ToDomainOrder metadata signalKind = %v, want risk-exit", got)
	}
}

func TestTradingReplayOrderEffectiveSignalKindFallsBackToMetadata(t *testing.T) {
	order := TradingReplayOrder{
		Metadata: map[string]any{"signalKind": " zero-initial-reentry "},
	}
	if got := order.EffectiveSignalKind(); got != "zero-initial-reentry" {
		t.Fatalf("EffectiveSignalKind() = %q, want zero-initial-reentry", got)
	}
}
