package service

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	defaultLiveTradePairLimit = 20
	maxLiveTradePairLimit     = 200
)

type liveTradeFillEvent struct {
	order           domain.Order
	fill            domain.Fill
	eventTime       time.Time
	intent          liveTradeIntentDetails
	decisionEventID string
}

type liveTradeIntentDetails struct {
	role              string
	reason            string
	signalKind        string
	targetPriceSource string
	stopLossSource    string
	recoveryTriggered bool
	manualTriggered   bool
}

type liveTradePairBuilder struct {
	index              int
	liveSessionID      string
	accountID          string
	strategyID         string
	symbol             string
	side               string
	entryAt            time.Time
	exitAt             time.Time
	entryQty           float64
	exitQty            float64
	entryNotional      float64
	exitNotional       float64
	realizedPnL        float64
	unrealizedPnL      float64
	fees               float64
	entryReason        string
	exitReason         string
	exitClassifier     string
	exitVerdict        string
	entryFillCount     int
	exitFillCount      int
	entryOrderIDs      []string
	exitOrderIDs       []string
	entryOrderSeen     map[string]struct{}
	exitOrderSeen      map[string]struct{}
	notes              []string
	notesSeen          map[string]struct{}
	recoveryExit       bool
	hasUnsafeExitOrder bool
	lastExitOrderID    string
}

func (p *Platform) CreateOrderCloseVerification(v domain.OrderCloseVerification) (domain.OrderCloseVerification, error) {
	return p.store.CreateOrderCloseVerification(v)
}

func (p *Platform) ListLiveTradePairs(query domain.LiveTradePairQuery) ([]domain.LiveTradePair, error) {
	startTime := time.Now()
	liveSessionID := strings.TrimSpace(query.LiveSessionID)
	if liveSessionID == "" {
		return nil, fmt.Errorf("liveSessionId is required")
	}

	defer func() {
		p.logger("service.live_trade_pairs", "session_id", liveSessionID).Info("ListLiveTradePairs completed", "elapsed", time.Since(startTime))
	}()

	session, err := p.store.GetLiveSession(liveSessionID)
	if err != nil {
		return nil, err
	}

	orders, err := p.store.QueryOrders(domain.OrderQuery{LiveSessionID: liveSessionID})
	if err != nil {
		return nil, err
	}

	orderByID := make(map[string]domain.Order)
	var orderIDs []string
	uniqueSymbols := make(map[string]struct{})
	for _, order := range orders {
		orderByID[order.ID] = order
		orderIDs = append(orderIDs, order.ID)
		uniqueSymbols[NormalizeSymbol(order.Symbol)] = struct{}{}
	}

	fills, err := p.store.QueryFills(domain.FillQuery{OrderIDs: orderIDs})
	if err != nil {
		return nil, err
	}

	positions, err := p.store.QueryPositions(domain.PositionQuery{AccountID: session.AccountID})
	if err != nil {
		return nil, err
	}

	currentPositionBySymbol := currentLivePositionBySymbol(session, positions)
	fillEvents := buildLiveTradeFillEvents(fills, orderByID, nil)
	if len(fillEvents) == 0 {
		return filterAndLimitLiveTradePairs(nil, normalizeLiveTradePairStatus(query.Status), normalizeLiveTradePairLimit(query.Limit)), nil
	}

	sort.SliceStable(fillEvents, func(i, j int) bool {
		left := fillEvents[i]
		right := fillEvents[j]
		switch {
		case left.eventTime.Before(right.eventTime):
			return true
		case left.eventTime.After(right.eventTime):
			return false
		case left.order.CreatedAt.Before(right.order.CreatedAt):
			return true
		case left.order.CreatedAt.After(right.order.CreatedAt):
			return false
		default:
			return left.fill.ID < right.fill.ID
		}
	})

	results := make([]domain.LiveTradePair, 0, len(fillEvents))
	eventsBySymbol := make(map[string][]liveTradeFillEvent, len(uniqueSymbols))
	for _, event := range fillEvents {
		symbol := NormalizeSymbol(event.order.Symbol)
		eventsBySymbol[symbol] = append(eventsBySymbol[symbol], event)
	}
	symbols := make([]string, 0, len(eventsBySymbol))
	for symbol := range eventsBySymbol {
		symbols = append(symbols, symbol)
	}
	sort.Strings(symbols)
	for _, symbol := range symbols {
		results = append(results, buildLiveTradePairsForSymbol(session, eventsBySymbol[symbol], currentPositionBySymbol)...)
	}

	sort.SliceStable(results, func(i, j int) bool {
		leftTime := results[i].EntryAt
		rightTime := results[j].EntryAt
		switch {
		case leftTime.After(rightTime):
			return true
		case leftTime.Before(rightTime):
			return false
		default:
			return results[i].ID > results[j].ID
		}
	})
	results = filterAndLimitLiveTradePairs(results, normalizeLiveTradePairStatus(query.Status), normalizeLiveTradePairLimit(query.Limit))
	p.enrichLiveTradePairs(results, orderByID)
	return results, nil
}

