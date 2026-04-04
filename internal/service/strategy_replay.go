package service

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type strategySignalBar struct {
	Time      time.Time
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	MA20      float64
	ATR       float64
	PrevHigh1 float64
	PrevHigh2 float64
	PrevLow1  float64
	PrevLow2  float64
}

type executionBar struct {
	Time  time.Time
	Open  float64
	High  float64
	Low   float64
	Close float64
}

type strategyPosition struct {
	Side       string
	EntryPrice float64
	StopLoss   float64
	Protected  bool
	Notional   float64
	Reason     string
	BarIndex   int
}

type strategyReplayConfig struct {
	SignalTimeframe     string
	ExecutionDataSource string
	Symbol              string
	From                time.Time
	To                  time.Time
	InitialBalance      float64
	Dir1ReentryConfirm  bool
	Dir2ZeroInitial     bool
	FixedSlippage       float64
	StopLossATR         float64
	MaxTradesPerBar     int
	ReentrySizeSchedule []float64
	StopMode            string
	ProfitProtectATR    float64
}

func (p *Platform) runStrategyReplay(backtest map[string]any) (map[string]any, error) {
	cfg := buildStrategyReplayConfig(backtest)
	signals, err := p.loadStrategySignalBars(cfg.SignalTimeframe)
	if err != nil {
		return nil, err
	}
	if len(signals) < 2 {
		return nil, fmt.Errorf("not enough %s signal bars", cfg.SignalTimeframe)
	}

	rangeStart, rangeEnd := resolveReplayRange(cfg, signals)
	signals = trimSignalBars(signals, rangeStart, rangeEnd)
	if len(signals) < 2 {
		return nil, fmt.Errorf("no %s signal bars in selected range", cfg.SignalTimeframe)
	}

	if cfg.ExecutionDataSource == "1min" {
		minuteBars, err := p.loadCandleBars()
		if err != nil {
			return nil, err
		}
		return runStrategyReplayOnMinuteBars(cfg, signals, minuteBars)
	}

	engine := newStrategyReplayEngine(cfg)
	switch cfg.ExecutionDataSource {
	case "tick":
		if err := p.walkTickExecutionBars(cfg.Symbol, rangeStart, rangeEnd, func(bar executionBar) error {
			engine.process(bar, signals)
			return nil
		}); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported execution data source: %s", cfg.ExecutionDataSource)
	}

	return engine.summary(signals), nil
}

