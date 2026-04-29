package service

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func (p *Platform) liveSignalWarmWindow() time.Duration {
	days := p.runtimePolicy.LiveSignalWarmWindowDays
	if days <= 0 {
		days = 400
	}
	return time.Duration(days) * 24 * time.Hour
}

func (p *Platform) liveFastSignalWarmWindow() time.Duration {
	days := p.runtimePolicy.LiveFastSignalWarmWindowDays
	if days <= 0 {
		days = 7
	}
	return time.Duration(days) * 24 * time.Hour
}

func (p *Platform) liveMinuteWarmWindow() time.Duration {
	days := p.runtimePolicy.LiveMinuteWarmWindowDays
	if days <= 0 {
		days = 30
	}
	return time.Duration(days) * 24 * time.Hour
}

var fetchLiveCandleRange = fetchBinanceFuturesCandleRange

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
	logger := p.logger("service.live_market")
	logger.Info("warming live market data", "symbol_count", len(symbols), "symbols", symbols)
	var firstErr error
	for _, symbol := range symbols {
		if ctx != nil {
			select {
			case <-ctx.Done():
				if firstErr != nil {
					logger.Warn("live market warm cancelled after partial failure", "error", firstErr)
					return firstErr
				}
				logger.Warn("live market warm cancelled", "error", ctx.Err())
				return ctx.Err()
			default:
			}
		}
		if err := p.refreshLiveMarketSnapshot(symbol); err != nil {
			p.logger("service.live_market", "symbol", NormalizeSymbol(symbol)).Warn("refresh live market snapshot failed", "error", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	if firstErr != nil {
		logger.Warn("live market warm completed with errors", "error", firstErr)
		return firstErr
	}
	logger.Info("live market warm completed")
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
	logger := p.logger("service.live_market", "symbol", normalizedSymbol)
	logger.Debug("refreshing live market snapshot")
	end := time.Now().UTC().Truncate(time.Minute)
	minuteStart := end.Add(-p.liveMinuteWarmWindow())
	signalStart := end.Add(-p.liveSignalWarmWindow())
	fastSignalStart := end.Add(-p.liveFastSignalWarmWindow())
	minuteBars, err := fetchLiveCandleRange(normalizedSymbol, "1", minuteStart, end)
	if err != nil {
		return err
	}
	if len(minuteBars) == 0 {
		return fmt.Errorf("no live market minute bars returned for %s", normalizedSymbol)
	}
	signal5M, err := p.syncStoredSignalBars(normalizedSymbol, "5m", fastSignalStart, end)
	if err != nil {
		return err
	}
	signal15M, err := p.syncStoredSignalBars(normalizedSymbol, "15m", fastSignalStart, end)
	if err != nil {
		return err
	}
	signal30M, err := p.syncStoredSignalBars(normalizedSymbol, "30m", fastSignalStart, end)
	if err != nil {
		return err
	}
	signal1D, err := p.syncStoredSignalBars(normalizedSymbol, "1d", signalStart, end)
	if err != nil {
		return err
	}
	signal4H, err := p.syncStoredSignalBars(normalizedSymbol, "4h", signalStart, end)
	if err != nil {
		return err
	}
	p.liveMarketMu.Lock()
	p.liveMarketData[normalizedSymbol] = liveMarketSnapshot{
		Symbol:     normalizedSymbol,
		MinuteBars: minuteBars,
		SignalBars: map[string][]strategySignalBar{
			"5m":  signal5M,
			"15m": signal15M,
			"30m": signal30M,
			"1d":  signal1D,
			"4h":  signal4H,
		},
		UpdatedAt: time.Now().UTC(),
	}
	p.liveMarketMu.Unlock()
	logger.Debug("live market snapshot refreshed",
		"minute_bar_count", len(minuteBars),
		"signal_5m_bar_count", len(signal5M),
		"signal_15m_bar_count", len(signal15M),
		"signal_30m_bar_count", len(signal30M),
		"signal_1d_bar_count", len(signal1D),
		"signal_4h_bar_count", len(signal4H),
	)
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
	ttl := 10 * time.Minute
	if p.runtimePolicy.LiveMarketCacheTTLMinutes > 0 {
		ttl = time.Duration(p.runtimePolicy.LiveMarketCacheTTLMinutes) * time.Minute
	}
	if ok && time.Since(snapshot.UpdatedAt) < ttl && len(snapshot.MinuteBars) > 0 {
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

func (p *Platform) syncStoredSignalBars(symbol, timeframe string, start, end time.Time) ([]strategySignalBar, error) {
	resolution := liveSignalResolution(timeframe)
	if resolution == "" {
		return nil, fmt.Errorf("unsupported signal timeframe: %s", timeframe)
	}
	fetchedBars, err := fetchLiveCandleRange(symbol, resolution, start, end)
	if err != nil {
		return nil, err
	}
	if len(fetchedBars) == 0 {
		return nil, fmt.Errorf("no live market %s bars returned for %s", timeframe, NormalizeSymbol(symbol))
	}
	if err := p.store.UpsertMarketBars(candleBarsToMarketBars("BINANCE", symbol, timeframe, fetchedBars, "binance-rest-warm")); err != nil {
		return nil, err
	}
	return buildStrategySignalBarsFromCandles(fetchedBars)
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
	if atrPercentile := finiteSignalBarIndicator(current.ATRPercentile); atrPercentile != nil {
		entry["atrPercentile"] = *atrPercentile
	}
	if index >= 1 {
		entry["prevBar1"] = strategySignalBarToStateEntry(bars[index-1], symbol, timeframe)
	}
	if index >= 2 {
		entry["prevBar2"] = strategySignalBarToStateEntry(bars[index-2], symbol, timeframe)
	}
	if index >= 3 {
		entry["prevBar3"] = strategySignalBarToStateEntry(bars[index-3], symbol, timeframe)
	}
	return map[string]any{
		fmt.Sprintf("market-cache|%s|signal|%s", NormalizeSymbol(symbol), strings.ToLower(strings.TrimSpace(timeframe))): entry,
	}, nil
}

func (p *Platform) ingestLiveSignalBarSummary(summary map[string]any, eventTime time.Time) error {
	if !strings.EqualFold(stringValue(summary["streamType"]), "signal_bar") {
		return nil
	}
	timeframe := normalizeSignalBarInterval(stringValue(summary["timeframe"]))
	if timeframe != "5m" && timeframe != "15m" && timeframe != "30m" && timeframe != "1d" && timeframe != "4h" {
		return nil
	}
	symbol := NormalizeSymbol(stringValue(summary["symbol"]))
	if symbol == "" {
		return nil
	}
	openTime := parseUnixMillisTime(summary["barStart"])
	closeTime := parseUnixMillisTime(summary["barEnd"])
	if openTime.IsZero() {
		return nil
	}
	bar := domain.MarketBar{
		ID:        fmt.Sprintf("BINANCE|%s|%s|%s", symbol, timeframe, openTime.UTC().Format(time.RFC3339)),
		Exchange:  "BINANCE",
		Symbol:    symbol,
		Timeframe: timeframe,
		OpenTime:  openTime,
		CloseTime: closeTime,
		Open:      parseFloatValue(summary["open"]),
		High:      parseFloatValue(summary["high"]),
		Low:       parseFloatValue(summary["low"]),
		Close:     parseFloatValue(summary["close"]),
		Volume:    parseFloatValue(summary["volume"]),
		IsClosed:  boolValue(summary["isClosed"]),
		Source:    stringValue(summary["adapter"]),
		UpdatedAt: eventTime.UTC(),
	}
	if err := p.store.UpsertMarketBars([]domain.MarketBar{bar}); err != nil {
		return err
	}
	start := eventTime.UTC().Add(-p.liveSignalWarmWindow())
	storedBars, err := p.store.ListMarketBars("BINANCE", symbol, timeframe, start.Unix(), eventTime.UTC().Unix(), 0)
	if err != nil {
		return err
	}
	signals, err := buildStrategySignalBarsFromCandles(marketBarsToCandles(storedBars))
	if err != nil {
		return err
	}
	p.liveMarketMu.Lock()
	snapshot := p.liveMarketData[symbol]
	if snapshot.SignalBars == nil {
		snapshot.SignalBars = map[string][]strategySignalBar{}
	}
	snapshot.Symbol = symbol
	snapshot.SignalBars[timeframe] = signals
	snapshot.UpdatedAt = eventTime.UTC()
	p.liveMarketData[symbol] = snapshot
	p.liveMarketMu.Unlock()
	return nil
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

func liveSignalResolution(timeframe string) string {
	switch normalizeSignalBarInterval(timeframe) {
	case "5m":
		return "5"
	case "15m":
		return "15"
	case "30m":
		return "30"
	case "1d":
		return "1D"
	case "4h":
		return "240"
	default:
		return ""
	}
}

func candleBarsToMarketBars(exchange, symbol, timeframe string, bars []candleBar, source string) []domain.MarketBar {
	step := resolutionToDuration(liveSignalResolution(timeframe))
	if step <= 0 {
		return nil
	}
	items := make([]domain.MarketBar, 0, len(bars))
	now := time.Now().UTC()
	for _, bar := range bars {
		openTime := bar.Time.UTC()
		items = append(items, domain.MarketBar{
			ID:        fmt.Sprintf("%s|%s|%s|%s", strings.ToUpper(strings.TrimSpace(exchange)), NormalizeSymbol(symbol), strings.ToLower(strings.TrimSpace(timeframe)), openTime.Format(time.RFC3339)),
			Exchange:  strings.ToUpper(strings.TrimSpace(exchange)),
			Symbol:    NormalizeSymbol(symbol),
			Timeframe: strings.ToLower(strings.TrimSpace(timeframe)),
			OpenTime:  openTime,
			CloseTime: openTime.Add(step),
			Open:      bar.Open,
			High:      bar.High,
			Low:       bar.Low,
			Close:     bar.Close,
			Volume:    bar.Volume,
			IsClosed:  true,
			Source:    source,
			UpdatedAt: now,
		})
	}
	return items
}

func marketBarsToCandles(items []domain.MarketBar) []candleBar {
	out := make([]candleBar, 0, len(items))
	for _, item := range items {
		out = append(out, candleBar{
			Time:   item.OpenTime.UTC(),
			Open:   item.Open,
			High:   item.High,
			Low:    item.Low,
			Close:  item.Close,
			Volume: item.Volume,
		})
	}
	return out
}

func buildStrategySignalBarsFromCandles(bars []candleBar) ([]strategySignalBar, error) {
	if len(bars) == 0 {
		return nil, fmt.Errorf("no signal bars available")
	}
	signals := make([]strategySignalBar, len(bars))
	closes := make([]float64, len(bars))
	trueRanges := make([]float64, len(bars))
	for i, bar := range bars {
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
			highClose := math.Abs(bar.High - bars[i-1].Close)
			lowClose := math.Abs(bar.Low - bars[i-1].Close)
			trueRanges[i] = math.Max(highLow, math.Max(highClose, lowClose))
		}
	}
	for i := range signals {
		signals[i].MA5 = rollingMean(closes, i, 5)
		signals[i].MA20 = rollingMean(closes, i, 20)
		signals[i].ATR = rollingMean(trueRanges, i, 14)
		signals[i].ATRPercentile = rollingLastPercentileFromSeries(trueRanges, i, 14, 240, 50)
		if i >= 1 {
			signals[i].PrevHigh1 = bars[i-1].High
			signals[i].PrevLow1 = bars[i-1].Low
		} else {
			signals[i].PrevHigh1 = math.NaN()
			signals[i].PrevLow1 = math.NaN()
		}
		if i >= 2 {
			signals[i].PrevHigh2 = bars[i-2].High
			signals[i].PrevLow2 = bars[i-2].Low
		} else {
			signals[i].PrevHigh2 = math.NaN()
			signals[i].PrevLow2 = math.NaN()
		}
		if i >= 3 {
			signals[i].PrevHigh3 = bars[i-3].High
			signals[i].PrevLow3 = bars[i-3].Low
		} else {
			signals[i].PrevHigh3 = math.NaN()
			signals[i].PrevLow3 = math.NaN()
		}
	}
	return signals, nil
}

func parseUnixMillisTime(value any) time.Time {
	raw := strings.TrimSpace(stringValue(value))
	if raw == "" {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(ms).UTC()
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
	replayExecutionSource := cfg.ExecutionDataSource
	if replayExecutionSource == "tick" {
		// Live sessions still evaluate and trigger on real-time tick events, but the
		// precomputed execution plan is built from the warmed minute cache because we
		// do not keep a live tick archive in memory.
		replayExecutionSource = "1min"
	}
	if replayExecutionSource != "1min" {
		return nil, fmt.Errorf("live market execution source not supported yet: %s", cfg.ExecutionDataSource)
	}
	cfg.ExecutionDataSource = replayExecutionSource

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
	case "bk-default", "bk-live-intrabar-sma5-t3-sep":
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
