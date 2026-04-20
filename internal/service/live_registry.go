package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
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
	Metadata   map[string]any `json:"metadata,omitempty"`
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
		"recvWindowMs":   maxIntValue(binding["recvWindowMs"], 5000),
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
		RecvWindowMs:   maxIntValue(normalized["recvWindowMs"], 5000),
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
	params["signature"] = signBinanceQuery(params, resolved.APISecret)
	payload, err := doBinanceSignedRequest(http.MethodGet, resolved, "/fapi/v1/allOrders", params)
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
	params["signature"] = signBinanceQuery(params, resolved.APISecret)
	responseBody, err := doBinanceSignedRequest(http.MethodPost, resolved, "/fapi/v1/order", params)
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
			"requestQuery":       encodeBinanceQuery(params, true),
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
	params["signature"] = signBinanceQuery(params, resolved.APISecret)
	responseBody, err := doBinanceSignedRequest(http.MethodGet, resolved, "/fapi/v1/order", params)
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
	if tradeErr == nil && len(tradeReports) > 0 && terminal {
		fills = tradeReports
	} else if filledQty > 0 && strings.EqualFold(status, "FILLED") {
		fills = append(fills, LiveFillReport{
			Price:    avgPrice,
			Quantity: filledQty,
			Fee:      0,
			Metadata: map[string]any{
				"source":          "binance-order-query",
				"exchange":        account.Exchange,
				"adapterKey":      a.Key(),
				"exchangeOrderId": normalizeBinanceOrderID(payload["orderId"], order.Metadata["exchangeOrderId"]),
				"clientOrderId":   stringValue(payload["clientOrderId"]),
				"tradeTime":       firstNonEmpty(parseBinanceMillisToRFC3339(payload["updateTime"]), time.Now().UTC().Format(time.RFC3339)),
				"executionMode":   "rest",
			},
		})
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
			"adapterMode":      "rest",
			"executionMode":    "rest",
			"restBaseUrl":      resolved.BaseURL,
			"requestPath":      "/fapi/v1/order",
			"requestQuery":     encodeBinanceQuery(params, true),
			"apiKeyRef":        resolved.APIKeyRef,
			"apiSecretRef":     resolved.APISecretRef,
			"requestReady":     true,
			"networkExecuted":  true,
			"exchange":         account.Exchange,
			"binanceStatus":    stringValue(payload["status"]),
			"clientOrderId":    stringValue(payload["clientOrderId"]),
			"cumQty":           parseFloatValue(payload["cumQty"]),
			"executedQty":      filledQty,
			"avgPrice":         avgPrice,
			"tradeReportCount": len(fills),
			"totalFee":         totalFee,
			"totalRealizedPnl": totalRealizedPnL,
			"updateTime":       resolvedSyncAt,
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
	params["signature"] = signBinanceQuery(params, resolved.APISecret)
	responseBody, err := doBinanceSignedRequest(http.MethodDelete, resolved, "/fapi/v1/order", params)
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
			"requestQuery":    encodeBinanceQuery(params, true),
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
	payload, err := binanceSignedGET(resolved, "/fapi/v1/userTrades", params)
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
	payload, err := binanceSignedGET(resolved, "/fapi/v1/userTrades", params)
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
		reports = append(reports, LiveFillReport{
			Price:      parseFloatValue(trade["price"]),
			Quantity:   qty,
			Fee:        parseFloatValue(trade["commission"]),
			FundingPnL: 0,
			Metadata: map[string]any{
				"source":          "binance-user-trades",
				"exchange":        account.Exchange,
				"adapterKey":      a.Key(),
				"exchangeOrderId": normalizeBinanceOrderID(trade["orderId"], fallbackOrderID),
				"tradeId":         stringifyBinanceID(trade["id"]),
				"commissionAsset": stringValue(trade["commissionAsset"]),
				"realizedPnl":     parseFloatValue(trade["realizedPnl"]),
				"maker":           trade["maker"],
				"buyer":           trade["buyer"],
				"tradeTime":       parseBinanceMillisToRFC3339(trade["time"]),
				"executionMode":   "rest",
			},
		})
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
)

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
	if rules.StepSize > 0 && baseQuantity != rawQuantity {
		quantityAdjustments = append(quantityAdjustments, "step_size")
	}
	if rules.MinQty > 0 && rawQuantity > 0 && rawQuantity < rules.MinQty {
		quantityAdjustments = append(quantityAdjustments, "min_qty")
	}
	if normalized.EffectiveReduceOnly() && baseQuantity > rawQuantity {
		return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("reduce-only order quantity %.12f is below minQty %.12f for %s", rawQuantity, rules.MinQty, rules.Symbol)
	}
	normalized.Quantity = baseQuantity
	if normalized.Quantity <= 0 {
		return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("normalized order quantity is invalid for %s", rules.Symbol)
	}
	if rules.MaxQty > 0 && normalized.Quantity > rules.MaxQty {
		return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("normalized order quantity %.12f exceeds maxQty %.12f for %s", normalized.Quantity, rules.MaxQty, rules.Symbol)
	}
	priceReference := rawPriceReference
	priceAdjustments := make([]string, 0)
	if orderType != "MARKET" {
		normalized.Price = normalizeBinancePrice(priceReference, rules)
		if normalized.Price <= 0 {
			return domain.Order{}, binanceSymbolRules{}, fmt.Errorf("normalized order price is invalid for %s", rules.Symbol)
		}
		if rules.TickSize > 0 && normalized.Price != priceReference {
			priceAdjustments = append(priceAdjustments, "tick_size")
		}
	}
	if requiredQty := requiredBinanceQuantityForMinNotional(normalized.Quantity, firstPositive(normalized.Price, priceReference), rules); requiredQty > normalized.Quantity {
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
		"normalizationApplied":     rawQuantity != normalized.Quantity || (orderType != "MARKET" && rawPriceReference != normalized.Price),
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
	requestURL := creds.BaseURL + "/fapi/v1/exchangeInfo?symbol=" + url.QueryEscape(normalizedSymbol)
	request, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return binanceSymbolRules{}, err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return binanceSymbolRules{}, err
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return binanceSymbolRules{}, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return binanceSymbolRules{}, fmt.Errorf("binance exchangeInfo failed: %s %s", response.Status, strings.TrimSpace(string(responseBody)))
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
		normalized = roundToStep(normalized, rules.StepSize)
	}
	if rules.MinQty > 0 && normalized < rules.MinQty {
		normalized = rules.MinQty
	}
	if rules.StepSize > 0 {
		normalized = roundToStep(normalized, rules.StepSize)
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
	return roundToStep(price, rules.TickSize)
}