func buildLiveTradePairsForSymbol(
	session domain.LiveSession,
	fillEvents []liveTradeFillEvent,
	currentPositionBySymbol map[string]domain.Position,
) []domain.LiveTradePair {
	state := pnlState{}
	results := make([]domain.LiveTradePair, 0, len(fillEvents))
	var current *liveTradePairBuilder
	pairIndex := 0
	for _, event := range fillEvents {
		signedQty := tradePairSignedQuantity(event.order.Side, event.fill.Quantity)
		if signedQty == 0 {
			continue
		}
		absQty := absFloat(signedQty)
		remainingFee := event.fill.Fee
		prevNetQty := state.netQty
		if prevNetQty == 0 || sameSign(prevNetQty, signedQty) {
			if current == nil {
				pairIndex++
				current = newLiveTradePairBuilder(pairIndex, session, event.order.Symbol, liveTradeSideFromSignedQty(signedQty))
			}
			current.addEntry(event, absQty, remainingFee)
			applyPnLFill(&state, event.order.Side, event.fill.Quantity, event.fill.Price)
			continue
		}

		closingQty := minFloat(absFloat(prevNetQty), absQty)
		if current == nil {
			pairIndex++
			current = newLiveTradePairBuilder(pairIndex, session, event.order.Symbol, liveTradeSideFromSignedQty(prevNetQty))
			current.addNote("missing-entry-fill-history")
			current.exitVerdict = "orphan-exit"
		}
		closingFee := proportionalFee(event.fill.Fee, closingQty, absQty)
		remainingFee -= closingFee
		realizedPnL := 0.0
		if prevNetQty > 0 {
			realizedPnL = (event.fill.Price - state.avgPrice) * closingQty
		} else {
			realizedPnL = (state.avgPrice - event.fill.Price) * closingQty
		}
		current.addExit(event, closingQty, closingFee, realizedPnL)

		applyPnLFill(&state, event.order.Side, event.fill.Quantity, event.fill.Price)
		remainingQty := absQty - closingQty
		if !tradingQuantityExceeds(absFloat(prevNetQty)-closingQty, 0) {
			results = append(results, current.finalizeClosed())
			current = nil
		}
		if tradingQuantityPositive(remainingQty) {
			pairIndex++
			current = newLiveTradePairBuilder(pairIndex, session, event.order.Symbol, liveTradeSideFromSignedQty(signedQty))
			current.addEntry(event, remainingQty, remainingFee)
		}
	}

	if current != nil {
		results = append(results, current.finalizeOpen(currentPositionBySymbol[current.symbol]))
	}
	return results
}

func liveTradePairRelevantFills(fills []domain.Fill, orderByID map[string]domain.Order) []domain.Fill {
	items := make([]domain.Fill, 0, len(fills))
	for _, fill := range fills {
		if _, ok := orderByID[fill.OrderID]; !ok {
			continue
		}
		items = append(items, fill)
	}
	return items
}

