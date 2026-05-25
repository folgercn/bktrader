package service

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestPretouchEventDetectorDetectsLongAndUsesBookCost(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	config := DefaultPretouchDetectorConfig()
	config.CostQ50Threshold = 0.005
	detector := NewPretouchEventDetector("ETHUSDT", config)
	detector.SyncBars(pretouchDetectorClosedBars(start), &HourlyBar{
		OpenTime: start,
		Open:     100,
		High:     100,
		Low:      100,
		Close:    100,
	})

	first := detector.OnTick(TickData{Time: start.Add(10 * time.Second), Price: 101})
	if first.Detected {
		t.Fatalf("first non-touch tick should not detect: %#v", first)
	}
	result := detector.OnTick(TickData{
		Time:    start.Add(60 * time.Second),
		Price:   105.10,
		BestBid: 105.00,
		BestAsk: 105.10,
	})
	if !result.Detected {
		t.Fatalf("expected long pretouch detection, got reason=%s", result.Reason)
	}
	if result.Event.Side != "long" {
		t.Fatalf("expected long side, got %s", result.Event.Side)
	}
	if math.Abs(result.Event.RoundtripCostATR-0.0103448275862069) > 1e-9 {
		t.Fatalf("expected bid/ask cost ATR from live spread, got %v", result.Event.RoundtripCostATR)
	}
	if result.Event.CostPenalty != 0.5 {
		t.Fatalf("expected cost penalty 0.5, got %v", result.Event.CostPenalty)
	}
	if _, ok := result.Event.Features["eff_300s"]; !ok {
		t.Fatalf("expected eff_300s in feature map: %#v", result.Event.Features)
	}
	if _, ok := result.Event.Features["pre_touch_seconds"]; !ok {
		t.Fatalf("expected pre_touch_seconds in feature map: %#v", result.Event.Features)
	}

	again := detector.OnTick(TickData{Time: start.Add(90 * time.Second), Price: 106})
	if again.Detected || again.Reason != "already_touched_this_bar" {
		t.Fatalf("expected same-bar dedupe, got detected=%v reason=%s", again.Detected, again.Reason)
	}
}

func TestPretouchEventDetectorAllowsNearEqualLongBreakoutWithinTolerance(t *testing.T) {
	start := time.Date(2026, 5, 25, 14, 0, 0, 0, time.UTC)
	config := DefaultPretouchDetectorConfig()
	config.StructureToleranceBps = 0.5
	config.MinSpeed300sATR = 0.0
	detector := NewPretouchEventDetector("ETHUSDT", config)
	closedBars := pretouchDetectorClosedBars(start)
	closedBars[len(closedBars)-2] = HourlyBar{OpenTime: start.Add(-2 * time.Hour), Open: 2113.88, High: 2118.26, Low: 2111.51, Close: 2116.33}
	closedBars[len(closedBars)-1] = HourlyBar{OpenTime: start.Add(-1 * time.Hour), Open: 2116.33, High: 2118.33, Low: 2113.74, Close: 2115.50}
	detector.SyncBars(closedBars, &HourlyBar{
		OpenTime: start,
		Open:     2115.50,
		High:     2115.50,
		Low:      2115.50,
		Close:    2115.50,
	})

	result := detector.OnTick(TickData{Time: start.Add(60 * time.Second), Price: 2127.69})
	if !result.Detected {
		t.Fatalf("expected near-equal long breakout within 0.5 bps tolerance, got reason=%s", result.Reason)
	}
	if result.Event.Side != "long" || result.Event.Level != 2118.26 {
		t.Fatalf("expected long breakout at prev_high_2, got side=%s level=%v", result.Event.Side, result.Event.Level)
	}
}

func TestPretouchEventDetectorSyncBarsSortsClosedHistory(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	bars := pretouchDetectorClosedBars(start)
	unsorted := []HourlyBar{bars[2], bars[0], bars[5], bars[1], bars[4], bars[3]}

	detector := NewPretouchEventDetector("ETHUSDT", DefaultPretouchDetectorConfig())
	detector.SyncBars(unsorted, &HourlyBar{
		OpenTime: start,
		Open:     100,
		High:     100,
		Low:      100,
		Close:    100,
	})

	first := detector.OnTick(TickData{Time: start.Add(10 * time.Second), Price: 101})
	if first.Detected {
		t.Fatalf("first non-touch tick should not detect: %#v", first)
	}
	result := detector.OnTick(TickData{Time: start.Add(60 * time.Second), Price: 105.10})
	if !result.Detected {
		t.Fatalf("expected sorted closed history to detect long pretouch, got reason=%s", result.Reason)
	}
	if result.Event.Level != 105 {
		t.Fatalf("expected level from chronological prev_high_2, got %v", result.Event.Level)
	}
}

func TestPretouchEventDetectorRejectsInsufficientHistory(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	detector := NewPretouchEventDetector("ETHUSDT", DefaultPretouchDetectorConfig())
	detector.SyncBars(nil, &HourlyBar{OpenTime: start, Open: 100, High: 100, Low: 100, Close: 100})
	result := detector.OnTick(TickData{Time: start, Price: 100})
	if result.Detected || result.Reason != "insufficient_bar_history" {
		t.Fatalf("expected insufficient history, got detected=%v reason=%s", result.Detected, result.Reason)
	}
}

func TestPretouchEventDetectorRejectsLatePretouch(t *testing.T) {
	start := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	config := DefaultPretouchDetectorConfig()
	config.MaxPreTouchSeconds = 30
	detector := NewPretouchEventDetector("ETHUSDT", config)
	detector.SyncBars(pretouchDetectorClosedBars(start), &HourlyBar{
		OpenTime: start,
		Open:     100,
		High:     100,
		Low:      100,
		Close:    100,
	})
	_ = detector.OnTick(TickData{Time: start.Add(10 * time.Second), Price: 101})
	result := detector.OnTick(TickData{Time: start.Add(60 * time.Second), Price: 105.1})
	if result.Detected || !strings.HasPrefix(result.Reason, "pre_touch_seconds=") {
		t.Fatalf("expected late pretouch rejection, got detected=%v reason=%s", result.Detected, result.Reason)
	}
}

func pretouchDetectorClosedBars(currentStart time.Time) []HourlyBar {
	return []HourlyBar{
		{OpenTime: currentStart.Add(-6 * time.Hour), Open: 100, High: 105, Low: 95, Close: 100},
		{OpenTime: currentStart.Add(-5 * time.Hour), Open: 100, High: 105, Low: 95, Close: 100},
		{OpenTime: currentStart.Add(-4 * time.Hour), Open: 100, High: 105, Low: 95, Close: 100},
		{OpenTime: currentStart.Add(-3 * time.Hour), Open: 100, High: 105, Low: 95, Close: 100},
		{OpenTime: currentStart.Add(-2 * time.Hour), Open: 100, High: 105, Low: 95, Close: 100},
		{OpenTime: currentStart.Add(-1 * time.Hour), Open: 100, High: 104, Low: 96, Close: 100},
	}
}
