package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

type StopLiveFlowResult struct {
	AccountID                string   `json:"accountId"`
	StoppedLiveSessionIDs    []string `json:"stoppedLiveSessionIds"`
	StoppedRuntimeSessionIDs []string `json:"stoppedRuntimeSessionIds"`
}

// StopLiveFlowWithForce is a process-local account stop helper. It closes the
// live/runtime sessions visible to this process and coordinates them with the
// account+strategy control lock, but it is not a distributed global stop
// barrier; multi-writer deployments still rely on runtime leases and persisted
// session state convergence outside this call.
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

	runningLiveSessions := make([]domain.LiveSession, 0)
	runningRuntimeSessions := make([]domain.SignalRuntimeSession, 0)
	strategyIDs := make(map[string]struct{})
	seenRuntimeIDs := make(map[string]struct{})

	for _, session := range liveSessions {
		if !strings.EqualFold(strings.TrimSpace(session.AccountID), accountID) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(session.Status), "RUNNING") {
			runningLiveSessions = append(runningLiveSessions, session)
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
		runningRuntimeSessions = append(runningRuntimeSessions, session)
	}

	strategyList := make([]string, 0, len(strategyIDs))
	for strategyID := range strategyIDs {
		strategyList = append(strategyList, strategyID)
	}
	sort.Strings(strategyList)
	lockRequests := make([]liveControlOperationInfo, 0, len(strategyList))
	for _, strategyID := range strategyList {
		lockRequests = append(lockRequests, liveControlOperationInfo{
			Operation:  liveControlOperationAccountStop,
			AccountID:  accountID,
			StrategyID: strategyID,
		})
	}
	release, acquired, current, lockErr := p.tryStartLiveControlOperations(lockRequests)
	if lockErr != nil {
		return StopLiveFlowResult{}, lockErr
	}
	if !acquired {
		return StopLiveFlowResult{}, liveControlOperationInProgressError(liveControlOperationInfo{
			Operation: liveControlOperationAccountStop,
			AccountID: accountID,
		}, current)
	}
	defer release()

	if !force {
		for _, strategyID := range strategyList {
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
	for _, session := range runningLiveSessions {
		if _, err := p.stopLiveSessionWithForceLocked(session, true); err != nil {
			return StopLiveFlowResult{}, err
		}
		result.StoppedLiveSessionIDs = append(result.StoppedLiveSessionIDs, session.ID)
	}
	for _, session := range runningRuntimeSessions {
		if _, err := p.stopSignalRuntimeSessionWithForceLocked(session, true); err != nil {
			return StopLiveFlowResult{}, err
		}
		result.StoppedRuntimeSessionIDs = append(result.StoppedRuntimeSessionIDs, session.ID)
	}
	return result, nil
}
