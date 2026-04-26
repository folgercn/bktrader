package postgres

import (
	"os"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestRuntimeLeaseAcquireHeartbeatReleasePostgres(t *testing.T) {
	dsn := os.Getenv("BKTRADER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("BKTRADER_TEST_POSTGRES_DSN is not set")
	}
	if err := Migrate(dsn); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	store, err := New(dsn)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	resourceID := "runtime-lease-pg-" + time.Now().UTC().Format("20060102150405.000000000")
	first, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   resourceID,
		OwnerID:      "runner-a",
		TTL:          time.Minute,
	})
	if err != nil || !ok {
		t.Fatalf("AcquireRuntimeLease first failed: ok=%v err=%v", ok, err)
	}
	blocked, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceSignalRuntimeSession,
		ResourceID:   resourceID,
		OwnerID:      "runner-b",
		TTL:          time.Minute,
	})
	if err != nil {
		t.Fatalf("AcquireRuntimeLease blocked owner errored: %v", err)
	}
	if ok || blocked.OwnerID != first.OwnerID {
		t.Fatalf("expected active runner-a lease to block runner-b, ok=%v lease=%#v", ok, blocked)
	}

	heartbeat, alive, err := store.HeartbeatRuntimeLease(domain.RuntimeLeaseResourceSignalRuntimeSession, resourceID, "runner-a", time.Minute)
	if err != nil || !alive {
		t.Fatalf("HeartbeatRuntimeLease failed: alive=%v err=%v", alive, err)
	}
	if heartbeat.OwnerID != "runner-a" {
		t.Fatalf("expected heartbeat to keep owner runner-a, got %#v", heartbeat)
	}

	released, err := store.ReleaseRuntimeLease(domain.RuntimeLeaseResourceSignalRuntimeSession, resourceID, "runner-b")
	if err != nil || released {
		t.Fatalf("expected non-owner release to no-op, released=%v err=%v", released, err)
	}
	released, err = store.ReleaseRuntimeLease(domain.RuntimeLeaseResourceSignalRuntimeSession, resourceID, "runner-a")
	if err != nil || !released {
		t.Fatalf("expected owner release, released=%v err=%v", released, err)
	}
}

func TestRuntimeLeaseExpiredTakeoverPostgres(t *testing.T) {
	dsn := os.Getenv("BKTRADER_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("BKTRADER_TEST_POSTGRES_DSN is not set")
	}
	if err := Migrate(dsn); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	store, err := New(dsn)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer store.Close()

	resourceID := "runtime-lease-pg-expired-" + time.Now().UTC().Format("20060102150405.000000000")
	if _, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceAccountSync,
		ResourceID:   resourceID,
		OwnerID:      "runner-a",
		TTL:          time.Millisecond,
	}); err != nil || !ok {
		t.Fatalf("AcquireRuntimeLease first failed: ok=%v err=%v", ok, err)
	}
	time.Sleep(5 * time.Millisecond)
	taken, ok, err := store.AcquireRuntimeLease(domain.RuntimeLeaseAcquireRequest{
		ResourceType: domain.RuntimeLeaseResourceAccountSync,
		ResourceID:   resourceID,
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
