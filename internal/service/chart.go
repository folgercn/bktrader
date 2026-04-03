package service

import (
	"encoding/csv"
	"fmt"
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

func (p *Platform) CandleSeries(symbol string, resolution string, from int64, to int64, limit int) []map[string]any {
	if limit <= 0 {
		limit = 400
	}
	if resolution == "" {
		resolution = "1"
	}

	bars, err := p.loadCandleBars()
	if err != nil || len(bars) == 0 {
		return []map[string]any{}
	}

	step := resolutionToDuration(resolution)
	if step <= 0 {
		step = time.Minute
	}

	end := bars[len(bars)-1].Time
	if to > 0 {
		end = time.Unix(to, 0).UTC()
	}
	start := end.Add(-time.Duration(limit-1) * step)
	if from > 0 {
		start = time.Unix(from, 0).UTC()
	}
	if start.After(end) {
		start = end.Add(-time.Duration(limit-1) * step)
	}

	filtered := filterCandleBars(bars, start, end)
	if step > time.Minute {
		filtered = aggregateCandleBars(filtered, resolution, step)
	}
	if len(filtered) > limit {
		filtered = filtered[len(filtered)-limit:]
	}

	series := make([]map[string]any, 0, len(filtered))
	for _, bar := range filtered {
		series = append(series, map[string]any{
			"symbol":     symbol,
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