func (p *Platform) fetchLiveTradeDecisionEvents(liveSessionID string, orderByID map[string]domain.Order) (map[string]domain.StrategyDecisionEvent, error) {
	ids := make([]string, 0, len(orderByID))
	seen := make(map[string]struct{})
	for _, order := range orderByID {
		decisionEventID := firstNonEmpty(
			stringValue(order.Metadata["decisionEventId"]),
			stringValue(mapValue(order.Metadata["executionProposal"])["decisionEventId"]),
			stringValue(mapValue(mapValue(order.Metadata["executionProposal"])["metadata"])["decisionEventId"]),
		)
		decisionEventID = strings.TrimSpace(decisionEventID)
		if decisionEventID == "" {
			continue
		}
		if _, ok := seen[decisionEventID]; ok {
			continue
		}
		seen[decisionEventID] = struct{}{}
		ids = append(ids, decisionEventID)
	}
	sort.Strings(ids)
	items := make(map[string]domain.StrategyDecisionEvent, len(ids))
	for _, decisionEventID := range ids {
		events, err := p.queryStrategyDecisionEvents(domain.StrategyDecisionEventQuery{
			LiveSessionID:   liveSessionID,
			DecisionEventID: decisionEventID,
			Limit:           1,
		})
		if err != nil {
			return nil, err
		}
		if len(events) == 0 {
			continue
		}
		items[decisionEventID] = events[0]
	}
	return items, nil
}

func (p *Platform) fetchLatestLiveTradeSnapshots(liveSessionID string, orderByID map[string]domain.Order) (map[string]domain.PositionAccountSnapshot, error) {
	orderIDs := make([]string, 0, len(orderByID))
	for orderID := range orderByID {
		orderIDs = append(orderIDs, orderID)
	}
	sort.Strings(orderIDs)
	items := make(map[string]domain.PositionAccountSnapshot, len(orderIDs))
	for _, orderID := range orderIDs {
		snapshots, err := p.queryPositionAccountSnapshots(domain.PositionAccountSnapshotQuery{
			LiveSessionID: liveSessionID,
			OrderID:       orderID,
			Limit:         1,
		})
		if err != nil {
			return nil, err
		}
		if len(snapshots) == 0 {
			continue
		}
		items[orderID] = snapshots[0]
	}
	return items, nil
}

func buildLiveTradeFillEvents(
	fills []domain.Fill,
	orderByID map[string]domain.Order,
	decisionByID map[string]domain.StrategyDecisionEvent,
) []liveTradeFillEvent {
	if decisionByID == nil {
		decisionByID = map[string]domain.StrategyDecisionEvent{}
	}
	items := make([]liveTradeFillEvent, 0, len(fills))
	for _, fill := range fills {
		order, ok := orderByID[fill.OrderID]
		if !ok {
			continue
		}
		if !liveTradeOrderCanContributeFill(order, fill) {
			continue
		}
		eventTime := fill.CreatedAt
		if fill.ExchangeTradeTime != nil && !fill.ExchangeTradeTime.IsZero() {
			eventTime = fill.ExchangeTradeTime.UTC()
		}
		decisionEventID := decisionEventIDFromOrder(order)
		items = append(items, liveTradeFillEvent{
			order:           order,
			fill:            fill,
			eventTime:       eventTime.UTC(),
			decisionEventID: decisionEventID,
			intent:          resolveLiveTradeIntentDetails(order, decisionByID[decisionEventID]),
		})
	}
	return items
}

func liveTradeOrderCanContributeFill(order domain.Order, fill domain.Fill) bool {
	switch strings.ToUpper(strings.TrimSpace(order.Status)) {
	case "FILLED", "PARTIALLY_FILLED":
		return true
	case "CANCELLED", "CANCELED", "REJECTED", "EXPIRED", "EXPIRED_IN_MATCH":
		if strings.TrimSpace(fill.ExchangeTradeID) != "" {
			return true
		}
		return tradingQuantityPositive(parseFloatValue(order.Metadata["filledQuantity"])) ||
			tradingQuantityPositive(parseFloatValue(order.Metadata["executedQty"])) ||
			tradingQuantityPositive(parseFloatValue(order.Metadata["cumQty"]))
	default:
		return false
	}
}

