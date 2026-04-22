package service

import (
	"fmt"
	"strings"
)

type StopLiveFlowResult struct {
	AccountID                string   `json:"accountId"`
	StoppedLiveSessionIDs    []string `json:"stoppedLiveSessionIds"`
	StoppedRuntimeSessionIDs []string `json:"stoppedRuntimeSessionIds"`
}

func (p *Platform) StopLiveFlowWithForce(accountID string, force bool) (StopLiveFlowResult, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return StopLiveFlowResult{}, fmt.Errorf("accountId is required")
	}
	if _, err := p.store.GetAccount(accountID); err != nil {
		return StopLiveFlowResult{}, err
	}

	liveSessions, err := p.ListLiveSessions()
	if err != nil {
		return StopLiveFlowResult{}, err
	}
	runtimeSessions := p.ListSignalRuntimeSessions()

	runningLiveSessions := make([]string, 0)
	runningRuntimeSessions := make([]string, 0)
	strategyIDs := make(map[string]struct{})
	seenRuntimeIDs := make(map[string]struct{})

	for _, session := range liveSessions {
		if !strings.EqualFold(strings.TrimSpace(session.AccountID), accountID) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(session.Status), "RUNNING") {
			runningLiveSessions = append(runningLiveSessions, session.ID)
		}
		if strategyID := strings.TrimSpace(session.StrategyID); strategyID != "" {
			strategyIDs[strategyID] = struct{}{}
		}
	}
	for _, session := range runtimeSessions {
		if !strings.EqualFold(strings.TrimSpace(session.AccountID), accountID) {
			continue
		}
		if strategyID := strings.TrimSpace(session.StrategyID); strategyID != "" {
			strategyIDs[strategyID] = struct{}{}
		}
		if !strings.EqualFold(strings.TrimSpace(session.Status), "RUNNING") {
			continue
		}
		if _, exists := seenRuntimeIDs[session.ID]; exists {
			continue
		}
		seenRuntimeIDs[session.ID] = struct{}{}
		runningRuntimeSessions = append(runningRuntimeSessions, session.ID)
	}

	if !force {
		for strategyID := range strategyIDs {
			if err := p.ensureNoActivePositionsOrOrders(accountID, strategyID); err != nil {
				return StopLiveFlowResult{}, err
			}
		}
	}

	result := StopLiveFlowResult{
		AccountID:                accountID,
		StoppedLiveSessionIDs:    make([]string, 0, len(runningLiveSessions)),
		StoppedRuntimeSessionIDs: make([]string, 0, len(runningRuntimeSessions)),
	}
	for _, sessionID := range runningLiveSessions {
		if _, err := p.StopLiveSessionWithForce(sessionID, true); err != nil {
			return StopLiveFlowResult{}, err
		}
		result.StoppedLiveSessionIDs = append(result.StoppedLiveSessionIDs, sessionID)
	}
	for _, sessionID := range runningRuntimeSessions {
		if _, err := p.StopSignalRuntimeSessionWithForce(sessionID, true); err != nil {
			return StopLiveFlowResult{}, err
		}
		result.StoppedRuntimeSessionIDs = append(result.StoppedRuntimeSessionIDs, sessionID)
	}
	return result, nil
}
