package service

import (
	"testing"
	"time"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestAckNotificationNotifiesDashboard(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	if _, err := platform.AckNotification("alert-1"); err != nil {
		t.Fatalf("AckNotification returned error: %v", err)
	}

	change := waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)
	if change.Domain != DashboardDomainNotifications || change.Reason != "notification-acked" {
		t.Fatalf("unexpected dashboard change: %+v", change)
	}
}

func TestUnackNotificationNotifiesDashboard(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	if err := platform.UnackNotification("alert-1"); err != nil {
		t.Fatalf("UnackNotification returned error: %v", err)
	}

	change := waitDashboardChange(t, platform.DashboardBroker(), 100*time.Millisecond)
	if change.Domain != DashboardDomainNotifications || change.Reason != "notification-unacked" {
		t.Fatalf("unexpected dashboard change: %+v", change)
	}
}

func waitDashboardChange(t *testing.T, broker *DashboardBroker, timeout time.Duration) dashboardChange {
	t.Helper()
	select {
	case change := <-broker.changes:
		return change
	case <-time.After(timeout):
		t.Fatal("timed out waiting for dashboard change")
		return dashboardChange{}
	}
}
