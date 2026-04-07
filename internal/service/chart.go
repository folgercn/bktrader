package service

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type candleBar struct {
	Time   time.Time
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

func (p *Platform) ListAnnotations(symbol string, from, to int64, limit int) []domain.ChartAnnotation {
	items := make([]domain.ChartAnnotation, 0, 256)

	if ledger, err := p.loadReplayLedger(); err == nil {
		for index, event := range ledger {
			annotation, ok := replayEventAnnotation(symbol, index, event)
			if ok {
				items = append(items, annotation)
			}
		}
	}

	orders, err := p.store.ListOrders()
	if err == nil {
		for _, order := range orders {
			annotation, ok := orderAnnotation(symbol, order)
			if ok {
				items = append(items, annotation)
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Time.Before(items[j].Time)
	})
	start := time.Time{}
	end := time.Time{}
	if from > 0 {
		start = time.Unix(from, 0).UTC()
	}
	if to > 0 {
		end = time.Unix(to, 0).UTC()
	}
	if !start.IsZero() || !end.IsZero() {
		filtered := make([]domain.ChartAnnotation, 0, len(items))
		for _, item := range items {
			if !start.IsZero() && item.Time.Before(start) {
				continue
			}
			if !end.IsZero() && item.Time.After(end) {
				continue
			}
			filtered = append(filtered, item)
		}
		items = filtered
	}
	if limit > 0 && len(items) > limit {
		items = items[len(items)-limit:]
	}
	return items
}

func (p *Platform) CandleSeries(symbol string, resolution string, from int64, to int64, limit int) ([]map[string]any, error) {
	if limit <= 0 {
		limit = 400
	}
	if resolution == "" {
		resolution = "1"
	}
	remoteBars, err := fetchBinanceFuturesCandles(symbol, resolution, from, to, limit)
	if err != nil {
		return nil, err
	}
	if len(remoteBars) == 0 {
		return nil, fmt.Errorf("no binance candles returned for %s %s", NormalizeSymbol(symbol), resolution)
	}
	return candleSeriesFromBars(symbol, resolution, remoteBars, limit), nil
}

func (p *Platform) CandleIndicators(symbol string, resolution string, from int64, to int64, limit int) (map[string]any, error) {
	if limit <= 0 {
		limit = 400
	}
	bars, err := fetchBinanceFuturesCandles(symbol, resolution, from, to, limit)
	if err != nil {
		return nil, err
	}
	if len(bars) == 0 {
		return nil, fmt.Errorf("no binance candles returned for %s %s", NormalizeSymbol(symbol), resolution)
	}
	return map[string]any{
		"symbol":     NormalizeSymbol(symbol),
		"resolution": resolution,
		"count":      len(bars),
		"sma7":       buildLineIndicator(bars, 7, smaValue),
		"sma20":      buildLineIndicator(bars, 20, smaValue),
		"ema7":       buildEMAIndicator(bars, 7),
		"ema20":      buildEMAIndicator(bars, 20),
		"atr14":      buildATRIndicator(bars, 14),
		"rsi14":      buildRSIIndicator(bars, 14),
		"vma20":      buildLineIndicator(bars, 20, volumeMAValue),
	}, nil
}

func candleSeriesFromBars(symbol, resolution string, bars []candleBar, limit int) []map[string]any {
	if len(bars) > limit && limit > 0 {
		bars = bars[len(bars)-limit:]
	}
	series := make([]map[string]any, 0, len(bars))
	for _, bar := range bars {
		series = append(series, map[string]any{
			"symbol":     NormalizeSymbol(symbol),
			"resolution": resolution,
			"time":       bar.Time,
			"open":       round2(bar.Open),
			"high":       round2(bar.High),
			"low":        round2(bar.Low),
			"close":      round2(bar.Close),
			"volume":     round2(bar.Volume),
		})
	}
	return series
}

func fetchBinanceFuturesCandles(symbol string, resolution string, from int64, to int64, limit int) ([]candleBar, error) {
	interval := resolutionToBinanceInterval(resolution)
	if interval == "" {
		return nil, fmt.Errorf("unsupported resolution: %s", resolution)
	}
	endpoint := os.Getenv("BINANCE_FUTURES_KLINE_BASE_URL")
	if strings.TrimSpace(endpoint) == "" {
		endpoint = "https://fapi.binance.com"
	}
	baseURL := strings.TrimRight(endpoint, "/") + "/fapi/v1/klines"
	params := url.Values{}
	params.Set("symbol", NormalizeSymbol(symbol))
	params.Set("interval", interval)
	if limit > 0 {
		if limit > 1500 {
			limit = 1500
		}
		params.Set("limit", strconv.Itoa(limit))
	}
	if from > 0 {
		params.Set("startTime", strconv.FormatInt(from*1000, 10))
	}
	if to > 0 {
		params.Set("endTime", strconv.FormatInt(to*1000, 10))
	}
	resp, err := http.Get(baseURL + "?" + params.Encode())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("binance klines request failed: %s", resp.Status)
	}
	var payload [][]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}
	bars := make([]candleBar, 0, len(payload))
	for _, row := range payload {
		if len(row) < 6 {
			continue
		}
		openTime, ok := toInt64(row[0])
		if !ok {
			continue
		}
		open, _ := toFloat64(row[1])
		high, _ := toFloat64(row[2])
		low, _ := toFloat64(row[3])
		closePrice, _ := toFloat64(row[4])
		volume, _ := toFloat64(row[5])
		bars = append(bars, candleBar{
			Time:   time.UnixMilli(openTime).UTC(),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closePrice,
			Volume: volume,
		})
	}
	return bars, nil
}

