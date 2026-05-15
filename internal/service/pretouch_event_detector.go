package service

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// PretouchDetectorConfig holds frozen thresholds from research training.
type PretouchDetectorConfig struct {
	// Quality filters (from research: touch30m_eff300le1)
	MaxPreTouchSeconds float64 // 1800 (30 minutes into signal bar)
	MaxEff300s         float64 // 1.0
	MinSpeed300sATR    float64 // train q10 = 0.228106

	// Cost sizing threshold
	CostQ50Threshold float64 // train q50 = 0.116865
	CostQ50Penalty   float64 // 0.5

	// Sizing
	BaseShare float64 // 0.80 (ETH-only)
}

// DefaultPretouchDetectorConfig returns the research-validated configuration.
func DefaultPretouchDetectorConfig() PretouchDetectorConfig {
	return PretouchDetectorConfig{
		MaxPreTouchSeconds: 1800,
		MaxEff300s:         1.0,
		MinSpeed300sATR:    0.228106,
		CostQ50Threshold:   0.116865,
		CostQ50Penalty:     0.50,
		BaseShare:          0.80,
	}
}

// HourlyBar represents a completed 1-hour OHLC bar.
type HourlyBar struct {
	OpenTime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
}

// TickData represents a single trade tick.
type TickData struct {
	Time   time.Time
	Price  float64
	Volume float64
	Side   string // "buy" / "sell"
}

// PretouchEventDetector monitors real-time tick data to detect pretouch
// breakout events matching the research-validated criteria.
type PretouchEventDetector struct {
	mu     sync.Mutex
	config PretouchDetectorConfig
	symbol string

	// 1h bar history (for ATR and level calculation)
	hourlyBars []HourlyBar // most recent 24 bars
	currentBar *HourlyBar  // current unclosed bar

	// 300s rolling tick window
	tickWindow []TickData

	// Event deduplication: only one touch per signal bar
	touchedThisBar bool
	lastTouchTime  time.Time
}

// NewPretouchEventDetector creates a detector for the given symbol.
func NewPretouchEventDetector(symbol string, config PretouchDetectorConfig) *PretouchEventDetector {
	return &PretouchEventDetector{
		config:     config,
		symbol:     symbol,
		hourlyBars: make([]HourlyBar, 0, 24),
		tickWindow: make([]TickData, 0, 512),
	}
}

// OnHourlyBarClose should be called when a 1h bar closes.
// It updates the bar history and resets per-bar state.
func (d *PretouchEventDetector) OnHourlyBarClose(bar HourlyBar) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.hourlyBars = append(d.hourlyBars, bar)
	if len(d.hourlyBars) > 24 {
		d.hourlyBars = d.hourlyBars[len(d.hourlyBars)-24:]
	}

	// Reset current bar and per-bar state
	d.currentBar = nil
	d.touchedThisBar = false
}

// OnNewHourlyBarOpen should be called when a new 1h bar opens.
func (d *PretouchEventDetector) OnNewHourlyBarOpen(openTime time.Time, openPrice float64) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.currentBar = &HourlyBar{
		OpenTime: openTime,
		Open:     openPrice,
		High:     openPrice,
		Low:      openPrice,
		Close:    openPrice,
	}
	d.touchedThisBar = false
}

// PretouchDetectionResult is the output of tick processing.
type PretouchDetectionResult struct {
	Detected bool
	Event    domain.PretouchEvent
	Reason   string // reason if not detected (for debugging)
}