func runStrategyReplayOnMinuteBars(cfg strategyReplayConfig, signals []strategySignalBar, minuteBars []candleBar) (map[string]any, error) {
	engine := newStrategyReplayEngine(cfg)
	barCursor := 0
	reentryATR := 0.1
	commission := 0.001
	initialUsage := 0.10
	if cfg.Dir2ZeroInitial {
		initialUsage = 0.0
	}

	for i := 0; i < len(signals)-1; i++ {
		startT, endT := signals[i].Time, signals[i+1].Time
		if startT.Before(cfg.From) {
			startT = cfg.From
		}
		if !cfg.To.IsZero() && endT.After(cfg.To) {
			endT = cfg.To
		}
		if !cfg.To.IsZero() && startT.After(cfg.To) {
			break
		}
		if endT.Before(startT) {
			continue
		}

		for barCursor < len(minuteBars) && minuteBars[barCursor].Time.Before(startT) {
			barCursor++
		}
		windowStart := barCursor
		windowEnd := windowStart
		for windowEnd < len(minuteBars) && !minuteBars[windowEnd].Time.After(endT) {
			windowEnd++
		}
		if windowStart >= windowEnd {
			continue
		}

		sig := signals[i]
		if math.IsNaN(sig.ATR) || sig.ATR <= 0 {
			continue
		}

		tradesInBar := 0
		currentIdx := windowStart
		if i-engine.lastExitBarIndex > 1 {
			engine.lastExitSide = ""
		}

		for currentIdx < windowEnd {
			bar := minuteBars[currentIdx]
			var prevBar *candleBar
			if currentIdx > windowStart {
				prevBar = &minuteBars[currentIdx-1]
			}

			if engine.position == nil {
				executed := false

				if sig.Close > sig.MA20 {
					reP := sig.PrevLow1 + reentryATR*sig.ATR
					if tradesInBar == 0 && sig.PrevHigh2 > sig.PrevHigh1 {
						if bar.High >= sig.PrevHigh2 {
							entry := math.Max(bar.Open, sig.PrevHigh2) * (1 + cfg.FixedSlippage)
							notional := engine.balance * initialUsage
							stopLoss := resolveStopPrice("long", entry, sig, cfg.StopMode, cfg.StopLossATR)
							engine.position = &strategyPosition{
								Side:       "long",
								EntryPrice: entry,
								StopLoss:   stopLoss,
								Protected:  false,
								Notional:   notional,
								Reason:     "Initial",
								BarIndex:   i,
							}
							engine.balance -= notional * commission
							engine.appendTrade("BUY", bar.Time, entry, "Initial", notional, 0)
							tradesInBar++
							executed = true
						}
					} else if engine.lastExitSide == "long" && i-engine.lastExitBarIndex <= 1 {
						triggered := false
						entryRaw := reP
						if cfg.Dir1ReentryConfirm {
							if prevBar != nil && bar.Close > reP && prevBar.Close > reP {
								triggered = true
								entryRaw = bar.Close
							}
						} else if bar.High >= reP {
							triggered = true
						}
						if triggered {
							reason := "PT-Reentry"
							if engine.lastExitReason == "SL" {
								reason = "SL-Reentry"
							}
							if (reason == "SL-Reentry" && tradesInBar < cfg.MaxTradesPerBar) || reason == "PT-Reentry" {
								size := getReentrySize(tradesInBar, cfg.ReentrySizeSchedule)
								notional := engine.balance * size
								entry := entryRaw * (1 + cfg.FixedSlippage)
								stopLoss := resolveStopPrice("long", entry, sig, cfg.StopMode, cfg.StopLossATR)
								engine.position = &strategyPosition{
									Side:       "long",
									EntryPrice: entry,
									StopLoss:   stopLoss,
									Protected:  false,
									Notional:   notional,
									Reason:     reason,
									BarIndex:   i,
								}
								engine.balance -= notional * commission
								engine.appendTrade("BUY", bar.Time, entry, reason, notional, 0)
								if reason == "SL-Reentry" {
									tradesInBar++
								}
								executed = true
							}
							engine.lastExitSide = ""
						}
					}
				} else if sig.Close < sig.MA20 {
					reP := sig.PrevHigh1
					if tradesInBar == 0 && sig.PrevLow2 < sig.PrevLow1 {
						if bar.Low <= sig.PrevLow2 {
							entry := math.Min(bar.Open, sig.PrevLow2) * (1 - cfg.FixedSlippage)
							notional := engine.balance * initialUsage
							stopLoss := resolveStopPrice("short", entry, sig, cfg.StopMode, cfg.StopLossATR)
							engine.position = &strategyPosition{
								Side:       "short",
								EntryPrice: entry,
								StopLoss:   stopLoss,
								Protected:  false,
								Notional:   notional,
								Reason:     "Initial",
								BarIndex:   i,
							}
							engine.balance -= notional * commission
							engine.appendTrade("SHORT", bar.Time, entry, "Initial", notional, 0)
							tradesInBar++
							executed = true
						}
					} else if engine.lastExitSide == "short" && i-engine.lastExitBarIndex <= 1 {
						triggered := false
						entryRaw := reP
						if cfg.Dir1ReentryConfirm {
							if prevBar != nil && bar.Close < reP && prevBar.Close < reP {
								triggered = true
								entryRaw = bar.Close
							}
						} else if bar.Low <= reP {
							triggered = true
						}
						if triggered {
							reason := "PT-Reentry"
							if engine.lastExitReason == "SL" {
								reason = "SL-Reentry"
							}
							if (reason == "SL-Reentry" && tradesInBar < cfg.MaxTradesPerBar) || reason == "PT-Reentry" {
								size := getReentrySize(tradesInBar, cfg.ReentrySizeSchedule)
								notional := engine.balance * size
								entry := entryRaw * (1 - cfg.FixedSlippage)
								stopLoss := resolveStopPrice("short", entry, sig, cfg.StopMode, cfg.StopLossATR)
								engine.position = &strategyPosition{
									Side:       "short",
									EntryPrice: entry,
									StopLoss:   stopLoss,
									Protected:  false,
									Notional:   notional,
									Reason:     reason,
									BarIndex:   i,
								}
								engine.balance -= notional * commission
								engine.appendTrade("SHORT", bar.Time, entry, reason, notional, 0)
								if reason == "SL-Reentry" {
									tradesInBar++
								}
								executed = true
							}
							engine.lastExitSide = ""
						}
					}
				}

				currentIdx++
				if executed {
					continue
				}
				continue
			}

			exitTriggered := false
			exitPrice := 0.0
			exitReason := ""
			if engine.position.Side == "long" {
				if !engine.position.Protected && bar.High >= engine.position.EntryPrice+cfg.ProfitProtectATR*sig.ATR {
					engine.position.Protected = true
				}
				if bar.Low <= engine.position.StopLoss {
					exitPrice = engine.position.StopLoss
					exitReason = "SL"
					exitTriggered = true
				} else if engine.position.Protected && bar.Low <= sig.PrevLow1 {
					exitPrice = sig.PrevLow1
					exitReason = "PT"
					exitTriggered = true
				}
			} else {
				if !engine.position.Protected && bar.Low <= engine.position.EntryPrice-cfg.ProfitProtectATR*sig.ATR {
					engine.position.Protected = true
				}
				if bar.High >= engine.position.StopLoss {
					exitPrice = engine.position.StopLoss
					exitReason = "SL"
					exitTriggered = true
				} else if engine.position.Protected && bar.High >= sig.PrevHigh1 {
					exitPrice = sig.PrevHigh1
					exitReason = "PT"
					exitTriggered = true
				}
			}

			if exitTriggered {
				sideMult := 1.0
				if engine.position.Side == "short" {
					sideMult = -1.0
					exitPrice *= (1 + cfg.FixedSlippage)
				} else {
					exitPrice *= (1 - cfg.FixedSlippage)
				}
				pnl := 0.0
				if engine.position.Notional > 0 {
					pnl = sideMult * (exitPrice - engine.position.EntryPrice) / engine.position.EntryPrice * engine.position.Notional
					engine.balance += pnl - engine.position.Notional*commission
				}
				engine.appendTrade("EXIT", bar.Time, exitPrice, exitReason, engine.position.Notional, pnl)
				engine.lastExitReason = exitReason
				engine.lastExitSide = engine.position.Side
				engine.lastExitBarIndex = i
				engine.position = nil
			}
			currentIdx++
		}
	}

	return engine.summary(signals), nil
}

