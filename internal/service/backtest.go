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
	from     time.Time
	to       time.Time
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

type bracketPlan struct {
	Side            string
	EntryPrice      float64
	StopLossPrice   float64
	TakeProfitPrice float64
	Quantity        float64
}

type replayTrade struct {
	Entry strategyReplayEvent
	Exit  strategyReplayEvent
}

func (p *Platform) runBacktestSkeleton(backtest domain.BacktestRun) domain.BacktestRun {
	executionSource := stringValue(backtest.Parameters["executionDataSource"])
	signalTimeframe := stringValue(backtest.Parameters["signalTimeframe"])
	symbol := stringValue(backtest.Parameters["symbol"])
	from := stringValue(backtest.Parameters["from"])
	to := stringValue(backtest.Parameters["to"])

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

	resultSummary := map[string]any{
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
	if from != "" {
		resultSummary["rangeFrom"] = from
	}
	if to != "" {
		resultSummary["rangeTo"] = to
	}
	if len(parseBracketPlans(backtest.Parameters)) == 0 && !shouldReplayLedger(backtest.Parameters) {
		strategySummary, err := p.runStrategyReplay(backtest.Parameters)
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
		for key, value := range strategySummary {
			resultSummary[key] = value
		}
		backtest.Status = "COMPLETED"
		backtest.ResultSummary = resultSummary
		return backtest
	}
	if strings.EqualFold(executionSource, "tick") {
		preview, err := p.previewTickArchiveRange(symbol, from, to, 5000)
		if err != nil {
			backtest.Status = "FAILED"
			backtest.ResultSummary = map[string]any{
				"return":              0,
				"maxDrawdown":         0,
				"tradePairs":          0,
				"signalTimeframe":     signalTimeframe,
				"executionDataSource": executionSource,
				"symbol":              symbol,
				"rangeFrom":           from,
				"rangeTo":             to,
				"error":               err.Error(),
			}
			return backtest
		}
		for key, value := range preview {
			resultSummary[key] = value
		}
		if plans := parseBracketPlans(backtest.Parameters); len(plans) > 0 {
			sim, err := p.simulateTickBrackets(symbol, from, to, plans)
			if err != nil {
				resultSummary["bracketError"] = err.Error()
			} else {
				for key, value := range sim {
					resultSummary[key] = value
				}
			}
		}
		if shouldReplayLedger(backtest.Parameters) {
			replaySummary, err := p.simulateReplayLedgerOnTick(symbol, from, to)
			if err != nil {
				resultSummary["replayLedgerError"] = err.Error()
			} else {
				for key, value := range replaySummary {
					resultSummary[key] = value
				}
			}
		}
	} else if strings.EqualFold(executionSource, "1min") {
		if plans := parseBracketPlans(backtest.Parameters); len(plans) > 0 {
			sim, err := p.simulateMinuteBrackets(symbol, from, to, plans)
			if err != nil {
				resultSummary["bracketError"] = err.Error()
			} else {
				for key, value := range sim {
					resultSummary[key] = value
				}
			}
		}
	}
	backtest.Status = "COMPLETED"
	backtest.ResultSummary = resultSummary
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

func (p *Platform) collectTradeArchiveFilesInRange(symbol string, from, to time.Time) ([]tradeArchiveManifestEntry, error) {
	files, err := p.collectTradeArchiveFiles(symbol)
	if err != nil {
		return nil, err
	}
	if from.IsZero() && to.IsZero() {
		return files, nil
	}
	filtered := make([]tradeArchiveManifestEntry, 0, len(files))
	for _, entry := range files {
		entryStart, err := time.Parse(time.RFC3339, entry.StartTime)
		if err != nil {
			return nil, err
		}
		entryEnd, err := time.Parse(time.RFC3339, entry.EndTime)
		if err != nil {
			return nil, err
		}
		if !from.IsZero() && entryEnd.Before(from) {
			continue
		}
		if !to.IsZero() && entryStart.After(to) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered, nil
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

func (p *Platform) newTickArchiveIteratorForRange(symbol, from, to string) (*tickArchiveIterator, error) {
	fromTime := parseOptionalRFC3339(from)
	toTime := parseOptionalRFC3339(to)
	files, err := p.collectTradeArchiveFilesInRange(normalizeBacktestSymbol(symbol), fromTime, toTime)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no tick archive files found for %s in range", normalizeBacktestSymbol(symbol))
	}
	return &tickArchiveIterator{
		files: files,
		from:  fromTime,
		to:    toTime,
	}, nil
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
		event, err := parseTradeArchiveEvent(it.currentF.Symbol, row)
		if err != nil {
			return tickEvent{}, err
		}
		if !it.from.IsZero() && event.Time.Before(it.from) {
			continue
		}
		if !it.to.IsZero() && event.Time.After(it.to) {
			_ = it.current.Close()
			it.current = nil
			it.reader = nil
			it.index++
			continue
		}
		return event, nil
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

func parseBracketPlan(parameters map[string]any) (bracketPlan, bool) {
	side := strings.ToUpper(strings.TrimSpace(stringValue(parameters["side"])))
	entryPrice := parseFloatValue(parameters["entryPrice"])
	stopLossPrice := parseFloatValue(parameters["stopLossPrice"])
	takeProfitPrice := parseFloatValue(parameters["takeProfitPrice"])
	quantity := parseFloatValue(parameters["quantity"])
	if quantity <= 0 {
		quantity = 1
	}
	if (side != "BUY" && side != "SELL" && side != "LONG" && side != "SHORT") || entryPrice <= 0 {
		return bracketPlan{}, false
	}
	return bracketPlan{
		Side:            normalizeBracketSide(side),
		EntryPrice:      entryPrice,
		StopLossPrice:   stopLossPrice,
		TakeProfitPrice: takeProfitPrice,
		Quantity:        quantity,
	}, true
}

func parseBracketPlans(parameters map[string]any) []bracketPlan {
	if rawPlans, ok := parameters["tradePlans"]; ok {
		plans := make([]bracketPlan, 0)
		switch items := rawPlans.(type) {
		case []any:
			for _, item := range items {
				plan, ok := parseBracketPlanFromAny(item)
				if ok {
					plans = append(plans, plan)
				}
			}
		case []map[string]any:
			for _, item := range items {
				plan, ok := parseBracketPlan(item)
				if ok {
					plans = append(plans, plan)
				}
			}
		}
		if len(plans) > 0 {
			return plans
		}
	}
	if plan, ok := parseBracketPlan(parameters); ok {
		return []bracketPlan{plan}
	}
	return nil
}

func parseBracketPlanFromAny(value any) (bracketPlan, bool) {
	switch item := value.(type) {
	case map[string]any:
		return parseBracketPlan(item)
	default:
		return bracketPlan{}, false
	}
}

func simulateBracketEvent(event tickEvent, plan bracketPlan, state string) (nextState string, payload map[string]any, done bool) {
	switch state {
	case "waiting_entry":
		if bracketEntryTriggered(event.Price, plan) {
			return "entered", map[string]any{
				"bracketEntryHit":      true,
				"bracketEntryTime":     event.Time.UTC().Format(time.RFC3339),
				"bracketEntryFill":     event.Price,
				"bracketEntryQuantity": plan.Quantity,
			}, false
		}
	case "entered":
		if bracketStopTriggered(event.Price, plan) {
			return "stopped", map[string]any{
				"bracketExitType":     "stop_loss",
				"bracketExitTime":     event.Time.UTC().Format(time.RFC3339),
				"bracketExitPrice":    event.Price,
				"bracketRealizedPnL":  bracketPnL(plan, event.Price),
				"bracketQuantity":     plan.Quantity,
				"bracketSimulationOk": true,
			}, true
		}
		if bracketTakeProfitTriggered(event.Price, plan) {
			return "took_profit", map[string]any{
				"bracketExitType":     "take_profit",
				"bracketExitTime":     event.Time.UTC().Format(time.RFC3339),
				"bracketExitPrice":    event.Price,
				"bracketRealizedPnL":  bracketPnL(plan, event.Price),
				"bracketQuantity":     plan.Quantity,
				"bracketSimulationOk": true,
			}, true
		}
	}
	return state, nil, false
}

func (p *Platform) simulateTickBracket(symbol, from, to string, plan bracketPlan) (map[string]any, error) {
	iterator, err := p.newTickArchiveIteratorForRange(symbol, from, to)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	result := map[string]any{
		"bracketSide":            plan.Side,
		"bracketEntryPrice":      plan.EntryPrice,
		"bracketStopLossPrice":   plan.StopLossPrice,
		"bracketTakeProfitPrice": plan.TakeProfitPrice,
		"bracketQuantity":        plan.Quantity,
		"bracketSimulationOk":    false,
	}

	state := "waiting_entry"
	processed := 0
	for {
		event, err := iterator.Next()
		if err == io.EOF {
			result["bracketProcessedTicks"] = processed
			result["bracketFinalState"] = state
			result["executionTrades"] = buildExecutionTradeRecords("tick", plan, result)
			result["executionTradeCount"] = len(result["executionTrades"].([]map[string]any))
			return result, nil
		}
		if err != nil {
			return nil, err
		}
		processed++
		nextState, payload, done := simulateBracketEvent(event, plan, state)
		state = nextState
		if payload != nil {
			for key, value := range payload {
				result[key] = value
			}
		}
		if done {
			result["bracketProcessedTicks"] = processed
			result["bracketFinalState"] = state
			result["executionTrades"] = buildExecutionTradeRecords("tick", plan, result)
			result["executionTradeCount"] = len(result["executionTrades"].([]map[string]any))
			return result, nil
		}
	}
}

func (p *Platform) simulateTickBrackets(symbol, from, to string, plans []bracketPlan) (map[string]any, error) {
	results := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		result, err := p.simulateTickBracket(symbol, from, to, plan)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return summarizeExecutionResults(results), nil
}

func (p *Platform) simulateMinuteBracket(symbol, from, to string, plan bracketPlan) (map[string]any, error) {
	if normalizeBacktestSymbol(symbol) != "BTCUSDT" {
		return nil, fmt.Errorf("1min replay currently supports BTCUSDT only")
	}

	bars, err := p.loadCandleBars()
	if err != nil {
		return nil, err
	}

	fromTime := parseOptionalRFC3339(from)
	toTime := parseOptionalRFC3339(to)
	filtered := make([]candleBar, 0, len(bars))
	for _, bar := range bars {
		if !fromTime.IsZero() && bar.Time.Before(fromTime) {
			continue
		}
		if !toTime.IsZero() && bar.Time.After(toTime) {
			continue
		}
		filtered = append(filtered, bar)
	}
	if len(filtered) == 0 {
		return nil, fmt.Errorf("no 1min bars found for %s in range", normalizeBacktestSymbol(symbol))
	}

	result := map[string]any{
		"bracketSide":            plan.Side,
		"bracketEntryPrice":      plan.EntryPrice,
		"bracketStopLossPrice":   plan.StopLossPrice,
		"bracketTakeProfitPrice": plan.TakeProfitPrice,
		"bracketQuantity":        plan.Quantity,
		"bracketSimulationOk":    false,
		"streamMode":             "minute_bar_replay",
		"streamPreviewTicks":     len(filtered),
		"streamPreviewStart":     filtered[0].Time.UTC().Format(time.RFC3339),
		"streamPreviewEnd":       filtered[len(filtered)-1].Time.UTC().Format(time.RFC3339),
		"streamPreviewLastPrice": filtered[len(filtered)-1].Close,
	}

	state := "waiting_entry"
	processed := 0
	for _, bar := range filtered {
		processed++
		switch state {
		case "waiting_entry":
			entryHit, entryFill := minuteBarEntryTriggered(bar, plan)
			if !entryHit {
				continue
			}
			state = "entered"
			result["bracketEntryHit"] = true
			result["bracketEntryTime"] = bar.Time.UTC().Format(time.RFC3339)
			result["bracketEntryFill"] = entryFill
			result["bracketEntryQuantity"] = plan.Quantity

			exitType, exitPrice, exitHit := minuteBarExitTriggered(bar, plan)
			if exitHit {
				result["bracketExitType"] = exitType
				result["bracketExitTime"] = bar.Time.UTC().Format(time.RFC3339)
				result["bracketExitPrice"] = exitPrice
				result["bracketRealizedPnL"] = bracketPnLWithFill(plan, entryFill, exitPrice)
				result["bracketQuantity"] = plan.Quantity
				result["bracketSimulationOk"] = true
				result["bracketProcessedTicks"] = processed
				result["bracketFinalState"] = exitType
				result["executionTrades"] = buildExecutionTradeRecords("1min", plan, result)
				result["executionTradeCount"] = len(result["executionTrades"].([]map[string]any))
				return result, nil
			}
		case "entered":
			exitType, exitPrice, exitHit := minuteBarExitTriggered(bar, plan)
			if !exitHit {
				continue
			}
			entryFill := parseFloatValue(result["bracketEntryFill"])
			result["bracketExitType"] = exitType
			result["bracketExitTime"] = bar.Time.UTC().Format(time.RFC3339)
			result["bracketExitPrice"] = exitPrice
			result["bracketRealizedPnL"] = bracketPnLWithFill(plan, entryFill, exitPrice)
			result["bracketQuantity"] = plan.Quantity
			result["bracketSimulationOk"] = true
			result["bracketProcessedTicks"] = processed
			result["bracketFinalState"] = exitType
			result["executionTrades"] = buildExecutionTradeRecords("1min", plan, result)
			result["executionTradeCount"] = len(result["executionTrades"].([]map[string]any))
			return result, nil
		}
	}

	result["bracketProcessedTicks"] = processed
	result["bracketFinalState"] = state
	result["executionTrades"] = buildExecutionTradeRecords("1min", plan, result)
	result["executionTradeCount"] = len(result["executionTrades"].([]map[string]any))
	return result, nil
}

func (p *Platform) simulateMinuteBrackets(symbol, from, to string, plans []bracketPlan) (map[string]any, error) {
	results := make([]map[string]any, 0, len(plans))
	for _, plan := range plans {
		result, err := p.simulateMinuteBracket(symbol, from, to, plan)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	return summarizeExecutionResults(results), nil
}

func shouldReplayLedger(parameters map[string]any) bool {
	value := parameters["replayLedger"]
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func (p *Platform) simulateReplayLedgerOnTick(symbol, from, to string) (map[string]any, error) {
	ledger, err := p.loadReplayLedger()
	if err != nil {
		return nil, err
	}
	trades := pairReplayTrades(ledger, normalizeBacktestSymbol(symbol), parseOptionalRFC3339(from), parseOptionalRFC3339(to))
	if len(trades) == 0 {
		return map[string]any{
			"replayLedgerTrades":           0,
			"replayLedgerCompleted":        0,
			"replayLedgerCompletedSamples": []map[string]any{},
			"replayLedgerSkipped":          0,
			"replayLedgerSkippedInvalid":   0,
			"replayLedgerSkippedEntry":     0,
			"replayLedgerSkippedExit":      0,
			"replayLedgerSkippedError":     0,
			"replayLedgerPnL":              0,
			"replayLedgerStopHits":         0,
			"replayLedgerTakeProfitHits":   0,
		}, nil
	}

	totalPnL := 0.0
	completed := 0
	skipped := 0
	stopHits := 0
	tpHits := 0
	skippedInvalid := 0
	skippedEntryNotHit := 0
	skippedExitNotHit := 0
	skippedErrors := 0
	completedSamples := make([]map[string]any, 0, 3)
	skippedSamples := make([]map[string]any, 0, 3)
	byReason := map[string]map[string]int{}

	for _, trade := range trades {
		reasonKey := normalizeReplayReason(trade.Entry.Reason)
		ensureReplayReasonBucket(byReason, reasonKey)
		byReason[reasonKey]["trades"]++

		plan, ok := bracketPlanFromReplayTrade(trade)
		if !ok {
			skipped++
			skippedInvalid++
			byReason[reasonKey]["skipped"]++
			byReason[reasonKey]["skippedInvalid"]++
			skippedSamples = appendReplaySkipSample(skippedSamples, trade, "invalid_trade_shape", nil)
			continue
		}
		result, err := p.simulateTickBracket(
			symbol,
			trade.Entry.Time.UTC().Format(time.RFC3339),
			trade.Exit.Time.UTC().Format(time.RFC3339),
			plan,
		)
		if err != nil {
			skipped++
			skippedErrors++
			byReason[reasonKey]["skipped"]++
			byReason[reasonKey]["skippedError"]++
			skippedSamples = appendReplaySkipSample(skippedSamples, trade, "simulation_error", map[string]any{
				"error": err.Error(),
			})
			continue
		}
		if ok, _ := result["bracketSimulationOk"].(bool); !ok {
			skipped++
			byReason[reasonKey]["skipped"]++
			switch stringValue(result["bracketFinalState"]) {
			case "waiting_entry":
				skippedEntryNotHit++
				byReason[reasonKey]["skippedEntry"]++
				skippedSamples = appendReplaySkipSample(skippedSamples, trade, "entry_not_hit", result)
			case "entered":
				skippedExitNotHit++
				byReason[reasonKey]["skippedExit"]++
				skippedSamples = appendReplaySkipSample(skippedSamples, trade, "exit_not_hit", result)
			default:
				skippedErrors++
				byReason[reasonKey]["skippedError"]++
				skippedSamples = appendReplaySkipSample(skippedSamples, trade, "simulation_error", result)
			}
			continue
		}
		completed++
		byReason[reasonKey]["completed"]++
		totalPnL += parseFloatValue(result["bracketRealizedPnL"])
		switch stringValue(result["bracketExitType"]) {
		case "stop_loss":
			stopHits++
			byReason[reasonKey]["stopHits"]++
		case "take_profit":
			tpHits++
			byReason[reasonKey]["takeProfitHits"]++
		}
		completedSamples = appendReplayCompletedSample(completedSamples, trade, result)
	}

	return map[string]any{
		"replayLedgerTrades":           len(trades),
		"replayLedgerCompleted":        completed,
		"replayLedgerCompletedSamples": completedSamples,
		"replayLedgerSkipped":          skipped,
		"replayLedgerSkippedInvalid":   skippedInvalid,
		"replayLedgerSkippedEntry":     skippedEntryNotHit,
		"replayLedgerSkippedExit":      skippedExitNotHit,
		"replayLedgerSkippedError":     skippedErrors,
		"replayLedgerSkippedSamples":   skippedSamples,
		"replayLedgerByReason":         byReason,
		"replayLedgerPnL":              totalPnL,
		"replayLedgerStopHits":         stopHits,
		"replayLedgerTakeProfitHits":   tpHits,
	}, nil
}

func ensureReplayReasonBucket(byReason map[string]map[string]int, reason string) {
	if _, ok := byReason[reason]; ok {
		return
	}
	byReason[reason] = map[string]int{
		"trades":         0,
		"completed":      0,
		"skipped":        0,
		"skippedInvalid": 0,
		"skippedEntry":   0,
		"skippedExit":    0,
		"skippedError":   0,
		"stopHits":       0,
		"takeProfitHits": 0,
	}
}

func appendReplayCompletedSample(samples []map[string]any, trade replayTrade, result map[string]any) []map[string]any {
	if len(samples) >= 3 {
		return samples
	}
	item := map[string]any{
		"entryTime":          trade.Entry.Time.UTC().Format(time.RFC3339),
		"entryType":          trade.Entry.Type,
		"entryPrice":         trade.Entry.Price,
		"entryCause":         trade.Entry.Reason,
		"exitTime":           trade.Exit.Time.UTC().Format(time.RFC3339),
		"exitPrice":          trade.Exit.Price,
		"exitCause":          trade.Exit.Reason,
		"notional":           trade.Entry.Notional,
		"bracketEntryFill":   result["bracketEntryFill"],
		"bracketEntryTime":   result["bracketEntryTime"],
		"bracketExitType":    result["bracketExitType"],
		"bracketExitPrice":   result["bracketExitPrice"],
		"bracketExitTime":    result["bracketExitTime"],
		"bracketRealizedPnL": result["bracketRealizedPnL"],
	}
	return append(samples, item)
}

func normalizeReplayReason(reason string) string {
	trimmed := strings.TrimSpace(reason)
	if trimmed == "" {
		return "UNKNOWN"
	}
	return trimmed
}

func appendReplaySkipSample(samples []map[string]any, trade replayTrade, reason string, extra map[string]any) []map[string]any {
	if len(samples) >= 3 {
		return samples
	}
	item := map[string]any{
		"reason":     reason,
		"entryTime":  trade.Entry.Time.UTC().Format(time.RFC3339),
		"entryType":  trade.Entry.Type,
		"entryPrice": trade.Entry.Price,
		"entryCause": trade.Entry.Reason,
		"exitTime":   trade.Exit.Time.UTC().Format(time.RFC3339),
		"exitPrice":  trade.Exit.Price,
		"exitCause":  trade.Exit.Reason,
		"notional":   trade.Entry.Notional,
	}
	for key, value := range extra {
		item[key] = value
	}
	return append(samples, item)
}

func pairReplayTrades(events []strategyReplayEvent, symbol string, from, to time.Time) []replayTrade {
	_ = symbol
	trades := make([]replayTrade, 0)
	var current *strategyReplayEvent
	for _, event := range events {
		if event.Notional <= 0 {
			continue
		}
		switch strings.ToUpper(strings.TrimSpace(event.Type)) {
		case "BUY", "SHORT":
			entry := event
			current = &entry
		case "EXIT":
			if current == nil {
				continue
			}
			trade := replayTrade{
				Entry: *current,
				Exit:  event,
			}
			if replayTradeInRange(trade, from, to) {
				trades = append(trades, trade)
			}
			current = nil
		}
	}
	return trades
}

func replayTradeInRange(trade replayTrade, from, to time.Time) bool {
	if !from.IsZero() && trade.Exit.Time.Before(from) {
		return false
	}
	if !to.IsZero() && trade.Entry.Time.After(to) {
		return false
	}
	return true
}

func bracketPlanFromReplayTrade(trade replayTrade) (bracketPlan, bool) {
	entryType := strings.ToUpper(strings.TrimSpace(trade.Entry.Type))
	exitReason := strings.ToUpper(strings.TrimSpace(trade.Exit.Reason))
	if entryType != "BUY" && entryType != "SHORT" {
		return bracketPlan{}, false
	}
	if trade.Entry.Price <= 0 || trade.Exit.Price <= 0 || trade.Entry.Notional <= 0 {
		return bracketPlan{}, false
	}

	quantity := trade.Entry.Notional / trade.Entry.Price
	plan := bracketPlan{
		Side:       normalizeBracketSide(entryType),
		EntryPrice: trade.Entry.Price,
		Quantity:   quantity,
	}
	if exitReason == "SL" {
		plan.StopLossPrice = trade.Exit.Price
	} else {
		plan.TakeProfitPrice = trade.Exit.Price
	}
	return plan, true
}

func normalizeBracketSide(side string) string {
	if side == "LONG" {
		return "BUY"
	}
	if side == "SHORT" {
		return "SELL"
	}
	return side
}

func bracketEntryTriggered(price float64, plan bracketPlan) bool {
	if plan.Side == "BUY" {
		return price <= plan.EntryPrice
	}
	return price >= plan.EntryPrice
}

func bracketStopTriggered(price float64, plan bracketPlan) bool {
	if plan.StopLossPrice <= 0 {
		return false
	}
	if plan.Side == "BUY" {
		return price <= plan.StopLossPrice
	}
	return price >= plan.StopLossPrice
}

func bracketTakeProfitTriggered(price float64, plan bracketPlan) bool {
	if plan.TakeProfitPrice <= 0 {
		return false
	}
	if plan.Side == "BUY" {
		return price >= plan.TakeProfitPrice
	}
	return price <= plan.TakeProfitPrice
}

func bracketPnL(plan bracketPlan, exitPrice float64) float64 {
	if plan.Side == "BUY" {
		return (exitPrice - plan.EntryPrice) * plan.Quantity
	}
	return (plan.EntryPrice - exitPrice) * plan.Quantity
}

func bracketPnLWithFill(plan bracketPlan, entryFill, exitPrice float64) float64 {
	if plan.Side == "BUY" {
		return (exitPrice - entryFill) * plan.Quantity
	}
	return (entryFill - exitPrice) * plan.Quantity
}

func buildExecutionTradeRecords(source string, plan bracketPlan, result map[string]any) []map[string]any {
	entryHit, _ := result["bracketEntryHit"].(bool)
	status := "pending_entry"
	if entryHit {
		status = "open"
	}
	if simulationOK, _ := result["bracketSimulationOk"].(bool); simulationOK {
		status = "closed"
	}

	record := map[string]any{
		"source":        source,
		"side":          plan.Side,
		"quantity":      plan.Quantity,
		"entryTarget":   plan.EntryPrice,
		"stopLoss":      plan.StopLossPrice,
		"takeProfit":    plan.TakeProfitPrice,
		"entryTime":     result["bracketEntryTime"],
		"entryPrice":    result["bracketEntryFill"],
		"exitTime":      result["bracketExitTime"],
		"exitPrice":     result["bracketExitPrice"],
		"exitType":      result["bracketExitType"],
		"realizedPnL":   result["bracketRealizedPnL"],
		"processedBars": result["bracketProcessedTicks"],
		"status":        status,
	}
	return []map[string]any{record}
}

func summarizeExecutionResults(results []map[string]any) map[string]any {
	summary := map[string]any{
		"executionTradeCount":   0,
		"executionClosedCount":  0,
		"executionOpenCount":    0,
		"executionPendingCount": 0,
		"executionWins":         0,
		"executionLosses":       0,
		"executionRealizedPnL":  0.0,
		"executionTrades":       []map[string]any{},
	}
	if len(results) == 0 {
		return summary
	}

	allTrades := make([]map[string]any, 0, len(results))
	totalPnL := 0.0
	closedCount := 0
	openCount := 0
	pendingCount := 0
	wins := 0
	losses := 0

	for index, result := range results {
		if index == 0 {
			for _, key := range []string{
				"bracketSide",
				"bracketEntryPrice",
				"bracketStopLossPrice",
				"bracketTakeProfitPrice",
				"bracketQuantity",
				"bracketEntryHit",
				"bracketEntryTime",
				"bracketEntryFill",
				"bracketExitType",
				"bracketExitTime",
				"bracketExitPrice",
				"bracketRealizedPnL",
				"bracketProcessedTicks",
				"bracketFinalState",
				"streamMode",
				"streamPreviewTicks",
				"streamPreviewStart",
				"streamPreviewEnd",
				"streamPreviewLastPrice",
			} {
				if value, ok := result[key]; ok {
					summary[key] = value
				}
			}
		}

		if trades, ok := result["executionTrades"].([]map[string]any); ok {
			for _, trade := range trades {
				allTrades = append(allTrades, trade)
				switch stringValue(trade["status"]) {
				case "closed":
					closedCount++
					pnl := parseFloatValue(trade["realizedPnL"])
					totalPnL += pnl
					if pnl > 0 {
						wins++
					} else if pnl < 0 {
						losses++
					}
				case "open":
					openCount++
				default:
					pendingCount++
				}
			}
		}
	}

	winRate := 0.0
	if closedCount > 0 {
		winRate = float64(wins) / float64(closedCount)
	}

	summary["executionTradeCount"] = len(allTrades)
	summary["executionClosedCount"] = closedCount
	summary["executionOpenCount"] = openCount
	summary["executionPendingCount"] = pendingCount
	summary["executionWins"] = wins
	summary["executionLosses"] = losses
	summary["executionRealizedPnL"] = totalPnL
	summary["executionWinRate"] = winRate
	summary["executionTrades"] = allTrades
	return summary
}

func minuteBarEntryTriggered(bar candleBar, plan bracketPlan) (bool, float64) {
	if plan.Side == "BUY" {
		if bar.Low <= plan.EntryPrice {
			return true, plan.EntryPrice
		}
		return false, 0
	}
	if bar.High >= plan.EntryPrice {
		return true, plan.EntryPrice
	}
	return false, 0
}

func minuteBarExitTriggered(bar candleBar, plan bracketPlan) (string, float64, bool) {
	stopHit := false
	tpHit := false
	if plan.Side == "BUY" {
		stopHit = plan.StopLossPrice > 0 && bar.Low <= plan.StopLossPrice
		tpHit = plan.TakeProfitPrice > 0 && bar.High >= plan.TakeProfitPrice
	} else {
		stopHit = plan.StopLossPrice > 0 && bar.High >= plan.StopLossPrice
		tpHit = plan.TakeProfitPrice > 0 && bar.Low <= plan.TakeProfitPrice
	}

	if stopHit && tpHit {
		return "stop_loss", plan.StopLossPrice, true
	}
	if stopHit {
		return "stop_loss", plan.StopLossPrice, true
	}
	if tpHit {
		return "take_profit", plan.TakeProfitPrice, true
	}
	return "", 0, false
}

func (p *Platform) previewTickArchiveRange(symbol, from, to string, limit int) (map[string]any, error) {
	iterator, err := p.newTickArchiveIteratorForRange(symbol, from, to)
	if err != nil {
		return nil, err
	}
	defer iterator.Close()

	firstTime := ""
	lastTime := ""
	lastPrice := 0.0
	count := 0

	for count < limit {
		event, err := iterator.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if count == 0 {
			firstTime = event.Time.UTC().Format(time.RFC3339)
		}
		lastTime = event.Time.UTC().Format(time.RFC3339)
		lastPrice = event.Price
		count++
	}

	files, err := p.collectTradeArchiveFilesInRange(normalizeBacktestSymbol(symbol), parseOptionalRFC3339(from), parseOptionalRFC3339(to))
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"streamMode":             "tick_archive_preview",
		"streamPreviewTicks":     count,
		"streamPreviewLimit":     limit,
		"streamPreviewTruncated": count == limit,
		"streamPreviewStart":     firstTime,
		"streamPreviewEnd":       lastTime,
		"streamPreviewLastPrice": lastPrice,
		"matchedArchiveFiles":    len(files),
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

func parseOptionalRFC3339(value string) time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return time.Time{}
	}
	return parsed.UTC()
}

func parseFloatValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return parsed
	default:
		return 0
	}
}
