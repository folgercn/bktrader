package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type LiveExecutionAdapter interface {
	Key() string
	Describe() map[string]any
	ValidateAccountConfig(config map[string]any) error
	SubmitOrder(account domain.Account, order domain.Order, binding map[string]any) (LiveOrderSubmission, error)
}

type LiveOrderSubmission struct {
	Status          string         `json:"status"`
	ExchangeOrderID string         `json:"exchangeOrderId"`
	AcceptedAt      string         `json:"acceptedAt"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

type liveAdapterBinding struct {
	AdapterKey     string         `json:"adapterKey"`
	ConnectionMode string         `json:"connectionMode"`
	AccountMode    string         `json:"accountMode"`
	MarginMode     string         `json:"marginMode"`
	PositionMode   string         `json:"positionMode"`
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