type strategyReplayEngine struct {
	cfg              strategyReplayConfig
	balance          float64
	position         *strategyPosition
	lastExitReason   string
	lastExitSide     string
	lastExitBarIndex int
	currentBarIndex  int
	tradesInBar      int
	prevExecBar      *executionBar
	equity           []float64
	trades           []map[string]any
}

func newStrategyReplayEngine(cfg strategyReplayConfig) *strategyReplayEngine {
	return &strategyReplayEngine{
		cfg:              cfg,
		balance:          cfg.InitialBalance,
		lastExitBarIndex: -999,
		equity:           []float64{cfg.InitialBalance},
		trades:           make([]map[string]any, 0, 128),
	}
}

func (e *strategyReplayEngine) process(bar executionBar, signals []strategySignalBar) {
	for e.currentBarIndex < len(signals)-2 && !bar.Time.Before(signals[e.currentBarIndex+1].Time) {
		e.currentBarIndex++
		e.tradesInBar = 0
		e.prevExecBar = nil
		if e.currentBarIndex-e.lastExitBarIndex > 1 {
			e.lastExitSide = ""
		}
	}

	if e.currentBarIndex >= len(signals)-1 {
		return
	}
	sig := signals[e.currentBarIndex]
	if math.IsNaN(sig.ATR) || sig.ATR <= 0 || math.IsNaN(sig.MA20) {
		e.prevExecBar = &bar
		return
	}

	if e.position == nil {
		e.tryEntry(bar, sig)
	} else {
		e.tryExit(bar, sig)
	}
	e.prevExecBar = &bar
}

