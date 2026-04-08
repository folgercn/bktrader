package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const liveMarketWarmWindow = 30 * 24 * time.Hour

type liveMarketSnapshot struct {
	Symbol     string
	MinuteBars []candleBar
	SignalBars map[string][]strategySignalBar
	UpdatedAt  time.Time
}

func (p *Platform) WarmLiveMarketData(ctx context.Context) error {
	symbols := p.collectLiveMarketSymbols()
	if len(symbols) == 0 {
		symbols = []string{"BTCUSDT"}
	}
	var firstErr error
	for _, symbol := range symbols {
		if ctx != nil {
			select {
			case <-ctx.Done():
				if firstErr != nil {
					return firstErr
				}
				return ctx.Err()
			default:
			}
		}
		if err := p.refreshLiveMarketSnapshot(symbol); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *Platform) collectLiveMarketSymbols() []string {
	symbols := []string{}
	strategies, err := p.ListStrategies()
	if err != nil {
		return []string{"BTCUSDT"}
	}
	for _, strategy := range strategies {
		id := stringValue(strategy["id"])
		if strings.TrimSpace(id) == "" {
			continue
		}
		bindings, bindErr := p.ListStrategySignalBindings(id)
		if bindErr != nil {
			continue
		}
		for _, binding := range bindings {
			symbol := NormalizeSymbol(binding.Symbol)
			if symbol == "" || slices.Contains(symbols, symbol) {
				continue
			}
			symbols = append(symbols, symbol)
		}
	}
	if len(symbols) == 0 {
		symbols = append(symbols, "BTCUSDT")
	}
	return symbols
}

func (p *Platform) refreshLiveMarketSnapshot(symbol string) error {
	normalizedSymbol := NormalizeSymbol(symbol)
	if normalizedSymbol == "" {
		normalizedSymbol = "BTCUSDT"
	}
	end := time.Now().UTC().Truncate(time.Minute)
	start := end.Add(-liveMarketWarmWindow)
	minuteBars, err := fetchBinanceFuturesCandleRange(normalizedSymbol, "1", start, end)
	if err != nil {
		return err
	}
	if len(minuteBars) == 0 {
		return fmt.Errorf("no live market minute bars returned for %s", normalizedSymbol)
	}
	signal1D, err := buildSignalBars(minuteBars, "1d")
	if err != nil {
		return err
	}
	signal4H, err := buildSignalBars(minuteBars, "4h")
	if err != nil {
		return err
	}
	p.liveMarketMu.Lock()
	p.liveMarketData[normalizedSymbol] = liveMarketSnapshot{
		Symbol:     normalizedSymbol,
		MinuteBars: minuteBars,
		SignalBars: map[string][]strategySignalBar{
			"1d": signal1D,
			"4h": signal4H,
		},
		UpdatedAt: time.Now().UTC(),
	}
	p.liveMarketMu.Unlock()
	return nil
}

func (p *Platform) liveMarketSnapshot(symbol string) (liveMarketSnapshot, error) {
	normalizedSymbol := NormalizeSymbol(symbol)
	if normalizedSymbol == "" {
		normalizedSymbol = "BTCUSDT"
	}
	p.liveMarketMu.RLock()
	snapshot, ok := p.liveMarketData[normalizedSymbol]
	p.liveMarketMu.RUnlock()
	if ok && time.Since(snapshot.UpdatedAt) < 10*time.Minute && len(snapshot.MinuteBars) > 0 {
		return snapshot, nil
	}
	if err := p.refreshLiveMarketSnapshot(normalizedSymbol); err != nil {
		if ok {
			return snapshot, nil
		}
		return liveMarketSnapshot{}, err
	}
	p.liveMarketMu.RLock()
	snapshot = p.liveMarketData[normalizedSymbol]
	p.liveMarketMu.RUnlock()
	return snapshot, nil
}

func (p *Platform) liveSignalBarStates(symbol, timeframe string) (map[string]any, error) {
	snapshot, err := p.liveMarketSnapshot(symbol)
	if err != nil {
		return nil, err
	}
	bars := snapshot.SignalBars[strings.ToLower(strings.TrimSpace(timeframe))]
	if len(bars) == 0 {
		return nil, fmt.Errorf("no live market %s signal bars cached for %s", timeframe, NormalizeSymbol(symbol))
	}
	index := len(bars) - 1
	current := bars[index]
	entry := map[string]any{
		"symbol":    NormalizeSymbol(symbol),
		"timeframe": strings.ToLower(strings.TrimSpace(timeframe)),
		"barCount":  index + 1,
		"sma5":      current.MA5,
		"ma20":      current.MA20,
		"atr14":     current.ATR,
		"current":   strategySignalBarToStateEntry(current, symbol, timeframe),
	}
	if index >= 1 {
		entry["prevBar1"] = strategySignalBarToStateEntry(bars[index-1], symbol, timeframe)
	}
	if index >= 2 {
		entry["prevBar2"] = strategySignalBarToStateEntry(bars[index-2], symbol, timeframe)
	}
	return map[string]any{
		fmt.Sprintf("market-cache|%s|signal|%s", NormalizeSymbol(symbol), strings.ToLower(strings.TrimSpace(timeframe))): entry,
	}, nil
}

func strategySignalBarToStateEntry(bar strategySignalBar, symbol, timeframe string) map[string]any {
	return map[string]any{
		"symbol":    NormalizeSymbol(symbol),
		"timeframe": strings.ToLower(strings.TrimSpace(timeframe)),
		"open":      bar.Open,
		"high":      bar.High,
		"low":       bar.Low,
		"close":     bar.Close,
		"volume":    bar.Volume,
		"updatedAt": bar.Time.UTC().Format(time.RFC3339),
		"isClosed":  true,
	}
}

func fetchBinanceFuturesCandleRange(symbol, resolution string, from, to time.Time) ([]candleBar, error) {
	if from.IsZero() || to.IsZero() || !to.After(from) {
		return nil, fmt.Errorf("invalid candle range for %s %s", NormalizeSymbol(symbol), resolution)
	}
	step := resolutionToDuration(resolution)
	if step <= 0 {
		return nil, fmt.Errorf("unsupported resolution: %s", resolution)
	}
	cursor := from.UTC()
	end := to.UTC()
	bars := make([]candleBar, 0, 4096)
	for !cursor.After(end) {
		chunk, err := fetchBinanceFuturesCandles(symbol, resolution, cursor.Unix(), end.Unix(), 1500)
		if err != nil {
			return nil, err
		}
		if len(chunk) == 0 {
			break
		}
		bars = appendCandleBarsDedup(bars, chunk)
		nextCursor := chunk[len(chunk)-1].Time.Add(step)
		if !nextCursor.After(cursor) {
			break
		}
		cursor = nextCursor
	}
	return bars, nil
}

func appendCandleBarsDedup(existing []candleBar, incoming []candleBar) []candleBar {
	if len(incoming) == 0 {
		return existing
	}
	if len(existing) == 0 {
		return append(existing, incoming...)
	}
	lastTime := existing[len(existing)-1].Time
	for _, bar := range incoming {
		if !bar.Time.After(lastTime) {
			continue
		}
		existing = append(existing, bar)
		lastTime = bar.Time
	}
	return existing
}

func (p *Platform) buildLiveExecutionPlanFromMarketData(
	session domain.LiveSession,
	version domain.StrategyVersion,
	engine StrategyEngine,
	engineKey string,
	parameters map[string]any,
	semantics StrategyExecutionSemantics,
) ([]paperPlannedOrder, error) {
	context := StrategyExecutionContext{
		StrategyEngineKey:   engineKey,
		StrategyVersionID:   version.ID,
		SignalTimeframe:     stringValue(parameters["signalTimeframe"]),
		ExecutionDataSource: stringValue(parameters["executionDataSource"]),
		Symbol:              stringValue(parameters["symbol"]),
		From:                parseOptionalRFC3339(stringValue(parameters["from"])),
		To:                  parseOptionalRFC3339(stringValue(parameters["to"])),
		Parameters:          parameters,
		Semantics:           semantics,
	}
	cfg := buildStrategyReplayConfig(context)
	if cfg.ExecutionDataSource != "1min" {
		return nil, fmt.Errorf("live market execution source not supported yet: %s", cfg.ExecutionDataSource)
	}

	snapshot, err := p.liveMarketSnapshot(cfg.Symbol)
	if err != nil {
		return nil, err
	}
	signals := snapshot.SignalBars[strings.ToLower(cfg.SignalTimeframe)]
	if len(signals) < 2 {
		return nil, fmt.Errorf("not enough %s signal bars from market source for %s", cfg.SignalTimeframe, cfg.Symbol)
	}
	minuteBars := snapshot.MinuteBars
	if len(minuteBars) == 0 {
		return nil, fmt.Errorf("no live market minute bars cached for %s", cfg.Symbol)
	}
	rangeStart, rangeEnd := resolveReplayRange(cfg, signals)
	signals = trimSignalBars(signals, rangeStart, rangeEnd)
	if len(signals) < 2 {
		return nil, fmt.Errorf("no %s signal bars in selected live market range", cfg.SignalTimeframe)
	}

	var result map[string]any
	switch normalized := normalizeStrategyEngineKey(engineKey); normalized {
	case "bk-default":
		result, err = runStrategyReplayOnMinuteBars(cfg, signals, minuteBars)
	default:
		result, err = engine.Run(context)
	}
	if err != nil {
		return nil, err
	}
	trades, err := executionTradesFromResult(result)
	if err != nil {
		return nil, err
	}
	return buildPaperExecutionPlan(domain.PaperSession{ID: session.ID, StrategyID: session.StrategyID}, version, engineKey, semantics, trades)
}
