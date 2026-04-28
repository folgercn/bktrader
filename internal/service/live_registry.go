package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type LiveExecutionAdapter interface {
	Key() string
	Describe() map[string]any
	ValidateAccountConfig(config map[string]any) error
	SubmitOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error)
	SyncOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error)
	CancelOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error)
}

type LiveAccountSyncAdapter interface {
	SyncAccountSnapshot(platform *Platform, account domain.Account, binding map[string]any) (domain.Account, error)
}

type LiveAccountReconcileAdapter interface {
	FetchRecentOrders(account domain.Account, binding map[string]any, symbol string, lookbackHours int) ([]map[string]any, error)
	FetchRecentTrades(account domain.Account, binding map[string]any, symbol string, lookbackHours int) ([]LiveFillReport, error)
}

type LiveOrderSubmission struct {
	Status          string         `json:"status"`
	ExchangeOrderID string         `json:"exchangeOrderId"`
	AcceptedAt      string         `json:"acceptedAt"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type LiveFillReport struct {
	Price      float64        `json:"price"`
	Quantity   float64        `json:"quantity"`
	Fee        float64        `json:"fee"`
	FundingPnL float64        `json:"fundingPnl"`
	Source     FillSource     `json:"source,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ExchangeFillReport struct {
	Exchange        string         `json:"exchange,omitempty"`
	AdapterKey      string         `json:"adapterKey,omitempty"`
	AccountID       string         `json:"accountId,omitempty"`
	OrderID         string         `json:"orderId,omitempty"`
	ExchangeOrderID string         `json:"exchangeOrderId,omitempty"`
	ExchangeTradeID string         `json:"exchangeTradeId,omitempty"`
	Symbol          string         `json:"symbol,omitempty"`
	Side            string         `json:"side,omitempty"`
	Price           float64        `json:"price"`
	Quantity        float64        `json:"quantity"`
	Fee             float64        `json:"fee"`
	FeeAsset        string         `json:"feeAsset,omitempty"`
	RealizedPnL     float64        `json:"realizedPnl,omitempty"`
	FundingPnL      float64        `json:"fundingPnl,omitempty"`
	TradeTime       string         `json:"tradeTime,omitempty"`
	Source          FillSource     `json:"source"`
	Raw             map[string]any `json:"raw,omitempty"`
}

func (report ExchangeFillReport) LiveFillReport() LiveFillReport {
	metadata := map[string]any{
		"source":          "exchange-fill-report",
		"reportSource":    "exchange-fill-report",
		"exchange":        report.Exchange,
		"adapterKey":      report.AdapterKey,
		"accountId":       report.AccountID,
		"orderId":         report.OrderID,
		"exchangeOrderId": report.ExchangeOrderID,
		"tradeId":         report.ExchangeTradeID,
		"commissionAsset": report.FeeAsset,
		"realizedPnl":     report.RealizedPnL,
		"tradeTime":       report.TradeTime,
	}
	if report.Raw != nil {
		metadata["raw"] = report.Raw
	}
	return LiveFillReport{
		Price:      report.Price,
		Quantity:   report.Quantity,
		Fee:        report.Fee,
		FundingPnL: report.FundingPnL,
		Source:     report.Source,
		Metadata:   metadata,
	}
}

type LiveOrderSync struct {
	Status     string           `json:"status"`
	SyncedAt   string           `json:"syncedAt"`
	Fills      []LiveFillReport `json:"fills,omitempty"`
	Metadata   map[string]any   `json:"metadata,omitempty"`
	Terminal   bool             `json:"terminal"`
	FeeSource  string           `json:"feeSource"`
	FundingSrc string           `json:"fundingSource"`
}

type liveAdapterBinding struct {
	AdapterKey     string         `json:"adapterKey"`
	ConnectionMode string         `json:"connectionMode"`
	AccountMode    string         `json:"accountMode"`
	MarginMode     string         `json:"marginMode"`
	PositionMode   string         `json:"positionMode"`
	ExecutionMode  string         `json:"executionMode"`
	FeeSource      string         `json:"feeSource"`
	FundingSource  string         `json:"fundingSource"`
	Sandbox        bool           `json:"sandbox"`
	RESTBaseURL    string         `json:"restBaseUrl,omitempty"`
	WSBaseURL      string         `json:"wsBaseUrl,omitempty"`
	RecvWindowMs   int            `json:"recvWindowMs,omitempty"`
	CredentialRefs map[string]any `json:"credentialRefs,omitempty"`
	UpdatedAt      string         `json:"updatedAt"`
}

func normalizeLiveAdapterKey(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "binance-futures"
	}
	return value
}

func (p *Platform) registerLiveAdapter(adapter LiveExecutionAdapter) {
	if adapter == nil {
		return
	}
	if p.liveAdapters == nil {
		p.liveAdapters = make(map[string]LiveExecutionAdapter)
	}
	p.liveAdapters[normalizeLiveAdapterKey(adapter.Key())] = adapter
}

func (p *Platform) registerBuiltInLiveAdapters() {
	p.registerLiveAdapter(binanceFuturesLiveAdapter{})
}

func (p *Platform) LiveAdapters() []map[string]any {
	items := make([]map[string]any, 0, len(p.liveAdapters))
	for _, adapter := range p.liveAdapters {
		items = append(items, adapter.Describe())
	}
	return items
}

func (p *Platform) BindLiveAccount(accountID string, binding map[string]any) (domain.Account, error) {
	account, err := p.store.GetAccount(accountID)
	if err != nil {
		return domain.Account{}, err
	}
	if account.Mode != "LIVE" {
		return domain.Account{}, fmt.Errorf("account %s is not a LIVE account", accountID)
	}

	adapterKey := normalizeLiveAdapterKey(stringValue(binding["adapterKey"]))
	adapter, ok := p.liveAdapters[adapterKey]
	if !ok {
		return domain.Account{}, fmt.Errorf("live adapter not registered: %s", adapterKey)
	}

	credentialRefs := defaultLiveCredentialRefs(adapterKey, boolValue(binding["sandbox"]), binding["credentialRefs"])
	normalized := map[string]any{
		"adapterKey":     adapterKey,
		"connectionMode": firstNonEmpty(stringValue(binding["connectionMode"]), "api-key-ref"),
		"accountMode":    firstNonEmpty(strings.ToUpper(strings.TrimSpace(stringValue(binding["accountMode"]))), "ONE_WAY"),
		"marginMode":     firstNonEmpty(strings.ToUpper(strings.TrimSpace(stringValue(binding["marginMode"]))), "CROSSED"),
		"positionMode":   firstNonEmpty(strings.ToUpper(strings.TrimSpace(stringValue(binding["positionMode"]))), "ONE_WAY"),
		"executionMode":  normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])),
		"feeSource":      "exchange",
		"fundingSource":  "exchange",
		"sandbox":        boolValue(binding["sandbox"]),
		"restBaseUrl":    stringValue(binding["restBaseUrl"]),
		"wsBaseUrl":      stringValue(binding["wsBaseUrl"]),
		"recvWindowMs":   maxIntValue(binding["recvWindowMs"], p.runtimePolicy.BinanceRecvWindowMs),
		"credentialRefs": credentialRefs,
	}
	if err := adapter.ValidateAccountConfig(normalized); err != nil {
		return domain.Account{}, err
	}

	account.Metadata = cloneMetadata(account.Metadata)
	account.Metadata["liveBinding"] = liveAdapterBinding{
		AdapterKey:     adapterKey,
		ConnectionMode: stringValue(normalized["connectionMode"]),
		AccountMode:    stringValue(normalized["accountMode"]),
		MarginMode:     stringValue(normalized["marginMode"]),
		PositionMode:   stringValue(normalized["positionMode"]),
		ExecutionMode:  stringValue(normalized["executionMode"]),
		FeeSource:      "exchange",
		FundingSource:  "exchange",
		Sandbox:        boolValue(normalized["sandbox"]),
		RESTBaseURL:    stringValue(normalized["restBaseUrl"]),
		WSBaseURL:      stringValue(normalized["wsBaseUrl"]),
		RecvWindowMs:   maxIntValue(normalized["recvWindowMs"], p.runtimePolicy.BinanceRecvWindowMs),
		CredentialRefs: normalizeCredentialRefs(normalized["credentialRefs"]),
		UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
	}
	if account.Status == "PENDING_SETUP" {
		account.Status = "CONFIGURED"
	}
	return p.store.UpdateAccount(account)
}