func (e *strategyReplayEngine) tryEntry(bar executionBar, sig strategySignalBar) {
	reentryATR := 0.1
	commission := 0.001
	initialUsage := 0.10
	if e.cfg.Dir2ZeroInitial {
		initialUsage = 0.0
	}

	if sig.Close > sig.MA20 {
		reP := sig.PrevLow1 + reentryATR*sig.ATR
		if e.tradesInBar == 0 && sig.PrevHigh2 > sig.PrevHigh1 {
			if bar.High >= sig.PrevHigh2 {
				entry := math.Max(bar.Open, sig.PrevHigh2) * (1 + e.cfg.FixedSlippage)
				notional := e.balance * initialUsage
				stopLoss := resolveStopPrice("long", entry, sig, e.cfg.StopMode, e.cfg.StopLossATR)
				e.position = &strategyPosition{
					Side:       "long",
					EntryPrice: entry,
					StopLoss:   stopLoss,
					Protected:  false,
					Notional:   notional,
					Reason:     "Initial",
					BarIndex:   e.currentBarIndex,
				}
				e.balance -= notional * commission
				e.tradesInBar++
				e.appendTrade("BUY", bar.Time, entry, "Initial", notional, 0)
				return
			}
		} else if e.lastExitSide == "long" && e.currentBarIndex-e.lastExitBarIndex <= 1 {
			triggered := false
			entryRaw := reP
			if e.cfg.Dir1ReentryConfirm {
				if e.prevExecBar != nil && bar.Close > reP && e.prevExecBar.Close > reP {
					triggered = true
					entryRaw = bar.Close
				}
			} else if bar.High >= reP {
				triggered = true
			}
			if triggered {
				reason := "PT-Reentry"
				if e.lastExitReason == "SL" {
					reason = "SL-Reentry"
				}
				if (reason == "SL-Reentry" && e.tradesInBar < e.cfg.MaxTradesPerBar) || reason == "PT-Reentry" {
					size := getReentrySize(e.tradesInBar, e.cfg.ReentrySizeSchedule)
					notional := e.balance * size
					entry := entryRaw * (1 + e.cfg.FixedSlippage)
					stopLoss := resolveStopPrice("long", entry, sig, e.cfg.StopMode, e.cfg.StopLossATR)
					e.position = &strategyPosition{
						Side:       "long",
						EntryPrice: entry,
						StopLoss:   stopLoss,
						Protected:  false,
						Notional:   notional,
						Reason:     reason,
						BarIndex:   e.currentBarIndex,
					}
					e.balance -= notional * commission
					if reason == "SL-Reentry" {
						e.tradesInBar++
					}
					e.appendTrade("BUY", bar.Time, entry, reason, notional, 0)
				}
				e.lastExitSide = ""
			}
		}
		return
	}

	if sig.Close < sig.MA20 {
		reP := sig.PrevHigh1
		if e.tradesInBar == 0 && sig.PrevLow2 < sig.PrevLow1 {
			if bar.Low <= sig.PrevLow2 {
				entry := math.Min(bar.Open, sig.PrevLow2) * (1 - e.cfg.FixedSlippage)
				notional := e.balance * initialUsage
				stopLoss := resolveStopPrice("short", entry, sig, e.cfg.StopMode, e.cfg.StopLossATR)
				e.position = &strategyPosition{
					Side:       "short",
					EntryPrice: entry,
					StopLoss:   stopLoss,
					Protected:  false,
					Notional:   notional,
					Reason:     "Initial",
					BarIndex:   e.currentBarIndex,
				}
				e.balance -= notional * commission
				e.tradesInBar++
				e.appendTrade("SHORT", bar.Time, entry, "Initial", notional, 0)
				return
			}
		} else if e.lastExitSide == "short" && e.currentBarIndex-e.lastExitBarIndex <= 1 {
			triggered := false
			entryRaw := reP
			if e.cfg.Dir1ReentryConfirm {
				if e.prevExecBar != nil && bar.Close < reP && e.prevExecBar.Close < reP {
					triggered = true
					entryRaw = bar.Close
				}
			} else if bar.Low <= reP {
				triggered = true
			}
			if triggered {
				reason := "PT-Reentry"
				if e.lastExitReason == "SL" {
					reason = "SL-Reentry"
				}
				if (reason == "SL-Reentry" && e.tradesInBar < e.cfg.MaxTradesPerBar) || reason == "PT-Reentry" {
					size := getReentrySize(e.tradesInBar, e.cfg.ReentrySizeSchedule)
					notional := e.balance * size
					entry := entryRaw * (1 - e.cfg.FixedSlippage)
					stopLoss := resolveStopPrice("short", entry, sig, e.cfg.StopMode, e.cfg.StopLossATR)
					e.position = &strategyPosition{
						Side:       "short",
						EntryPrice: entry,
						StopLoss:   stopLoss,
						Protected:  false,
						Notional:   notional,
						Reason:     reason,
						BarIndex:   e.currentBarIndex,
					}
					e.balance -= notional * commission
					if reason == "SL-Reentry" {
						e.tradesInBar++
					}
					e.appendTrade("SHORT", bar.Time, entry, reason, notional, 0)
				}
				e.lastExitSide = ""
			}
		}
	}
}

