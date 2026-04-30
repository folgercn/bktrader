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

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type strategySignalBar struct {
	Time          time.Time
	Open          float64
	High          float64
	Low           float64
	Close         float64
	Volume        float64
	MA5           float64
	MA20          float64
	ATR           float64
	ATRPercentile float64
	PrevHigh1     float64
	PrevHigh2     float64
	PrevHigh3     float64
	PrevLow1      float64
	PrevLow2      float64
	PrevLow3      float64
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
	EntryTime  time.Time
	HWM        float64
	LWM        float64
}

type strategyReplayConfig struct {
	SignalTimeframe           string
	ExecutionDataSource       string
	Symbol                    string
	From                      time.Time
	To                        time.Time
	InitialBalance            float64
	Dir1ReentryConfirm        bool
	Dir2ZeroInitial           bool
	ZeroInitialMode           string
	FixedSlippage             float64
	StopLossATR               float64
	MaxTradesPerBar           int
	ReentrySizeSchedule       []float64
	LongReentryATR            float64
	ShortReentryATR           float64
	StopMode                  string
	ProfitProtectATR          float64
	TrailingStopATR           float64
	DelayedTrailingATR        float64
	TradingFeeRate            float64
	FundingRate               float64
	FundingIntervalHours      int
	BreakoutShape             string
	BreakoutShapeToleranceBps float64
	UseSMA5IntradayStructure  bool
}

func (p *Platform) runStrategyReplay(context StrategyExecutionContext) (map[string]any, error) {
	cfg := buildStrategyReplayConfig(context)
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
	if cfg.ExecutionDataSource == "tick" {
		return p.runStrategyReplayOnTick(cfg, signals)
	}

	return nil, fmt.Errorf("unsupported execution data source: %s", cfg.ExecutionDataSource)
}

