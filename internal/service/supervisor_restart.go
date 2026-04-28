package service

import (
	"fmt"
	"strings"
	"time"
)

type SupervisorBackoffPolicy struct {
	First  time.Duration
	Repeat time.Duration
	Max    time.Duration
}

func RestartBackoff(policy SupervisorBackoffPolicy, attempt int) time.Duration {
	backoff := policy.First
	if attempt > 1 && policy.Repeat > 0 {
		backoff = policy.Repeat
	}
	if policy.Max > 0 && backoff > policy.Max {
		return policy.Max
	}
	return backoff
}

func RestartAttempt(state map[string]any, key string) int {
	switch value := state[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(strings.TrimSpace(value), "%d", &parsed); err == nil {
			return parsed
		}
	}
	return 0
}

func ParseRestartTime(state map[string]any, key string) (time.Time, bool) {
	raw := stringValue(state[key])
	if raw == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

func ClearRestartState(state map[string]any, keys []string) {
	for _, key := range keys {
		delete(state, key)
	}
}