func (e *strategyReplayEngine) tryExit(bar executionBar, sig strategySignalBar) {
	if e.position == nil {
		return
	}
	sideMult := 1.0
	reason := ""
	exitPrice := 0.0
	commission := 0.001

	if e.position.Side == "long" {
		if !e.position.Protected && bar.High >= e.position.EntryPrice+e.cfg.ProfitProtectATR*sig.ATR {
			e.position.Protected = true
		}
		if bar.Low <= e.position.StopLoss {
			reason = "SL"
			exitPrice = e.position.StopLoss * (1 - e.cfg.FixedSlippage)
		} else if e.position.Protected && bar.Low <= sig.PrevLow1 {
			reason = "PT"
			exitPrice = sig.PrevLow1 * (1 - e.cfg.FixedSlippage)
		}
	} else {
		sideMult = -1.0
		if !e.position.Protected && bar.Low <= e.position.EntryPrice-e.cfg.ProfitProtectATR*sig.ATR {
			e.position.Protected = true
		}
		if bar.High >= e.position.StopLoss {
			reason = "SL"
			exitPrice = e.position.StopLoss * (1 + e.cfg.FixedSlippage)
		} else if e.position.Protected && bar.High >= sig.PrevHigh1 {
			reason = "PT"
			exitPrice = sig.PrevHigh1 * (1 + e.cfg.FixedSlippage)
		}
	}

	if reason == "" {
		return
	}

	pnl := 0.0
	if e.position.Notional > 0 {
		pnl = sideMult * (exitPrice - e.position.EntryPrice) / e.position.EntryPrice * e.position.Notional
		e.balance += pnl - e.position.Notional*commission
	}
	e.appendTrade("EXIT", bar.Time, exitPrice, reason, e.position.Notional, pnl)
	e.lastExitReason = reason
	e.lastExitSide = e.position.Side
	e.lastExitBarIndex = e.currentBarIndex
	e.position = nil
}

func (e *strategyReplayEngine) appendTrade(kind string, at time.Time, price float64, reason string, notional float64, pnl float64) {
	record := map[string]any{
		"time":     at.UTC().Format(time.RFC3339),
		"type":     kind,
		"price":    price,
		"reason":   reason,
		"notional": notional,
		"balance":  e.balance,
		"pnl":      pnl,
	}
	e.equity = append(e.equity, e.balance)
	e.trades = append(e.trades, record)
}

func (e *strategyReplayEngine) summary(signals []strategySignalBar) map[string]any {
	result := map[string]any{
		"runnerMode":          "strategy_replay",
		"signalTimeframe":     e.cfg.SignalTimeframe,
		"executionDataSource": e.cfg.ExecutionDataSource,
		"symbol":              e.cfg.Symbol,
		"tradePairs":          0,
		"return":              0.0,
		"maxDrawdown":         0.0,
		"executionTrades":     []map[string]any{},
		"executionTradeCount": 0,
	}
	if len(e.trades) == 0 {
		return result
	}

	executionTrades := pairStrategyTrades(e.trades, e.cfg.ExecutionDataSource)
	stats := summarizeExecutionTradeRecords(executionTrades)
	for key, value := range stats {
		result[key] = value
	}
	result["tradePairs"] = len(executionTrades)
	result["return"] = e.balance/e.cfg.InitialBalance - 1
	result["maxDrawdown"] = computeMaxDrawdown(e.equity)
	result["finalBalance"] = e.balance
	result["signalBars"] = len(signals)
	return result
}