func defaultLiveCredentialRefs(adapterKey string, sandbox bool, raw any) map[string]any {
	refs := normalizeCredentialRefs(raw)
	switch normalizeLiveAdapterKey(adapterKey) {
	case "binance-futures":
		if strings.TrimSpace(stringValue(refs["apiKeyRef"])) == "" {
			if sandbox {
				refs["apiKeyRef"] = "BINANCE_TESTNET_API_KEY"
			} else {
				refs["apiKeyRef"] = "BINANCE_API_KEY"
			}
		}
		if strings.TrimSpace(stringValue(refs["apiSecretRef"])) == "" {
			if sandbox {
				refs["apiSecretRef"] = "BINANCE_TESTNET_API_SECRET"
			} else {
				refs["apiSecretRef"] = "BINANCE_API_SECRET"
			}
		}
	}
	return refs
}

type binanceFuturesLiveAdapter struct{}

func (a binanceFuturesLiveAdapter) Key() string {
	return "binance-futures"
}

func (a binanceFuturesLiveAdapter) Describe() map[string]any {
	return map[string]any{
		"key":                a.Key(),
		"name":               "Binance Futures Adapter",
		"environments":       []string{"live", "testnet"},
		"requiresCredential": true,
		"feeSource":          "exchange",
		"fundingSource":      "exchange",
		"positionModes":      []string{"ONE_WAY", "HEDGE"},
		"marginModes":        []string{"CROSSED", "ISOLATED"},
		"executionModes":     []string{"mock", "rest"},
	}
}

func (a binanceFuturesLiveAdapter) ValidateAccountConfig(config map[string]any) error {
	credentialRefs := normalizeCredentialRefs(config["credentialRefs"])
	if strings.TrimSpace(stringValue(credentialRefs["apiKeyRef"])) == "" {
		return fmt.Errorf("live adapter binding requires credentialRefs.apiKeyRef")
	}
	if strings.TrimSpace(stringValue(credentialRefs["apiSecretRef"])) == "" {
		return fmt.Errorf("live adapter binding requires credentialRefs.apiSecretRef")
	}
	positionMode := strings.ToUpper(strings.TrimSpace(stringValue(config["positionMode"])))
	if positionMode != "ONE_WAY" && positionMode != "HEDGE" {
		return fmt.Errorf("unsupported positionMode: %s", positionMode)
	}
	marginMode := strings.ToUpper(strings.TrimSpace(stringValue(config["marginMode"])))
	if marginMode != "CROSSED" && marginMode != "ISOLATED" {
		return fmt.Errorf("unsupported marginMode: %s", marginMode)
	}
	return nil
}

func (a binanceFuturesLiveAdapter) SubmitOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	switch normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])) {
	case "rest":
		return a.submitRESTOrder(account, order, binding)
	default:
		return a.submitMockOrder(account, order, binding)
	}
}

func (a binanceFuturesLiveAdapter) SyncOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	switch normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])) {
	case "rest":
		return a.syncRESTOrder(account, order, binding)
	default:
		return a.syncMockOrder(account, order, binding)
	}
}

func (a binanceFuturesLiveAdapter) SyncAccountSnapshot(platform *Platform, account domain.Account, binding map[string]any) (domain.Account, error) {
	if platform == nil {
		return domain.Account{}, fmt.Errorf("platform is required")
	}
	return platform.syncLiveAccountFromBinance(account, binding)
}

func (a binanceFuturesLiveAdapter) FetchRecentOrders(account domain.Account, binding map[string]any, symbol string, lookbackHours int) ([]map[string]any, error) {
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return nil, err
	}
	endTime := time.Now().UTC()
	startTime := endTime.Add(-time.Duration(maxInt(lookbackHours, 1)) * time.Hour)
	params := map[string]string{
		"symbol":     NormalizeSymbol(symbol),
		"timestamp":  fmt.Sprintf("%d", endTime.UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
		"startTime":  fmt.Sprintf("%d", startTime.UnixMilli()),
		"endTime":    fmt.Sprintf("%d", endTime.UnixMilli()),
		"limit":      "1000",
	}
	payload, err := doBinanceSignedRequest(http.MethodGet, resolved, "/fapi/v1/allOrders", params, binanceRESTCategoryHistoryRead)
	if err != nil {
		return nil, err
	}
	var orders []map[string]any
	if err := json.Unmarshal(payload, &orders); err != nil {
		return nil, err
	}
	return orders, nil
}

func (a binanceFuturesLiveAdapter) FetchRecentTrades(account domain.Account, binding map[string]any, symbol string, lookbackHours int) ([]LiveFillReport, error) {
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return nil, err
	}
	return a.fetchRESTTradeReportsForSymbol(account, binding, resolved, symbol, lookbackHours)
}

func (a binanceFuturesLiveAdapter) PersistsLiveAccountSyncSuccess() bool {
	return true
}

func (a binanceFuturesLiveAdapter) CancelOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	switch normalizeLiveExecutionMode(binding["executionMode"], boolValue(binding["sandbox"])) {
	case "rest":
		return a.cancelRESTOrder(account, order, binding)
	default:
		return a.cancelMockOrder(account, order, binding)
	}
}

