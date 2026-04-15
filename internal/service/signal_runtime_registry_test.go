package service

import (
	"testing"

	"github.com/wuyaocheng/bktrader/internal/store/memory"
)

func TestBuildSignalRuntimePlanWithoutBindingsIsNotReady(t *testing.T) {
	platform := NewPlatform(memory.NewStore())

	plan, err := platform.BuildSignalRuntimePlan("live-main", "strategy-bk-1d")
	if err != nil {
		t.Fatalf("build runtime plan failed: %v", err)
	}
	if boolValue(plan["ready"]) {
		t.Fatalf("expected runtime plan without subscriptions to be not ready: %#v", plan)
	}
	if subscriptions := metadataList(plan["subscriptions"]); len(subscriptions) != 0 {
		t.Fatalf("expected no subscriptions for unbound strategy, got %#v", subscriptions)
	}
	if matched := metadataList(plan["matchedBindings"]); len(matched) != 0 {
		t.Fatalf("expected no matched bindings for unbound strategy, got %#v", matched)
	}
	if missing := metadataList(plan["missingBindings"]); len(missing) != 0 {
		t.Fatalf("expected no missing bindings for unbound strategy, got %#v", missing)
	}
}
