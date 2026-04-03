package service

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type executionDatasetSummary struct {
	SourcePath string
	Records    int
	StartTime  string
	EndTime    string
}

func (p *Platform) runBacktestSkeleton(backtest domain.BacktestRun) domain.BacktestRun {
	executionSource := stringValue(backtest.Parameters["executionDataSource"])
	signalTimeframe := stringValue(backtest.Parameters["signalTimeframe"])
	symbol := stringValue(backtest.Parameters["symbol"])

	summary, err := loadExecutionDatasetSummary(executionSource, symbol)
	if err != nil {
		backtest.Status = "FAILED"
		backtest.ResultSummary = map[string]any{
			"return":              0,
			"maxDrawdown":         0,
			"tradePairs":          0,
			"signalTimeframe":     signalTimeframe,
			"executionDataSource": executionSource,
			"symbol":              symbol,
			"error":               err.Error(),
		}
		return backtest
	}

	backtest.Status = "COMPLETED"
	backtest.ResultSummary = map[string]any{
		"return":                0,
		"maxDrawdown":           0,
		"tradePairs":            0,
		"signalTimeframe":       signalTimeframe,
		"executionDataSource":   executionSource,
		"symbol":                symbol,
		"executionDatasetPath":  summary.SourcePath,
		"executionDatasetRows":  summary.Records,
		"executionDatasetStart": summary.StartTime,
		"executionDatasetEnd":   summary.EndTime,
		"runnerMode":            "skeleton",
	}
	return backtest
}

func loadExecutionDatasetSummary(executionSource, symbol string) (executionDatasetSummary, error) {
	switch strings.ToLower(strings.TrimSpace(executionSource)) {
	case "1min":
		return summarizeCSVExecutionData([]string{
			fmt.Sprintf("%s_1min_Clean.csv", normalizeSymbolForDataset(symbol)),
			"BTC_1min_Clean.csv",
		}, "timestamp")
	case "tick":
		return summarizeCSVExecutionData([]string{
			fmt.Sprintf("%s_tick_Clean.csv", normalizeSymbolForDataset(symbol)),
			fmt.Sprintf("%s_tick.csv", normalizeSymbolForDataset(symbol)),
			"BTC_tick_Clean.csv",
			"BTC_tick.csv",
		}, "timestamp")
	default:
		return executionDatasetSummary{}, fmt.Errorf("unsupported execution data source: %s", executionSource)
	}
}

func summarizeCSVExecutionData(candidates []string, timeColumn string) (executionDatasetSummary, error) {
	resolved, err := resolveExistingDataset(candidates)
	if err != nil {
		return executionDatasetSummary{}, err
	}

	file, err := os.Open(filepath.Clean(resolved))
	if err != nil {
		return executionDatasetSummary{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	header, err := reader.Read()
	if err != nil {
		return executionDatasetSummary{}, err
	}

	timeIndex := -1
	for idx, item := range header {
		if strings.EqualFold(strings.TrimSpace(item), timeColumn) {
			timeIndex = idx
			break
		}
	}
	if timeIndex < 0 {
		return executionDatasetSummary{}, fmt.Errorf("time column %s not found in %s", timeColumn, resolved)
	}

	count := 0
	firstTime := ""
	lastTime := ""
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return executionDatasetSummary{}, err
		}
		if timeIndex >= len(row) {
			continue
		}
		timestamp := strings.TrimSpace(row[timeIndex])
		if timestamp == "" {
			continue
		}
		if count == 0 {
			firstTime = timestamp
		}
		lastTime = timestamp
		count++
	}

	if count == 0 {
		return executionDatasetSummary{}, fmt.Errorf("dataset %s contains no rows", resolved)
	}

	return executionDatasetSummary{
		SourcePath: resolved,
		Records:    count,
		StartTime:  firstTime,
		EndTime:    lastTime,
	}, nil
}

func resolveExistingDataset(candidates []string) (string, error) {
	_, currentFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(currentFile), "..", "..")
	seen := map[string]struct{}{}
	uniqueCandidates := make([]string, 0, len(candidates))

	for _, candidate := range candidates {
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		uniqueCandidates = append(uniqueCandidates, candidate)
	}

	for _, candidate := range uniqueCandidates {
		path := candidate
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, candidate)
		}
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("execution dataset not found, tried: %s", strings.Join(uniqueCandidates, ", "))
}

func normalizeSymbolForDataset(symbol string) string {
	normalized := strings.ToUpper(strings.TrimSpace(symbol))
	if strings.HasSuffix(normalized, "USDT") {
		normalized = strings.TrimSuffix(normalized, "USDT")
	}
	return normalized
}

func parseBacktestPercent(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case string:
		parsed, _ := strconv.ParseFloat(v, 64)
		return parsed
	default:
		return 0
	}
}

func parseBacktestTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339, value)
	return parsed
}
