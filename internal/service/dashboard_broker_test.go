package service

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func newTestDashboardBroker(window time.Duration) *DashboardBroker {
	b := NewDashboardBroker(nil)
	b.coalesceWindow = window
	return b
}

func TestNotifyChangedNonBlocking(t *testing.T) {
	b := newTestDashboardBroker(10 * time.Millisecond)
	for i := 0; i < cap(b.changes); i++ {
		b.changes <- dashboardChange{Domain: DashboardDomainOrders, Reason: "fill-buffer"}
	}

	done := make(chan struct{})
	go func() {
		b.NotifyChanged(DashboardDomainOrders, "channel-full")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("NotifyChanged blocked when change channel was full")
	}
}

func TestDashboardBrokerCoalescesSameDomain(t *testing.T) {
	b := newTestDashboardBroker(20 * time.Millisecond)
	var fetches atomic.Int64
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		fetches.Add(1)
		return map[string]any{"orders": []string{"order-1"}}, nil
	})
	_, ch := b.Subscribe(10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.StartEventLoop(ctx)

	for i := 0; i < 100; i++ {
		b.NotifyChanged(DashboardDomainOrders, "test")
	}

	event := waitDashboardEvent(t, ch, 250*time.Millisecond)
	if event.Type != string(DashboardDomainOrders) || event.Action != "snapshot" {
		t.Fatalf("unexpected event: %+v", event)
	}
	time.Sleep(40 * time.Millisecond)
	if got := fetches.Load(); got != 1 {
		t.Fatalf("expected one fetch for coalesced domain, got %d", got)
	}
}

func TestDashboardBrokerCoalescesMultipleDomains(t *testing.T) {
	b := newTestDashboardBroker(20 * time.Millisecond)
	var orderFetches atomic.Int64
	var fillFetches atomic.Int64
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		orderFetches.Add(1)
		return map[string]any{"orders": []string{"order-1"}}, nil
	})
	b.RegisterDashboardFetchFunc(DashboardDomainFills, func() (any, error) {
		fillFetches.Add(1)
		return map[string]any{"fills": []string{"fill-1"}}, nil
	})
	_, ch := b.Subscribe(10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go b.StartEventLoop(ctx)

	for i := 0; i < 10; i++ {
		b.NotifyChanged(DashboardDomainOrders, "test")
		b.NotifyChanged(DashboardDomainFills, "test")
	}

	seen := make(map[string]bool)
	for len(seen) < 2 {
		event := waitDashboardEvent(t, ch, 250*time.Millisecond)
		seen[event.Type] = true
	}
	if !seen[string(DashboardDomainOrders)] || !seen[string(DashboardDomainFills)] {
		t.Fatalf("expected orders and fills events, got %#v", seen)
	}
	if got := orderFetches.Load(); got != 1 {
		t.Fatalf("expected one orders fetch, got %d", got)
	}
	if got := fillFetches.Load(); got != 1 {
		t.Fatalf("expected one fills fetch, got %d", got)
	}
}

func TestDashboardBrokerNoSubscriberSkipsFetch(t *testing.T) {
	b := newTestDashboardBroker(10 * time.Millisecond)
	var fetches atomic.Int64
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		fetches.Add(1)
		return map[string]any{"orders": []string{"order-1"}}, nil
	})

	b.publishSnapshotForDomain(DashboardDomainOrders)

	if got := fetches.Load(); got != 0 {
		t.Fatalf("expected no fetch without subscribers, got %d", got)
	}
}

func TestDashboardBrokerRegisterFetchFuncReplacesFetcher(t *testing.T) {
	b := newTestDashboardBroker(10 * time.Millisecond)
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		return map[string]any{"orders": []string{"old"}}, nil
	})
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		return map[string]any{"orders": []string{"new"}}, nil
	})
	_, ch := b.Subscribe(10)

	b.publishSnapshotForDomain(DashboardDomainOrders)

	event := waitDashboardEvent(t, ch, 100*time.Millisecond)
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected map payload, got %#v", event.Payload)
	}
	orders, ok := payload["orders"].([]string)
	if !ok || len(orders) != 1 || orders[0] != "new" {
		t.Fatalf("expected replacement fetcher payload, got %#v", event.Payload)
	}
}

func TestDashboardBrokerRegisterFetchFuncIgnoresInvalidInput(t *testing.T) {
	b := newTestDashboardBroker(10 * time.Millisecond)
	b.RegisterDashboardFetchFunc("", func() (any, error) {
		return map[string]any{"orders": []string{"unexpected"}}, nil
	})
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, nil)
	if len(b.fetchFuncs) != 0 {
		t.Fatalf("expected invalid fetch funcs to be ignored, got %#v", b.fetchFuncs)
	}
}