func buildLineIndicator(bars []candleBar, period int, valueFn func([]candleBar) float64) []map[string]any {
	if len(bars) == 0 || period <= 0 {
		return []map[string]any{}
	}
	items := make([]map[string]any, 0, len(bars))
	for idx := range bars {
		if idx+1 < period {
			continue
		}
		window := bars[idx+1-period : idx+1]
		items = append(items, map[string]any{
			"time":  bars[idx].Time,
			"value": round2(valueFn(window)),
		})
	}
	return items
}

func buildEMAIndicator(bars []candleBar, period int) []map[string]any {
	if len(bars) == 0 || period <= 0 || len(bars) < period {
		return []map[string]any{}
	}
	items := make([]map[string]any, 0, len(bars)-period+1)
	multiplier := 2.0 / float64(period+1)
	ema := smaValue(bars[:period])
	items = append(items, map[string]any{"time": bars[period-1].Time, "value": round2(ema)})
	for idx := period; idx < len(bars); idx++ {
		ema = ((bars[idx].Close - ema) * multiplier) + ema
		items = append(items, map[string]any{"time": bars[idx].Time, "value": round2(ema)})
	}
	return items
}

func buildATRIndicator(bars []candleBar, period int) []map[string]any {
	if len(bars) < period+1 || period <= 0 {
		return []map[string]any{}
	}
	trs := make([]float64, 0, len(bars)-1)
	for idx := 1; idx < len(bars); idx++ {
		current := bars[idx]
		prev := bars[idx-1]
		tr := maxFloat(current.High-current.Low, maxFloat(absoluteFloat(current.High-prev.Close), absoluteFloat(current.Low-prev.Close)))
		trs = append(trs, tr)
	}
	items := make([]map[string]any, 0, len(trs)-period+1)
	atr := averageFloat(trs[:period])
	items = append(items, map[string]any{"time": bars[period].Time, "value": round2(atr)})
	for idx := period; idx < len(trs); idx++ {
		atr = ((atr * float64(period-1)) + trs[idx]) / float64(period)
		items = append(items, map[string]any{"time": bars[idx+1].Time, "value": round2(atr)})
	}
	return items
}

func buildRSIIndicator(bars []candleBar, period int) []map[string]any {
	if len(bars) < period+1 || period <= 0 {
		return []map[string]any{}
	}
	gains := make([]float64, 0, len(bars)-1)
	losses := make([]float64, 0, len(bars)-1)
	for idx := 1; idx < len(bars); idx++ {
		delta := bars[idx].Close - bars[idx-1].Close
		if delta >= 0 {
			gains = append(gains, delta)
			losses = append(losses, 0)
		} else {
			gains = append(gains, 0)
			losses = append(losses, -delta)
		}
	}
	avgGain := averageFloat(gains[:period])
	avgLoss := averageFloat(losses[:period])
	items := make([]map[string]any, 0, len(gains)-period+1)
	items = append(items, map[string]any{"time": bars[period].Time, "value": round2(computeRSI(avgGain, avgLoss))})
	for idx := period; idx < len(gains); idx++ {
		avgGain = ((avgGain * float64(period-1)) + gains[idx]) / float64(period)
		avgLoss = ((avgLoss * float64(period-1)) + losses[idx]) / float64(period)
		items = append(items, map[string]any{"time": bars[idx+1].Time, "value": round2(computeRSI(avgGain, avgLoss))})
	}
	return items
}

