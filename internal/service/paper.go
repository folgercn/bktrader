package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

// strategyReplayEvent 表示从策略交易账本 CSV 中解析出的单条回放事件。
type strategyReplayEvent struct {
	Time     time.Time // 事件时间
	Type     string    // 事件类型：BUY / SHORT / EXIT
	Price    float64   // 执行价格
	Reason   string    // 交易原因
	Notional float64   // 名义金额
	Balance  float64   // 账户余额
}

// --- 模拟交易会话管理 ---

// ListPaperSessions 获取所有模拟交易会话。
func (p *Platform) ListPaperSessions() ([]domain.PaperSession, error) {
	return p.store.ListPaperSessions()
}

// CreatePaperSession 创建新的模拟交易会话，并捕获初始净值快照。
func (p *Platform) CreatePaperSession(accountID, strategyID string, startEquity float64) (domain.PaperSession, error) {
	session, err := p.store.CreatePaperSession(accountID, strategyID, startEquity)
	if err != nil {
		return domain.PaperSession{}, err
	}
	if err := p.captureAccountSnapshot(accountID); err != nil {
		return domain.PaperSession{}, err
	}
	return session, nil
}

// StartPaperSession 启动模拟交易会话的后台回放循环。
// 如果会话已在运行则忽略重复启动。
func (p *Platform) StartPaperSession(sessionID string) (domain.PaperSession, error) {
	session, err := p.store.GetPaperSession(sessionID)
	if err != nil {
		return domain.PaperSession{}, err
	}

	p.mu.Lock()
	if _, exists := p.run[sessionID]; exists {
		p.mu.Unlock()
		return p.store.UpdatePaperSessionStatus(sessionID, "RUNNING")
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.run[sessionID] = cancel
	p.mu.Unlock()

	session, err = p.store.UpdatePaperSessionStatus(sessionID, "RUNNING")
	if err != nil {
		p.mu.Lock()
		delete(p.run, sessionID)
		p.mu.Unlock()
		cancel()
		return domain.PaperSession{}, err
	}

	// 启动后台 goroutine 执行策略回放
	go p.runPaperSessionLoop(ctx, session)
	return session, nil
}

// StopPaperSession 停止模拟交易会话，取消后台回放循环。
func (p *Platform) StopPaperSession(sessionID string) (domain.PaperSession, error) {
	session, err := p.store.UpdatePaperSessionStatus(sessionID, "STOPPED")
	if err != nil {
		return domain.PaperSession{}, err
	}

	p.mu.Lock()
	cancel, exists := p.run[sessionID]
	if exists {
		delete(p.run, sessionID)
	}
	p.mu.Unlock()

	if exists {
		cancel()
	}
	return session, nil
}

// TickPaperSession 手动触发会话前进一步（处理下一个回放事件）。
func (p *Platform) TickPaperSession(sessionID string) (domain.Order, error) {
	session, err := p.store.GetPaperSession(sessionID)
	if err != nil {
		return domain.Order{}, err
	}
	return p.placePaperSessionOrder(session)
}

// SetTickInterval 设置模拟盘后台循环的 Ticker 间隔（秒）。
func (p *Platform) SetTickInterval(seconds int) {
	if seconds > 0 {
		p.tickInterval = seconds
	}
}

// --- 后台回放循环 ---

// runPaperSessionLoop 后台循环执行策略回放，按 tickInterval 间隔逐步前进。
func (p *Platform) runPaperSessionLoop(ctx context.Context, session domain.PaperSession) {
	interval := p.tickInterval
	if interval <= 0 {
		interval = 15 // 默认 15 秒
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()
	defer p.removeRunner(session.ID)

	// 立即执行第一步
	_, _ = p.placePaperSessionOrder(session)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ctx.Err() != nil {
				return
			}
			current, err := p.store.GetPaperSession(session.ID)
			if err != nil || current.Status != "RUNNING" {
				return
			}
			_, _ = p.placePaperSessionOrder(current)
		}
	}
}

