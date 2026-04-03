package service

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
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
	Name       string `json:"name"`
	Path       string `json:"path"`
	Symbol     string `json:"symbol"`
	Format     string `json:"format,omitempty"`
	FileCount  int    `json:"fileCount,omitempty"`
	TimeColumn string `json:"timeColumn,omitempty"`
}

type tradeArchiveManifestEntry struct {
	Symbol     string
	Month      string
	Directory  string
	FilePath   string
	StartTime  string
	EndTime    string
	RowFormat  string
	TimeColumn string
}

type tickArchiveIterator struct {
	files    []tradeArchiveManifestEntry
	current  *os.File
	reader   *csv.Reader
	index    int
	currentF tradeArchiveManifestEntry
}

type tickEvent struct {
	Symbol   string
	Time     time.Time
	Price    float64
	Quantity float64
	Side     string
	Source   string
	TradeID  string
	IsMaker  bool
	IsBest   bool
	Raw      []string
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
		if summary, ok, err := p.summarizeTradeArchiveExecutionData(symbol); ok || err != nil {
			return summary, err
		}
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

func (p *Platform) hasExecutionDataset(executionSource, symbol string) bool {
	datasets := p.discoverExecutionDatasets(executionSource)
	normalizedSymbol := normalizeBacktestSymbol(symbol)
	for _, dataset := range datasets {
		if dataset.Symbol == normalizedSymbol {
			return true
		}
	}
	return false
}

func (p *Platform) discoverExecutionDatasets(executionSource string) []executionDatasetDescriptor {
	var candidates []string
	switch strings.ToLower(strings.TrimSpace(executionSource)) {
	case "1min":
		candidates = []string{"*_1min_Clean.csv", "*_1min.csv"}
	case "tick":
		items := p.discoverTradeArchiveDatasets()
		items = append(items, discoverMatchingDatasets(p.tickDataDir, []string{"*_tick_Clean.csv", "*_tick.csv"})...)
		return dedupeDatasetDescriptors(items)
	default:
		return nil
	}

	baseDir := p.minuteDataDir
	if strings.EqualFold(strings.TrimSpace(executionSource), "tick") {
		baseDir = p.tickDataDir
	}

	return discoverMatchingDatasets(baseDir, candidates)
}

func (p *Platform) summarizeTradeArchiveExecutionData(symbol string) (executionDatasetSummary, bool, error) {
	datasets := p.discoverTradeArchiveDatasets()
	normalizedSymbol := normalizeBacktestSymbol(symbol)
	for _, dataset := range datasets {
		if dataset.Symbol != normalizedSymbol {
			continue
		}

		files, err := p.collectTradeArchiveFiles(normalizedSymbol)
		if err != nil {
			return executionDatasetSummary{}, true, err
		}
		if len(files) == 0 {
			return executionDatasetSummary{}, true, fmt.Errorf("no tick archive files found for %s in %s", normalizedSymbol, dataset.Path)
		}

		return executionDatasetSummary{
			SourcePath: dataset.Path,
			Records:    len(files),
			StartTime:  files[0].StartTime,
			EndTime:    files[len(files)-1].EndTime,
		}, true, nil
	}
	return executionDatasetSummary{}, false, nil
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
					Name:       filepath.Base(absPath),
					Path:       absPath,
					Symbol:     extractDatasetSymbol(filepath.Base(absPath)),
					Format:     "flat_csv",
					TimeColumn: "timestamp",
				})
			}
		}
	}

	return items
}