func requiredBinanceQuantityForMinNotional(quantity, price float64, rules binanceSymbolRules) float64 {
	if quantity <= 0 || price <= 0 || rules.MinNotional <= 0 {
		return quantity
	}
	if quantity*price >= rules.MinNotional {
		return quantity
	}
	required := rules.MinNotional / price
	if rules.StepSize > 0 {
		required = ceilToStep(required, rules.StepSize)
	}
	if rules.MinQty > 0 && required < rules.MinQty {
		required = rules.MinQty
	}
	if rules.StepSize > 0 {
		required = ceilToStep(required, rules.StepSize)
	}
	return required
}

func roundToStep(value, step float64) float64 {
	if value <= 0 || step <= 0 {
		return value
	}
	return math.Floor((value/step)+1e-9) * step
}

func ceilToStep(value, step float64) float64 {
	if value <= 0 || step <= 0 {
		return value
	}
	return math.Ceil((value-1e-12)/step) * step
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
	params = cloneStringMap(params)
	params["signature"] = signBinanceQuery(params, creds.APISecret)
	return doBinanceSignedRequest(http.MethodGet, creds, path, params)
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

func doBinanceSignedRequest(method string, creds binanceRESTCredentials, path string, params map[string]string) ([]byte, error) {
	query := encodeBinanceQuery(params, false)
	requestURL := creds.BaseURL + path
	var body io.Reader
	if method == http.MethodGet || method == http.MethodDelete {
		requestURL += "?" + query
	} else {
		body = strings.NewReader(query)
	}
	request, err := http.NewRequest(method, requestURL, body)
	if err != nil {
		return nil, err
	}
	request.Header.Set("X-MBX-APIKEY", creds.APIKey)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if method != http.MethodGet && method != http.MethodDelete {
		request.Header.Set("Accept", "application/json")
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(response.Body)
	if readErr != nil {
		return nil, readErr
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("binance request failed: %s %s", response.Status, strings.TrimSpace(string(responseBody)))
	}
	return responseBody, nil
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