func TestDashboardBrokerRegisterFetchFuncConcurrentWithSnapshots(t *testing.T) {
	b := newTestDashboardBroker(10 * time.Millisecond)
	subID, ch := b.Subscribe(512)
	done := make(chan struct{})
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for {
			select {
			case <-done:
				return
			case <-ch:
			}
		}
	}()
	defer func() {
		close(done)
		<-drained
		b.Unsubscribe(subID)
	}()

	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		return map[string]any{"orders": []string{"initial"}}, nil
	})

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			value := i
			b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
				return map[string]any{"orders": []int{value}}, nil
			})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			b.publishSnapshotForDomain(DashboardDomainOrders)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			b.PushInitialSnapshot(subID)
		}
	}()
	wg.Wait()
}

func TestDashboardBrokerInitialSnapshotDoesNotSuppressBroadcast(t *testing.T) {
	b := newTestDashboardBroker(10 * time.Millisecond)
	currentPayload := atomic.Value{}
	currentPayload.Store(map[string]any{"orders": []string{"h1"}})
	b.RegisterDashboardFetchFunc(DashboardDomainOrders, func() (any, error) {
		return currentPayload.Load(), nil
	})

	subA, chA := b.Subscribe(10)
	b.PushInitialSnapshot(subA)
	eventA := waitDashboardEvent(t, chA, 100*time.Millisecond)
	if eventA.Type != string(DashboardDomainOrders) {
		t.Fatalf("expected initial orders event for subscriber A, got %+v", eventA)
	}

	currentPayload.Store(map[string]any{"orders": []string{"h2"}})

	subB, chB := b.Subscribe(10)
	b.PushInitialSnapshot(subB)
	eventB := waitDashboardEvent(t, chB, 100*time.Millisecond)
	if eventB.Type != string(DashboardDomainOrders) {
		t.Fatalf("expected initial orders event for subscriber B, got %+v", eventB)
	}

	b.publishSnapshotForDomain(DashboardDomainOrders)

	broadcastA := waitDashboardEvent(t, chA, 100*time.Millisecond)
	if broadcastA.Type != string(DashboardDomainOrders) || broadcastA.Action != "snapshot" {
		t.Fatalf("expected orders broadcast for subscriber A, got %+v", broadcastA)
	}
	broadcastB := waitDashboardEvent(t, chB, 100*time.Millisecond)
	if broadcastB.Type != string(DashboardDomainOrders) || broadcastB.Action != "snapshot" {
		t.Fatalf("expected orders broadcast for subscriber B, got %+v", broadcastB)
	}
}

func TestDashboardBrokerPollingTriggersNotifyChanged(t *testing.T) {
	b := newTestDashboardBroker(10 * time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b.StartPolling(ctx, config.Config{
		DashboardLiveSessionsPollMs:  10,
		DashboardPositionsPollMs:     10,
		DashboardOrdersPollMs:        10,
		DashboardFillsPollMs:         10,
		DashboardAlertsPollMs:        10,
		DashboardNotificationsPollMs: 10,
		DashboardMonitorHealthPollMs: 10,
	})

	select {
	case change := <-b.changes:
		if change.Domain == "" || change.Reason != "polling" {
			t.Fatalf("unexpected polling change: %+v", change)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected polling to enqueue a dashboard change")
	}
}

func TestStartDashboardBrokerPollingPublishesThroughEventLoop(t *testing.T) {
	platform := NewPlatform(memory.NewStore())
	b := platform.DashboardBroker()
	b.coalesceWindow = 10 * time.Millisecond
	b.RegisterDashboardFetchFunc(DashboardDomainNotifications, func() (any, error) {
		return map[string]any{"notifications": []string{"notification-1"}}, nil
	})
	_, ch := b.Subscribe(10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	slowPollMs := int((24 * time.Hour) / time.Millisecond)
	platform.StartDashboardBroker(ctx, config.Config{
		DashboardLiveSessionsPollMs:  slowPollMs,
		DashboardPositionsPollMs:     slowPollMs,
		DashboardOrdersPollMs:        slowPollMs,
		DashboardFillsPollMs:         slowPollMs,
		DashboardAlertsPollMs:        slowPollMs,
		DashboardNotificationsPollMs: 10,
		DashboardMonitorHealthPollMs: slowPollMs,
	})

	event := waitDashboardEvent(t, ch, 250*time.Millisecond)
	if event.Type != string(DashboardDomainNotifications) || event.Action != "snapshot" {
		t.Fatalf("unexpected event from broker startup path: %+v", event)
	}
}

func waitDashboardEvent(t *testing.T, ch <-chan DashboardEvent, timeout time.Duration) DashboardEvent {
	t.Helper()
	select {
	case event := <-ch:
		return event
	case <-time.After(timeout):
		t.Fatal("timed out waiting for dashboard event")
		return DashboardEvent{}
	}
}