func (p *Platform) runStrategyReplayOnTick(cfg strategyReplayConfig, signals []strategySignalBar) (map[string]any, error) {
	rangeStart, rangeEnd := resolveReplayRange(cfg, signals)
	iterator, err := p.newTickArchiveIteratorForRange(cfg.Symbol, rangeStart.UTC().Format(time.RFC3339), rangeEnd.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	engine := newStrategyReplayEngine(cfg)
	commission := cfg.TradingFeeRate
	initialUsage := 0.10
	if cfg.Dir2ZeroInitial {
		initialUsage = 0.0
	}

	nextEvent, hasEvent, err := nextTickEvent(iterator)
	if err != nil {
		return nil, err
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

		for hasEvent && nextEvent.Time.Before(startT) {
			nextEvent, hasEvent, err = nextTickEvent(iterator)
			if err != nil {
				return nil, err
			}
		}
		if !hasEvent {
			break
		}

		sig := signals[i]
		if math.IsNaN(sig.ATR) || sig.ATR <= 0 {
			continue
		}
		tradesInBar := 0
		if i-engine.lastExitBarIndex > 1 {
			engine.lastExitSide = ""
		}
		engine.clearExpiredZeroInitialWindow(i)

		for hasEvent && !nextEvent.Time.After(endT) {
			current := nextEvent
			engine.processedCount++
			if engine.position == nil {
				executed := false

				longRegimeReady, shortRegimeReady := strategySignalRegimeReady(sig, engine.cfg.SignalTimeframe, engine.cfg.UseSMA5IntradayStructure)
				if longRegimeReady {
					reP := sig.PrevLow1 + cfg.LongReentryATR*sig.ATR
					if breakout := resolveReplayInitialBreakout(sig, "long", current.Price, cfg); tradesInBar == 0 && breakout.Ready {
						if engine.zeroInitialReentryWindowEnabled() {
							engine.armZeroInitialWindow("long", i)
						} else {
							entry := math.Max(current.Price, breakout.Level) * (1 + cfg.FixedSlippage)
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
								EntryTime:  current.Time,
								HWM:        entry,
								LWM:        entry,
							}
							tradingFee := notional * commission
							engine.balance -= tradingFee
							engine.totalTradingFees += tradingFee
							engine.appendTrade("BUY", current.Time, entry, "Initial", notional, 0, tradingFee, 0)
							tradesInBar++
							executed = true
						}
					}
					hasExitWindow := engine.lastExitSide == "long" && i-engine.lastExitBarIndex <= 1
					hasZeroWindow := engine.hasZeroInitialWindow("long", i)
					if hasExitWindow || hasZeroWindow {
						if current.Price >= reP {
							reason := "Zero-Initial-Reentry"
							size := getReentrySize(1, cfg.ReentrySizeSchedule)
							effectiveTradesInBar := 0
							ok := size > 0
							if hasExitWindow {
								reason = "PT-Reentry"
								if engine.lastExitReason == "SL" {
									reason = "SL-Reentry"
								}
								size, effectiveTradesInBar, ok = resolveReplayReentrySlot(tradesInBar, cfg.MaxTradesPerBar, cfg.ReentrySizeSchedule)
							}
							if ok {
								notional := engine.balance * size
								entry := reP * (1 + cfg.FixedSlippage)
								stopLoss := resolveStopPrice("long", entry, sig, cfg.StopMode, cfg.StopLossATR)
								engine.position = &strategyPosition{
									Side:       "long",
									EntryPrice: entry,
									StopLoss:   stopLoss,
									Protected:  false,
									Notional:   notional,
									Reason:     reason,
									BarIndex:   i,
									EntryTime:  current.Time,
									HWM:        entry,
									LWM:        entry,
								}
								tradingFee := notional * commission
								engine.balance -= tradingFee
								engine.totalTradingFees += tradingFee
								engine.appendTrade("BUY", current.Time, entry, reason, notional, 0, tradingFee, 0)
								if hasExitWindow {
									tradesInBar = effectiveTradesInBar + 1
								} else {
									tradesInBar = 1
								}
								executed = true
							}
							if hasExitWindow {
								engine.lastExitSide = ""
							}
							if hasZeroWindow {
								engine.clearZeroInitialWindow()
							}
						}
					}
				} else if shortRegimeReady {
					reP := sig.PrevHigh1 + cfg.ShortReentryATR*sig.ATR
					if breakout := resolveReplayInitialBreakout(sig, "short", current.Price, cfg); tradesInBar == 0 && breakout.Ready {
						if engine.zeroInitialReentryWindowEnabled() {
							engine.armZeroInitialWindow("short", i)
						} else {
							entry := math.Min(current.Price, breakout.Level) * (1 - cfg.FixedSlippage)
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
								EntryTime:  current.Time,
								HWM:        entry,
								LWM:        entry,
							}
							tradingFee := notional * commission
							engine.balance -= tradingFee
							engine.totalTradingFees += tradingFee
							engine.appendTrade("SHORT", current.Time, entry, "Initial", notional, 0, tradingFee, 0)
							tradesInBar++
							executed = true
						}
					}
					hasExitWindow := engine.lastExitSide == "short" && i-engine.lastExitBarIndex <= 1
					hasZeroWindow := engine.hasZeroInitialWindow("short", i)
					if hasExitWindow || hasZeroWindow {
						if current.Price <= reP {
							reason := "Zero-Initial-Reentry"
							size := getReentrySize(1, cfg.ReentrySizeSchedule)
							effectiveTradesInBar := 0
							ok := size > 0
							if hasExitWindow {
								reason = "PT-Reentry"
								if engine.lastExitReason == "SL" {
									reason = "SL-Reentry"
								}
								size, effectiveTradesInBar, ok = resolveReplayReentrySlot(tradesInBar, cfg.MaxTradesPerBar, cfg.ReentrySizeSchedule)
							}
							if ok {
								notional := engine.balance * size
								entry := reP * (1 - cfg.FixedSlippage)
								stopLoss := resolveStopPrice("short", entry, sig, cfg.StopMode, cfg.StopLossATR)
								engine.position = &strategyPosition{
									Side:       "short",
									EntryPrice: entry,
									StopLoss:   stopLoss,
									Protected:  false,
									Notional:   notional,
									Reason:     reason,
									BarIndex:   i,
									EntryTime:  current.Time,
									HWM:        entry,
									LWM:        entry,
								}
								tradingFee := notional * commission
								engine.balance -= tradingFee
								engine.totalTradingFees += tradingFee
								engine.appendTrade("SHORT", current.Time, entry, reason, notional, 0, tradingFee, 0)
								if hasExitWindow {
									tradesInBar = effectiveTradesInBar + 1
								} else {
									tradesInBar = 1
								}
								executed = true
							}
							if hasExitWindow {
								engine.lastExitSide = ""
							}
							if hasZeroWindow {
								engine.clearZeroInitialWindow()
							}
						}
					}
				}

				nextEvent, hasEvent, err = advanceTickIterator(iterator, current.Time)
				if err != nil {
					return nil, err
				}
				if executed {
					continue
				}
				continue
			}

			exitReason, exitPrice, exitTriggered := evaluateReplayPositionExit(engine.position, sig, cfg, current.Price, current.Price)

			nextEvent, hasEvent, err = advanceTickIterator(iterator, current.Time)
			if err != nil {
				return nil, err
			}

			if !exitTriggered {
				continue
			}

			sideMult := 1.0
			if engine.position.Side == "short" {
				sideMult = -1.0
				exitPrice *= (1 + cfg.FixedSlippage)
			} else {
				exitPrice *= (1 - cfg.FixedSlippage)
			}
			pnl := 0.0
			tradingFee := 0.0
			fundingPnL := 0.0
			if engine.position.Notional > 0 {
				tradingFee = engine.position.Notional * commission
				fundingPnL = computeFundingPnL(engine.position, current.Time, cfg)
				pnl = sideMult*(exitPrice-engine.position.EntryPrice)/engine.position.EntryPrice*engine.position.Notional + fundingPnL
				engine.balance += pnl - tradingFee
				engine.totalTradingFees += tradingFee
				engine.totalFundingPnL += fundingPnL
			}
			engine.appendTrade("EXIT", current.Time, exitPrice, exitReason, engine.position.Notional, pnl, tradingFee, fundingPnL)
			engine.lastExitReason = exitReason
			engine.lastExitSide = engine.position.Side
			engine.lastExitBarIndex = i
			engine.position = nil
		}
	}

	result := engine.summary(signals)
	result["matchedArchiveFiles"] = 1
	return result, nil
}