func decisionEventIDFromOrder(order domain.Order) string {
	return firstNonEmpty(
		stringValue(order.Metadata["decisionEventId"]),
		stringValue(mapValue(order.Metadata["executionProposal"])["decisionEventId"]),
		stringValue(mapValue(mapValue(order.Metadata["executionProposal"])["metadata"])["decisionEventId"]),
	)
}

func resolveLiveTradeIntentDetails(order domain.Order, decision domain.StrategyDecisionEvent) liveTradeIntentDetails {
	metadata := cloneMetadata(order.Metadata)
	proposal := cloneMetadata(mapValue(firstNonEmptyMapValue(metadata["executionProposal"], metadata["intent"])))
	proposalMeta := cloneMetadata(mapValue(proposal["metadata"]))
	eventProposal := cloneMetadata(mapValue(decision.ExecutionProposal))
	eventProposalMeta := cloneMetadata(mapValue(eventProposal["metadata"]))
	eventIntent := cloneMetadata(mapValue(decision.SignalIntent))
	eventIntentMeta := cloneMetadata(mapValue(eventIntent["metadata"]))
	decisionMeta := cloneMetadata(mapValue(decision.DecisionMetadata))
	signalBarDecision := cloneMetadata(mapValue(decisionMeta["signalBarDecision"]))
	currentPosition := firstNonEmptyMapValue(
		proposalMeta["currentPosition"],
		eventIntentMeta["currentPosition"],
		eventProposalMeta["currentPosition"],
	)
	targetPriceSource := firstNonEmpty(
		stringValue(signalBarDecision["targetPriceSource"]),
		stringValue(proposalMeta["targetPriceSource"]),
		stringValue(eventIntentMeta["targetPriceSource"]),
		stringValue(eventProposalMeta["targetPriceSource"]),
	)
	stopLossSource := firstNonEmpty(
		stringValue(signalBarDecision["stopLossSource"]),
		stringValue(currentPosition["stopLossSource"]),
		stringValue(proposalMeta["stopLossSource"]),
		stringValue(eventProposalMeta["stopLossSource"]),
	)
	return liveTradeIntentDetails{
		role: firstNonEmpty(
			stringValue(proposal["role"]),
			stringValue(eventProposal["role"]),
			stringValue(eventIntent["role"]),
			liveOrderRoleFromOrder(order),
		),
		reason: firstNonEmpty(
			stringValue(proposal["reason"]),
			stringValue(eventProposal["reason"]),
			stringValue(eventIntent["reason"]),
			decision.Reason,
			stringValue(metadata["reason"]),
		),
		signalKind: firstNonEmpty(
			stringValue(proposal["signalKind"]),
			stringValue(eventProposal["signalKind"]),
			stringValue(eventIntent["signalKind"]),
			decision.SignalKind,
		),
		targetPriceSource: targetPriceSource,
		stopLossSource:    stopLossSource,
		recoveryTriggered: boolValue(metadata["recoveryTriggered"]) ||
			boolValue(proposalMeta["recoveryTriggered"]) ||
			boolValue(eventProposalMeta["recoveryTriggered"]),
		manualTriggered: strings.Contains(strings.ToLower(firstNonEmpty(
			stringValue(proposal["reason"]),
			stringValue(eventProposal["reason"]),
			decision.Reason,
		)), "manual"),
	}
}

func currentLivePositionBySymbol(session domain.LiveSession, positions []domain.Position) map[string]domain.Position {
	result := make(map[string]domain.Position)
	for _, item := range positions {
		if item.AccountID != session.AccountID {
			continue
		}
		result[NormalizeSymbol(item.Symbol)] = item
	}
	return result
}

func newLiveTradePairBuilder(index int, session domain.LiveSession, symbol, side string) *liveTradePairBuilder {
	return &liveTradePairBuilder{
		index:          index,
		liveSessionID:  session.ID,
		accountID:      session.AccountID,
		strategyID:     session.StrategyID,
		symbol:         NormalizeSymbol(symbol),
		side:           side,
		entryOrderSeen: make(map[string]struct{}),
		exitOrderSeen:  make(map[string]struct{}),
		notesSeen:      make(map[string]struct{}),
		exitVerdict:    "open",
	}
}

