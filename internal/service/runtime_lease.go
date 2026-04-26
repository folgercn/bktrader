package service

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const (
	runtimeLeaseTTL               = 30 * time.Second
	runtimeLeaseHeartbeatInterval = 10 * time.Second
)

var ErrRuntimeLeaseNotAcquired = errors.New("runtime lease not acquired")

func (p *Platform) setRuntimeLeaseOwnerIDForTest(ownerID string) {
	p.runtimeLeaseOwnerID = ownerID
}

func (p *Platform) acquireSignalRuntimeSessionLease(ctx context.Context, sessionID string) (context.Context, func(), bool, error) {
	return p.acquireRuntimeLease(ctx, domain.RuntimeLeaseResourceSignalRuntimeSession, sessionID)
}

func (p *Platform) acquireRuntimeLease(ctx context.Context, resourceType, resourceID string) (context.Context, func(), bool, error) {
	return p.acquireRuntimeLeaseWithTiming(ctx, resourceType, resourceID, runtimeLeaseTTL, runtimeLeaseHeartbeatInterval)
}

func (p *Platform) acquireRuntimeLeaseWithTiming(ctx context.Context, resourceType, resourceID string, ttl, heartbeatInterval time.Duration) (context.Context, func(), bool, error) {
	ownerID := p.runtimeLeaseOwnerID
	lease, ok, err := p.store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		OwnerID:      ownerID,
		TTL:          ttl,
	})
	if err != nil || !ok {
		return ctx, func() {}, ok, err
	}

	leaseCtx, cancel := context.WithCancel(ctx)
	logger := p.logger("service.runtime_lease",
		"resource_type", resourceType,
		"resource_id", resourceID,
		"owner_id", ownerID,
	)
	logger.Debug("runtime lease acquired", "expires_at", lease.ExpiresAt)

	var once sync.Once
	var ownershipLost atomic.Bool
	release := func() {
		once.Do(func() {
			cancel()
			if ownershipLost.Load() {
				logger.Debug("runtime lease release skipped after ownership loss")
				return
			}
			if released, err := p.store.ReleaseRuntimeLease(resourceType, resourceID, ownerID); err != nil {
				logger.Warn("release runtime lease failed", "error", err)
			} else if released {
				logger.Debug("runtime lease released")
			}
		})
	}

	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-leaseCtx.Done():
				return
			case <-ticker.C:
				updated, alive, err := p.store.HeartbeatRuntimeLease(resourceType, resourceID, ownerID, ttl)
				if err != nil || !alive {
					if err != nil {
						logger.Warn("runtime lease heartbeat failed; cancelling owned runner", "error", err)
					} else {
						logger.Warn("runtime lease ownership lost; cancelling owned runner")
					}
					ownershipLost.Store(true)
					cancel()
					return
				}
				logger.Debug("runtime lease heartbeat extended", "expires_at", updated.ExpiresAt)
			}
		}
	}()

	return leaseCtx, release, true, nil
}
