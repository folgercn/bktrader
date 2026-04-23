package service

import "math"

const (
	tradingQuantityAbsTolerance = 1e-9
	tradingPriceAbsTolerance    = 1e-6

	exchangeIncrementAbsTolerance = 1e-12
	exchangeIncrementRelTolerance = 1e-9

	exchangeNotionalAbsTolerance = 1e-9
	exchangeNotionalRelTolerance = 1e-12
)

type precisionToleranceSpec struct {
	absolute float64
	relative float64
	scale    float64
}

func (spec precisionToleranceSpec) tolerance() float64 {
	tolerance := spec.absolute
	if spec.scale > 0 && spec.relative > 0 {
		tolerance = math.Max(tolerance, math.Abs(spec.scale)*spec.relative)
	}
	return tolerance
}

func (spec precisionToleranceSpec) equal(left, right float64) bool {
	return math.Abs(left-right) <= spec.tolerance()
}

func (spec precisionToleranceSpec) differs(left, right float64) bool {
	return !spec.equal(left, right)
}

func (spec precisionToleranceSpec) exceeds(left, right float64) bool {
	return left-right > spec.tolerance()
}

func (spec precisionToleranceSpec) below(left, right float64) bool {
	return right-left > spec.tolerance()
}

func (spec precisionToleranceSpec) positive(value float64) bool {
	return value > spec.tolerance()
}

func tradingQuantityPrecision() precisionToleranceSpec {
	return precisionToleranceSpec{absolute: tradingQuantityAbsTolerance}
}

func tradingQuantityEqual(left, right float64) bool {
	return tradingQuantityPrecision().equal(left, right)
}

func tradingQuantityDiffers(left, right float64) bool {
	return tradingQuantityPrecision().differs(left, right)
}

func tradingQuantityExceeds(left, right float64) bool {
	return tradingQuantityPrecision().exceeds(left, right)
}

func tradingQuantityBelow(left, right float64) bool {
	return tradingQuantityPrecision().below(left, right)
}

func tradingQuantityPositive(value float64) bool {
	return tradingQuantityPrecision().positive(value)
}

func tradingPricePrecision() precisionToleranceSpec {
	return precisionToleranceSpec{absolute: tradingPriceAbsTolerance}
}

func tradingPriceDiffers(left, right float64) bool {
	return tradingPricePrecision().differs(left, right)
}

func exchangeIncrementPrecision(increment float64) precisionToleranceSpec {
	return precisionToleranceSpec{
		absolute: exchangeIncrementAbsTolerance,
		relative: exchangeIncrementRelTolerance,
		scale:    increment,
	}
}

func exchangeIncrementDiffers(left, right, increment float64) bool {
	return exchangeIncrementPrecision(increment).differs(left, right)
}

func exchangeIncrementExceeds(left, right, increment float64) bool {
	return exchangeIncrementPrecision(increment).exceeds(left, right)
}

func exchangeIncrementBelow(left, right, increment float64) bool {
	return exchangeIncrementPrecision(increment).below(left, right)
}

func exchangeNotionalSatisfiesMinimum(actual, minimum float64) bool {
	if minimum <= 0 {
		return true
	}
	tolerance := precisionToleranceSpec{
		absolute: exchangeNotionalAbsTolerance,
		relative: exchangeNotionalRelTolerance,
		scale:    minimum,
	}.tolerance()
	return actual+tolerance >= minimum
}

func roundDownToIncrement(value, increment float64) float64 {
	if value <= 0 || increment <= 0 {
		return value
	}
	return math.Floor((value/increment)+exchangeIncrementPrecision(increment).tolerance()/increment) * increment
}

func ceilToIncrement(value, increment float64) float64 {
	if value <= 0 || increment <= 0 {
		return value
	}
	return math.Ceil((value/increment)-exchangeIncrementPrecision(increment).tolerance()/increment) * increment
}
