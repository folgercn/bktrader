package service

import (
	"math"
	"testing"
)

func TestTradingQuantityPrecisionTolerance(t *testing.T) {
	if !tradingQuantityEqual(0.013, 0.013000000000000001) {
		t.Fatalf("expected quantity helper to tolerate float64 tail drift")
	}
	if !tradingQuantityBelow(0.012999998, 0.013) {
		t.Fatalf("expected material quantity shortfall to stay detectable")
	}
	if !tradingQuantityExceeds(0.013000002, 0.013) {
		t.Fatalf("expected material quantity expansion to stay detectable")
	}
}

func TestTradingPricePrecisionTolerance(t *testing.T) {
	if tradingPriceDiffers(78168.3, 78168.3000001) {
		t.Fatalf("expected price helper to tolerate float64 tail drift")
	}
	if !tradingPriceDiffers(78168.3, 78168.30001) {
		t.Fatalf("expected material price difference to stay detectable")
	}
}

func TestExchangeIncrementPrecisionTolerance(t *testing.T) {
	if exchangeIncrementDiffers(78168.3, 78168.30000000002, 0.1) {
		t.Fatalf("expected tick helper to tolerate float64 tail drift")
	}
	if !exchangeIncrementDiffers(78168.31, 78168.3, 0.1) {
		t.Fatalf("expected material tick difference to stay detectable")
	}
	rounded := roundDownToIncrement(0.013000000000000001, 0.0001)
	if math.Abs(rounded-0.013) > 1e-12 {
		t.Fatalf("expected exact-step value to round down to 0.013, got %.18f", rounded)
	}
	ceiled := ceilToIncrement(0.006000000000000001, 0.001)
	if math.Abs(ceiled-0.006) > 1e-12 {
		t.Fatalf("expected exact-step value to ceil to 0.006, got %.18f", ceiled)
	}
}

func TestExchangeNotionalPrecisionTolerance(t *testing.T) {
	if !exchangeNotionalSatisfiesMinimum(99.9999999995, 100) {
		t.Fatalf("expected min notional helper to tolerate boundary drift")
	}
	if exchangeNotionalSatisfiesMinimum(99.99, 100) {
		t.Fatalf("expected material min notional shortfall to stay detectable")
	}
}