func nextTickEvent(iterator *tickArchiveIterator) (tickEvent, bool, error) {
	event, err := iterator.Next()
	if err != nil {
		if err.Error() == "EOF" {
			return tickEvent{}, false, nil
		}
		return tickEvent{}, false, err
	}
	return event, true, nil
}

func advanceTickIterator(iterator *tickArchiveIterator, currentTime time.Time) (tickEvent, bool, error) {
	for {
		event, ok, err := nextTickEvent(iterator)
		if err != nil || !ok {
			return event, ok, err
		}
		if event.Time.After(currentTime) {
			return event, true, nil
		}
	}
}

func runStrategyReplayOnMinuteBars(cfg strategyReplayConfig, signals []strategySignalBar, minuteBars []candleBar) (map[string]any, error) {
	engine := newStrategyReplayEngine(cfg)
	barCursor := 0
	commission := cfg.TradingFeeRate
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
		engine.clearExpiredZeroInitialWindow(i)

		for currentIdx < windowEnd {
			bar := minuteBars[currentIdx]
			engine.processedCount++
			var prevBar *candleBar
			if currentIdx > windowStart {
				prevBar = &minuteBars[currentIdx-1]
			}

			if engine.position == nil {
				executed := false

				longRegimeReady, shortRegimeReady := strategySignalRegimeReady(sig, engine.cfg.SignalTimeframe, engine.cfg.UseSMA5IntradayStructure)
				if longRegimeReady {
					reP := sig.PrevLow1 + cfg.LongReentryATR*sig.ATR
					if breakout := resolveReplayInitialBreakout(sig, "long", bar.High, cfg); tradesInBar == 0 && breakout.Ready {
						if engine.zeroInitialReentryWindowEnabled() {
							engine.armZeroInitialWindow("long", i)
						} else {
							entry := math.Max(bar.Open, breakout.Level) * (1 + cfg.FixedSlippage)
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
								EntryTime:  bar.Time,
								HWM:        entry,
								LWM:        entry,
							}
							tradingFee := notional * commission
							engine.balance -= tradingFee
							engine.totalTradingFees += tradingFee
							engine.appendTrade("BUY", bar.Time, entry, "Initial", notional, 0, tradingFee, 0)
							tradesInBar++
							executed = true
						}
					}
					hasExitWindow := engine.lastExitSide == "long" && i-engine.lastExitBarIndex <= 1
					hasZeroWindow := engine.hasZeroInitialWindow("long", i)
					if hasExitWindow || hasZeroWindow {
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
							reason := "Zero-Initial-Reentry"
							size := getReentrySize(1, cfg.ReentrySizeSchedule)
							effectiveTradesInBar := 0
							ok := size > 0
							if hasExitWindow {
								reason = "PT-Reentry"
								if engine.lastExitReason == "SL" {
									reason = "SL-Reentry"
								}
								size, effectiveTradesInBar, ok = resolveReplayReentrySlot(tradesInBar, cfg.MaxTradesPerBar, cfg.ReentrySizeSchedule)
							}
							if ok {
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
									EntryTime:  bar.Time,
									HWM:        entry,
									LWM:        entry,
								}
								tradingFee := notional * commission
								engine.balance -= tradingFee
								engine.totalTradingFees += tradingFee
								engine.appendTrade("BUY", bar.Time, entry, reason, notional, 0, tradingFee, 0)
								if hasExitWindow {
									tradesInBar = effectiveTradesInBar + 1
								} else {
									tradesInBar = 1
								}
								executed = true
							}
							if hasExitWindow {
								engine.lastExitSide = ""
							}
							if hasZeroWindow {
								engine.clearZeroInitialWindow()
							}
						}
					}
				} else if shortRegimeReady {
					reP := sig.PrevHigh1 + cfg.ShortReentryATR*sig.ATR
					if breakout := resolveReplayInitialBreakout(sig, "short", bar.Low, cfg); tradesInBar == 0 && breakout.Ready {
						if engine.zeroInitialReentryWindowEnabled() {
							engine.armZeroInitialWindow("short", i)
						} else {
							entry := math.Min(bar.Open, breakout.Level) * (1 - cfg.FixedSlippage)
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
								EntryTime:  bar.Time,
								HWM:        entry,
								LWM:        entry,
							}
							tradingFee := notional * commission
							engine.balance -= tradingFee
							engine.totalTradingFees += tradingFee
							engine.appendTrade("SHORT", bar.Time, entry, "Initial", notional, 0, tradingFee, 0)
							tradesInBar++
							executed = true
						}
					}
					hasExitWindow := engine.lastExitSide == "short" && i-engine.lastExitBarIndex <= 1
					hasZeroWindow := engine.hasZeroInitialWindow("short", i)
					if hasExitWindow || hasZeroWindow {
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
							reason := "Zero-Initial-Reentry"
							size := getReentrySize(1, cfg.ReentrySizeSchedule)
							effectiveTradesInBar := 0
							ok := size > 0
							if hasExitWindow {
								reason = "PT-Reentry"
								if engine.lastExitReason == "SL" {
									reason = "SL-Reentry"
								}
								size, effectiveTradesInBar, ok = resolveReplayReentrySlot(tradesInBar, cfg.MaxTradesPerBar, cfg.ReentrySizeSchedule)
							}
							if ok {
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
									EntryTime:  bar.Time,
									HWM:        entry,
									LWM:        entry,
								}
								tradingFee := notional * commission
								engine.balance -= tradingFee
								engine.totalTradingFees += tradingFee
								engine.appendTrade("SHORT", bar.Time, entry, reason, notional, 0, tradingFee, 0)
								if hasExitWindow {
									tradesInBar = effectiveTradesInBar + 1
								} else {
									tradesInBar = 1
								}
								executed = true
							}
							if hasExitWindow {
								engine.lastExitSide = ""
							}
							if hasZeroWindow {
								engine.clearZeroInitialWindow()
							}
						}
					}
				}

				currentIdx++
				if executed {
					continue
				}
				continue
			}

			exitReason, exitPrice, exitTriggered := evaluateReplayPositionExit(engine.position, sig, cfg, bar.High, bar.Low)

			if exitTriggered {
				sideMult := 1.0
				if engine.position.Side == "short" {
					sideMult = -1.0
					exitPrice *= (1 + cfg.FixedSlippage)
				} else {
					exitPrice *= (1 - cfg.FixedSlippage)
				}
				pnl := 0.0
				tradingFee := 0.0
				fundingPnL := 0.0
				if engine.position.Notional > 0 {
					tradingFee = engine.position.Notional * commission
					fundingPnL = computeFundingPnL(engine.position, bar.Time, cfg)
					pnl = sideMult*(exitPrice-engine.position.EntryPrice)/engine.position.EntryPrice*engine.position.Notional + fundingPnL
					engine.balance += pnl - tradingFee
					engine.totalTradingFees += tradingFee
					engine.totalFundingPnL += fundingPnL
				}
				engine.appendTrade("EXIT", bar.Time, exitPrice, exitReason, engine.position.Notional, pnl, tradingFee, fundingPnL)
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
	cfg                        strategyReplayConfig
	balance                    float64
	position                   *strategyPosition
	lastExitReason             string
	lastExitSide               string
	lastExitBarIndex           int
	pendingZeroInitialSide     string
	pendingZeroInitialBarIndex int
	currentBarIndex            int
	tradesInBar                int
	prevExecBar                *executionBar
	equity                     []float64
	trades                     []map[string]any
	processedCount             int
	totalTradingFees           float64
	totalFundingPnL            float64
}