func smaValue(window []candleBar) float64 {
	total := 0.0
	for _, bar := range window {
		total += bar.Close
	}
	return total / float64(len(window))
}

func volumeMAValue(window []candleBar) float64 {
	total := 0.0
	for _, bar := range window {
		total += bar.Volume
	}
	return total / float64(len(window))
}

func averageFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func computeRSI(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}

func absoluteFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func resolutionToBinanceInterval(resolution string) string {
	switch strings.ToUpper(strings.TrimSpace(resolution)) {
	case "1":
		return "1m"
	case "3":
		return "3m"
	case "5":
		return "5m"
	case "15":
		return "15m"
	case "30":
		return "30m"
	case "60":
		return "1h"
	case "120":
		return "2h"
	case "240":
		return "4h"
	case "1D", "D":
		return "1d"
	default:
		return ""
	}
}

func toInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case float64:
		return int64(v), true
	case int64:
		return v, true
	case int:
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (p *Platform) loadCandleBars() ([]candleBar, error) {
	p.candleOnce.Do(func() {
		p.candles, p.candleErr = readOneMinuteCandles("BTC_1min_Clean.csv")
	})
	return p.candles, p.candleErr
}

func readOneMinuteCandles(path string) ([]candleBar, error) {
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
		return nil, fmt.Errorf("candle csv is empty: %s", resolved)
	}

	bars := make([]candleBar, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}
		ts, err := time.Parse("2006-01-02 15:04:05Z07:00", row[0])
		if err != nil {
			return nil, fmt.Errorf("parse candle time %q: %w", row[0], err)
		}
		open, err := strconv.ParseFloat(row[1], 64)
		if err != nil {
			return nil, err
		}
		high, err := strconv.ParseFloat(row[2], 64)
		if err != nil {
			return nil, err
		}
		low, err := strconv.ParseFloat(row[3], 64)
		if err != nil {
			return nil, err
		}
		closeValue, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, err
		}
		volume, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, err
		}
		bars = append(bars, candleBar{
			Time:   ts.UTC(),
			Open:   open,
			High:   high,
			Low:    low,
			Close:  closeValue,
			Volume: volume,
		})
	}

	return bars, nil
}

func filterCandleBars(bars []candleBar, start, end time.Time) []candleBar {
	filtered := make([]candleBar, 0, len(bars))
	for _, bar := range bars {
		if bar.Time.Before(start) || bar.Time.After(end) {
			continue
		}
		filtered = append(filtered, bar)
	}
	return filtered
}

func aggregateCandleBars(bars []candleBar, resolution string, step time.Duration) []candleBar {
	if len(bars) == 0 {
		return bars
	}

	aggregated := make([]candleBar, 0, len(bars))
	var current candleBar
	var bucketStart time.Time
	initialized := false

	for _, bar := range bars {
		nextBucket := candleBucketStart(bar.Time, resolution, step)
		if !initialized || !nextBucket.Equal(bucketStart) {
			if initialized {
				aggregated = append(aggregated, current)
			}
			current = candleBar{
				Time:   nextBucket,
				Open:   bar.Open,
				High:   bar.High,
				Low:    bar.Low,
				Close:  bar.Close,
				Volume: bar.Volume,
			}
			bucketStart = nextBucket
			initialized = true
			continue
		}
		current.High = maxFloat(current.High, bar.High)
		current.Low = minFloat(current.Low, bar.Low)
		current.Close = bar.Close
		current.Volume += bar.Volume
	}

	if initialized {
		aggregated = append(aggregated, current)
	}
	return aggregated
}

func candleBucketStart(ts time.Time, resolution string, step time.Duration) time.Time {
	if strings.EqualFold(resolution, "D") || strings.EqualFold(resolution, "1D") {
		year, month, day := ts.UTC().Date()
		return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	}
	unix := ts.UTC().Unix()
	seconds := int64(step / time.Second)
	if seconds <= 0 {
		return ts.UTC().Truncate(time.Minute)
	}
	return time.Unix(unix-(unix%seconds), 0).UTC()
}