func (b *liveTradePairBuilder) addEntry(event liveTradeFillEvent, qty, fee float64) {
	if b == nil || qty <= 0 {
		return
	}
	if b.entryAt.IsZero() || event.eventTime.Before(b.entryAt) {
		b.entryAt = event.eventTime
	}
	if b.symbol == "" {
		b.symbol = NormalizeSymbol(event.order.Symbol)
	}
	if b.side == "" {
		b.side = liveTradeSideFromOrder(event.order.Side)
	}
	b.entryQty += qty
	b.entryNotional += qty * event.fill.Price
	b.fees += fee
	b.entryFillCount++
	b.appendEntryOrderID(event.order.ID)
	if b.entryReason == "" {
		b.entryReason = liveTradeReasonLabel(event.intent.reason)
	}
}

func (b *liveTradePairBuilder) addExit(event liveTradeFillEvent, qty, fee, realizedPnL float64) {
	if b == nil || qty <= 0 {
		return
	}
	if b.exitAt.IsZero() || event.eventTime.After(b.exitAt) {
		b.exitAt = event.eventTime
	}
	b.exitQty += qty
	b.exitNotional += qty * event.fill.Price
	b.realizedPnL += realizedPnL
	b.fees += fee
	b.exitFillCount++
	b.appendExitOrderID(event.order.ID)
	b.lastExitOrderID = event.order.ID
	b.exitReason = liveTradeReasonLabel(event.intent.reason)
	b.exitClassifier = classifyLiveTradeExit(event.intent)
	b.recoveryExit = b.recoveryExit || event.intent.recoveryTriggered
	if !event.order.EffectiveReduceOnly() && !event.order.EffectiveClosePosition() {
		b.hasUnsafeExitOrder = true
		b.addNote("exit-order-without-reduce-only")
	}
	if strings.TrimSpace(strings.ToLower(event.intent.role)) != "exit" &&
		!event.order.EffectiveReduceOnly() &&
		!event.order.EffectiveClosePosition() {
		b.addNote("closing-fill-came-from-entry-order")
	}
}

func (b *liveTradePairBuilder) finalizeClosed() domain.LiveTradePair {
	verdict := "normal"
	if b.exitQty <= 0 {
		verdict = "orphan-exit"
	} else if b.hasUnsafeExitOrder {
		verdict = "mismatch"
	} else if b.recoveryExit {
		verdict = "recovery-close"
	}
	b.exitVerdict = verdict
	return b.build(0, 0)
}

func (b *liveTradePairBuilder) finalizeOpen(position domain.Position) domain.LiveTradePair {
	openQty := b.entryQty - b.exitQty
	if openQty < 0 {
		openQty = 0
	}
	if NormalizeSymbol(position.Symbol) == b.symbol &&
		strings.EqualFold(position.AccountID, b.accountID) &&
		strings.EqualFold(position.Side, b.side) &&
		position.MarkPrice > 0 &&
		openQty > 0 &&
		b.entryAvgPrice() > 0 {
		switch strings.ToUpper(strings.TrimSpace(b.side)) {
		case "SHORT":
			b.unrealizedPnL = (b.entryAvgPrice() - position.MarkPrice) * openQty
		default:
			b.unrealizedPnL = (position.MarkPrice - b.entryAvgPrice()) * openQty
		}
	} else if openQty > 0 {
		b.addNote("live-position-snapshot-unavailable")
	}
	b.exitVerdict = "open"
	return b.build(openQty, b.unrealizedPnL)
}