func newStrategyReplayEngine(cfg strategyReplayConfig) *strategyReplayEngine {
	return &strategyReplayEngine{
		cfg:                        cfg,
		balance:                    cfg.InitialBalance,
		lastExitBarIndex:           -999,
		pendingZeroInitialBarIndex: -999,
		equity:                     []float64{cfg.InitialBalance},
		trades:                     make([]map[string]any, 0, 128),
	}
}

func (e *strategyReplayEngine) zeroInitialReentryWindowEnabled() bool {
	return e.cfg.Dir2ZeroInitial && e.cfg.ZeroInitialMode == strategyZeroInitialModeReentryWindow
}

func (e *strategyReplayEngine) clearExpiredZeroInitialWindow(currentBarIndex int) {
	if currentBarIndex-e.pendingZeroInitialBarIndex > 1 {
		e.pendingZeroInitialSide = ""
		e.pendingZeroInitialBarIndex = -999
	}
}

func (e *strategyReplayEngine) armZeroInitialWindow(side string, currentBarIndex int) {
	e.pendingZeroInitialSide = strings.ToLower(strings.TrimSpace(side))
	e.pendingZeroInitialBarIndex = currentBarIndex
}

func (e *strategyReplayEngine) hasZeroInitialWindow(side string, currentBarIndex int) bool {
	return strings.EqualFold(strings.TrimSpace(e.pendingZeroInitialSide), strings.TrimSpace(side)) &&
		currentBarIndex-e.pendingZeroInitialBarIndex <= 1
}

func (e *strategyReplayEngine) clearZeroInitialWindow() {
	e.pendingZeroInitialSide = ""
	e.pendingZeroInitialBarIndex = -999
}

