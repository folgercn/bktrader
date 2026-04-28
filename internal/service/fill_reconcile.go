package service

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

const syntheticRemainderFingerprintPrefix = "synthetic-remainder|"

type FillSource string

const (
	FillSourceReal      FillSource = "real"
	FillSourceSynthetic FillSource = "synthetic"
	FillSourceRemainder FillSource = "remainder"
	FillSourcePaper     FillSource = "paper"
)

type FillReconcilePolicy struct {
	AllowSyntheticFallback bool
}

type FillReconciliationPlan struct {
	DeleteFillIDs      []string
	CreateFills        []domain.Fill
	ApplyPositionFills []domain.Fill
	UpdatedMetadata    map[string]any
	Warnings           []string
}

func BuildFillReconciliationPlan(order domain.Order, existing []domain.Fill, incoming []domain.Fill, policy FillReconcilePolicy) (FillReconciliationPlan, error) {
	_ = policy
	plan := FillReconciliationPlan{
		UpdatedMetadata: map[string]any{},
	}
	if strings.TrimSpace(order.ID) == "" {
		return plan, errors.New("order id is required")
	}
	if invalidFillQuantity(order.Quantity) || !tradingQuantityPositive(order.Quantity) {
		return plan, fmt.Errorf("order quantity must be positive: %v", order.Quantity)
	}

	existingRealTradeIDs := map[string]struct{}{}
	existingFallbackFingerprints := map[string]struct{}{}
	existingRealQty := 0.0
	existingPlaceholderQty := 0.0
	existingTotalQty := 0.0
	var existingPlaceholderIDs []string
	lastKnownPrice := order.Price

	for _, fill := range existing {
		if strings.TrimSpace(fill.OrderID) != "" && fill.OrderID != order.ID {
			continue
		}
		if err := validateFillQuantity(fill); err != nil {
			return plan, fmt.Errorf("existing fill %s: %w", fill.ID, err)
		}
		if tradingQuantityPositive(fill.Price) {
			lastKnownPrice = fill.Price
		}
		existingTotalQty += fill.Quantity
		if fillReconcileSource(fill) == FillSourceReal {
			existingRealQty += fill.Quantity
			existingRealTradeIDs[strings.TrimSpace(fill.ExchangeTradeID)] = struct{}{}
			continue
		}
		fingerprint := strings.TrimSpace(fill.DedupFingerprint)
		if fingerprint == "" {
			continue
		}
		existingFallbackFingerprints[fingerprint] = struct{}{}
		existingPlaceholderQty += fill.Quantity
		if strings.TrimSpace(fill.ID) != "" {
			existingPlaceholderIDs = append(existingPlaceholderIDs, fill.ID)
		}
	}

	newRealQty := 0.0
	hasNewRealFill := false
	incomingHasRealFill := hasIncomingRealFill(incoming)
	for _, fill := range incoming {
		if strings.TrimSpace(fill.OrderID) == "" {
			fill.OrderID = order.ID
		}
		if fill.OrderID != order.ID {
			return plan, fmt.Errorf("incoming fill order mismatch: got %s want %s", fill.OrderID, order.ID)
		}
		if err := validateFillQuantity(fill); err != nil {
			return plan, fmt.Errorf("incoming fill: %w", err)
		}
		if tradingQuantityPositive(fill.Price) {
			lastKnownPrice = fill.Price
		}

		if fillReconcileSource(fill) == FillSourceReal {
			tradeID := strings.TrimSpace(fill.ExchangeTradeID)
			if _, exists := existingRealTradeIDs[tradeID]; exists {
				continue
			}
			existingRealTradeIDs[tradeID] = struct{}{}
			hasNewRealFill = true
			newRealQty += fill.Quantity
			plan.CreateFills = append(plan.CreateFills, fill)
			continue
		}

		if incomingHasRealFill {
			continue
		}
		fingerprint := strings.TrimSpace(fill.DedupFingerprint)
		if fingerprint == "" {
			fingerprint = fill.FallbackFingerprint()
			fill.DedupFingerprint = fingerprint
		}
		if _, exists := existingFallbackFingerprints[fingerprint]; exists {
			continue
		}
		remainingQty := order.Quantity - existingTotalQty
		if !tradingQuantityPositive(remainingQty) {
			continue
		}
		if tradingQuantityExceeds(fill.Quantity, remainingQty) {
			fill.Quantity = remainingQty
		}
		plan.CreateFills = append(plan.CreateFills, fill)
		plan.ApplyPositionFills = append(plan.ApplyPositionFills, fill)
		existingTotalQty += fill.Quantity
	}

	if hasNewRealFill {
		plan.DeleteFillIDs = append(plan.DeleteFillIDs, existingPlaceholderIDs...)
		applyRealQty := newRealQty
		if tradingQuantityPositive(existingPlaceholderQty) {
			if tradingQuantityExceeds(existingPlaceholderQty, applyRealQty) || tradingQuantityEqual(existingPlaceholderQty, applyRealQty) {
				applyRealQty = 0
			} else {
				applyRealQty -= existingPlaceholderQty
			}
		}
		if tradingQuantityPositive(applyRealQty) {
			plan.ApplyPositionFills = append(plan.ApplyPositionFills, splitApplyPositionFills(plan.CreateFills, applyRealQty)...)
		}

		remainderQty := existingPlaceholderQty - newRealQty
		if tradingQuantityPositive(remainderQty) {
			plan.CreateFills = append(plan.CreateFills, domain.Fill{
				OrderID:          order.ID,
				Price:            firstPositive(lastKnownPrice, order.Price),
				Quantity:         remainderQty,
				Fee:              0,
				DedupFingerprint: fmt.Sprintf("%s%s|%.12f", syntheticRemainderFingerprintPrefix, order.ID, remainderQty),
			})
		} else if remainderQty < 0 && !tradingQuantityExceeds(-remainderQty, 0) {
			remainderQty = 0
		}

		filledQty := existingRealQty + newRealQty
		if tradingQuantityPositive(remainderQty) {
			filledQty += remainderQty
		}
		setFillReconcileMetadata(&plan, order, filledQty)
		if tradingQuantityExceeds(filledQty, order.Quantity) {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("real fill quantity exceeds order quantity: filled=%.12f order=%.12f", filledQty, order.Quantity))
		}
		return plan, nil
	}

	setFillReconcileMetadata(&plan, order, existingTotalQty)
	if tradingQuantityExceeds(existingTotalQty, order.Quantity) {
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("fill quantity exceeds order quantity: filled=%.12f order=%.12f", existingTotalQty, order.Quantity))
	}
	return plan, nil
}