func (a binanceFuturesLiveAdapter) submitMockOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	credentialRefs := normalizeCredentialRefs(binding["credentialRefs"])
	if strings.TrimSpace(stringValue(credentialRefs["apiKeyRef"])) == "" {
		return LiveOrderSubmission{}, fmt.Errorf("live adapter binding requires credentialRefs.apiKeyRef")
	}
	if strings.TrimSpace(stringValue(credentialRefs["apiSecretRef"])) == "" {
		return LiveOrderSubmission{}, fmt.Errorf("live adapter binding requires credentialRefs.apiSecretRef")
	}
	acceptedAt := time.Now().UTC()
	return LiveOrderSubmission{
		Status:          "ACCEPTED",
		ExchangeOrderID: fmt.Sprintf("binance-mock-%d", acceptedAt.UnixNano()),
		AcceptedAt:      acceptedAt.Format(time.RFC3339),
		Metadata: map[string]any{
			"adapterMode":    "mock-submission",
			"executionMode":  "mock",
			"accountMode":    stringValue(binding["accountMode"]),
			"marginMode":     stringValue(binding["marginMode"]),
			"positionMode":   stringValue(binding["positionMode"]),
			"positionSide":   stringValue(order.Metadata["positionSide"]),
			"sandbox":        boolValue(binding["sandbox"]),
			"symbol":         order.Symbol,
			"side":           order.Side,
			"type":           order.Type,
			"quantity":       order.Quantity,
			"submittedPrice": order.Price,
			"feeSource":      "exchange",
			"fundingSource":  "exchange",
			"exchange":       account.Exchange,
		},
	}, nil
}

func (a binanceFuturesLiveAdapter) syncMockOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	if stringValue(order.Metadata["exchangeOrderId"]) == "" {
		return LiveOrderSync{}, fmt.Errorf("live order has no exchangeOrderId")
	}
	exchangeOrderID := stringValue(order.Metadata["exchangeOrderId"])
	syncedAt := time.Now().UTC()
	executionPrice := order.Price
	if executionPrice <= 0 {
		executionPrice = resolveExecutionPrice(order)
	}
	fee := executionPrice * order.Quantity * 0.001
	return LiveOrderSync{
		Status:   "FILLED",
		SyncedAt: syncedAt.Format(time.RFC3339),
		Fills: []LiveFillReport{{
			Price:    executionPrice,
			Quantity: order.Quantity,
			Fee:      fee,
			Metadata: map[string]any{
				"source":          "exchange-sync",
				"exchange":        account.Exchange,
				"adapterKey":      a.Key(),
				"exchangeOrderId": exchangeOrderID,
				"tradeTime":       syncedAt.Format(time.RFC3339),
				"executionMode":   "mock",
			},
		}},
		Metadata: map[string]any{
			"adapterMode":   "mock-sync",
			"executionMode": "mock",
			"sandbox":       boolValue(binding["sandbox"]),
		},
		Terminal:   true,
		FeeSource:  "exchange",
		FundingSrc: "exchange",
	}, nil
}

func (a binanceFuturesLiveAdapter) cancelMockOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	return LiveOrderSync{
		Status:   "CANCELLED",
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
		Metadata: map[string]any{
			"adapterMode":   "mock-cancel",
			"executionMode": "mock",
			"exchange":      account.Exchange,
		},
		Terminal:   true,
		FeeSource:  "exchange",
		FundingSrc: "exchange",
	}, nil
}

func (a binanceFuturesLiveAdapter) submitRESTOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return LiveOrderSubmission{}, err
	}
	normalizedOrder, rules, err := a.normalizeRESTOrder(order, resolved)
	if err != nil {
		return LiveOrderSubmission{}, err
	}
	params := map[string]string{
		"symbol":           NormalizeSymbol(normalizedOrder.Symbol),
		"side":             strings.ToUpper(strings.TrimSpace(normalizedOrder.Side)),
		"type":             strings.ToUpper(strings.TrimSpace(firstNonEmpty(normalizedOrder.Type, "MARKET"))),
		"timestamp":        fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow":       fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
		"newOrderRespType": "RESULT",
		"newClientOrderId": normalizedOrder.ID,
	}
	if normalizedOrder.Quantity > 0 {
		params["quantity"] = formatBinanceDecimal(normalizedOrder.Quantity, rules.StepSize)
	}
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(normalizedOrder.Type, "MARKET")))
	timeInForce := strings.ToUpper(strings.TrimSpace(stringValue(normalizedOrder.Metadata["timeInForce"])))
	if boolValue(normalizedOrder.Metadata["postOnly"]) {
		timeInForce = "GTX"
	}
	if normalizedOrder.Price > 0 && orderType != "MARKET" {
		params["price"] = formatBinanceDecimal(normalizedOrder.Price, rules.TickSize)
	}
	if orderType != "MARKET" && timeInForce != "" {
		params["timeInForce"] = timeInForce
	}
	if positionSide := resolveBinancePositionSideForSubmission(binding, normalizedOrder); positionSide != "" {
		params["positionSide"] = positionSide
	}
	if shouldSendBinanceReduceOnlyFlag(binding, normalizedOrder) {
		params["reduceOnly"] = "true"
	}
	if normalizedOrder.EffectiveClosePosition() {
		params["closePosition"] = "true"
	}
	responseBody, err := doBinanceSignedRequest(http.MethodPost, resolved, "/fapi/v1/order", params, binanceRESTCategoryTradeCritical)
	if err != nil {
		return LiveOrderSubmission{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return LiveOrderSubmission{}, err
	}
	status := mapBinanceOrderStatus(stringValue(payload["status"]))
	if status == "" {
		status = "ACCEPTED"
	}
	acceptedAt := parseBinanceMillisToRFC3339(payload["updateTime"])
	if acceptedAt == "" {
		acceptedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return LiveOrderSubmission{
		Status:          status,
		ExchangeOrderID: normalizeBinanceOrderID(payload["orderId"], payload["clientOrderId"]),
		AcceptedAt:      acceptedAt,
		Metadata: map[string]any{
			"adapterMode":        "rest",
			"executionMode":      "rest",
			"restBaseUrl":        resolved.BaseURL,
			"requestPath":        "/fapi/v1/order",
			"apiKeyRef":          resolved.APIKeyRef,
			"apiSecretRef":       resolved.APISecretRef,
			"requestReady":       true,
			"networkExecuted":    true,
			"exchange":           account.Exchange,
			"binanceStatus":      stringValue(payload["status"]),
			"clientOrderId":      stringValue(payload["clientOrderId"]),
			"cumQty":             parseFloatValue(payload["cumQty"]),
			"executedQty":        parseFloatValue(payload["executedQty"]),
			"avgPrice":           parseFloatValue(payload["avgPrice"]),
			"origType":           stringValue(payload["origType"]),
			"timeInForce":        stringValue(payload["timeInForce"]),
			"updateTime":         acceptedAt,
			"rawQuantity":        order.Quantity,
			"rawPriceReference":  firstPositive(order.Price, parseFloatValue(order.Metadata["priceHint"])),
			"normalizedQuantity": normalizedOrder.Quantity,
			"normalizedPrice":    normalizedOrder.Price,
			"normalization":      cloneMetadata(mapValue(normalizedOrder.Metadata["normalization"])),
			"symbolRules": map[string]any{
				"tickSize":    rules.TickSize,
				"stepSize":    rules.StepSize,
				"minQty":      rules.MinQty,
				"maxQty":      rules.MaxQty,
				"minNotional": rules.MinNotional,
			},
		},
	}, nil
}

func (a binanceFuturesLiveAdapter) syncRESTOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return LiveOrderSync{}, err
	}
	params := map[string]string{
		"symbol":     NormalizeSymbol(order.Symbol),
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	}
	if exchangeOrderID := normalizeBinanceOrderID(order.Metadata["exchangeOrderId"], nil); exchangeOrderID != "" {
		params["orderId"] = exchangeOrderID
	} else {
		params["origClientOrderId"] = order.ID
	}
	responseBody, err := doBinanceSignedRequest(http.MethodGet, resolved, "/fapi/v1/order", params, binanceRESTCategoryTradeCritical)
	if err != nil {
		return LiveOrderSync{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return LiveOrderSync{}, err
	}
	status := mapBinanceOrderStatus(stringValue(payload["status"]))
	if status == "" {
		status = firstNonEmpty(order.Status, "ACCEPTED")
	}
	filledQty := parseFloatValue(payload["executedQty"])
	avgPrice := firstPositive(parseFloatValue(payload["avgPrice"]), parseFloatValue(payload["price"]))
	fills := []LiveFillReport{}
	tradeReports, tradeErr := a.fetchRESTTradeReports(account, order, binding, resolved)
	terminal := status == "FILLED" || status == "CANCELLED" || status == "REJECTED"
	emptyTradeRetryRequired := false
	attempts := 0
	if val, ok := order.Metadata["emptyTradeSyncAttempts"]; ok {
		if num, ok := toFloat64(val); ok {
			attempts = int(num)
		}
	}
	if tradeErr == nil && len(tradeReports) > 0 && terminal {
		fills = tradeReports
	} else if filledQty > 0 && strings.EqualFold(status, "FILLED") {
		if tradeErr == nil && len(tradeReports) == 0 {
			if attempts < 3 {
				emptyTradeRetryRequired = true
			}
		} else if tradeErr != nil {
			emptyTradeRetryRequired = true
		}

		if !emptyTradeRetryRequired {
			fills = append(fills, LiveFillReport{
				Price:    avgPrice,
				Quantity: filledQty,
				Fee:      0,
				Source:   FillSourceSynthetic,
				Metadata: map[string]any{
					"source":          "binance-order-query",
					"reportSource":    "binance-order-query",
					"exchange":        account.Exchange,
					"adapterKey":      a.Key(),
					"exchangeOrderId": normalizeBinanceOrderID(payload["orderId"], order.Metadata["exchangeOrderId"]),
					"clientOrderId":   stringValue(payload["clientOrderId"]),
					"tradeTime":       firstNonEmpty(parseBinanceMillisToRFC3339(payload["updateTime"]), time.Now().UTC().Format(time.RFC3339)),
					"executionMode":   "rest",
					"syntheticFill":   true,
				},
			})
		}
	}
	resolvedSyncAt := firstNonEmpty(parseBinanceMillisToRFC3339(payload["updateTime"]), time.Now().UTC().Format(time.RFC3339))
	totalFee := 0.0
	totalRealizedPnL := 0.0
	for _, fill := range fills {
		totalFee += fill.Fee
		if fill.Metadata != nil {
			totalRealizedPnL += parseFloatValue(fill.Metadata["realizedPnl"])
		}
	}
	return LiveOrderSync{
		Status:   status,
		SyncedAt: resolvedSyncAt,
		Fills:    fills,
		Metadata: map[string]any{
			"adapterMode":             "rest",
			"executionMode":           "rest",
			"restBaseUrl":             resolved.BaseURL,
			"requestPath":             "/fapi/v1/order",
			"apiKeyRef":               resolved.APIKeyRef,
			"apiSecretRef":            resolved.APISecretRef,
			"requestReady":            true,
			"networkExecuted":         true,
			"exchange":                account.Exchange,
			"binanceStatus":           stringValue(payload["status"]),
			"clientOrderId":           stringValue(payload["clientOrderId"]),
			"cumQty":                  parseFloatValue(payload["cumQty"]),
			"executedQty":             filledQty,
			"avgPrice":                avgPrice,
			"tradeReportCount":        len(fills),
			"totalFee":                totalFee,
			"totalRealizedPnl":        totalRealizedPnL,
			"updateTime":              resolvedSyncAt,
			"emptyTradeRetryRequired": emptyTradeRetryRequired,
			"emptyTradeSyncAttempts":  attempts + 1,
		},
		Terminal:   terminal,
		FeeSource:  "exchange",
		FundingSrc: "exchange",
	}, nil
}

