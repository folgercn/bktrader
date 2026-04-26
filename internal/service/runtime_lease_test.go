package service

import (
	"context"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestRuntimeLeaseHeartbeatLossCancelsLeaseContext(t *testing.T) {
	store := memory.NewStore()
	platform := NewPlatform(store)
	platform.setRuntimeLeaseOwnerIDForTest("runner-local")
	resourceID := "runtime-heartbeat-loss"

	leaseCtx, release, acquired, err := platform.acquireRuntimeLeaseWithTiming(
		context.Background(),
		domain.RuntimeLeaseResourceSignalRuntimeSession,
		resourceID,
		time.Minute,
		time.Millisecond,
	)
	if err != nil || !acquired {
		t.Fatalf("acquireRuntimeLeaseWithTiming failed: acquired=%v err=%v", acquired, err)
	}
	defer release()

	if released, err := store.ReleaseRuntimeLease(
		domain.RuntimeLeaseResourceSignalRuntimeSession,
		resourceID,
		"runner-local",
	); err != nil || !released {
		t.Fatalf("pre-release lease failed: released=%v err=%v", released, err)
	}
	if _, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   resourceID,
		OwnerID:      "runner-other",
		TTL:          time.Minute,
	}); err != nil || !ok {
		t.Fatalf("takeover lease failed: ok=%v err=%v", ok, err)
	}

	select {
	case <-leaseCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected heartbeat ownership loss to cancel lease context")
	}
	lease, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   resourceID,
		OwnerID:      "runner-third",
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatalf("third acquire failed: %v", err)
	}
	if ok || lease.OwnerID != "runner-other" {
		t.Fatalf("expected heartbeat loss not to release takeover owner, ok=%v lease=%#v", ok, lease)
	}
}