func (e *strategyReplayEngine) process(bar executionBar, signals []strategySignalBar) {
	for e.currentBarIndex < len(signals)-2 && !bar.Time.Before(signals[e.currentBarIndex+1].Time) {
		e.currentBarIndex++
		e.tradesInBar = 0
		e.prevExecBar = nil
		if e.currentBarIndex-e.lastExitBarIndex > 1 {
			e.lastExitSide = ""
		}
		e.clearExpiredZeroInitialWindow(e.currentBarIndex)
	}

	if e.currentBarIndex >= len(signals)-1 {
		return
	}
	sig := signals[e.currentBarIndex]
	if math.IsNaN(sig.ATR) || sig.ATR <= 0 {
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
	commission := e.cfg.TradingFeeRate
	initialUsage := 0.10
	if e.cfg.Dir2ZeroInitial {
		initialUsage = 0.0
	}

	longRegimeReady, shortRegimeReady := strategySignalRegimeReady(sig, e.cfg.SignalTimeframe, e.cfg.UseSMA5IntradayStructure)
	if longRegimeReady {
		reP := sig.PrevLow1 + e.cfg.LongReentryATR*sig.ATR
		if breakout := resolveReplayInitialBreakout(sig, "long", bar.High, e.cfg); e.tradesInBar == 0 && breakout.Ready {
			if e.zeroInitialReentryWindowEnabled() {
				e.armZeroInitialWindow("long", e.currentBarIndex)
			} else {
				entry := math.Max(bar.Open, breakout.Level) * (1 + e.cfg.FixedSlippage)
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
					HWM:        entry,
					LWM:        entry,
				}
				e.balance -= notional * commission
				e.tradesInBar++
				e.appendTrade("BUY", bar.Time, entry, "Initial", notional, 0, notional*commission, 0)
				return
			}
		}
		hasExitWindow := e.lastExitSide == "long" && e.currentBarIndex-e.lastExitBarIndex <= 1
		hasZeroWindow := e.hasZeroInitialWindow("long", e.currentBarIndex)
		if hasExitWindow || hasZeroWindow {
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
				reason := "Zero-Initial-Reentry"
				size := getReentrySize(1, e.cfg.ReentrySizeSchedule)
				effectiveTradesInBar := 0
				ok := size > 0
				if hasExitWindow {
					reason = "PT-Reentry"
					if e.lastExitReason == "SL" {
						reason = "SL-Reentry"
					}
					size, effectiveTradesInBar, ok = resolveReplayReentrySlot(e.tradesInBar, e.cfg.MaxTradesPerBar, e.cfg.ReentrySizeSchedule)
				}
				if ok {
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
						HWM:        entry,
						LWM:        entry,
					}
					e.balance -= notional * commission
					if hasExitWindow {
						e.tradesInBar = effectiveTradesInBar + 1
					} else {
						e.tradesInBar = 1
					}
					e.appendTrade("BUY", bar.Time, entry, reason, notional, 0, notional*commission, 0)
				}
				if hasExitWindow {
					e.lastExitSide = ""
				}
				if hasZeroWindow {
					e.clearZeroInitialWindow()
				}
			}
		}
		return
	}

	if shortRegimeReady {
		reP := sig.PrevHigh1 + e.cfg.ShortReentryATR*sig.ATR
		if breakout := resolveReplayInitialBreakout(sig, "short", bar.Low, e.cfg); e.tradesInBar == 0 && breakout.Ready {
			if e.zeroInitialReentryWindowEnabled() {
				e.armZeroInitialWindow("short", e.currentBarIndex)
			} else {
				entry := math.Min(bar.Open, breakout.Level) * (1 - e.cfg.FixedSlippage)
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
					HWM:        entry,
					LWM:        entry,
				}
				e.balance -= notional * commission
				e.tradesInBar++
				e.appendTrade("SHORT", bar.Time, entry, "Initial", notional, 0, notional*commission, 0)
				return
			}
		}
		hasExitWindow := e.lastExitSide == "short" && e.currentBarIndex-e.lastExitBarIndex <= 1
		hasZeroWindow := e.hasZeroInitialWindow("short", e.currentBarIndex)
		if hasExitWindow || hasZeroWindow {
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
				reason := "Zero-Initial-Reentry"
				size := getReentrySize(1, e.cfg.ReentrySizeSchedule)
				effectiveTradesInBar := 0
				ok := size > 0
				if hasExitWindow {
					reason = "PT-Reentry"
					if e.lastExitReason == "SL" {
						reason = "SL-Reentry"
					}
					size, effectiveTradesInBar, ok = resolveReplayReentrySlot(e.tradesInBar, e.cfg.MaxTradesPerBar, e.cfg.ReentrySizeSchedule)
				}
				if ok {
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
						HWM:        entry,
						LWM:        entry,
					}
					e.balance -= notional * commission
					if hasExitWindow {
						e.tradesInBar = effectiveTradesInBar + 1
					} else {
						e.tradesInBar = 1
					}
					e.appendTrade("SHORT", bar.Time, entry, reason, notional, 0, notional*commission, 0)
				}
				if hasExitWindow {
					e.lastExitSide = ""
				}
				if hasZeroWindow {
					e.clearZeroInitialWindow()
				}
			}
		}
	}
}

func (e *strategyReplayEngine) tryExit(bar executionBar, sig strategySignalBar) {
	if e.position == nil {
		return
	}
	commission := e.cfg.TradingFeeRate
	reason, exitPrice, exitTriggered := evaluateReplayPositionExit(e.position, sig, e.cfg, bar.High, bar.Low)
	if !exitTriggered {
		return
	}

	sideMult := 1.0
	if e.position.Side == "short" {
		sideMult = -1.0
		exitPrice *= (1 + e.cfg.FixedSlippage)
	} else {
		exitPrice *= (1 - e.cfg.FixedSlippage)
	}
	pnl := 0.0
	if e.position.Notional > 0 {
		pnl = sideMult * (exitPrice - e.position.EntryPrice) / e.position.EntryPrice * e.position.Notional
		e.balance += pnl - e.position.Notional*commission
	}
	e.appendTrade("EXIT", bar.Time, exitPrice, reason, e.position.Notional, pnl, e.position.Notional*commission, 0)
	e.lastExitReason = reason
	e.lastExitSide = e.position.Side
	e.lastExitBarIndex = e.currentBarIndex
	e.position = nil
}