func (a binanceFuturesLiveAdapter) cancelRESTOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return LiveOrderSync{}, err
	}
	params := map[string]string{
		"symbol":     NormalizeSymbol(order.Symbol),
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	}
	if exchangeOrderID := normalizeBinanceOrderID(order.Metadata["exchangeOrderId"], nil); exchangeOrderID != "" {
		params["orderId"] = exchangeOrderID
	} else {
		params["origClientOrderId"] = order.ID
	}
	responseBody, err := doBinanceSignedRequest(http.MethodDelete, resolved, "/fapi/v1/order", params, binanceRESTCategoryTradeCritical)
	if err != nil {
		return LiveOrderSync{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return LiveOrderSync{}, err
	}
	status := mapBinanceOrderStatus(stringValue(payload["status"]))
	if status == "" {
		status = "CANCELLED"
	}
	resolvedCancelAt := firstNonEmpty(parseBinanceMillisToRFC3339(payload["updateTime"]), time.Now().UTC().Format(time.RFC3339))
	return LiveOrderSync{
		Status:   status,
		SyncedAt: resolvedCancelAt,
		Metadata: map[string]any{
			"adapterMode":     "rest-cancel",
			"executionMode":   "rest",
			"exchange":        account.Exchange,
			"requestPath":     "/fapi/v1/order",
			"binanceStatus":   stringValue(payload["status"]),
			"clientOrderId":   stringValue(payload["clientOrderId"]),
			"exchangeOrderId": normalizeBinanceOrderID(payload["orderId"], order.Metadata["exchangeOrderId"]),
			"updateTime":      resolvedCancelAt,
		},
		Terminal:   true,
		FeeSource:  "exchange",
		FundingSrc: "exchange",
	}, nil
}

func (a binanceFuturesLiveAdapter) fetchRESTTradeReports(account domain.Account, order domain.Order, binding map[string]any, resolved binanceRESTCredentials) ([]LiveFillReport, error) {
	symbol := NormalizeSymbol(order.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("live order has no symbol")
	}
	params := map[string]string{
		"symbol":     symbol,
		"timestamp":  fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	}
	if exchangeOrderID := normalizeBinanceOrderID(order.Metadata["exchangeOrderId"], nil); exchangeOrderID != "" {
		params["orderId"] = exchangeOrderID
	}
	payload, err := binanceSignedGETWithCategory(resolved, "/fapi/v1/userTrades", params, binanceRESTCategoryHistoryRead)
	if err != nil {
		return nil, err
	}
	var trades []map[string]any
	if err := unmarshalJSONUseNumber(payload, &trades); err != nil {
		return nil, err
	}
	return a.tradeReportsFromBinanceTrades(account, trades, order.Metadata["exchangeOrderId"]), nil
}

