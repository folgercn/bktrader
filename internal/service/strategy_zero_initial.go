package service

import (
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	strategyZeroInitialModePosition      = "position"
	strategyZeroInitialModeReentryWindow = domain.ResearchBaselineZeroInitialMode
)

func normalizeStrategyZeroInitialMode(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "", strategyZeroInitialModePosition:
		return strategyZeroInitialModePosition
	case strategyZeroInitialModeReentryWindow:
		return strategyZeroInitialModeReentryWindow
	default:
		return strategyZeroInitialModePosition
	}
}

func resolveStrategyZeroInitialMode(enabled bool, raw any) string {
	if !enabled {
		return strategyZeroInitialModePosition
	}
	mode := normalizeStrategyZeroInitialMode(stringValue(raw))
	if mode == strategyZeroInitialModePosition && strings.TrimSpace(stringValue(raw)) == "" {
		return strategyZeroInitialModeReentryWindow
	}
	return mode
}

func strategyZeroInitialReentryWindowEnabled(parameters map[string]any) bool {
	enabled := domain.ResearchBaselineDir2ZeroInitial
	if _, ok := parameters["dir2_zero_initial"]; ok {
		enabled = boolValue(parameters["dir2_zero_initial"])
	}
	return resolveStrategyZeroInitialMode(enabled, parameters["zero_initial_mode"]) == strategyZeroInitialModeReentryWindow
}
