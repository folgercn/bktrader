package service

import "strings"

const (
	strategyZeroInitialModePosition      = "position"
	strategyZeroInitialModeReentryWindow = "reentry_window"
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
	enabled := true
	if _, ok := parameters["dir2_zero_initial"]; ok {
		enabled = boolValue(parameters["dir2_zero_initial"])
	}
	return resolveStrategyZeroInitialMode(enabled, parameters["zero_initial_mode"]) == strategyZeroInitialModeReentryWindow
}