func (a binanceFuturesLiveAdapter) fetchRESTTradeReportsForSymbol(account domain.Account, binding map[string]any, resolved binanceRESTCredentials, symbol string, lookbackHours int) ([]LiveFillReport, error) {
	endTime := time.Now().UTC()
	startTime := endTime.Add(-time.Duration(maxInt(lookbackHours, 1)) * time.Hour)
	params := map[string]string{
		"symbol":     NormalizeSymbol(symbol),
		"timestamp":  fmt.Sprintf("%d", endTime.UnixMilli()),
		"recvWindow": fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
		"startTime":  fmt.Sprintf("%d", startTime.UnixMilli()),
		"endTime":    fmt.Sprintf("%d", endTime.UnixMilli()),
		"limit":      "1000",
	}
	payload, err := binanceSignedGETWithCategory(resolved, "/fapi/v1/userTrades", params, binanceRESTCategoryHistoryRead)
	if err != nil {
		return nil, err
	}
	var trades []map[string]any
	if err := unmarshalJSONUseNumber(payload, &trades); err != nil {
		return nil, err
	}
	return a.tradeReportsFromBinanceTrades(account, trades, nil), nil
}

func (a binanceFuturesLiveAdapter) tradeReportsFromBinanceTrades(account domain.Account, trades []map[string]any, fallbackOrderID any) []LiveFillReport {
	reports := make([]LiveFillReport, 0, len(trades))
	for _, trade := range trades {
		qty := parseFloatValue(trade["qty"])
		if qty <= 0 {
			continue
		}
		reports = append(reports, ExchangeFillReport{
			Exchange:        account.Exchange,
			AdapterKey:      a.Key(),
			AccountID:       account.ID,
			ExchangeOrderID: normalizeBinanceOrderID(trade["orderId"], fallbackOrderID),
			ExchangeTradeID: stringifyBinanceID(trade["id"]),
			Price:           parseFloatValue(trade["price"]),
			Quantity:        qty,
			Fee:             parseFloatValue(trade["commission"]),
			FeeAsset:        stringValue(trade["commissionAsset"]),
			RealizedPnL:     parseFloatValue(trade["realizedPnl"]),
			TradeTime:       parseBinanceMillisToRFC3339(trade["time"]),
			Source:          FillSourceReal,
			Raw:             trade,
		}.LiveFillReport())
		metadata := reports[len(reports)-1].Metadata
		metadata["source"] = "binance-user-trades"
		metadata["reportSource"] = "binance-user-trades"
		metadata["maker"] = trade["maker"]
		metadata["buyer"] = trade["buyer"]
		metadata["executionMode"] = "rest"
	}
	return reports
}

func normalizeCredentialRefs(value any) map[string]any {
	switch refs := value.(type) {
	case map[string]any:
		return refs
	default:
		return map[string]any{}
	}
}

func resolveLiveBinding(account domain.Account) map[string]any {
	if account.Metadata == nil {
		return nil
	}
	switch binding := account.Metadata["liveBinding"].(type) {
	case map[string]any:
		return binding
	case liveAdapterBinding:
		return map[string]any{
			"adapterKey":     binding.AdapterKey,
			"connectionMode": binding.ConnectionMode,
			"accountMode":    binding.AccountMode,
			"marginMode":     binding.MarginMode,
			"positionMode":   binding.PositionMode,
			"executionMode":  binding.ExecutionMode,
			"feeSource":      binding.FeeSource,
			"fundingSource":  binding.FundingSource,
			"sandbox":        binding.Sandbox,
			"restBaseUrl":    binding.RESTBaseURL,
			"wsBaseUrl":      binding.WSBaseURL,
			"recvWindowMs":   binding.RecvWindowMs,
			"credentialRefs": binding.CredentialRefs,
			"updatedAt":      binding.UpdatedAt,
		}
	default:
		return nil
	}
}

type binanceRESTCredentials struct {
	APIKeyRef    string
	APISecretRef string
	APIKey       string
	APISecret    string
	BaseURL      string
}

type binanceSymbolRules struct {
	Symbol      string
	TickSize    float64
	StepSize    float64
	MinQty      float64
	MaxQty      float64
	MinNotional float64
	UpdatedAt   time.Time
}

var (
	binanceSymbolRulesCache   = map[string]binanceSymbolRules{}
	binanceSymbolRulesCacheMu sync.Mutex
	binanceRESTLimiterState   = newBinanceRESTLimiter()
)

var (
	binanceRESTRequestsPerSecond = 30
	binanceRESTBurst             = 50
	binanceRESTBackoffDuration   = 60 * time.Second
)

func (p *Platform) UpdateBinanceRESTLimits() {
	rps := p.runtimePolicy.RESTLimiterRPS
	burst := p.runtimePolicy.RESTLimiterBurst
	backoffSec := p.runtimePolicy.RESTBackoffSeconds

	// 此时修改全局限流参数。注意：由于 binanceRESTRequestsPerSecond 等是 package-level 全局变量，
	// 此处的修改会影响到所有正在运行的 binance adapter。
	if rps > 0 {
		binanceRESTRequestsPerSecond = rps
	}
	if burst > 0 {
		binanceRESTBurst = burst
	}
	if backoffSec > 0 {
		binanceRESTBackoffDuration = time.Duration(backoffSec) * time.Second
	}

	// 关键：清空 gates 以便使用新的限制参数重新创建。
	// [IMPORTANT] 这是一个破坏性重置：所有已存在的限流计数器（tokens）将被丢弃，
	// 导致在参数更新瞬间，各基准 URL 的限流状态被“归零”。
	// 在高频交易场景下，这可能导致瞬间产生一批超过原限流阈值的请求。
	binanceRESTLimiterState.mu.Lock()
	binanceRESTLimiterState.gates = make(map[string]*binanceRESTGate)
	binanceRESTLimiterState.mu.Unlock()

	p.logger("service.platform").Info("binance rest limits updated and limiter gates reset",
		"rps", binanceRESTRequestsPerSecond,
		"burst", binanceRESTBurst,
		"backoff_duration", binanceRESTBackoffDuration,
	)
}

type binanceRESTRequestCategory string

const (
	binanceRESTCategoryTradeCritical binanceRESTRequestCategory = "trade-critical"
	binanceRESTCategoryAccountSync   binanceRESTRequestCategory = "account-sync"
	binanceRESTCategoryReconcile     binanceRESTRequestCategory = "reconcile"
	binanceRESTCategoryHistoryRead   binanceRESTRequestCategory = "history-read"
	binanceRESTCategoryMetadataRead  binanceRESTRequestCategory = "metadata-read"
	binanceRESTCategoryMarketData    binanceRESTRequestCategory = "market-data"
)

type binanceRESTLimiter struct {
	mu    sync.Mutex
	gates map[string]*binanceRESTGate
}

type binanceRESTGate struct {
	requests chan binanceRESTAcquireRequest
	mu       sync.Mutex
	block    time.Time
}

type binanceRESTAcquireRequest struct {
	category binanceRESTRequestCategory
	ready    chan struct{}
}

func newBinanceRESTLimiter() *binanceRESTLimiter {
	return &binanceRESTLimiter{gates: make(map[string]*binanceRESTGate)}
}

func (l *binanceRESTLimiter) gate(key string) *binanceRESTGate {
	l.mu.Lock()
	defer l.mu.Unlock()
	if gate, ok := l.gates[key]; ok {
		return gate
	}
	gate := newBinanceRESTGate(maxInt(binanceRESTRequestsPerSecond, 1), maxInt(binanceRESTBurst, 1))
	l.gates[key] = gate
	return gate
}

