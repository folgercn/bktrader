package service

import (
	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store"
)

type dashboardLiveSessionNotifyingStore struct {
	store.Repository
	notify func(DashboardDomain, string)
}

type dashboardLiveSessionNotifyingCASStore struct {
	*dashboardLiveSessionNotifyingStore
	cas liveSessionControlStateCompareAndSwapStore
}

type storeWrapper interface {
	UnwrapStoreRepository() store.Repository
}

func newDashboardLiveSessionNotifyingStore(repo store.Repository, notify func(DashboardDomain, string)) store.Repository {
	base := &dashboardLiveSessionNotifyingStore{
		Repository: repo,
		notify:     notify,
	}
	if cas, ok := repo.(liveSessionControlStateCompareAndSwapStore); ok {
		return &dashboardLiveSessionNotifyingCASStore{
			dashboardLiveSessionNotifyingStore: base,
			cas:                                cas,
		}
	}
	return base
}

func dashboardRootRepository(repo store.Repository) store.Repository {
	for {
		unwrapper, ok := repo.(storeWrapper)
		if !ok {
			return repo
		}
		repo = unwrapper.UnwrapStoreRepository()
	}
}

func (s *dashboardLiveSessionNotifyingStore) UnwrapStoreRepository() store.Repository {
	return s.Repository
}

func (s *dashboardLiveSessionNotifyingStore) notifyLiveSessions(reason string) {
	if s.notify == nil {
		return
	}
	s.notify(DashboardDomainLiveSessions, reason)
}

func (s *dashboardLiveSessionNotifyingStore) CreateLiveSession(accountID, strategyID string) (domain.LiveSession, error) {
	session, err := s.Repository.CreateLiveSession(accountID, strategyID)
	if err == nil {
		s.notifyLiveSessions("live-session-created")
	}
	return session, err
}

func (s *dashboardLiveSessionNotifyingStore) UpdateLiveSession(session domain.LiveSession) (domain.LiveSession, error) {
	updated, err := s.Repository.UpdateLiveSession(session)
	if err == nil {
		s.notifyLiveSessions("live-session-updated")
	}
	return updated, err
}

func (s *dashboardLiveSessionNotifyingStore) DeleteLiveSession(sessionID string) error {
	err := s.Repository.DeleteLiveSession(sessionID)
	if err == nil {
		s.notifyLiveSessions("live-session-deleted")
	}
	return err
}

func (s *dashboardLiveSessionNotifyingStore) UpdateLiveSessionStatus(sessionID, status string) (domain.LiveSession, error) {
	session, err := s.Repository.UpdateLiveSessionStatus(sessionID, status)
	if err == nil {
		s.notifyLiveSessions("live-session-status-updated")
	}
	return session, err
}

func (s *dashboardLiveSessionNotifyingStore) UpdateLiveSessionState(sessionID string, state map[string]any) (domain.LiveSession, error) {
	session, err := s.Repository.UpdateLiveSessionState(sessionID, state)
	if err == nil {
		s.notifyLiveSessions("live-session-state-updated")
	}
	return session, err
}

func (s *dashboardLiveSessionNotifyingCASStore) UpdateLiveSessionStateIfControlRequest(sessionID, requestID string, version int64, state map[string]any) (domain.LiveSession, bool, error) {
	session, ok, err := s.cas.UpdateLiveSessionStateIfControlRequest(sessionID, requestID, version, state)
	if err == nil && ok {
		s.notifyLiveSessions("live-session-control-state-updated")
	}
	return session, ok, err
}