func (p *Platform) discoverTradeArchiveDatasets() []executionDatasetDescriptor {
	manifest, err := p.getTradeArchiveManifest()
	if err != nil {
		return nil
	}
	grouped := map[string][]tradeArchiveManifestEntry{}
	for _, entry := range manifest {
		grouped[entry.Symbol] = append(grouped[entry.Symbol], entry)
	}
	items := make([]executionDatasetDescriptor, 0, len(grouped))
	for symbol, entries := range grouped {
		sort.Slice(entries, func(i, j int) bool { return entries[i].Month < entries[j].Month })
		items = append(items, executionDatasetDescriptor{
			Name:       fmt.Sprintf("%s trade archive", symbol),
			Path:       p.tickDataDir,
			Symbol:     symbol,
			Format:     "binance_monthly_trade_archive",
			FileCount:  len(entries),
			TimeColumn: "time_ms",
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Symbol < items[j].Symbol })
	return items
}

func dedupeDatasetDescriptors(items []executionDatasetDescriptor) []executionDatasetDescriptor {
	seen := map[string]struct{}{}
	out := make([]executionDatasetDescriptor, 0, len(items))
	for _, item := range items {
		key := item.Symbol + "|" + item.Path + "|" + item.Format + "|" + item.Name
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, item)
	}
	return out
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
	normalized := normalizeBacktestSymbol(symbol)
	if strings.HasSuffix(normalized, "USDT") {
		return strings.TrimSuffix(normalized, "USDT")
	}
	return normalized
}

func normalizeBacktestSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func extractDatasetSymbol(filename string) string {
	base := strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename))
	parts := strings.Split(base, "_")
	if len(parts) == 0 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(parts[0])) + "USDT"
}

func extractArchiveSymbol(dirname string) string {
	base := strings.TrimSpace(dirname)
	if base == "" {
		return ""
	}
	parts := strings.Split(base, "-trades-")
	if len(parts) < 2 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(parts[0]))
}

func extractArchiveMonth(dirname string) string {
	base := strings.TrimSpace(dirname)
	parts := strings.Split(base, "-trades-")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (p *Platform) collectTradeArchiveFiles(symbol string) ([]tradeArchiveManifestEntry, error) {
	manifest, err := p.getTradeArchiveManifest()
	if err != nil {
		return nil, err
	}
	out := make([]tradeArchiveManifestEntry, 0)
	for _, entry := range manifest {
		if entry.Symbol == symbol {
			out = append(out, entry)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Month < out[j].Month })
	return out, nil
}

func firstTradeArchiveTimestamp(path string) (string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	row, err := reader.Read()
	if err != nil {
		return "", err
	}
	return parseTradeArchiveTime(row)
}

func lastTradeArchiveTimestamp(path string) (string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	const maxCapacity = 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxCapacity)
	last := ""
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			last = line
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if last == "" {
		return "", fmt.Errorf("empty trade archive file: %s", path)
	}
	return parseTradeArchiveLineTime(last)
}

func parseTradeArchiveLineTime(line string) (string, error) {
	reader := csv.NewReader(strings.NewReader(line))
	row, err := reader.Read()
	if err != nil {
		return "", err
	}
	return parseTradeArchiveTime(row)
}

func parseTradeArchiveTime(row []string) (string, error) {
	if len(row) < 5 {
		return "", fmt.Errorf("trade archive row has insufficient columns")
	}
	value, err := parseTradeArchiveTimeValue(row[4])
	if err != nil {
		return "", err
	}
	return value.UTC().Format(time.RFC3339), nil
}

func parseTradeArchiveTimeValue(value string) (time.Time, error) {
	ms := strings.TrimSpace(value)
	parsed, err := strconv.ParseInt(ms, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(parsed).UTC(), nil
}

func dedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func archiveMonthRange(month string) (string, string, error) {
	start, err := time.Parse("2006-01", month)
	if err != nil {
		return "", "", err
	}
	end := start.AddDate(0, 1, 0).Add(-time.Millisecond)
	return start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339), nil
}

func (p *Platform) getTradeArchiveManifest() ([]tradeArchiveManifestEntry, error) {
	p.manifestMu.Lock()
	defer p.manifestMu.Unlock()
	if p.tickManifest != nil {
		return append([]tradeArchiveManifestEntry(nil), p.tickManifest...), nil
	}
	manifest, err := buildTradeArchiveManifest(p.tickDataDir)
	if err != nil {
		return nil, err
	}
	p.tickManifest = manifest
	return append([]tradeArchiveManifestEntry(nil), p.tickManifest...), nil
}