func newBinanceRESTGate(requestsPerSecond, burst int) *binanceRESTGate {
	gate := &binanceRESTGate{requests: make(chan binanceRESTAcquireRequest)}
	interval := time.Second
	if requestsPerSecond > 0 {
		interval = time.Second / time.Duration(requestsPerSecond)
	}
	go gate.run(maxInt(burst, 1), interval)
	return gate
}

func (g *binanceRESTGate) run(burst int, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	tokens := burst
	queues := make(map[binanceRESTRequestCategory][]chan struct{})
	drain := func() {
		g.mu.Lock()
		blockedUntil := g.block
		g.mu.Unlock()
		if time.Now().UTC().Before(blockedUntil) {
			return
		}
		for tokens > 0 {
			category, ok := nextBinanceRESTQueueCategory(queues)
			if !ok {
				return
			}
			queue := queues[category]
			ready := queue[0]
			if len(queue) == 1 {
				delete(queues, category)
			} else {
				queues[category] = queue[1:]
			}
			tokens--
			close(ready)
		}
	}
	for {
		select {
		case request := <-g.requests:
			category := normalizeBinanceRESTRequestCategory(request.category)
			queues[category] = append(queues[category], request.ready)
			drain()
		case <-ticker.C:
			if tokens < burst {
				tokens++
			}
			drain()
		}
	}
}

func nextBinanceRESTQueueCategory(queues map[binanceRESTRequestCategory][]chan struct{}) (binanceRESTRequestCategory, bool) {
	for _, category := range binanceRESTCategoryPriorityOrder {
		if len(queues[category]) > 0 {
			return category, true
		}
	}
	return "", false
}

var binanceRESTCategoryPriorityOrder = []binanceRESTRequestCategory{
	binanceRESTCategoryTradeCritical,
	binanceRESTCategoryAccountSync,
	binanceRESTCategoryReconcile,
	binanceRESTCategoryHistoryRead,
	binanceRESTCategoryMetadataRead,
	binanceRESTCategoryMarketData,
}

func normalizeBinanceRESTRequestCategory(category binanceRESTRequestCategory) binanceRESTRequestCategory {
	for _, known := range binanceRESTCategoryPriorityOrder {
		if category == known {
			return category
		}
	}
	return binanceRESTCategoryMarketData
}

func (g *binanceRESTGate) acquire(category binanceRESTRequestCategory) error {
	g.mu.Lock()
	blockedUntil := g.block
	g.mu.Unlock()
	if time.Now().UTC().Before(blockedUntil) {
		return fmt.Errorf("binance rest temporarily rate-limited until %s", blockedUntil.Format(time.RFC3339))
	}
	ready := make(chan struct{})
	g.requests <- binanceRESTAcquireRequest{
		category: category,
		ready:    ready,
	}
	<-ready
	g.mu.Lock()
	blockedUntil = g.block
	g.mu.Unlock()
	if time.Now().UTC().Before(blockedUntil) {
		return fmt.Errorf("binance rest temporarily rate-limited until %s", blockedUntil.Format(time.RFC3339))
	}
	return nil
}

func (g *binanceRESTGate) markBackoff(duration time.Duration) {
	if duration <= 0 {
		duration = binanceRESTBackoffDuration
	}
	until := time.Now().UTC().Add(duration)
	g.mu.Lock()
	if until.After(g.block) {
		g.block = until
	}
	g.mu.Unlock()
}

func binanceRESTLimiterKey(baseURL string) string {
	return strings.TrimRight(strings.TrimSpace(baseURL), "/")
}

func binanceRESTLimiterKeyForCreds(creds binanceRESTCredentials) string {
	return binanceRESTLimiterKey(creds.BaseURL)
}