func (b *liveTradePairBuilder) build(openQty, unrealizedPnL float64) domain.LiveTradePair {
	status := "closed"
	var exitAt *time.Time
	if tradingQuantityPositive(openQty) || b.exitAt.IsZero() {
		status = "open"
	} else {
		exitTime := b.exitAt.UTC()
		exitAt = &exitTime
	}
	netPnL := b.realizedPnL + unrealizedPnL - b.fees
	return domain.LiveTradePair{
		ID:             fmt.Sprintf("trade-pair-%s-%s-%d", b.liveSessionID, b.symbol, b.index),
		LiveSessionID:  b.liveSessionID,
		AccountID:      b.accountID,
		StrategyID:     b.strategyID,
		Symbol:         b.symbol,
		Status:         status,
		Side:           b.side,
		EntryOrderIDs:  append([]string(nil), b.entryOrderIDs...),
		ExitOrderIDs:   append([]string(nil), b.exitOrderIDs...),
		EntryAt:        b.entryAt.UTC(),
		ExitAt:         exitAt,
		EntryAvgPrice:  b.entryAvgPrice(),
		ExitAvgPrice:   b.exitAvgPrice(),
		EntryQuantity:  b.entryQty,
		ExitQuantity:   b.exitQty,
		OpenQuantity:   openQty,
		EntryReason:    b.entryReason,
		ExitReason:     b.exitReason,
		ExitClassifier: b.exitClassifier,
		ExitVerdict:    b.exitVerdict,
		RealizedPnL:    b.realizedPnL,
		UnrealizedPnL:  unrealizedPnL,
		Fees:           b.fees,
		NetPnL:         netPnL,
		EntryFillCount: b.entryFillCount,
		ExitFillCount:  b.exitFillCount,
		Notes:          append([]string(nil), b.notes...),
	}
}

func (b *liveTradePairBuilder) entryAvgPrice() float64 {
	if b == nil || b.entryQty <= 0 {
		return 0
	}
	return b.entryNotional / b.entryQty
}

func (b *liveTradePairBuilder) exitAvgPrice() float64 {
	if b == nil || b.exitQty <= 0 {
		return 0
	}
	return b.exitNotional / b.exitQty
}

func (b *liveTradePairBuilder) appendEntryOrderID(orderID string) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return
	}
	if _, ok := b.entryOrderSeen[orderID]; ok {
		return
	}
	b.entryOrderSeen[orderID] = struct{}{}
	b.entryOrderIDs = append(b.entryOrderIDs, orderID)
}

func (b *liveTradePairBuilder) appendExitOrderID(orderID string) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return
	}
	if _, ok := b.exitOrderSeen[orderID]; ok {
		return
	}
	b.exitOrderSeen[orderID] = struct{}{}
	b.exitOrderIDs = append(b.exitOrderIDs, orderID)
}

func (b *liveTradePairBuilder) addNote(note string) {
	note = strings.TrimSpace(note)
	if note == "" {
		return
	}
	if _, ok := b.notesSeen[note]; ok {
		return
	}
	b.notesSeen[note] = struct{}{}
	b.notes = append(b.notes, note)
}

func proportionalFee(totalFee, partialQty, totalQty float64) float64 {
	if totalFee == 0 || partialQty <= 0 || totalQty <= 0 {
		return 0
	}
	if partialQty >= totalQty {
		return totalFee
	}
	return totalFee * (partialQty / totalQty)
}

func tradePairSignedQuantity(side string, qty float64) float64 {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "SELL", "SHORT":
		return -qty
	default:
		return qty
	}
}

func liveTradeSideFromOrder(side string) string {
	switch strings.ToUpper(strings.TrimSpace(side)) {
	case "SELL", "SHORT":
		return "SHORT"
	default:
		return "LONG"
	}
}

func liveTradeSideFromSignedQty(qty float64) string {
	if qty < 0 {
		return "SHORT"
	}
	return "LONG"
}

func classifyLiveTradeExit(details liveTradeIntentDetails) string {
	if details.recoveryTriggered {
		return "recovery"
	}
	if details.manualTriggered {
		return "manual"
	}
	switch normalizeStrategyReasonTag(details.reason) {
	case "pt":
		return "TP"
	case "sl":
		if strings.EqualFold(details.targetPriceSource, "trailing-stop") || strings.EqualFold(details.stopLossSource, "trailing-stop") {
			return "TSL"
		}
		return "SL"
	default:
		return strings.ToUpper(strings.TrimSpace(details.reason))
	}
}

func liveTradeReasonLabel(reason string) string {
	tag := normalizeStrategyReasonTag(reason)
	switch tag {
	case "":
		return ""
	case "pt":
		return "PT"
	case "sl":
		return "SL"
	default:
		return strings.ToUpper(strings.ReplaceAll(tag, "_", "-"))
	}
}

