package service

import (
	"math"
	"strings"
	"testing"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func TestBuildFillReconciliationPlanCreatesNewRealFill(t *testing.T) {
	order := fillReconcileTestOrder(1)
	realFill := fillReconcileReal(order.ID, "trade-1", 1)

	plan, err := BuildFillReconciliationPlan(order, nil, []FillReconciliationInput{fillReconcileInput(realFill, FillSourceReal)}, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	requireFillReconcileQuantity(t, sumFillQty(plan.CreateFills), 1)
	requireFillReconcileQuantity(t, sumFillQty(plan.ApplyPositionFills), 1)
	requireFillReconcileQuantity(t, metadataFloat(plan, "filledQuantity"), 1)
	requireFillReconcileQuantity(t, metadataFloat(plan, "remainingQuantity"), 0)
	if len(plan.DeleteFillIDs) != 0 {
		t.Fatalf("expected no deletes, got %v", plan.DeleteFillIDs)
	}
}

func TestBuildFillReconciliationPlanReplacesSyntheticWithRealWithoutDoubleApply(t *testing.T) {
	order := fillReconcileTestOrder(1)
	existing := []FillReconciliationInput{fillReconcileInput(fillReconcileSynthetic("fill-synthetic", order.ID, 1), FillSourceSynthetic)}
	incoming := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 1), FillSourceReal)}

	plan, err := BuildFillReconciliationPlan(order, existing, incoming, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	requireFillReconcileDeleteIDs(t, plan.DeleteFillIDs, "fill-synthetic")
	requireFillReconcileQuantity(t, sumRealQty(plan.CreateFills), 1)
	requireFillReconcileQuantity(t, sumRemainderQty(plan.CreateFills), 0)
	requireFillReconcileQuantity(t, sumFillQty(plan.ApplyPositionFills), 0)
	requireFillReconcileQuantity(t, metadataFloat(plan, "filledQuantity"), 1)
}

func TestBuildFillReconciliationPlanCreatesRemainderForPartialRealFill(t *testing.T) {
	order := fillReconcileTestOrder(1)
	existing := []FillReconciliationInput{fillReconcileInput(fillReconcileSynthetic("fill-synthetic", order.ID, 1), FillSourceSynthetic)}
	incoming := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 0.4), FillSourceReal)}

	plan, err := BuildFillReconciliationPlan(order, existing, incoming, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	requireFillReconcileDeleteIDs(t, plan.DeleteFillIDs, "fill-synthetic")
	requireFillReconcileQuantity(t, sumRealQty(plan.CreateFills), 0.4)
	requireFillReconcileQuantity(t, sumRemainderQty(plan.CreateFills), 0.6)
	requireFillReconcileQuantity(t, sumFillQty(plan.ApplyPositionFills), 0)
	requireFillReconcileQuantity(t, metadataFloat(plan, "filledQuantity"), 1)
}

func TestBuildFillReconciliationPlanBatchedRealFillsShrinkRemainder(t *testing.T) {
	order := fillReconcileTestOrder(1)
	existing := []FillReconciliationInput{
		fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 0.4), FillSourceReal),
		fillReconcileInput(fillReconcileRemainder("fill-remainder", order.ID, 0.6), FillSourceRemainder),
	}
	incoming := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-2", 0.3), FillSourceReal)}

	plan, err := BuildFillReconciliationPlan(order, existing, incoming, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	requireFillReconcileDeleteIDs(t, plan.DeleteFillIDs, "fill-remainder")
	requireFillReconcileQuantity(t, sumRealQty(plan.CreateFills), 0.3)
	requireFillReconcileQuantity(t, sumRemainderQty(plan.CreateFills), 0.3)
	requireFillReconcileQuantity(t, sumFillQty(plan.ApplyPositionFills), 0)
	requireFillReconcileQuantity(t, metadataFloat(plan, "filledQuantity"), 1)
}

func TestBuildFillReconciliationPlanAppliesOnlyRealQtyBeyondSynthetic(t *testing.T) {
	order := fillReconcileTestOrder(1)
	existing := []FillReconciliationInput{fillReconcileInput(fillReconcileSynthetic("fill-synthetic", order.ID, 0.6), FillSourceSynthetic)}
	incoming := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 1), FillSourceReal)}

	plan, err := BuildFillReconciliationPlan(order, existing, incoming, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	requireFillReconcileQuantity(t, sumRealQty(plan.CreateFills), 1)
	requireFillReconcileQuantity(t, sumRemainderQty(plan.CreateFills), 0)
	requireFillReconcileQuantity(t, sumFillQty(plan.ApplyPositionFills), 0.4)
	requireFillReconcileQuantity(t, metadataFloat(plan, "filledQuantity"), 1)
}

func TestBuildFillReconciliationPlanSkipsDuplicateRealTradeID(t *testing.T) {
	order := fillReconcileTestOrder(1)
	existing := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 1), FillSourceReal)}
	incoming := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 1), FillSourceReal)}

	plan, err := BuildFillReconciliationPlan(order, existing, incoming, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	if len(plan.CreateFills) != 0 {
		t.Fatalf("expected duplicate real fill to create nothing, got %+v", plan.CreateFills)
	}
	if len(plan.ApplyPositionFills) != 0 {
		t.Fatalf("expected duplicate real fill to apply nothing, got %+v", plan.ApplyPositionFills)
	}
	requireFillReconcileQuantity(t, metadataFloat(plan, "filledQuantity"), 1)
}

