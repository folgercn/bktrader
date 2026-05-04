package memory

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestRuntimeLeaseAcquireBlocksActiveOwnerAndAllowsSameOwnerHeartbeatRelease(t *testing.T) {
	store := NewStore()
	req := domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   "runtime-1",
		OwnerID:      "runner-a",
		TTL:          time.Minute,
	}
	first, ok, err := store.AcquireRuntimeLease(req)
	if err != nil || !ok {
		t.Fatalf("AcquireRuntimeLease first failed: ok=%v err=%v", ok, err)
	}
	if first.OwnerID != "runner-a" {
		t.Fatalf("expected runner-a owner, got %#v", first)
	}

	blocked, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		OwnerID:      "runner-b",
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatalf("AcquireRuntimeLease second owner failed: %v", err)
	}
	if ok || blocked.OwnerID != "runner-a" {
		t.Fatalf("expected active runner-a lease to block runner-b, ok=%v lease=%#v", ok, blocked)
	}

	again, ok, err := store.AcquireRuntimeLease(req)
	if err != nil || !ok {
		t.Fatalf("AcquireRuntimeLease same owner failed: ok=%v err=%v", ok, err)
	}
	if !again.AcquiredAt.Equal(first.AcquiredAt) {
		t.Fatalf("expected same owner reacquire to preserve acquiredAt, first=%s again=%s", first.AcquiredAt, again.AcquiredAt)
	}

	time.Sleep(time.Millisecond)
	heartbeat, alive, err := store.HeartbeatRuntimeLease(req.ResourceType, req.ResourceID, req.OwnerID, time.Minute)
	if err != nil || !alive {
		t.Fatalf("HeartbeatRuntimeLease owner failed: alive=%v err=%v", alive, err)
	}
	if !heartbeat.ExpiresAt.After(first.ExpiresAt) {
		t.Fatalf("expected heartbeat to extend expiresAt, first=%s heartbeat=%s", first.ExpiresAt, heartbeat.ExpiresAt)
	}
	if _, alive, err := store.HeartbeatRuntimeLease(req.ResourceType, req.ResourceID, "runner-b", time.Minute); err != nil || alive {
		t.Fatalf("expected non-owner heartbeat to fail, alive=%v err=%v", alive, err)
	}

	released, err := store.ReleaseRuntimeLease(req.ResourceType, req.ResourceID, "runner-b")
	if err != nil || released {
		t.Fatalf("expected non-owner release to no-op, released=%v err=%v", released, err)
	}
	released, err = store.ReleaseRuntimeLease(req.ResourceType, req.ResourceID, req.OwnerID)
	if err != nil || !released {
		t.Fatalf("expected owner release to delete lease, released=%v err=%v", released, err)
	}
}

func TestRuntimeLeaseExpiredTakeover(t *testing.T) {
	store := NewStore()
	req := domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceLiveSession,
		ResourceID:   "live-1",
		OwnerID:      "runner-a",
		TTL:          time.Millisecond,
	}
	if _, ok, err := store.AcquireRuntimeLease(req); err != nil || !ok {
		t.Fatalf("AcquireRuntimeLease first failed: ok=%v err=%v", ok, err)
	}
	time.Sleep(5 * time.Millisecond)

	taken, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		OwnerID:      "runner-b",
		TTL:          time.Minute,
	})
	if err != nil || !ok {
		t.Fatalf("AcquireRuntimeLease takeover failed: ok=%v err=%v", ok, err)
	}
	if taken.OwnerID != "runner-b" {
		t.Fatalf("expected runner-b takeover, got %#v", taken)
	}
}

func TestRuntimeLeaseConcurrentAcquireSingleOwnerWins(t *testing.T) {
	store := NewStore()
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
				ResourceType: domain.RuntimeLeaseResourceAccountSync,
				ResourceID:   "account-1",
				OwnerID:      fmt.Sprintf("runner-%d", i),
				TTL:          time.Minute,
			})
			if err != nil {
				t.Errorf("AcquireRuntimeLease failed: %v", err)
				return
			}
			if ok {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := wins.Load(); got != 1 {
		t.Fatalf("expected exactly one owner to acquire active lease, got %d", got)
	}
}
