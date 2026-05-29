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

func TestPretouchEventDetectorDetectsLeadIntrabarExtremeTouch(t *testing.T) {
	start := time.Date(2026, 5, 27, 6, 0, 0, 0, time.UTC)
	config := DefaultPretouchDetectorConfig()
	config.MinSpeed300sATR = 0
	detector := NewPretouchEventDetector("ETHUSDT", config)
	closedBars := pretouchDetectorClosedBars(start)
	detector.SyncBars(closedBars, &HourlyBar{
		OpenTime: start,
		Open:     100,
		High:     100,
		Low:      100,
		Close:    100,
	})

	first := detector.OnTick(TickData{Time: start.Add(10 * time.Second), Price: 100})
	if first.Detected {
		t.Fatalf("first non-touch tick should not detect: %#v", first)
	}

	detector.SyncBars(closedBars, &HourlyBar{
		OpenTime: start,
		Open:     100,
		High:     105.2,
		Low:      99.5,
		Close:    103.5,
	})
	result := detector.OnTick(TickData{Time: start.Add(60 * time.Second), Price: 103.5})
	if !result.Detected {
		t.Fatalf("expected lead detection from current bar high, got reason=%s", result.Reason)
	}
	if result.Event.Side != "long" {
		t.Fatalf("expected long lead event, got %s", result.Event.Side)
	}
	if result.Event.Level != 105 {
		t.Fatalf("expected T2 level 105, got %v", result.Event.Level)
	}
	if result.Event.TouchPrice != 105.2 {
		t.Fatalf("expected current bar high as touch price, got %v", result.Event.TouchPrice)
	}

	again := detector.OnTick(TickData{Time: start.Add(90 * time.Second), Price: 106})
	if again.Detected || again.Reason != "already_touched_this_bar" {
		t.Fatalf("expected same-bar lead dedupe, got detected=%v reason=%s", again.Detected, again.Reason)
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

func TestPretouchEventDetectorT3OverlayDetectsIntrabarExtremeTouch(t *testing.T) {
	start := time.Date(2026, 5, 27, 6, 0, 0, 0, time.UTC)
	config := DefaultPretouchDetectorConfig()
	config.MinSpeed300sATR = 0
	detector := NewPretouchEventDetector("ETHUSDT", config)
	closedBars := []HourlyBar{
		{OpenTime: start.Add(-6 * time.Hour), Open: 100, High: 106, Low: 94, Close: 100},
		{OpenTime: start.Add(-5 * time.Hour), Open: 100, High: 106, Low: 94, Close: 100},
		{OpenTime: start.Add(-4 * time.Hour), Open: 100, High: 106, Low: 94, Close: 100},
		{OpenTime: start.Add(-3 * time.Hour), Open: 100, High: 105, Low: 95, Close: 100},
		{OpenTime: start.Add(-2 * time.Hour), Open: 100, High: 99, Low: 92, Close: 96},
		{OpenTime: start.Add(-1 * time.Hour), Open: 96, High: 104, Low: 95, Close: 103},
	}
	detector.SyncBars(closedBars, &HourlyBar{
		OpenTime: start,
		Open:     103,
		High:     103,
		Low:      103,
		Close:    103,
	})

	first := detector.OnTickT3Overlay(TickData{Time: start.Add(10 * time.Second), Price: 103})
	if first.Detected {
		t.Fatalf("first non-touch tick should not detect: %#v", first)
	}

	detector.SyncBars(closedBars, &HourlyBar{
		OpenTime: start,
		Open:     103,
		High:     105.2,
		Low:      102.5,
		Close:    103.5,
	})
	result := detector.OnTickT3Overlay(TickData{Time: start.Add(60 * time.Second), Price: 103.5})
	if !result.Detected {
		t.Fatalf("expected T3 overlay detection from current bar high, got reason=%s diagnostics=%#v", result.Reason, result.Diagnostics)
	}
	if result.Event.Side != "long" {
		t.Fatalf("expected long T3 overlay, got %s", result.Event.Side)
	}
	if result.Event.Level != 105 {
		t.Fatalf("expected T3 level 105, got %v", result.Event.Level)
	}
	if result.Event.TouchPrice != 105.2 {
		t.Fatalf("expected current bar high as touch price, got %v", result.Event.TouchPrice)
	}
	if got := stringValue(result.Diagnostics["structureMode"]); got != "strict_current" {
		t.Fatalf("expected strict_current structure diagnostics, got %s in %#v", got, result.Diagnostics)
	}
	if !boolValue(result.Diagnostics["strictWouldTrigger"]) || !boolValue(result.Diagnostics["relaxedWouldTrigger"]) {
		t.Fatalf("expected strict and relaxed diagnostics to trigger, got %#v", result.Diagnostics)
	}

	again := detector.OnTickT3Overlay(TickData{Time: start.Add(90 * time.Second), Price: 106})
	if again.Detected || again.Reason != "t3_already_touched_this_bar" {
		t.Fatalf("expected same-bar T3 dedupe, got detected=%v reason=%s", again.Detected, again.Reason)
	}
}

func TestPretouchEventDetectorT3OverlayCanUseRelaxedPrev3DominatesStructure(t *testing.T) {
	start := time.Date(2026, 5, 29, 5, 0, 0, 0, time.UTC)
	config := DefaultPretouchDetectorConfig()
	config.MinSpeed300sATR = 0
	config.T3StructureMode = "prev3_dominates"
	detector := NewPretouchEventDetector("ETHUSDT", config)
	closedBars := []HourlyBar{
		{OpenTime: start.Add(-6 * time.Hour), Open: 2000, High: 2001, Low: 1990, Close: 1998},
		{OpenTime: start.Add(-5 * time.Hour), Open: 1998, High: 2002, Low: 1991, Close: 1999},
		{OpenTime: start.Add(-4 * time.Hour), Open: 1999, High: 2003, Low: 1992, Close: 2000},
		{OpenTime: start.Add(-3 * time.Hour), Open: 2000, High: 2009.40, Low: 1994, Close: 2001},
		{OpenTime: start.Add(-2 * time.Hour), Open: 2001, High: 2006.86, Low: 1995, Close: 2002},
		{OpenTime: start.Add(-1 * time.Hour), Open: 2002, High: 2005.88, Low: 1996, Close: 2003},
	}
	detector.SyncBars(closedBars, &HourlyBar{
		OpenTime: start,
		Open:     2003,
		High:     2003,
		Low:      2003,
		Close:    2003,
	})

	result := detector.OnTickT3Overlay(TickData{Time: start.Add(60 * time.Second), Price: 2013.92})
	if !result.Detected {
		t.Fatalf("expected relaxed T3 overlay detection, got reason=%s diagnostics=%#v", result.Reason, result.Diagnostics)
	}
	if result.Event.Side != "long" || result.Event.Level != 2009.40 {
		t.Fatalf("expected relaxed long at prev3 high, got side=%s level=%v", result.Event.Side, result.Event.Level)
	}
	if got := stringValue(result.Diagnostics["structureMode"]); got != "prev3_dominates" {
		t.Fatalf("expected prev3_dominates structure mode, got %s in %#v", got, result.Diagnostics)
	}
	if boolValue(result.Diagnostics["strictWouldTrigger"]) {
		t.Fatalf("expected old strict structure not to trigger, got %#v", result.Diagnostics)
	}
	if !boolValue(result.Diagnostics["relaxedWouldTrigger"]) || !boolValue(result.Diagnostics["longRelaxedReady"]) {
		t.Fatalf("expected relaxed structure diagnostics to trigger, got %#v", result.Diagnostics)
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