func TestBuildFillReconciliationPlanClampsIncomingRealToRemainingQuantity(t *testing.T) {
	order := fillReconcileTestOrder(1)
	incoming := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 1.2), FillSourceReal)}

	plan, err := BuildFillReconciliationPlan(order, nil, incoming, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	requireFillReconcileQuantity(t, sumRealQty(plan.CreateFills), 1)
	requireFillReconcileQuantity(t, sumFillQty(plan.ApplyPositionFills), 1)
	requireFillReconcileQuantity(t, metadataFloat(plan, "remainingQuantity"), 0)
}

func TestBuildFillReconciliationPlanRejectsInvalidQuantity(t *testing.T) {
	order := fillReconcileTestOrder(1)
	_, err := BuildFillReconciliationPlan(order, nil, []FillReconciliationInput{fillReconcileInput(domain.Fill{
		OrderID:         order.ID,
		ExchangeTradeID: "trade-1",
		Price:           68000,
		Quantity:        math.NaN(),
	}, FillSourceReal)}, FillReconcilePolicy{})
	if err == nil {
		t.Fatal("expected invalid quantity error")
	}
}

func TestBuildFillReconciliationPlanRequiresExplicitFillSource(t *testing.T) {
	order := fillReconcileTestOrder(1)
	_, err := BuildFillReconciliationPlan(order, nil, []FillReconciliationInput{{Fill: fillReconcileReal(order.ID, "trade-1", 1)}}, FillReconcilePolicy{})
	if err == nil {
		t.Fatal("expected missing fill source error")
	}
}

func TestBuildFillReconciliationPlanDoesNotDeleteSyntheticWithoutExplicitSource(t *testing.T) {
	order := fillReconcileTestOrder(1)
	existing := []FillReconciliationInput{fillReconcileInput(fillReconcileSynthetic("fill-synthetic", order.ID, 1), FillSourceReal)}
	_, err := BuildFillReconciliationPlan(order, existing, nil, FillReconcilePolicy{})
	if err == nil {
		t.Fatal("expected source/id mismatch error")
	}
}

func TestBuildFillReconciliationPlanClampsNegativeRemainingQuantity(t *testing.T) {
	order := fillReconcileTestOrder(1)
	existing := []FillReconciliationInput{fillReconcileInput(fillReconcileReal(order.ID, "trade-1", 1.2), FillSourceReal)}

	plan, err := BuildFillReconciliationPlan(order, existing, nil, FillReconcilePolicy{})
	if err != nil {
		t.Fatalf("BuildFillReconciliationPlan failed: %v", err)
	}

	requireFillReconcileQuantity(t, metadataFloat(plan, "remainingQuantity"), 0)
	if len(plan.Warnings) == 0 {
		t.Fatal("expected overfill warning")
	}
}

func fillReconcileTestOrder(quantity float64) domain.Order {
	return domain.Order{
		ID:       "order-1",
		Symbol:   "BTCUSDT",
		Side:     "BUY",
		Type:     "MARKET",
		Quantity: quantity,
		Price:    68000,
		Metadata: map[string]any{},
	}
}

func fillReconcileReal(orderID, tradeID string, quantity float64) domain.Fill {
	return domain.Fill{
		OrderID:         orderID,
		ExchangeTradeID: tradeID,
		Price:           68000,
		Quantity:        quantity,
	}
}

func fillReconcileInput(fill domain.Fill, source FillSource) FillReconciliationInput {
	return FillReconciliationInput{Fill: fill, Source: source}
}

func fillReconcileSynthetic(id, orderID string, quantity float64) domain.Fill {
	return domain.Fill{
		ID:               id,
		OrderID:          orderID,
		Price:            68000,
		Quantity:         quantity,
		DedupFingerprint: "synthetic|" + id,
	}
}

func fillReconcileRemainder(id, orderID string, quantity float64) domain.Fill {
	fill := fillReconcileSynthetic(id, orderID, quantity)
	fill.DedupFingerprint = syntheticRemainderFingerprintPrefix + orderID
	return fill
}

func requireFillReconcileDeleteIDs(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected delete ids %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected delete ids %v, got %v", want, got)
		}
	}
}

func requireFillReconcileQuantity(t *testing.T, got, want float64) {
	t.Helper()
	if !tradingQuantityEqual(got, want) {
		t.Fatalf("expected quantity %.12f, got %.12f", want, got)
	}
}

func metadataFloat(plan FillReconciliationPlan, key string) float64 {
	value, _ := plan.UpdatedMetadata[key].(float64)
	return value
}

func sumFillQty(fills []domain.Fill) float64 {
	total := 0.0
	for _, fill := range fills {
		total += fill.Quantity
	}
	return total
}

func sumRealQty(fills []domain.Fill) float64 {
	total := 0.0
	for _, fill := range fills {
		if strings.TrimSpace(fill.ExchangeTradeID) != "" {
			total += fill.Quantity
		}
	}
	return total
}

func sumRemainderQty(fills []domain.Fill) float64 {
	total := 0.0
	for _, fill := range fills {
		if strings.HasPrefix(strings.TrimSpace(fill.DedupFingerprint), syntheticRemainderFingerprintPrefix) {
			total += fill.Quantity
		}
	}
	return total
}