func normalizeLiveTradePairStatus(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "", "all":
		return ""
	case "open", "closed":
		return value
	default:
		return ""
	}
}

func normalizeLiveTradePairLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultLiveTradePairLimit
	case limit > maxLiveTradePairLimit:
		return maxLiveTradePairLimit
	default:
		return limit
	}
}

func filterAndLimitLiveTradePairs(items []domain.LiveTradePair, status string, limit int) []domain.LiveTradePair {
	filtered := make([]domain.LiveTradePair, 0, len(items))
	for _, item := range items {
		if status != "" && !strings.EqualFold(item.Status, status) {
			continue
		}
		filtered = append(filtered, item)
		if len(filtered) >= limit {
			break
		}
	}
	return filtered
}

func (p *Platform) enrichLiveTradePairs(items []domain.LiveTradePair, orderByID map[string]domain.Order) {
	if len(items) == 0 {
		return
	}

	decisionByID := make(map[string]domain.StrategyDecisionEvent)
	var allExitOrderIDs []string
	var allDecisionEventIDs []string

	for _, item := range items {
		for _, orderID := range item.EntryOrderIDs {
			if order, ok := orderByID[orderID]; ok {
				if id := decisionEventIDFromOrder(order); id != "" {
					allDecisionEventIDs = append(allDecisionEventIDs, id)
				}
			}
		}
		for _, orderID := range item.ExitOrderIDs {
			allExitOrderIDs = append(allExitOrderIDs, orderID)
			if order, ok := orderByID[orderID]; ok {
				if id := decisionEventIDFromOrder(order); id != "" {
					allDecisionEventIDs = append(allDecisionEventIDs, id)
				}
			}
		}
	}

	if len(allDecisionEventIDs) > 0 {
		if events, err := p.queryStrategyDecisionEvents(domain.StrategyDecisionEventQuery{
			DecisionEventIDs: allDecisionEventIDs,
		}); err == nil {
			for _, event := range events {
				if _, ok := decisionByID[event.ID]; !ok {
					decisionByID[event.ID] = event
				}
			}
		}
	}

	closeVerificationsByOrderID := make(map[string]domain.OrderCloseVerification)
	if len(allExitOrderIDs) > 0 {
		if verifications, err := p.store.QueryOrderCloseVerifications(domain.OrderCloseVerificationQuery{
			OrderIDs: allExitOrderIDs,
		}); err == nil {
			for _, v := range verifications {
				if _, ok := closeVerificationsByOrderID[v.OrderID]; !ok {
					closeVerificationsByOrderID[v.OrderID] = v
				}
			}
		}
	}

	for index := range items {
		pair := &items[index]
		if len(pair.EntryOrderIDs) > 0 {
			if order, ok := orderByID[pair.EntryOrderIDs[0]]; ok {
				intent := resolveLiveTradeIntentDetails(order, decisionByID[decisionEventIDFromOrder(order)])
				if pair.EntryReason == "" {
					pair.EntryReason = liveTradeReasonLabel(intent.reason)
				}
			}
		}
		if len(pair.ExitOrderIDs) > 0 {
			lastExitOrderID := pair.ExitOrderIDs[len(pair.ExitOrderIDs)-1]
			if order, ok := orderByID[lastExitOrderID]; ok {
				intent := resolveLiveTradeIntentDetails(order, decisionByID[decisionEventIDFromOrder(order)])
				if pair.ExitReason == "" {
					pair.ExitReason = liveTradeReasonLabel(intent.reason)
				}
				if pair.ExitClassifier == "" {
					pair.ExitClassifier = classifyLiveTradeExit(intent)
				}

				if pair.Status == "closed" {
					if verification, ok := closeVerificationsByOrderID[lastExitOrderID]; ok {
						if verification.VerifiedClosed {
							pair.ExitVerdict = "normal"
						} else {
							pair.ExitVerdict = "mismatch"
						}
					}
				}

				if pair.ExitVerdict == "normal" && intent.recoveryTriggered {
					pair.ExitVerdict = "recovery-close"
				}
			}
		}
	}
}
