package service

const (
	liveSessionSummaryTimelineLimit        = 50
	liveSessionSummaryBreakoutHistoryLimit = 12
)

var liveSessionSummaryOmittedStateKeys = map[string]struct{}{
	"lastStrategyEvaluationSourceStates":    {},
	"lastStrategyEvaluationSignalBarStates": {},
}

var sourceStateAllowedKeys = map[string]struct{}{
	"sourceKey":   {},
	"role":        {},
	"streamType":  {},
	"symbol":      {},
	"timeframe":   {},
	"event":       {},
	"lastEventAt": {},
}

var signalBarStateAllowedKeys = map[string]struct{}{
	"symbol":         {},
	"timeframe":      {},
	"barCount":       {},
	"closedBarCount": {},
	"currentClosed":  {},
	"current":        {},
	"prevBar1":       {},
	"prevBar2":       {},
	"sma5":           {},
	"ma20":           {},
	"atr14":          {},
}

// stripHeavyState removes large objects from a session state map to reduce the
// live-sessions summary payload size. It returns a new map and does not mutate
// the original input. If the input is nil, it returns nil.
func stripHeavyState(state map[string]any) map[string]any {
	if state == nil {
		return nil
	}
	newState := make(map[string]any, len(state))
	for k, v := range state {
		if _, omitted := liveSessionSummaryOmittedStateKeys[k]; omitted {
			continue
		}
		switch k {
		case "timeline":
			v = trimStateList(v, liveSessionSummaryTimelineLimit)
		case "breakoutHistory":
			v = trimStateList(v, liveSessionSummaryBreakoutHistoryLimit)
		case "sourceStates":
			v = stripMapEntriesByWhitelist(v, sourceStateAllowedKeys)
		case "signalBarStates":
			v = stripMapEntriesByWhitelist(v, signalBarStateAllowedKeys)
		}
		newState[k] = v
	}
	return newState
}

func stripMapEntriesByWhitelist(v any, whitelist map[string]struct{}) any {
	m, ok := v.(map[string]any)
	if !ok {
		return v
	}
	newM := make(map[string]any, len(m))
	for k, entry := range m {
		e, ok := entry.(map[string]any)
		if !ok {
			newM[k] = entry
			continue
		}
		newE := make(map[string]any, len(whitelist))
		for ek, ev := range e {
			if _, allowed := whitelist[ek]; allowed {
				newE[ek] = ev
			}
		}
		newM[k] = newE
	}
	return newM
}

func trimStateList(value any, limit int) any {
	if limit <= 0 {
		return value
	}
	switch items := value.(type) {
	case []any:
		if len(items) <= limit {
			return append([]any(nil), items...)
		}
		return append([]any(nil), items[len(items)-limit:]...)
	case []map[string]any:
		if len(items) <= limit {
			return append([]map[string]any(nil), items...)
		}
		return append([]map[string]any(nil), items[len(items)-limit:]...)
	default:
		return value
	}
}
