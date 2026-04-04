package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type ExecutionMode string

const (
	ExecutionModeBacktest ExecutionMode = "BACKTEST"
	ExecutionModePaper    ExecutionMode = "PAPER"
	ExecutionModeLive     ExecutionMode = "LIVE"
)

type SlippageMode string

const (
	SlippageModeSimulated SlippageMode = "SIMULATED"
	SlippageModeObserved  SlippageMode = "OBSERVED"
)

type StrategyExecutionSemantics struct {
	Mode                  ExecutionMode `json:"mode"`
	SlippageMode          SlippageMode  `json:"slippageMode"`
	SimulatedSlippageBps  float64       `json:"simulatedSlippageBps"`
	UseCanonicalExecution bool          `json:"useCanonicalExecution"`
}

type StrategyExecutionContext struct {
	StrategyEngineKey   string
	StrategyVersionID   string
	SignalTimeframe     string
	ExecutionDataSource string
	Symbol              string
	From                time.Time
	To                  time.Time
	Parameters          map[string]any
	Semantics           StrategyExecutionSemantics
}

type StrategyEngine interface {
	Key() string
	Describe() map[string]any
	Run(context StrategyExecutionContext) (map[string]any, error)
}

func defaultExecutionSemantics(mode ExecutionMode, parameters map[string]any) StrategyExecutionSemantics {
	semantics := StrategyExecutionSemantics{
		Mode:                  mode,
		UseCanonicalExecution: true,
		SlippageMode:          SlippageModeObserved,
		SimulatedSlippageBps:  0,
	}
	if mode == ExecutionModeBacktest {
		semantics.SlippageMode = SlippageModeSimulated
		semantics.SimulatedSlippageBps = firstPositive(parseFloatValue(parameters["fixed_slippage"])*10000, 5)
	}
	return semantics
}

func normalizeStrategyEngineKey(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "bk-default"
	}
	return value
}

func (p *Platform) registerStrategyEngine(engine StrategyEngine) {
	if engine == nil {
		return
	}
	if p.strategyEngines == nil {
		p.strategyEngines = make(map[string]StrategyEngine)
	}
	p.strategyEngines[normalizeStrategyEngineKey(engine.Key())] = engine
}

func (p *Platform) registerBuiltInStrategyEngines() {
	p.registerStrategyEngine(bkStrategyEngine{platform: p})
}

func (p *Platform) resolveStrategyEngine(strategyVersionID string, parameters map[string]any) (StrategyEngine, string, error) {
	engineKey := normalizeStrategyEngineKey(stringValue(parameters["strategyEngine"]))
	if engineKey == "bk-default" {
		if resolved := p.resolveStrategyEngineFromVersion(strategyVersionID); resolved != "" {
			engineKey = normalizeStrategyEngineKey(resolved)
		}
	}
	engine, ok := p.strategyEngines[engineKey]
	if !ok {
		return nil, engineKey, fmt.Errorf("strategy engine not registered: %s", engineKey)
	}
	return engine, engineKey, nil
}

func (p *Platform) resolveStrategyEngineFromVersion(strategyVersionID string) string {
	items, err := p.ListStrategies()
	if err != nil {
		return ""
	}
	for _, item := range items {
		switch currentVersion := item["currentVersion"].(type) {
		case domain.StrategyVersion:
			if currentVersion.ID == strategyVersionID {
				return stringValue(currentVersion.Parameters["strategyEngine"])
			}
		case map[string]any:
			if stringValue(currentVersion["id"]) != strategyVersionID {
				continue
			}
			if params, ok := currentVersion["parameters"].(map[string]any); ok {
				return stringValue(params["strategyEngine"])
			}
		}
	}
	return ""
}

type bkStrategyEngine struct {
	platform *Platform
}

func (e bkStrategyEngine) Key() string {
	return "bk-default"
}

func (e bkStrategyEngine) Describe() map[string]any {
	return map[string]any{
		"key":                  e.Key(),
		"name":                 "BK Default Strategy",
		"supportedSignalBars":  []string{"4h", "1d"},
		"supportedExecutions":  []string{"tick", "1min"},
		"runtimeConsistency":   "canonical-execution-shared",
		"backtestSlippageOnly": true,
	}
}

func (e bkStrategyEngine) Run(context StrategyExecutionContext) (map[string]any, error) {
	return e.platform.runStrategyReplay(context)
}
