package app

import (
	"context"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/config"
	"github.com/wuyaocheng/bktrader/internal/service"
	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestRuntimeOptionsForSignalRuntimeRunner(t *testing.T) {
	options := RuntimeOptionsForRole("signal-runtime-runner")
	if !options.WarmLiveMarketData {
		t.Fatal("expected signal-runtime-runner to warm market data")
	}
	if !options.StartSignalRuntimeScanner {
		t.Fatal("expected signal-runtime-runner to start signal runtime scanner")
	}
	if options.RecoverLiveTrading || options.StartLiveSync || options.StartRuntimeEventConsumer || options.StartTelegram || options.StartDashboard || options.StartReadOnlyRuntimeSupervisor || options.StartPretouchModelScheduler {
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
	if !options.StartPretouchModelScheduler {
		t.Fatal("expected live-runner to start pretouch model scheduler")
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
	if options.StartPretouchModelScheduler {
		t.Fatal("expected api role not to start pretouch model scheduler")
	}
}

func TestRuntimeOptionsForAPIStartsReadOnlySupervisorWhenTargetsConfigured(t *testing.T) {
	options := RuntimeOptionsForConfig(config.Config{
		ProcessRole:       "api",
		SupervisorTargets: []string{"api=http://127.0.0.1:8080"},
	})
	if !options.StartReadOnlyRuntimeSupervisor {
		t.Fatal("expected api role to start read-only supervisor when SUPERVISOR_TARGETS is configured")
	}
	if options.StartLiveSessionControlScanner || options.RecoverLiveTrading || options.StartLiveSync || options.StartPretouchModelScheduler {
		t.Fatalf("expected api role to avoid live runtime components, got %+v", options)
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
		options.StartLiveSessionControlScanner ||
		options.StartPretouchModelScheduler {
		t.Fatalf("expected supervisor role to avoid business runtime components, got %+v", options)
	}
}

func TestRuntimeSupervisorOptionsForConfigWiresNoopExecutorReadiness(t *testing.T) {
	options := runtimeSupervisorOptionsForConfig(config.Config{
		SupervisorContainerRestart:   true,
		SupervisorFallbackAutoSubmit: true,
		SupervisorContainerExecutor:  "noop",
	})
	if !options.EnableContainerFallback {
		t.Fatal("expected container fallback opt-in to remain enabled")
	}
	if !options.ContainerFallbackAutoSubmit {
		t.Fatal("expected container fallback auto-submit opt-in to be wired")
	}
	if options.ContainerFallbackExecutor == nil || !options.ContainerFallbackExecutor.Configured() {
		t.Fatalf("expected configured noop executor, got %+v", options.ContainerFallbackExecutor)
	}
	descriptor := options.ContainerFallbackExecutor.Descriptor()
	if descriptor.Kind != "noop" || !descriptor.DryRun {
		t.Fatalf("expected dry-run noop descriptor, got %+v", descriptor)
	}
	result, err := options.ContainerFallbackExecutor.Restart(context.Background(), service.RuntimeSupervisorTarget{Name: "api"}, "test")
	if err != nil {
		t.Fatalf("noop executor restart returned error: %v", err)
	}
	if result.Executed {
		t.Fatalf("noop executor must not execute restart, got %+v", result)
	}
}

func TestRuntimeSupervisorOptionsForConfigLeavesExecutorUnconfiguredByDefault(t *testing.T) {
	options := runtimeSupervisorOptionsForConfig(config.Config{
		SupervisorContainerRestart:  true,
		SupervisorContainerExecutor: "",
	})
	if options.ContainerFallbackExecutor != nil {
		t.Fatalf("expected no container fallback executor by default, got %+v", options.ContainerFallbackExecutor)
	}
	if options.ContainerFallbackAutoSubmit {
		t.Fatalf("expected container fallback auto submit to be disabled by default, got %+v", options)
	}
}

func TestRuntimeSupervisorOptionsForConfigWiresArmedCommandExecutor(t *testing.T) {
	options := runtimeSupervisorOptionsForConfig(config.Config{
		SupervisorContainerRestart:          true,
		SupervisorContainerExecutor:         "command",
		SupervisorContainerExecutorArmed:    true,
		SupervisorContainerExecutorCommands: `{"api":{"path":"/bin/echo","args":["restart","api"],"timeoutSeconds":5}}`,
	})
	if !options.EnableContainerFallback || !options.ContainerFallbackExecutorArmed {
		t.Fatalf("expected armed command executor options, got %+v", options)
	}
	if options.ContainerFallbackExecutor == nil || !options.ContainerFallbackExecutor.Configured() {
		t.Fatalf("expected configured command executor, got %+v", options.ContainerFallbackExecutor)
	}
	descriptor := options.ContainerFallbackExecutor.Descriptor()
	if descriptor.Kind != "command" || descriptor.DryRun {
		t.Fatalf("expected non-dry-run command descriptor, got %+v", descriptor)
	}
	allowlist, ok := options.ContainerFallbackExecutor.(service.ContainerFallbackTargetAllowlist)
	if !ok || !allowlist.ContainerFallbackTargetAllowed(service.RuntimeSupervisorTarget{Name: "api"}) || allowlist.ContainerFallbackTargetAllowed(service.RuntimeSupervisorTarget{Name: "worker"}) {
		t.Fatalf("expected command executor target allowlist to be enforced")
	}
}

func TestRuntimeSupervisorOptionsForConfigWiresNodeAgentExecutor(t *testing.T) {
	options := runtimeSupervisorOptionsForConfig(config.Config{
		SupervisorTargets:                 []string{"api=http://127.0.0.1:8080"},
		SupervisorContainerRestart:        true,
		SupervisorContainerExecutor:       "node-agent",
		SupervisorContainerExecutorArmed:  true,
		SupervisorNodeAgentBaseURL:        "http://127.0.0.1:18081",
		SupervisorNodeAgentToken:          "agent-token",
		SupervisorNodeAgentTimeoutSeconds: 10,
	})
	if !options.EnableContainerFallback || !options.ContainerFallbackExecutorArmed {
		t.Fatalf("expected armed node-agent executor options, got %+v", options)
	}
	if options.ContainerFallbackExecutor == nil || !options.ContainerFallbackExecutor.Configured() {
		t.Fatalf("expected configured node-agent executor, got %+v", options.ContainerFallbackExecutor)
	}
	descriptor := options.ContainerFallbackExecutor.Descriptor()
	if descriptor.Kind != "node-agent" || descriptor.DryRun {
		t.Fatalf("expected non-dry-run node-agent descriptor, got %+v", descriptor)
	}
	allowlist, ok := options.ContainerFallbackExecutor.(service.ContainerFallbackTargetAllowlist)
	if !ok || !allowlist.ContainerFallbackTargetAllowed(service.RuntimeSupervisorTarget{Name: "api"}) || allowlist.ContainerFallbackTargetAllowed(service.RuntimeSupervisorTarget{Name: "worker"}) {
		t.Fatalf("expected node-agent executor target allowlist to be enforced")
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
		!options.StartLiveSessionControlScanner ||
		!options.StartPretouchModelScheduler {
		t.Fatalf("expected monolith to start all runtime components, got %+v", options)
	}
	if options.StartReadOnlyRuntimeSupervisor {
		t.Fatal("expected monolith not to start read-only supervisor by default")
	}
}

func TestStartRuntimeComponentsContinuesAfterRuntimeEventConsumerFailure(t *testing.T) {
	platform := service.NewPlatform(memory.NewStore())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	StartRuntimeComponents(ctx, platform, config.Config{
		ProcessRole:     "live-runner",
		RuntimeEventBus: "nats",
		NATSURL:         "nats://127.0.0.1:1",
	}, RuntimeOptions{
		StartRuntimeEventConsumer:      true,
		StartLiveSessionControlScanner: true,
	})

	if platform.RuntimeEventConsumerEnabled() {
		t.Fatal("expected runtime event consumer to remain disabled after connection failure")
	}
	status := platform.LiveSessionControlScannerStatus()
	if !status.Enabled {
		t.Fatalf("expected live session control scanner to start after consumer failure, got %+v", status)
	}
}
