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

	leaseCtx, release, acquired, err := platform.acquireRuntimeLeaseWithTiming(
		context.Background(),
		domain.RuntimeLeaseResourceSignalRuntimeSession,
		"runtime-heartbeat-loss",
		time.Minute,
		time.Millisecond,
	)
	if err != nil || !acquired {
		t.Fatalf("acquireRuntimeLeaseWithTiming failed: acquired=%v err=%v", acquired, err)
	}
	defer release()

	if released, err := store.ReleaseRuntimeLease(
		domain.RuntimeLeaseResourceSignalRuntimeSession,
		"runtime-heartbeat-loss",
		"runner-local",
	); err != nil || !released {
		t.Fatalf("pre-release lease failed: released=%v err=%v", released, err)
	}

	select {
	case <-leaseCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("expected heartbeat ownership loss to cancel lease context")
	}
}