func replayEventAnnotation(symbol string, index int, event strategyReplayEvent) (domain.ChartAnnotation, bool) {
	if NormalizeSymbol(symbol) != "BTCUSDT" {
		return domain.ChartAnnotation{}, false
	}

	annotationType := classifyAnnotationType(event.Type, event.Reason)

	return domain.ChartAnnotation{
		ID:     fmt.Sprintf("backtest-%d", index),
		Source: "backtest",
		Type:   annotationType,
		Symbol: "BTCUSDT",
		Time:   event.Time,
		Price:  event.Price,
		Label:  event.Reason,
		Metadata: map[string]any{
			"notional":       event.Notional,
			"balance":        event.Balance,
			"eventType":      event.Type,
			"reason":         event.Reason,
			"annotationType": annotationType,
		},
	}, true
}

func orderAnnotation(symbol string, order domain.Order) (domain.ChartAnnotation, bool) {
	if NormalizeSymbol(symbol) != NormalizeSymbol(order.Symbol) {
		return domain.ChartAnnotation{}, false
	}

	eventTime := order.CreatedAt
	if order.Metadata != nil {
		if raw, ok := order.Metadata["eventTime"].(string); ok && raw != "" {
			if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
				eventTime = parsed
			}
		}
	}

	label := strings.TrimSpace(order.Side)
	if reason, ok := order.Metadata["reason"].(string); ok && reason != "" {
		label = reason
	}
	reason := ""
	if order.Metadata != nil {
		if raw, ok := order.Metadata["reason"].(string); ok {
			reason = raw
		}
	}
	annotationType := classifyAnnotationType(order.Side, reason)

	source := "live"
	if order.Metadata != nil {
		if raw, ok := order.Metadata["source"].(string); ok && strings.Contains(raw, "paper") {
			source = "paper"
		}
	}

	return domain.ChartAnnotation{
		ID:     order.ID,
		Source: source,
		Type:   annotationType,
		Symbol: NormalizeSymbol(order.Symbol),
		Time:   eventTime.UTC(),
		Price:  order.Price,
		Label:  label,
		Metadata: map[string]any{
			"accountId":      order.AccountID,
			"liveSessionId":  order.Metadata["liveSessionId"],
			"paperSession":   order.Metadata["paperSession"],
			"strategyId":     order.Metadata["strategyId"],
			"orderStatus":    order.Status,
			"orderSide":      order.Side,
			"executionMode":  order.Metadata["executionMode"],
			"reason":         reason,
			"annotationType": annotationType,
		},
	}, true
}

func classifyAnnotationType(eventType, reason string) string {
	normalizedEventType := strings.ToUpper(strings.TrimSpace(eventType))
	normalizedReason := strings.ToUpper(strings.TrimSpace(reason))

	switch normalizedEventType {
	case "BUY":
		switch normalizedReason {
		case "INITIAL":
			return "entry-initial-long"
		case "PT-REENTRY":
			return "entry-pt-reentry-long"
		case "SL-REENTRY":
			return "entry-sl-reentry-long"
		default:
			return "entry-long"
		}
	case "SHORT", "SELL":
		if normalizedReason == "PT" || normalizedReason == "SL" {
			if normalizedReason == "PT" {
				return "exit-pt"
			}
			return "exit-sl"
		}
		switch normalizedReason {
		case "INITIAL":
			return "entry-initial-short"
		case "PT-REENTRY":
			return "entry-pt-reentry-short"
		case "SL-REENTRY":
			return "entry-sl-reentry-short"
		default:
			return "entry-short"
		}
	case "EXIT":
		switch normalizedReason {
		case "PT":
			return "exit-pt"
		case "SL":
			return "exit-sl"
		default:
			return "exit"
		}
	default:
		return "event"
	}
}

func resolutionToDuration(resolution string) time.Duration {
	switch resolution {
	case "1":
		return time.Minute
	case "5":
		return 5 * time.Minute
	case "15":
		return 15 * time.Minute
	case "60":
		return time.Hour
	case "240":
		return 4 * time.Hour
	case "1D", "D":
		return 24 * time.Hour
	default:
		if minutes, err := strconv.Atoi(resolution); err == nil && minutes > 0 {
			return time.Duration(minutes) * time.Minute
		}
		return 0
	}
}
