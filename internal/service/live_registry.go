package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type LiveExecutionAdapter interface {
	Key() string
	Describe() map[string]any
	ValidateAccountConfig(config map[string]any) error
	SubmitOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error)
	SyncOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error)
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
		"credentialRefs": normalizeCredentialRefs(binding["credentialRefs"]),
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
				"exchangeOrderId": stringValue(order.Metadata["exchangeOrderId"]),
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

func (a binanceFuturesLiveAdapter) submitRESTOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error) {
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return LiveOrderSubmission{}, err
	}
	params := map[string]string{
		"symbol":      NormalizeSymbol(order.Symbol),
		"side":        strings.ToUpper(strings.TrimSpace(order.Side)),
		"type":        strings.ToUpper(strings.TrimSpace(firstNonEmpty(order.Type, "MARKET"))),
		"timestamp":   fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow":  fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
		"newOrderRespType": "RESULT",
	}
	if order.Quantity > 0 {
		params["quantity"] = trimFloat(order.Quantity)
	}
	if order.Price > 0 {
		params["price"] = trimFloat(order.Price)
	}
	params["signature"] = signBinanceQuery(params, resolved.APISecret)
	return LiveOrderSubmission{
		Status:          "ACCEPTED",
		ExchangeOrderID: fmt.Sprintf("binance-rest-pending-%d", time.Now().UTC().UnixNano()),
		AcceptedAt:      time.Now().UTC().Format(time.RFC3339),
		Metadata: map[string]any{
			"adapterMode":    "rest-skeleton",
			"executionMode":  "rest",
			"restBaseUrl":    resolved.BaseURL,
			"requestPath":    "/fapi/v1/order",
			"requestQuery":   encodeBinanceQuery(params, true),
			"apiKeyRef":      resolved.APIKeyRef,
			"apiSecretRef":   resolved.APISecretRef,
			"requestReady":   true,
			"networkExecuted": false,
			"exchange":       account.Exchange,
		},
	}, nil
}

func (a binanceFuturesLiveAdapter) syncRESTOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSync, error) {
	if stringValue(order.Metadata["exchangeOrderId"]) == "" {
		return LiveOrderSync{}, fmt.Errorf("live order has no exchangeOrderId")
	}
	resolved, err := resolveBinanceRESTCredentials(binding)
	if err != nil {
		return LiveOrderSync{}, err
	}
	params := map[string]string{
		"symbol":      NormalizeSymbol(order.Symbol),
		"origClientOrderId": order.ID,
		"timestamp":   fmt.Sprintf("%d", time.Now().UTC().UnixMilli()),
		"recvWindow":  fmt.Sprintf("%d", maxIntValue(binding["recvWindowMs"], 5000)),
	}
	params["signature"] = signBinanceQuery(params, resolved.APISecret)
	return LiveOrderSync{
		Status:   firstNonEmpty(order.Status, "ACCEPTED"),
		SyncedAt: time.Now().UTC().Format(time.RFC3339),
		Metadata: map[string]any{
			"adapterMode":     "rest-skeleton",
			"executionMode":   "rest",
			"restBaseUrl":     resolved.BaseURL,
			"requestPath":     "/fapi/v1/order",
			"requestQuery":    encodeBinanceQuery(params, true),
			"apiKeyRef":       resolved.APIKeyRef,
			"apiSecretRef":    resolved.APISecretRef,
			"requestReady":    true,
			"networkExecuted": false,
			"exchange":        account.Exchange,
		},
		Terminal:   false,
		FeeSource:  "exchange",
		FundingSrc: "exchange",
	}, nil
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