func buildTradeArchiveManifest(baseDir string) ([]tradeArchiveManifestEntry, error) {
	searchRoots := resolveSearchRoots(baseDir)
	entries := make([]tradeArchiveManifestEntry, 0)
	seen := map[string]struct{}{}
	for _, searchRoot := range searchRoots {
		matches, err := filepath.Glob(filepath.Join(searchRoot, "*-trades-????-??", "*.csv"))
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			absPath, err := filepath.Abs(match)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[absPath]; ok {
				continue
			}
			seen[absPath] = struct{}{}
			dirName := filepath.Base(filepath.Dir(absPath))
			symbol := extractArchiveSymbol(dirName)
			if symbol == "" {
				continue
			}
			month := extractArchiveMonth(dirName)
			startTime, endTime, err := archiveMonthRange(month)
			if err != nil {
				return nil, err
			}
			entries = append(entries, tradeArchiveManifestEntry{
				Symbol:     symbol,
				Month:      month,
				Directory:  filepath.Dir(absPath),
				FilePath:   absPath,
				StartTime:  startTime,
				EndTime:    endTime,
				RowFormat:  "binance_monthly_trade_archive",
				TimeColumn: "time_ms",
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Symbol == entries[j].Symbol {
			return entries[i].Month < entries[j].Month
		}
		return entries[i].Symbol < entries[j].Symbol
	})
	return entries, nil
}

func (p *Platform) newTickArchiveIterator(symbol string) (*tickArchiveIterator, error) {
	files, err := p.collectTradeArchiveFiles(normalizeBacktestSymbol(symbol))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no tick archive files found for %s", normalizeBacktestSymbol(symbol))
	}
	return &tickArchiveIterator{files: files}, nil
}

func (it *tickArchiveIterator) Next() (tickEvent, error) {
	for {
		if it.reader == nil {
			if it.index >= len(it.files) {
				return tickEvent{}, io.EOF
			}
			entry := it.files[it.index]
			file, err := os.Open(filepath.Clean(entry.FilePath))
			if err != nil {
				return tickEvent{}, err
			}
			it.current = file
			it.reader = csv.NewReader(file)
			it.currentF = entry
		}

		row, err := it.reader.Read()
		if err == io.EOF {
			_ = it.current.Close()
			it.current = nil
			it.reader = nil
			it.index++
			continue
		}
		if err != nil {
			return tickEvent{}, err
		}
		return parseTradeArchiveEvent(it.currentF.Symbol, row)
	}
}

func (it *tickArchiveIterator) Close() error {
	if it.current != nil {
		err := it.current.Close()
		it.current = nil
		it.reader = nil
		return err
	}
	return nil
}

func parseTradeArchiveEvent(symbol string, row []string) (tickEvent, error) {
	if len(row) < 7 {
		return tickEvent{}, fmt.Errorf("trade archive row has insufficient columns")
	}
	tradeTime, err := parseTradeArchiveTimeValue(row[4])
	if err != nil {
		return tickEvent{}, err
	}
	price, err := strconv.ParseFloat(strings.TrimSpace(row[1]), 64)
	if err != nil {
		return tickEvent{}, err
	}
	qty, _ := strconv.ParseFloat(strings.TrimSpace(row[2]), 64)
	isMaker, _ := strconv.ParseBool(strings.TrimSpace(row[5]))
	isBest, _ := strconv.ParseBool(strings.TrimSpace(row[6]))
	side := "buy"
	if isMaker {
		side = "sell"
	}
	return tickEvent{
		Symbol:   symbol,
		Time:     tradeTime,
		Price:    price,
		Quantity: qty,
		Side:     side,
		Source:   "tick_archive",
		TradeID:  strings.TrimSpace(row[0]),
		IsMaker:  isMaker,
		IsBest:   isBest,
		Raw:      row,
	}, nil
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