// OnTick processes a new tick and checks for pretouch event detection.
// Returns a detection result; Detected=true means a valid pretouch event fired.
func (d *PretouchEventDetector) OnTick(tick TickData) PretouchDetectionResult {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Update 300s rolling window
	cutoff := tick.Time.Add(-300 * time.Second)
	newWindow := make([]TickData, 0, len(d.tickWindow))
	for _, t := range d.tickWindow {
		if t.Time.After(cutoff) {
			newWindow = append(newWindow, t)
		}
	}
	newWindow = append(newWindow, tick)
	d.tickWindow = newWindow

	// Update current bar high/low/close
	if d.currentBar != nil {
		if tick.Price > d.currentBar.High {
			d.currentBar.High = tick.Price
		}
		if tick.Price < d.currentBar.Low {
			d.currentBar.Low = tick.Price
		}
		d.currentBar.Close = tick.Price
	}

	// --- Check detection conditions ---

	// Need at least 2 completed hourly bars for level calculation
	if len(d.hourlyBars) < 2 {
		return PretouchDetectionResult{Reason: "insufficient_bar_history"}
	}

	// Already touched this bar
	if d.touchedThisBar {
		return PretouchDetectionResult{Reason: "already_touched_this_bar"}
	}

	// Need current bar open
	if d.currentBar == nil {
		return PretouchDetectionResult{Reason: "no_current_bar"}
	}

	// Calculate ATR (average of last N bars' ranges)
	atr := d.computeATR()
	if atr <= 0 {
		return PretouchDetectionResult{Reason: "zero_atr"}
	}

	// Calculate breakout levels
	prevBar1 := d.hourlyBars[len(d.hourlyBars)-1] // most recent closed bar
	prevBar2 := d.hourlyBars[len(d.hourlyBars)-2] // bar before that

	longLevel := prevBar2.High  // prev_high_2
	shortLevel := prevBar2.Low  // prev_low_2

	// Check if tick touches a level
	var side string
	var level float64

	if tick.Price >= longLevel && prevBar1.High < longLevel {
		// Long breakout: current tick touches prev_high_2 for first time
		side = "long"
		level = longLevel
	} else if tick.Price <= shortLevel && prevBar1.Low > shortLevel {
		// Short breakout: current tick touches prev_low_2 for first time
		side = "short"
		level = shortLevel
	} else {
		return PretouchDetectionResult{Reason: "no_level_touch"}
	}

	// --- Compute pretouch features ---

	touchTime := tick.Time
	touchPrice := tick.Price
	signalBarStart := d.currentBar.OpenTime

	// pre_touch_seconds
	preTouchSeconds := touchTime.Sub(signalBarStart).Seconds()
	if preTouchSeconds > d.config.MaxPreTouchSeconds {
		return PretouchDetectionResult{Reason: fmt.Sprintf("pre_touch_seconds=%.0f > %.0f", preTouchSeconds, d.config.MaxPreTouchSeconds)}
	}

	// touch_extension_atr
	touchExtATR := (touchPrice - level) / atr
	if side == "short" {
		touchExtATR = (level - touchPrice) / atr
	}

	// speed_300s_atr (price change over 300s / ATR)
	speed300s := d.computeSpeed300s(atr)
	if math.Abs(speed300s) < d.config.MinSpeed300sATR {
		return PretouchDetectionResult{Reason: fmt.Sprintf("speed_300s=%.4f < %.4f", math.Abs(speed300s), d.config.MinSpeed300sATR)}
	}

	// eff_300s (efficiency = |net_move| / total_range)
	eff300s := d.computeEff300s()
	if eff300s > d.config.MaxEff300s {
		return PretouchDetectionResult{Reason: fmt.Sprintf("eff_300s=%.4f > %.4f", eff300s, d.config.MaxEff300s)}
	}

	// roundtrip_cost_atr (simplified: use spread proxy from tick data)
	// In live, this would come from order book. For now use a conservative estimate.
	roundtripCostATR := 0.10 // placeholder; will be replaced by order book data

	// cost penalty
	costPenalty := 1.0
	if roundtripCostATR >= d.config.CostQ50Threshold {
		costPenalty = d.config.CostQ50Penalty
	}

	// Mark as touched
	d.touchedThisBar = true
	d.lastTouchTime = touchTime

	// Build features map for ML inference
	features := map[string]float64{
		"touch_extension_atr":     touchExtATR,
		"speed_300s_atr":          speed300s,
		"roundtrip_cost_atr":      roundtripCostATR,
		"signal_atr_percentile":   d.computeATRPercentile(atr),
		"prev1_body_atr":          math.Abs(prevBar1.Close-prevBar1.Open) / atr,
		"prev1_range_atr":         (prevBar1.High - prevBar1.Low) / atr,
		"prev1_close_pos_side":    d.computeClosePosSide(prevBar1, side),
		"prev_sma5_gap_atr":       d.computeSMA5Gap(atr),
		"prev_sma5_slope_atr":     d.computeSMA5Slope(atr),
		"level_to_prev_close_atr": d.computeLevelToPrevClose(level, prevBar1.Close, atr, side),
		"level_to_signal_open_atr": (level - d.currentBar.Open) / atr,
	}

	event := domain.PretouchEvent{
		EventID:           fmt.Sprintf("%s_%s", d.symbol, touchTime.Format("20060102_150405")),
		Symbol:            d.symbol,
		Side:              side,
		TouchTime:         touchTime,
		TouchPrice:        touchPrice,
		Level:             level,
		ATR:               atr,
		TouchExtensionATR: touchExtATR,
		Speed300sATR:      speed300s,
		Eff300s:           eff300s,
		PreTouchSeconds:   preTouchSeconds,
		RoundtripCostATR:  roundtripCostATR,
		SignalBarStart:    signalBarStart,
		Features:          features,
		CostPenalty:       costPenalty,
	}

	return PretouchDetectionResult{
		Detected: true,
		Event:    event,
	}
}

