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

type executionDatasetDescriptor struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func (p *Platform) runBacktestSkeleton(backtest domain.BacktestRun) domain.BacktestRun {
	executionSource := stringValue(backtest.Parameters["executionDataSource"])
	signalTimeframe := stringValue(backtest.Parameters["signalTimeframe"])
	symbol := stringValue(backtest.Parameters["symbol"])

	summary, err := p.loadExecutionDatasetSummary(executionSource, symbol)
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

func (p *Platform) loadExecutionDatasetSummary(executionSource, symbol string) (executionDatasetSummary, error) {
	switch strings.ToLower(strings.TrimSpace(executionSource)) {
	case "1min":
		return summarizeCSVExecutionData(p.minuteDataDir, []string{
			fmt.Sprintf("%s_1min_Clean.csv", normalizeSymbolForDataset(symbol)),
			"BTC_1min_Clean.csv",
		}, "timestamp")
	case "tick":
		return summarizeCSVExecutionData(p.tickDataDir, []string{
			fmt.Sprintf("%s_tick_Clean.csv", normalizeSymbolForDataset(symbol)),
			fmt.Sprintf("%s_tick.csv", normalizeSymbolForDataset(symbol)),
			"BTC_tick_Clean.csv",
			"BTC_tick.csv",
		}, "timestamp")
	default:
		return executionDatasetSummary{}, fmt.Errorf("unsupported execution data source: %s", executionSource)
	}
}

func (p *Platform) discoverExecutionDatasets(executionSource string) []executionDatasetDescriptor {
	var candidates []string
	switch strings.ToLower(strings.TrimSpace(executionSource)) {
	case "1min":
		candidates = []string{"*_1min_Clean.csv", "*_1min.csv"}
	case "tick":
		candidates = []string{"*_tick_Clean.csv", "*_tick.csv"}
	default:
		return nil
	}

	baseDir := p.minuteDataDir
	if strings.EqualFold(strings.TrimSpace(executionSource), "tick") {
		baseDir = p.tickDataDir
	}

	return discoverMatchingDatasets(baseDir, candidates)
}

func summarizeCSVExecutionData(baseDir string, candidates []string, timeColumn string) (executionDatasetSummary, error) {
	resolved, err := resolveExistingDataset(baseDir, candidates)
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

func resolveExistingDataset(baseDir string, candidates []string) (string, error) {
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
		searchRoots := []string{}
		if strings.TrimSpace(baseDir) != "" {
			searchRoots = append(searchRoots, baseDir)
		}
		searchRoots = append(searchRoots, root)

		for _, searchRoot := range searchRoots {
			path := candidate
			if !filepath.IsAbs(path) {
				if filepath.IsAbs(searchRoot) {
					path = filepath.Join(searchRoot, candidate)
				} else {
					path = filepath.Join(root, searchRoot, candidate)
				}
			}
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
	}
	return "", fmt.Errorf("execution dataset not found in %q, tried: %s", baseDir, strings.Join(uniqueCandidates, ", "))
}

func discoverMatchingDatasets(baseDir string, patterns []string) []executionDatasetDescriptor {
	searchRoots := resolveSearchRoots(baseDir)
	seen := map[string]struct{}{}
	items := make([]executionDatasetDescriptor, 0)

	for _, searchRoot := range searchRoots {
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(searchRoot, pattern))
			if err != nil {
				continue
			}
			for _, match := range matches {
				absPath, err := filepath.Abs(match)
				if err != nil {
					continue
				}
				if _, ok := seen[absPath]; ok {
					continue
				}
				info, err := os.Stat(absPath)
				if err != nil || info.IsDir() {
					continue
				}
				seen[absPath] = struct{}{}
				items = append(items, executionDatasetDescriptor{
					Name: filepath.Base(absPath),
					Path: absPath,
				})
			}
		}
	}

	return items
}

func resolveSearchRoots(baseDir string) []string {
	_, currentFile, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(currentFile), "..", "..")
	searchRoots := make([]string, 0, 2)

	if strings.TrimSpace(baseDir) != "" {
		if filepath.IsAbs(baseDir) {
			searchRoots = append(searchRoots, filepath.Clean(baseDir))
		} else {
			searchRoots = append(searchRoots, filepath.Join(root, baseDir))
		}
	}

	searchRoots = append(searchRoots, root)
	seen := map[string]struct{}{}
	unique := make([]string, 0, len(searchRoots))
	for _, path := range searchRoots {
		cleaned := filepath.Clean(path)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		unique = append(unique, cleaned)
	}
	return unique
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