// placePaperSessionOrder 从回放账本中取下一个事件，转换为订单并执行。
// 跳过 notional=0 的事件（零初始化引导事件）。
func (p *Platform) placePaperSessionOrder(session domain.PaperSession) (domain.Order, error) {
	ledger, err := p.loadReplayLedger()
	if err != nil {
		return domain.Order{}, err
	}

	state := cloneMetadata(session.State)
	index := 0
	if value, ok := toFloat64(state["ledgerIndex"]); ok && value >= 0 {
		index = int(value)
	}

	positions, err := p.store.ListPositions()
	if err != nil {
		return domain.Order{}, err
	}

	// 遍历账本直到找到一个可执行的事件
	for index < len(ledger) {
		event := ledger[index]
		index++
		state["runner"] = "strategy-replay"
		state["ledgerIndex"] = index
		state["lastLedgerTime"] = event.Time.Format(time.RFC3339)
		state["lastLedgerType"] = event.Type
		state["lastLedgerReason"] = event.Reason

		updatedSession, err := p.store.UpdatePaperSessionState(session.ID, state)
		if err != nil {
			return domain.Order{}, err
		}
		session = updatedSession

		order, ok, err := p.translateReplayEvent(session, positions, event)
		if err != nil {
			return domain.Order{}, err
		}
		if !ok {
			continue
		}

		created, err := p.CreateOrder(order)
		if err != nil {
			return domain.Order{}, err
		}
		return created, nil
	}

	// 账本回放完毕，标记会话完成
	state["runner"] = "strategy-replay"
	state["completedAt"] = time.Now().UTC().Format(time.RFC3339)
	if _, err := p.store.UpdatePaperSessionState(session.ID, state); err != nil {
		return domain.Order{}, err
	}
	_, _ = p.store.UpdatePaperSessionStatus(session.ID, "STOPPED")
	return domain.Order{}, fmt.Errorf("模拟会话 %s 已完成所有回放事件", session.ID)
}

// removeRunner 从运行中列表移除指定会话。
func (p *Platform) removeRunner(sessionID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.run, sessionID)
}

// --- CSV 账本回放 ---

// loadReplayLedger 延迟加载策略交易账本 CSV（使用 sync.Once 确保只读取一次）。
func (p *Platform) loadReplayLedger() ([]strategyReplayEvent, error) {
	p.once.Do(func() {
		p.ledger, p.ledgerErr = readStrategyReplayLedger("FINAL_1D_LEDGER_BEST_SL.csv")
	})
	return p.ledger, p.ledgerErr
}

// readStrategyReplayLedger 读取策略回放账本 CSV 文件，解析为事件列表。
// CSV 格式：时间, 类型, 价格, 原因, 名义金额, 余额
func readStrategyReplayLedger(path string) ([]strategyReplayEvent, error) {
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
		return nil, fmt.Errorf("策略回放账本为空: %s", resolved)
	}

	events := make([]strategyReplayEvent, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 6 {
			continue
		}
		eventTime, err := time.Parse("2006-01-02 15:04:05Z07:00", row[0])
		if err != nil {
			return nil, fmt.Errorf("解析回放时间 %q: %w", row[0], err)
		}
		price, err := strconv.ParseFloat(row[1+1], 64)
		if err != nil {
			return nil, fmt.Errorf("解析回放价格 %q: %w", row[2], err)
		}
		notional, err := strconv.ParseFloat(row[4], 64)
		if err != nil {
			return nil, fmt.Errorf("解析回放名义金额 %q: %w", row[4], err)
		}
		balance, err := strconv.ParseFloat(row[5], 64)
		if err != nil {
			return nil, fmt.Errorf("解析回放余额 %q: %w", row[5], err)
		}
		events = append(events, strategyReplayEvent{
			Time:     eventTime,
			Type:     strings.ToUpper(strings.TrimSpace(row[1])),
			Price:    price,
			Reason:   strings.TrimSpace(row[3]),
			Notional: notional,
			Balance:  balance,
		})
	}

	// 按时间排序确保回放顺序正确
	sort.Slice(events, func(i, j int) bool { return events[i].Time.Before(events[j].Time) })
	return events, nil
}