// --- Internal computation helpers ---

func (d *PretouchEventDetector) computeATR() float64 {
	if len(d.hourlyBars) < 5 {
		return 0
	}
	n := len(d.hourlyBars)
	if n > 14 {
		n = 14
	}
	bars := d.hourlyBars[len(d.hourlyBars)-n:]
	sum := 0.0
	for _, b := range bars {
		sum += b.High - b.Low
	}
	return sum / float64(len(bars))
}

func (d *PretouchEventDetector) computeSpeed300s(atr float64) float64 {
	if len(d.tickWindow) < 2 || atr <= 0 {
		return 0
	}
	first := d.tickWindow[0].Price
	last := d.tickWindow[len(d.tickWindow)-1].Price
	return (last - first) / atr
}

func (d *PretouchEventDetector) computeEff300s() float64 {
	if len(d.tickWindow) < 2 {
		return 0
	}
	first := d.tickWindow[0].Price
	last := d.tickWindow[len(d.tickWindow)-1].Price
	netMove := math.Abs(last - first)

	high := d.tickWindow[0].Price
	low := d.tickWindow[0].Price
	for _, t := range d.tickWindow {
		if t.Price > high {
			high = t.Price
		}
		if t.Price < low {
			low = t.Price
		}
	}
	totalRange := high - low
	if totalRange <= 0 {
		return 0
	}
	return netMove / totalRange
}

func (d *PretouchEventDetector) computeATRPercentile(currentATR float64) float64 {
	if len(d.hourlyBars) < 5 {
		return 0.5
	}
	count := 0
	for _, b := range d.hourlyBars {
		r := b.High - b.Low
		if r < currentATR {
			count++
		}
	}
	return float64(count) / float64(len(d.hourlyBars))
}

func (d *PretouchEventDetector) computeClosePosSide(bar HourlyBar, side string) float64 {
	barRange := bar.High - bar.Low
	if barRange <= 0 {
		return 0.5
	}
	pos := (bar.Close - bar.Low) / barRange
	if side == "short" {
		pos = 1.0 - pos
	}
	return pos
}

func (d *PretouchEventDetector) computeSMA5Gap(atr float64) float64 {
	if len(d.hourlyBars) < 5 || atr <= 0 {
		return 0
	}
	bars := d.hourlyBars[len(d.hourlyBars)-5:]
	sum := 0.0
	for _, b := range bars {
		sum += b.Close
	}
	sma5 := sum / 5.0
	prevClose := d.hourlyBars[len(d.hourlyBars)-1].Close
	return (prevClose - sma5) / atr
}

func (d *PretouchEventDetector) computeSMA5Slope(atr float64) float64 {
	if len(d.hourlyBars) < 6 || atr <= 0 {
		return 0
	}
	bars := d.hourlyBars[len(d.hourlyBars)-6:]
	sum5Current := 0.0
	for _, b := range bars[1:] {
		sum5Current += b.Close
	}
	sum5Prev := 0.0
	for _, b := range bars[:5] {
		sum5Prev += b.Close
	}
	return (sum5Current/5.0 - sum5Prev/5.0) / atr
}

func (d *PretouchEventDetector) computeLevelToPrevClose(level, prevClose, atr float64, side string) float64 {
	if atr <= 0 {
		return 0
	}
	diff := level - prevClose
	if side == "short" {
		diff = -diff
	}
	return diff / atr
}