func buildStrategyReplayConfig(parameters map[string]any) strategyReplayConfig {
	reentrySizes := []float64{0.10, 0.20}
	if raw, ok := parameters["reentry_size_schedule"]; ok {
		switch items := raw.(type) {
		case []any:
			reentrySizes = make([]float64, 0, len(items))
			for _, item := range items {
				reentrySizes = append(reentrySizes, parseFloatValue(item))
			}
		}
	}
	if len(reentrySizes) == 0 {
		reentrySizes = []float64{0.10, 0.20}
	}
	stopMode := stringValue(parameters["stop_mode"])
	if stopMode == "" {
		stopMode = "atr"
	}
	stopLossATR := parseFloatValue(parameters["stop_loss_atr"])
	if stopLossATR <= 0 {
		stopLossATR = 0.05
	}
	return strategyReplayConfig{
		SignalTimeframe:     strings.ToLower(stringValue(parameters["signalTimeframe"])),
		ExecutionDataSource: strings.ToLower(stringValue(parameters["executionDataSource"])),
		Symbol:              normalizeBacktestSymbol(stringValue(parameters["symbol"])),
		From:                parseOptionalRFC3339(stringValue(parameters["from"])),
		To:                  parseOptionalRFC3339(stringValue(parameters["to"])),
		InitialBalance:      100000.0,
		Dir1ReentryConfirm:  false,
		Dir2ZeroInitial:     true,
		FixedSlippage:       firstPositive(parseFloatValue(parameters["fixed_slippage"]), 0.0005),
		StopLossATR:         stopLossATR,
		MaxTradesPerBar:     maxIntValue(parameters["max_trades_per_bar"], 3),
		ReentrySizeSchedule: reentrySizes,
		StopMode:            stopMode,
		ProfitProtectATR:    firstPositive(parseFloatValue(parameters["profit_protect_atr"]), 1.0),
	}
}

func buildSignalBars(minuteBars []candleBar, timeframe string) ([]strategySignalBar, error) {
	resolution := "240"
	if timeframe == "1d" {
		resolution = "1D"
	}
	step := resolutionToDuration(resolution)
	if step <= 0 {
		return nil, fmt.Errorf("unsupported signal timeframe: %s", timeframe)
	}
	aggregated := aggregateCandleBars(minuteBars, resolution, step)
	if len(aggregated) == 0 {
		return nil, fmt.Errorf("no aggregated bars for timeframe %s", timeframe)
	}

	signals := make([]strategySignalBar, len(aggregated))
	closes := make([]float64, len(aggregated))
	trueRanges := make([]float64, len(aggregated))
	for i, bar := range aggregated {
		signals[i] = strategySignalBar{
			Time:   bar.Time,
			Open:   bar.Open,
			High:   bar.High,
			Low:    bar.Low,
			Close:  bar.Close,
			Volume: bar.Volume,
		}
		closes[i] = bar.Close
		if i == 0 {
			trueRanges[i] = bar.High - bar.Low
		} else {
			highLow := bar.High - bar.Low
			highClose := math.Abs(bar.High - aggregated[i-1].Close)
			lowClose := math.Abs(bar.Low - aggregated[i-1].Close)
			trueRanges[i] = math.Max(highLow, math.Max(highClose, lowClose))
		}
	}

	for i := range signals {
		signals[i].MA20 = rollingMean(closes, i, 20)
		signals[i].ATR = rollingMean(trueRanges, i, 14)
		if i >= 1 {
			signals[i].PrevHigh1 = aggregated[i-1].High
			signals[i].PrevLow1 = aggregated[i-1].Low
		} else {
			signals[i].PrevHigh1 = math.NaN()
			signals[i].PrevLow1 = math.NaN()
		}
		if i >= 2 {
			signals[i].PrevHigh2 = aggregated[i-2].High
			signals[i].PrevLow2 = aggregated[i-2].Low
		} else {
			signals[i].PrevHigh2 = math.NaN()
			signals[i].PrevLow2 = math.NaN()
		}
	}
	return signals, nil
}