// translateReplayEvent 将回放事件转换为交易订单。
// BUY -> 买入做多，SHORT -> 卖出做空，EXIT -> 平掉当前持仓。
func (p *Platform) translateReplayEvent(session domain.PaperSession, positions []domain.Position, event strategyReplayEvent) (domain.Order, bool, error) {
	symbol := "BTCUSDT"
	position, hasPosition := findAccountPosition(positions, session.AccountID, symbol)
	strategyVersionID, _ := p.resolveStrategyVersionID(session.StrategyID)

	switch event.Type {
	case "BUY":
		if event.Notional <= 0 || event.Price <= 0 {
			return domain.Order{}, false, nil
		}
		return domain.Order{
			AccountID:         session.AccountID,
			StrategyVersionID: strategyVersionID,
			Symbol:            symbol,
			Side:              "BUY",
			Type:              "MARKET",
			Quantity:          roundQuantity(event.Notional / event.Price),
			Price:             event.Price,
			Metadata: map[string]any{
				"markPrice":    event.Price,
				"source":       "paper-session-replay",
				"paperSession": session.ID,
				"strategyId":   session.StrategyID,
				"eventTime":    event.Time.Format(time.RFC3339),
				"reason":       event.Reason,
				"balance":      event.Balance,
				"notional":     event.Notional,
			},
		}, true, nil
	case "SHORT":
		if event.Notional <= 0 || event.Price <= 0 {
			return domain.Order{}, false, nil
		}
		return domain.Order{
			AccountID:         session.AccountID,
			StrategyVersionID: strategyVersionID,
			Symbol:            symbol,
			Side:              "SELL",
			Type:              "MARKET",
			Quantity:          roundQuantity(event.Notional / event.Price),
			Price:             event.Price,
			Metadata: map[string]any{
				"markPrice":    event.Price,
				"source":       "paper-session-replay",
				"paperSession": session.ID,
				"strategyId":   session.StrategyID,
				"eventTime":    event.Time.Format(time.RFC3339),
				"reason":       event.Reason,
				"balance":      event.Balance,
				"notional":     event.Notional,
			},
		}, true, nil
	case "EXIT":
		if !hasPosition || position.Quantity <= 0 {
			return domain.Order{}, false, nil
		}
		side := "SELL"
		if strings.EqualFold(position.Side, "SHORT") {
			side = "BUY"
		}
		return domain.Order{
			AccountID:         session.AccountID,
			StrategyVersionID: firstNonEmpty(strategyVersionID, position.StrategyVersionID),
			Symbol:            symbol,
			Side:              side,
			Type:              "MARKET",
			Quantity:          roundQuantity(position.Quantity),
			Price:             event.Price,
			Metadata: map[string]any{
				"markPrice":    event.Price,
				"source":       "paper-session-replay",
				"paperSession": session.ID,
				"strategyId":   session.StrategyID,
				"eventTime":    event.Time.Format(time.RFC3339),
				"reason":       event.Reason,
				"balance":      event.Balance,
				"notional":     event.Notional,
			},
		}, true, nil
	default:
		return domain.Order{}, false, nil
	}
}

// resolveStrategyVersionID 从策略 ID 查找当前版本 ID。
func (p *Platform) resolveStrategyVersionID(strategyID string) (string, error) {
	strategies, err := p.store.ListStrategies()
	if err != nil {
		return "", err
	}
	for _, strategy := range strategies {
		id, _ := strategy["id"].(string)
		if id != strategyID {
			continue
		}
		currentVersion, ok := strategy["currentVersion"].(domain.StrategyVersion)
		if ok {
			return currentVersion.ID, nil
		}
	}
	return "", nil
}

// findAccountPosition 在持仓列表中查找指定账户和交易对的持仓。
func findAccountPosition(positions []domain.Position, accountID, symbol string) (domain.Position, bool) {
	for _, position := range positions {
		if position.AccountID == accountID && position.Symbol == symbol {
			return position, true
		}
	}
	return domain.Position{}, false
}

// roundQuantity 将数量精确到小数点后 6 位。
func roundQuantity(quantity float64) float64 {
	return math.Round(quantity*1_000_000) / 1_000_000
}