func (e *strategyReplayEngine) appendTrade(kind string, at time.Time, price float64, reason string, notional float64, pnl float64, tradingFee float64, fundingPnL float64) {
	record := map[string]any{
		"time":       at.UTC().Format(time.RFC3339),
		"type":       kind,
		"price":      price,
		"reason":     reason,
		"notional":   notional,
		"balance":    e.balance,
		"pnl":        pnl,
		"tradingFee": tradingFee,
		"fundingPnL": fundingPnL,
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
	if e.cfg.ExecutionDataSource == "tick" {
		result["processedTicks"] = e.processedCount
	} else {
		result["processedBars"] = e.processedCount
	}
	result["tradingFees"] = e.totalTradingFees
	result["fundingPnL"] = e.totalFundingPnL
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

func buildStrategyReplayConfig(context StrategyExecutionContext) strategyReplayConfig {
	parameters := context.Parameters
	reentrySizes := normalizeBacktestFloatSlice(parameters["reentry_size_schedule"], domain.ResearchBaselineReentrySizeSchedule())
	stopMode := stringValue(parameters["stop_mode"])
	if stopMode == "" {
		stopMode = "atr"
	}
	dir2ZeroInitial := domain.ResearchBaselineDir2ZeroInitial
	if _, ok := parameters["dir2_zero_initial"]; ok {
		dir2ZeroInitial = boolValue(parameters["dir2_zero_initial"])
	}
	stopLossATR := parseFloatValue(parameters["stop_loss_atr"])
	if stopLossATR <= 0 {
		stopLossATR = 0.05
	}
	return strategyReplayConfig{
		SignalTimeframe:           normalizeSignalBarInterval(context.SignalTimeframe),
		ExecutionDataSource:       strings.ToLower(context.ExecutionDataSource),
		Symbol:                    normalizeBacktestSymbol(context.Symbol),
		From:                      context.From,
		To:                        context.To,
		InitialBalance:            100000.0,
		Dir1ReentryConfirm:        false,
		Dir2ZeroInitial:           dir2ZeroInitial,
		ZeroInitialMode:           resolveStrategyZeroInitialMode(dir2ZeroInitial, parameters["zero_initial_mode"]),
		FixedSlippage:             strategyReplaySlippage(context, parameters),
		StopLossATR:               stopLossATR,
		MaxTradesPerBar:           maxIntValue(parameters["max_trades_per_bar"], domain.ResearchBaselineMaxTradesPerBar),
		ReentrySizeSchedule:       reentrySizes,
		LongReentryATR:            parseFloatValue(firstNonNil(parameters["long_reentry_atr"], 0.1)),
		ShortReentryATR:           parseFloatValue(firstNonNil(parameters["short_reentry_atr"], 0.0)),
		StopMode:                  stopMode,
		ProfitProtectATR:          firstPositive(parseFloatValue(parameters["profit_protect_atr"]), 1.0),
		TrailingStopATR:           parseFloatValue(parameters["trailing_stop_atr"]),
		DelayedTrailingATR:        parseFloatValue(parameters["delayed_trailing_activation_atr"]),
		TradingFeeRate:            context.Semantics.TradingFeeBps / 10000.0,
		FundingRate:               context.Semantics.FundingRateBps / 10000.0,
		FundingIntervalHours:      maxIntValue(context.Semantics.FundingIntervalHours, 8),
		BreakoutShape:             strings.ToLower(strings.TrimSpace(stringValue(parameters["breakout_shape"]))),
		BreakoutShapeToleranceBps: parseFloatValue(parameters["breakout_shape_tolerance_bps"]),
		UseSMA5IntradayStructure:  boolValue(parameters["use_sma5_intraday_structure"]),
	}
}

func strategyReplaySlippage(context StrategyExecutionContext, parameters map[string]any) float64 {
	if context.Semantics.SlippageMode != SlippageModeSimulated {
		return 0
	}
	return firstPositive(parseFloatValue(parameters["fixed_slippage"]), 0.0005)
}

func computeFundingPnL(position *strategyPosition, exitTime time.Time, cfg strategyReplayConfig) float64 {
	if position == nil || position.Notional <= 0 || cfg.FundingRate == 0 || cfg.FundingIntervalHours <= 0 {
		return 0
	}
	intervals := countFundingIntervals(position.EntryTime, exitTime, cfg.FundingIntervalHours)
	if intervals <= 0 {
		return 0
	}
	sideMult := -1.0
	if position.Side == "short" {
		sideMult = 1.0
	}
	return sideMult * cfg.FundingRate * float64(intervals) * position.Notional
}

func countFundingIntervals(entryTime, exitTime time.Time, intervalHours int) int {
	if intervalHours <= 0 || !exitTime.After(entryTime) {
		return 0
	}
	step := time.Duration(intervalHours) * time.Hour
	firstFunding := entryTime.Truncate(step).Add(step)
	count := 0
	for t := firstFunding; !t.After(exitTime); t = t.Add(step) {
		count++
	}
	return count
}

func buildSignalBars(minuteBars []candleBar, timeframe string) ([]strategySignalBar, error) {
	resolution := liveSignalResolution(timeframe)
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
		signals[i].MA5 = rollingMean(closes, i, 5)
		signals[i].MA20 = rollingMean(closes, i, 20)
		signals[i].ATR = rollingMean(trueRanges, i, 14)
		signals[i].ATRPercentile = rollingLastPercentileFromSeries(trueRanges, i, 14, 240, 50)
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
		if i >= 3 {
			signals[i].PrevHigh3 = aggregated[i-3].High
			signals[i].PrevLow3 = aggregated[i-3].Low
		} else {
			signals[i].PrevHigh3 = math.NaN()
			signals[i].PrevLow3 = math.NaN()
		}
	}
	return signals, nil
}

type replayInitialBreakout struct {
	Ready bool
	Level float64
	Shape string
}

func resolveReplayInitialBreakout(sig strategySignalBar, side string, observedPrice float64, cfg strategyReplayConfig) replayInitialBreakout {
	switch strings.ToLower(strings.TrimSpace(side)) {
	case "long":
		if t2LongBreakoutShapeReady(sig.PrevHigh2, sig.PrevHigh1, cfg.BreakoutShapeToleranceBps) && observedPrice >= sig.PrevHigh2 {
			return replayInitialBreakout{Ready: true, Level: sig.PrevHigh2, Shape: "original_t2"}
		}
	case "short":
		if t2ShortBreakoutShapeReady(sig.PrevLow2, sig.PrevLow1, cfg.BreakoutShapeToleranceBps) && observedPrice <= sig.PrevLow2 {
			return replayInitialBreakout{Ready: true, Level: sig.PrevLow2, Shape: "original_t2"}
		}
	}
	return replayInitialBreakout{}
}

func strategySignalRegimeReady(sig strategySignalBar, timeframe string, useSMA5Intraday ...bool) (bool, bool) {
	tf := normalizeSignalBarInterval(timeframe)
	if tf == "1d" {
		if math.IsNaN(sig.ATR) || sig.ATR <= 0 {
			return false, false
		}
		if math.IsNaN(sig.MA5) || sig.MA5 <= 0 {
			if math.IsNaN(sig.MA20) || sig.MA20 <= 0 {
				return false, false
			}
			return sig.Close > sig.MA20, sig.Close < sig.MA20
		}
		earlyBand := 0.06 * sig.ATR
		longHard := sig.Close > sig.MA5
		shortHard := sig.Close < sig.MA5
		longEarly := sig.Close >= (sig.MA5-earlyBand) && sig.PrevHigh2 > sig.PrevHigh1 && sig.PrevLow1 >= sig.PrevLow2
		shortEarly := sig.Close <= (sig.MA5+earlyBand) && sig.PrevLow2 < sig.PrevLow1 && sig.PrevHigh1 <= sig.PrevHigh2
		return longHard || longEarly, shortHard || shortEarly
	}
	if len(useSMA5Intraday) > 0 && useSMA5Intraday[0] && !math.IsNaN(sig.MA5) && sig.MA5 > 0 {
		return sig.Close > sig.MA5, sig.Close < sig.MA5
	}
	if math.IsNaN(sig.MA20) || sig.MA20 <= 0 {
		return false, false
	}
	return sig.Close > sig.MA20, sig.Close < sig.MA20
}

func (p *Platform) loadStrategySignalBars(timeframe string) ([]strategySignalBar, error) {
	switch normalizeSignalBarInterval(timeframe) {
	case "5m":
		minuteBars, err := p.loadCandleBars()
		if err != nil {
			return nil, err
		}
		return buildSignalBars(minuteBars, "5m")
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
		ma5Value := math.NaN()
		if len(row) >= 13 {
			ma5Value, err = parseCSVFloatOrNaN(row[12])
			if err != nil {
				return nil, err
			}
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
			MA5:       ma5Value,
			MA20:      ma20Value,
			ATR:       atrValue,
			PrevHigh1: prevHigh1Value,
			PrevHigh2: prevHigh2Value,
			PrevHigh3: math.NaN(),
			PrevLow1:  prevLow1Value,
			PrevLow2:  prevLow2Value,
			PrevLow3:  math.NaN(),
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

func resolveReplayReentrySlot(tradesInBar, maxTradesPerBar int, schedule []float64) (float64, int, bool) {
	effectiveTradesInBar := tradesInBar
	if effectiveTradesInBar <= 0 {
		effectiveTradesInBar = 1
	}
	if effectiveTradesInBar >= maxTradesPerBar {
		return 0, tradesInBar, false
	}
	size := getReentrySize(effectiveTradesInBar, schedule)
	if size <= 0 {
		return 0, effectiveTradesInBar, false
	}
	return size, effectiveTradesInBar, true
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

func evaluateReplayPositionExit(position *strategyPosition, sig strategySignalBar, cfg strategyReplayConfig, observedHigh, observedLow float64) (string, float64, bool) {
	if position == nil {
		return "", 0, false
	}
	if observedHigh < observedLow {
		observedHigh, observedLow = observedLow, observedHigh
	}
	prevHWM := firstPositive(position.HWM, position.EntryPrice)
	prevLWM := firstPositive(position.LWM, position.EntryPrice)
	protectedBefore := position.Protected

	if cfg.TrailingStopATR > 0 && sig.ATR > 0 {
		switch position.Side {
		case "long":
			if replayTrailingActive("long", position.EntryPrice, prevHWM, prevLWM, sig.ATR, cfg.DelayedTrailingATR) {
				position.StopLoss = math.Max(position.StopLoss, prevHWM-cfg.TrailingStopATR*sig.ATR)
			}
		case "short":
			if replayTrailingActive("short", position.EntryPrice, prevHWM, prevLWM, sig.ATR, cfg.DelayedTrailingATR) {
				position.StopLoss = math.Min(position.StopLoss, prevLWM+cfg.TrailingStopATR*sig.ATR)
			}
		}
	}

	switch position.Side {
	case "long":
		if observedLow <= position.StopLoss {
			return "SL", position.StopLoss, true
		}
		if protectedBefore && observedLow <= sig.PrevLow1 {
			return "PT", sig.PrevLow1, true
		}
		position.HWM = math.Max(prevHWM, observedHigh)
		if !position.Protected && position.HWM >= position.EntryPrice+cfg.ProfitProtectATR*sig.ATR {
			position.Protected = true
		}
		if cfg.TrailingStopATR > 0 && sig.ATR > 0 && replayTrailingActive("long", position.EntryPrice, position.HWM, prevLWM, sig.ATR, cfg.DelayedTrailingATR) {
			position.StopLoss = math.Max(position.StopLoss, position.HWM-cfg.TrailingStopATR*sig.ATR)
		}
	case "short":
		if observedHigh >= position.StopLoss {
			return "SL", position.StopLoss, true
		}
		if protectedBefore && observedHigh >= sig.PrevHigh1 {
			return "PT", sig.PrevHigh1, true
		}
		position.LWM = math.Min(prevLWM, observedLow)
		if !position.Protected && position.LWM <= position.EntryPrice-cfg.ProfitProtectATR*sig.ATR {
			position.Protected = true
		}
		if cfg.TrailingStopATR > 0 && sig.ATR > 0 && replayTrailingActive("short", position.EntryPrice, prevHWM, position.LWM, sig.ATR, cfg.DelayedTrailingATR) {
			position.StopLoss = math.Min(position.StopLoss, position.LWM+cfg.TrailingStopATR*sig.ATR)
		}
	}
	return "", 0, false
}

func replayTrailingActive(side string, entryPrice, hwm, lwm, atr, delayedActivationATR float64) bool {
	if atr <= 0 {
		return false
	}
	if delayedActivationATR <= 0 {
		return true
	}
	profitATR := 0.0
	switch side {
	case "long":
		profitATR = (hwm - entryPrice) / atr
	case "short":
		profitATR = (entryPrice - lwm) / atr
	}
	return profitATR >= delayedActivationATR
}

func rollingMean(values []float64, end, window int) float64 {
	return rollingMeanOrNaN(values, end, window)
}

func rollingMeanOrNaN(values []float64, end, window int) float64 {
	if window <= 0 || end < 0 || end >= len(values) || end-window+1 < 0 {
		return math.NaN()
	}
	sum := 0.0
	for i := end - window + 1; i <= end; i++ {
		sum += values[i]
	}
	return sum / float64(window)
}

func rollingLastPercentile(values []float64, end, window, minPeriods int) float64 {
	if end < 0 || end >= len(values) {
		return math.NaN()
	}
	start := end - window + 1
	if start < 0 {
		start = 0
	}
	clean := make([]float64, 0, end-start+1)
	for i := start; i <= end; i++ {
		value := values[i]
		if math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		clean = append(clean, value)
	}
	if len(clean) < minPeriods {
		return math.NaN()
	}
	last := clean[len(clean)-1]
	lessOrEqual := 0
	for _, value := range clean {
		if value <= last {
			lessOrEqual++
		}
	}
	return float64(lessOrEqual) / float64(len(clean)) * 100.0
}

func rollingLastPercentileFromSeries(values []float64, end, sourceWindow, percentileWindow, minPeriods int) float64 {
	rolled := make([]float64, end+1)
	for i := 0; i <= end; i++ {
		rolled[i] = rollingMeanOrNaN(values, i, sourceWindow)
	}
	return rollingLastPercentile(rolled, end, percentileWindow, minPeriods)
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
			entryPrice := parseFloatValue(current["price"])
			notional := parseFloatValue(current["notional"])
			quantity := 0.0
			if entryPrice > 0 && notional > 0 {
				quantity = notional / entryPrice
			}
			trades = append(trades, map[string]any{
				"source":          source,
				"side":            stringValue(current["type"]),
				"quantity":        quantity,
				"notional":        notional,
				"entryTarget":     current["price"],
				"entryTime":       current["time"],
				"entryPrice":      current["price"],
				"entryReason":     current["reason"],
				"entryTradingFee": current["tradingFee"],
				"exitTime":        event["time"],
				"exitPrice":       event["price"],
				"exitType":        event["reason"],
				"exitTradingFee":  event["tradingFee"],
				"fundingPnL":      event["fundingPnL"],
				"realizedPnL":     event["pnl"],
				"processedBars":   0,
				"status":          "closed",
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
