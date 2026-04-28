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
	if options.RecoverLiveTrading || options.StartLiveSync || options.StartRuntimeEventConsumer || options.StartTelegram || options.StartDashboard || options.StartReadOnlyRuntimeSupervisor {
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
	if options.StartReadOnlyRuntimeSupervisor {
		t.Fatal("expected live-runner not to start read-only supervisor")
	}
}

func TestRuntimeOptionsForAPIDoesNotStartLiveSessionScanner(t *testing.T) {
	options := RuntimeOptionsForRole("api")
	if options.StartLiveSessionControlScanner {
		t.Fatal("expected api role not to start live session scanner")
	}
	if !options.StartDashboard {
		t.Fatal("expected api role to start dashboard")
	}
	if options.StartReadOnlyRuntimeSupervisor {
		t.Fatal("expected api role not to start read-only supervisor")
	}
}

func TestRuntimeOptionsForSupervisorOnlyStartsReadOnlySupervisor(t *testing.T) {
	options := RuntimeOptionsForRole("supervisor")
	if !options.StartReadOnlyRuntimeSupervisor {
		t.Fatal("expected supervisor role to start read-only runtime supervisor")
	}
	if options.WarmLiveMarketData ||
		options.StartTelegram ||
		options.RecoverLiveTrading ||
		options.StartLiveSync ||
		options.StartDashboard ||
		options.StartRuntimeEventConsumer ||
		options.StartSignalRuntimeScanner ||
		options.StartLiveSessionControlScanner {
		t.Fatalf("expected supervisor role to avoid business runtime components, got %+v", options)
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
	if options.StartReadOnlyRuntimeSupervisor {
		t.Fatal("expected monolith not to start read-only supervisor by default")
	}
}
