package app

import "testing"

func TestRuntimeOptionsForSignalRuntimeRunner(t *testing.T) {
	options := RuntimeOptionsForRole("signal-runtime-runner")
	if !options.WarmLiveMarketData {
		t.Fatal("expected signal-runtime-runner to warm market data")
	}
	if !options.StartSignalRuntimeScanner {
		t.Fatal("expected signal-runtime-runner to start signal runtime scanner")
	}
	if options.RecoverLiveTrading || options.StartLiveSync || options.StartRuntimeEventConsumer || options.StartTelegram || options.StartDashboard {
		t.Fatalf("expected signal-runtime-runner to avoid live/telegram/dashboard components, got %+v", options)
	}
}

func TestRuntimeOptionsForLiveRunnerDoesNotStartSignalRuntimeScanner(t *testing.T) {
	options := RuntimeOptionsForRole("live-runner")
	if options.WarmLiveMarketData {
		t.Fatal("expected live-runner to leave market data warmup to signal-runtime-runner")
	}
	if options.StartSignalRuntimeScanner {
		t.Fatal("expected live-runner not to start signal runtime scanner")
	}
	if !options.RecoverLiveTrading || !options.StartLiveSync || !options.StartRuntimeEventConsumer || !options.StartLiveSessionControlScanner {
		t.Fatalf("expected live-runner to keep live recovery/sync/event consumer, got %+v", options)
	}
}

func TestRuntimeOptionsForMonolithStartAllRuntimeComponents(t *testing.T) {
	options := RuntimeOptionsForRole("monolith")
	if !options.WarmLiveMarketData ||
		!options.StartTelegram ||
		!options.RecoverLiveTrading ||
		!options.StartLiveSync ||
		!options.StartDashboard ||
		!options.StartRuntimeEventConsumer ||
		!options.StartSignalRuntimeScanner ||
		!options.StartLiveSessionControlScanner {
		t.Fatalf("expected monolith to start all runtime components, got %+v", options)
	}
}