func fillReconcileSource(fill domain.Fill) FillSource {
	if strings.TrimSpace(fill.ExchangeTradeID) != "" {
		return FillSourceReal
	}
	if strings.HasPrefix(strings.TrimSpace(fill.DedupFingerprint), syntheticRemainderFingerprintPrefix) {
		return FillSourceRemainder
	}
	return FillSourceSynthetic
}

func hasIncomingRealFill(fills []domain.Fill) bool {
	for _, fill := range fills {
		if fillReconcileSource(fill) == FillSourceReal {
			return true
		}
	}
	return false
}

func splitApplyPositionFills(fills []domain.Fill, quantity float64) []domain.Fill {
	if !tradingQuantityPositive(quantity) {
		return nil
	}
	remaining := quantity
	var result []domain.Fill
	for _, fill := range fills {
		if fillReconcileSource(fill) != FillSourceReal || !tradingQuantityPositive(remaining) {
			continue
		}
		apply := fill
		if tradingQuantityExceeds(apply.Quantity, remaining) {
			apply.Quantity = remaining
		}
		result = append(result, apply)
		remaining -= apply.Quantity
		if remaining < 0 && !tradingQuantityExceeds(-remaining, 0) {
			remaining = 0
		}
	}
	return result
}

func setFillReconcileMetadata(plan *FillReconciliationPlan, order domain.Order, filledQty float64) {
	if filledQty < 0 && !tradingQuantityExceeds(-filledQty, 0) {
		filledQty = 0
	}
	remainingQty := order.Quantity - filledQty
	if remainingQty < 0 && !tradingQuantityExceeds(-remainingQty, 0) {
		remainingQty = 0
	}
	plan.UpdatedMetadata["filledQuantity"] = filledQty
	plan.UpdatedMetadata["remainingQuantity"] = remainingQty
}

func validateFillQuantity(fill domain.Fill) error {
	if invalidFillQuantity(fill.Quantity) {
		return fmt.Errorf("quantity must be finite: %v", fill.Quantity)
	}
	if !tradingQuantityPositive(fill.Quantity) {
		return fmt.Errorf("quantity must be positive: %v", fill.Quantity)
	}
	return nil
}

func invalidFillQuantity(value float64) bool {
	return math.IsNaN(value) || math.IsInf(value, 0)
}