func (p *Platform) loadStrategySignalBars(timeframe string) ([]strategySignalBar, error) {
	switch strings.ToLower(timeframe) {
	case "4h":
		return readSignalBarsCSV("BTC_4H_Signals.csv")
	case "1d":
		minuteBars, err := p.loadCandleBars()
		if err != nil {
			return nil, err
		}
		return buildSignalBars(minuteBars, "1d")
	default:
		return nil, fmt.Errorf("unsupported signal timeframe: %s", timeframe)
	}
}

func readSignalBarsCSV(path string) ([]strategySignalBar, error) {
	resolved := path
	if !filepath.IsAbs(path) {
		_, currentFile, _, _ := runtime.Caller(0)
		resolved = filepath.Join(filepath.Dir(currentFile), "..", "..", path)
	}

	file, err := os.Open(filepath.Clean(resolved))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, fmt.Errorf("signal csv is empty: %s", resolved)
	}

	signals := make([]strategySignalBar, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 12 {
			continue
		}
		ts, err := time.Parse("2006-01-02 15:04:05Z07:00", row[0])
		if err != nil {
			return nil, fmt.Errorf("parse signal time %q: %w", row[0], err)
		}
		openValue, err := strconv.ParseFloat(row[1], 64)
		if err != nil {
			return nil, err
		}
		highValue, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, err
		}
		lowValue, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return nil, err
		}
		closeValue, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, err
		}
		volumeValue, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, err
		}
		ma20Value, err := parseCSVFloatOrNaN(row[6])
		if err != nil {
			return nil, err
		}
		atrValue, err := parseCSVFloatOrNaN(row[7])
		if err != nil {
			return nil, err
		}
		prevHigh1Value, err := parseCSVFloatOrNaN(row[8])
		if err != nil {
			return nil, err
		}
		prevHigh2Value, err := parseCSVFloatOrNaN(row[9])
		if err != nil {
			return nil, err
		}
		prevLow1Value, err := parseCSVFloatOrNaN(row[10])
		if err != nil {
			return nil, err
		}
		prevLow2Value, err := parseCSVFloatOrNaN(row[11])
		if err != nil {
			return nil, err
		}
		signals = append(signals, strategySignalBar{
			Time:      ts.UTC(),
			Open:      openValue,
			High:      highValue,
			Low:       lowValue,
			Close:     closeValue,
			Volume:    volumeValue,
			MA20:      ma20Value,
			ATR:       atrValue,
			PrevHigh1: prevHigh1Value,
			PrevHigh2: prevHigh2Value,
			PrevLow1:  prevLow1Value,
			PrevLow2:  prevLow2Value,
		})
	}
	return signals, nil
}

func parseCSVFloatOrNaN(raw string) (float64, error) {
	if strings.TrimSpace(raw) == "" {
		return math.NaN(), nil
	}
	return strconv.ParseFloat(raw, 64)
}

func trimSignalBars(signals []strategySignalBar, from, to time.Time) []strategySignalBar {
	if from.IsZero() && to.IsZero() {
		return signals
	}
	filtered := make([]strategySignalBar, 0, len(signals))
	for _, sig := range signals {
		if !from.IsZero() && sig.Time.Before(from) {
			continue
		}
		if !to.IsZero() && sig.Time.After(to) {
			continue
		}
		filtered = append(filtered, sig)
	}
	return filtered
}

func resolveReplayRange(cfg strategyReplayConfig, signals []strategySignalBar) (time.Time, time.Time) {
	start := signals[0].Time
	end := signals[len(signals)-1].Time
	if !cfg.From.IsZero() {
		start = cfg.From
	}
	if !cfg.To.IsZero() {
		end = cfg.To
	}
	return start, end
}