func parseBinanceRetryAfter(headers http.Header) time.Duration {
	raw := strings.TrimSpace(headers.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func resolveBinanceRESTCredentials(binding map[string]any) (binanceRESTCredentials, error) {
	credentialRefs := normalizeCredentialRefs(binding["credentialRefs"])
	apiKeyRef := strings.TrimSpace(stringValue(credentialRefs["apiKeyRef"]))
	apiSecretRef := strings.TrimSpace(stringValue(credentialRefs["apiSecretRef"]))
	if apiKeyRef == "" {
		return binanceRESTCredentials{}, fmt.Errorf("live adapter binding requires credentialRefs.apiKeyRef")
	}
	if apiSecretRef == "" {
		return binanceRESTCredentials{}, fmt.Errorf("live adapter binding requires credentialRefs.apiSecretRef")
	}
	apiKey := strings.TrimSpace(os.Getenv(apiKeyRef))
	apiSecret := strings.TrimSpace(os.Getenv(apiSecretRef))
	if apiKey == "" {
		return binanceRESTCredentials{}, fmt.Errorf("api key env not found for ref %s", apiKeyRef)
	}
	if apiSecret == "" {
		return binanceRESTCredentials{}, fmt.Errorf("api secret env not found for ref %s", apiSecretRef)
	}
	baseURL := strings.TrimSpace(stringValue(binding["restBaseUrl"]))
	if baseURL == "" {
		baseURL = "https://fapi.binance.com"
		if boolValue(binding["sandbox"]) {
			baseURL = "https://testnet.binancefuture.com"
		}
	}
	return binanceRESTCredentials{
		APIKeyRef:    apiKeyRef,
		APISecretRef: apiSecretRef,
		APIKey:       apiKey,
		APISecret:    apiSecret,
		BaseURL:      strings.TrimRight(baseURL, "/"),
	}, nil
}

func (a binanceFuturesLiveAdapter) normalizeRESTOrder(order domain.Order, creds binanceRESTCredentials) (domain.Order, binanceSymbolRules, error) {
	normalized := order
	normalized.Metadata = cloneMetadata(order.Metadata)
	normalized.NormalizeExecutionFlags()
	rules, err := fetchBinanceSymbolRules(creds, NormalizeSymbol(order.Symbol))
	if err != nil {
		return domain.Order{}, binanceSymbolRules{}, err
	}
	orderType := strings.ToUpper(strings.TrimSpace(firstNonEmpty(order.Type, "MARKET")))
	rawQuantity := order.Quantity
	rawPriceReference := firstPositive(order.Price, parseFloatValue(order.Metadata["priceHint"]))
	quantityAdjustments := make([]string, 0)
	baseQuantity := normalizeBinanceQuantity(rawQuantity, rules)
	if rules.StepSize > 0 && exchangeIncrementDiffers(baseQuantity, rawQuantity, rules.StepSize) {
		quantityAdjustments = append(quantityAdjustments, "step_size")
	}
	if rules.MinQty > 0 && rawQuantity > 0 && exchangeIncrementBelow(rawQuantity, rules.MinQty, rules.StepSize) {
		quantityAdjustments = append(quantityAdjustments, "min_qty")
	}
	if normalized.EffectiveReduceOnly() && exchangeIncrementExceeds(baseQuantity, rawQuantity, rules.StepSize) {
		return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("reduce-only order quantity %.12f is below minQty %.12f for %s", rawQuantity, rules.MinQty, rules.Symbol)
	}
	normalized.Quantity = baseQuantity
	if normalized.Quantity <= 0 {
		return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("normalized order quantity is invalid for %s", rules.Symbol)
	}
	if rules.MaxQty > 0 && exchangeIncrementExceeds(normalized.Quantity, rules.MaxQty, rules.StepSize) {
		return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("normalized order quantity %.12f exceeds maxQty %.12f for %s", normalized.Quantity, rules.MaxQty, rules.Symbol)
	}
	priceReference := rawPriceReference
	priceAdjustments := make([]string, 0)
	if orderType != "MARKET" {
		normalized.Price = normalizeBinancePrice(priceReference, rules)
		if normalized.Price <= 0 {
			return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("normalized order price is invalid for %s", rules.Symbol)
		}
		if rules.TickSize > 0 && exchangeIncrementDiffers(normalized.Price, priceReference, rules.TickSize) {
			priceAdjustments = append(priceAdjustments, "tick_size")
		}
	}
	if requiredQty := requiredBinanceQuantityForMinNotional(normalized.Quantity, firstPositive(normalized.Price, priceReference), rules); exchangeIncrementExceeds(requiredQty, normalized.Quantity, rules.StepSize) {
		if normalized.EffectiveReduceOnly() {
			return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("reduce-only order quantity %.12f does not satisfy minNotional %.12f for %s", normalized.Quantity, rules.MinNotional, rules.Symbol)
		}
		normalized.Quantity = requiredQty
		quantityAdjustments = append(quantityAdjustments, "min_notional")
	}
	normalized.Metadata["normalizedQuantity"] = normalized.Quantity
	if normalized.Price > 0 {
		normalized.Metadata["normalizedPrice"] = normalized.Price
	}
	normalized.Metadata["normalization"] = map[string]any{
		"rawQuantity":              rawQuantity,
		"rawPriceReference":        rawPriceReference,
		"normalizedQuantity":       normalized.Quantity,
		"normalizedPrice":          normalized.Price,
		"quantityAdjustments":      quantityAdjustments,
		"priceAdjustments":         priceAdjustments,
		"normalizationApplied":     exchangeIncrementDiffers(rawQuantity, normalized.Quantity, rules.StepSize) || (orderType != "MARKET" && exchangeIncrementDiffers(rawPriceReference, normalized.Price, rules.TickSize)),
		"minNotionalAdjusted":      containsString(quantityAdjustments, "min_notional"),
		"stepSizeAdjusted":         containsString(quantityAdjustments, "step_size"),
		"minQtyAdjusted":           containsString(quantityAdjustments, "min_qty"),
		"tickSizeAdjusted":         containsString(priceAdjustments, "tick_size"),
		"normalizationReasonCount": len(quantityAdjustments) + len(priceAdjustments),
	}
	normalized.Metadata["symbolRules"] = map[string]any{
		"tickSize":    rules.TickSize,
		"stepSize":    rules.StepSize,
		"minQty":      rules.MinQty,
		"maxQty":      rules.MaxQty,
		"minNotional": rules.MinNotional,
	}
	return normalized, rules, nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func shouldSendBinanceReduceOnlyFlag(binding map[string]any, order domain.Order) bool {
	if !order.EffectiveReduceOnly() {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(stringValue(binding["positionMode"])), "HEDGE")
}

func resolveBinancePositionSideForSubmission(binding map[string]any, order domain.Order) string {
	positionSide := strings.ToUpper(strings.TrimSpace(stringValue(order.Metadata["positionSide"])))
	if positionSide == "" {
		return ""
	}
	if strings.EqualFold(strings.TrimSpace(stringValue(binding["positionMode"])), "HEDGE") {
		return positionSide
	}
	if positionSide == "BOTH" {
		return ""
	}
	return positionSide
}

func fetchBinanceSymbolRules(creds binanceRESTCredentials, symbol string) (binanceSymbolRules, error) {
	normalizedSymbol := NormalizeSymbol(symbol)
	if normalizedSymbol == "" {
		return binanceSymbolRules{}, fmt.Errorf("binance symbol is required")
	}
	cacheKey := creds.BaseURL + "|" + normalizedSymbol
	binanceSymbolRulesCacheMu.Lock()
	cached, ok := binanceSymbolRulesCache[cacheKey]
	binanceSymbolRulesCacheMu.Unlock()
	if ok && time.Since(cached.UpdatedAt) < 30*time.Minute {
		return cached, nil
	}
	responseBody, _, err := doBinancePublicGET(
		creds.BaseURL,
		"/fapi/v1/exchangeInfo",
		map[string]string{"symbol": normalizedSymbol},
		binanceRESTCategoryMetadataRead,
	)
	if err != nil {
		if strings.Contains(err.Error(), "rate-limited") {
			return binanceSymbolRules{}, err
		}
		if strings.HasPrefix(err.Error(), "binance request failed:") {
			return binanceSymbolRules{}, liveControlAdapterErrorf("binance exchangeInfo failed: %s", strings.TrimPrefix(err.Error(), "binance request failed: "))
		}
		return binanceSymbolRules{}, err
	}
	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return binanceSymbolRules{}, err
	}
	symbols, _ := payload["symbols"].([]any)
	for _, item := range symbols {
		entry, _ := item.(map[string]any)
		if NormalizeSymbol(stringValue(entry["symbol"])) != normalizedSymbol {
			continue
		}
		rules := parseBinanceSymbolRules(entry)
		rules.Symbol = normalizedSymbol
		rules.UpdatedAt = time.Now().UTC()
		binanceSymbolRulesCacheMu.Lock()
		binanceSymbolRulesCache[cacheKey] = rules
		binanceSymbolRulesCacheMu.Unlock()
		return rules, nil
	}
	return binanceSymbolRules{}, fmt.Errorf("binance symbol rules not found for %s", normalizedSymbol)
}

func parseBinanceSymbolRules(entry map[string]any) binanceSymbolRules {
	rules := binanceSymbolRules{}
	filters, _ := entry["filters"].([]any)
	for _, raw := range filters {
		filter, _ := raw.(map[string]any)
		switch strings.ToUpper(strings.TrimSpace(stringValue(filter["filterType"]))) {
		case "PRICE_FILTER":
			rules.TickSize = parseFloatValue(filter["tickSize"])
		case "LOT_SIZE", "MARKET_LOT_SIZE":
			if rules.StepSize <= 0 {
				rules.StepSize = parseFloatValue(filter["stepSize"])
			}
			if rules.MinQty <= 0 {
				rules.MinQty = parseFloatValue(filter["minQty"])
			}
			if rules.MaxQty <= 0 {
				rules.MaxQty = parseFloatValue(filter["maxQty"])
			}
		case "MIN_NOTIONAL", "NOTIONAL":
			if rules.MinNotional <= 0 {
				rules.MinNotional = firstPositive(parseFloatValue(filter["notional"]), parseFloatValue(filter["minNotional"]))
			}
		}
	}
	return rules
}

func normalizeBinanceQuantity(quantity float64, rules binanceSymbolRules) float64 {
	normalized := quantity
	if rules.StepSize > 0 {
		normalized = roundDownToIncrement(normalized, rules.StepSize)
	}
	if rules.MinQty > 0 && exchangeIncrementBelow(normalized, rules.MinQty, rules.StepSize) {
		normalized = rules.MinQty
	}
	if rules.StepSize > 0 {
		normalized = roundDownToIncrement(normalized, rules.StepSize)
	}
	return normalized
}

func normalizeBinancePrice(price float64, rules binanceSymbolRules) float64 {
	if price <= 0 {
		return 0
	}
	if rules.TickSize <= 0 {
		return price
	}
	return roundDownToIncrement(price, rules.TickSize)
}

func requiredBinanceQuantityForMinNotional(quantity, price float64, rules binanceSymbolRules) float64 {
	if quantity <= 0 || price <= 0 || rules.MinNotional <= 0 {
		return quantity
	}
	if exchangeNotionalSatisfiesMinimum(quantity*price, rules.MinNotional) {
		return quantity
	}
	required := rules.MinNotional / price
	if rules.StepSize > 0 {
		required = ceilToIncrement(required, rules.StepSize)
	}
	if rules.MinQty > 0 && required < rules.MinQty {
		required = rules.MinQty
	}
	if rules.StepSize > 0 {
		required = ceilToIncrement(required, rules.StepSize)
	}
	return required
}

func encodeBinanceQuery(params map[string]string, redactSignature bool) string {
	if len(params) == 0 {
		return ""
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := url.Values{}
	for _, key := range keys {
		value := params[key]
		if redactSignature && key == "signature" && value != "" {
			value = "<redacted>"
		}
		values.Set(key, value)
	}
	return values.Encode()
}

func signBinanceQuery(params map[string]string, secret string) string {
	payload := encodeBinanceQuery(params, false)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func trimFloat(value float64) string {
	text := fmt.Sprintf("%.12f", value)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
}

func formatBinanceDecimal(value, step float64) string {
	if step <= 0 {
		return strconv.FormatFloat(value, 'f', -1, 64)
	}
	precision := decimalPlacesForStep(step)
	text := strconv.FormatFloat(value, 'f', precision, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
}

func decimalPlacesForStep(step float64) int {
	text := strconv.FormatFloat(step, 'f', -1, 64)
	if idx := strings.IndexByte(text, '.'); idx >= 0 {
		return len(strings.TrimRight(text[idx+1:], "0"))
	}
	return 0
}

func binanceSignedGET(creds binanceRESTCredentials, path string, params map[string]string) ([]byte, error) {
	return binanceSignedGETWithCategory(creds, path, params, binanceRESTCategoryTradeCritical)
}

func binanceSignedGETWithCategory(creds binanceRESTCredentials, path string, params map[string]string, category binanceRESTRequestCategory) ([]byte, error) {
	return doBinanceSignedRequest(http.MethodGet, creds, path, params, category)
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func doBinancePublicGET(baseURL, path string, params map[string]string, category binanceRESTRequestCategory) ([]byte, http.Header, error) {
	query := encodeBinanceQuery(params, false)
	return doBinanceRESTRequest(http.MethodGet, baseURL, path, query, nil, nil, category)
}

func doBinanceSignedRequest(method string, creds binanceRESTCredentials, path string, params map[string]string, category binanceRESTRequestCategory) ([]byte, error) {
	gate := binanceRESTLimiterState.gate(binanceRESTLimiterKey(creds.BaseURL))
	if err := gate.acquire(category); err != nil {
		return nil, wrapLiveControlAdapterError(err)
	}
	params = cloneStringMap(params)
	delete(params, "signature")
	params["timestamp"] = fmt.Sprintf("%d", time.Now().UTC().UnixMilli())
	params["signature"] = signBinanceQuery(params, creds.APISecret)
	query := encodeBinanceQuery(params, false)
	headers := map[string]string{
		"X-MBX-APIKEY": creds.APIKey,
		"Content-Type": "application/x-www-form-urlencoded",
	}
	var body io.Reader
	if method != http.MethodGet && method != http.MethodDelete {
		body = strings.NewReader(query)
		headers["Accept"] = "application/json"
	}
	responseBody, _, err := doBinanceRESTRequestAfterAcquire(method, creds.BaseURL, path, query, headers, body, category, gate)
	return responseBody, err
}

func doBinanceRESTRequest(method, baseURL, path, query string, headers map[string]string, body io.Reader, category binanceRESTRequestCategory) ([]byte, http.Header, error) {
	gate := binanceRESTLimiterState.gate(binanceRESTLimiterKey(baseURL))
	if err := gate.acquire(category); err != nil {
		return nil, nil, wrapLiveControlAdapterError(err)
	}
	return doBinanceRESTRequestAfterAcquire(method, baseURL, path, query, headers, body, category, gate)
}

func doBinanceRESTRequestAfterAcquire(method, baseURL, path, query string, headers map[string]string, body io.Reader, category binanceRESTRequestCategory, gate *binanceRESTGate) ([]byte, http.Header, error) {
	requestURL := strings.TrimRight(baseURL, "/") + path
	if (method == http.MethodGet || method == http.MethodDelete) && query != "" {
		requestURL += "?" + query
	}
	request, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, nil, err
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, nil, wrapLiveControlAdapterError(err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return nil, response.Header, wrapLiveControlAdapterError(readErr)
	}
	if response.StatusCode == http.StatusTooManyRequests {
		if gate != nil {
			gate.markBackoff(firstPositiveDuration(parseBinanceRetryAfter(response.Header), binanceRESTBackoffDuration))
		}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, response.Header, liveControlAdapterErrorf("binance request failed: %s %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, response.Header, nil
}

func firstPositiveDuration(values ...time.Duration) time.Duration {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func mapBinanceOrderStatus(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case "NEW":
		return "ACCEPTED"
	case "PARTIALLY_FILLED":
		return "PARTIALLY_FILLED"
	case "FILLED":
		return "FILLED"
	case "CANCELED", "CANCELLED", "EXPIRED":
		return "CANCELLED"
	case "REJECTED":
		return "REJECTED"
	default:
		return ""
	}
}

func normalizeBinanceOrderID(primary any, fallback any) string {
	if value := stringifyBinanceID(primary); value != "" {
		return value
	}
	return stringifyBinanceID(fallback)
}

func stringifyBinanceID(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case float64:
		return fmt.Sprintf("%.0f", v)
	case float32:
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case uint64:
		return fmt.Sprintf("%d", v)
	case uint32:
		return fmt.Sprintf("%d", v)
	case json.Number:
		return strings.TrimSpace(v.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func parseBinanceMillisToRFC3339(value any) string {
	millis, ok := toInt64(value)
	if !ok || millis <= 0 {
		return ""
	}
	return time.UnixMilli(millis).UTC().Format(time.RFC3339)
}

func unmarshalJSONUseNumber(payload []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	return decoder.Decode(target)
}

func normalizeLiveExecutionMode(value any, sandbox bool) string {
	raw := strings.ToLower(strings.TrimSpace(stringValue(value)))
	switch raw {
	case "rest", "real", "live":
		return "rest"
	case "mock", "paper", "simulated":
		return "mock"
	case "":
		if sandbox {
			return "mock"
		}
		return "mock"
	default:
		return "mock"
	}
}

func boolValue(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}
