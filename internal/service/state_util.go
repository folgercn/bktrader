package service

// stripHeavyState removes large objects like sourceStates or signalBarStates
// from a session state map to reduce the summary payload size. It returns a new
// map and does not mutate the original input. If the input is nil, it returns nil.
func stripHeavyState(state map[string]any) map[string]any {
	if state == nil {
		return nil
	}
	newState := make(map[string]any, len(state))
	for k, v := range state {
		if k == "sourceStates" || k == "signalBarStates" {
			continue
		}
		newState[k] = v
	}
	return newState
}