func (p *Platform) walkMinuteExecutionBars(from, to time.Time, fn func(executionBar) error) error {
	bars, err := p.loadCandleBars()
	if err != nil {
		return err
	}
	for _, bar := range bars {
		if bar.Time.Before(from) || bar.Time.After(to) {
			continue
		}
		if err := fn(executionBar{
			Time:  bar.Time,
			Open:  bar.Open,
			High:  bar.High,
			Low:   bar.Low,
			Close: bar.Close,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (p *Platform) walkTickExecutionBars(symbol string, from, to time.Time, fn func(executionBar) error) error {
	iterator, err := p.newTickArchiveIteratorForRange(symbol, from.UTC().Format(time.RFC3339), to.UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}
	defer iterator.Close()
	for {
		event, err := iterator.Next()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			if err == nil {
				continue
			}
		}
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			return err
		}
		if err := fn(executionBar{
			Time:  event.Time,
			Open:  event.Price,
			High:  event.Price,
			Low:   event.Price,
			Close: event.Price,
		}); err != nil {
			return err
		}
	}
}

func resolveStopPrice(side string, entry float64, sig strategySignalBar, stopMode string, stopLossATR float64) float64 {
	structural := sig.PrevLow1
	atrStop := entry - stopLossATR*sig.ATR
	if side == "short" {
		structural = sig.PrevHigh1
		atrStop = entry + stopLossATR*sig.ATR
	}
	switch stopMode {
	case "structural":
		return structural
	case "hybrid_tighter":
		if side == "long" {
			return math.Max(structural, atrStop)
		}
		return math.Min(structural, atrStop)
	case "hybrid_wider":
		if side == "long" {
			return math.Min(structural, atrStop)
		}
		return math.Max(structural, atrStop)
	default:
		return atrStop
	}
}

func getReentrySize(tradesInBar int, schedule []float64) float64 {
	if tradesInBar <= 0 {
		return 0
	}
	idx := tradesInBar - 1
	if idx >= len(schedule) {
		idx = len(schedule) - 1
	}
	if idx < 0 {
		return 0
	}
	return schedule[idx]
}

func rollingMean(values []float64, end, window int) float64 {
	if end+1 < window {
		return math.NaN()
	}
	sum := 0.0
	for i := end - window + 1; i <= end; i++ {
		sum += values[i]
	}
	return sum / float64(window)
}

func firstPositive(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

func maxIntValue(value any, fallback int) int {
	n := int(parseFloatValue(value))
	if n > 0 {
		return n
	}
	return fallback
}

func pairStrategyTrades(events []map[string]any, source string) []map[string]any {
	trades := make([]map[string]any, 0)
	var current map[string]any
	for _, event := range events {
		kind := strings.ToUpper(stringValue(event["type"]))
		switch kind {
		case "BUY", "SHORT":
			current = event
		case "EXIT":
			if current == nil {
				continue
			}
			trades = append(trades, map[string]any{
				"source":        source,
				"side":          stringValue(current["type"]),
				"quantity":      float64(1),
				"entryTarget":   current["price"],
				"entryTime":     current["time"],
				"entryPrice":    current["price"],
				"entryReason":   current["reason"],
				"exitTime":      event["time"],
				"exitPrice":     event["price"],
				"exitType":      event["reason"],
				"realizedPnL":   event["pnl"],
				"processedBars": 0,
				"status":        "closed",
			})
			current = nil
		}
	}
	return trades
}

func summarizeExecutionTradeRecords(trades []map[string]any) map[string]any {
	closed := 0
	wins := 0
	losses := 0
	totalPnL := 0.0
	for _, trade := range trades {
		if stringValue(trade["status"]) != "closed" {
			continue
		}
		closed++
		pnl := parseFloatValue(trade["realizedPnL"])
		totalPnL += pnl
		if pnl > 0 {
			wins++
		} else if pnl < 0 {
			losses++
		}
	}
	winRate := 0.0
	if closed > 0 {
		winRate = float64(wins) / float64(closed)
	}
	return map[string]any{
		"executionTrades":       trades,
		"executionTradeCount":   len(trades),
		"executionClosedCount":  closed,
		"executionOpenCount":    0,
		"executionPendingCount": 0,
		"executionWins":         wins,
		"executionLosses":       losses,
		"executionWinRate":      winRate,
		"executionRealizedPnL":  totalPnL,
	}
}

func computeMaxDrawdown(equity []float64) float64 {
	if len(equity) == 0 {
		return 0
	}
	peak := equity[0]
	maxDD := 0.0
	for _, value := range equity {
		if value > peak {
			peak = value
		}
		dd := value/peak - 1
		if dd < maxDD {
			maxDD = dd
		}
	}
	return maxDD
}
