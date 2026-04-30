package domain

import "testing"

func TestTradingConstraintSignalKindContract(t *testing.T) {
	t.Run("entry signal cannot produce exit intent", func(t *testing.T) {
		violation, ok := CheckSignalKindContract("order-entry-close", "zero-initial-reentry", OrderIntentCloseLong)
		if !ok {
			t.Fatal("expected signalKind contract violation")
		}
		if violation.Code != ViolationSignalKindIntentMismatch {
			t.Fatalf("violation code = %s, want %s", violation.Code, ViolationSignalKindIntentMismatch)
		}
		if violation.Message != `signalKind "zero-initial-reentry" expects entry intent, got CLOSE_LONG` {
			t.Fatalf("violation message = %q", violation.Message)
		}
	})

	t.Run("exit signal cannot produce entry intent", func(t *testing.T) {
		violation, ok := CheckSignalKindContract("order-exit-open", "risk-exit", OrderIntentOpenLong)
		if !ok {
			t.Fatal("expected signalKind contract violation")
		}
		if violation.Code != ViolationSignalKindIntentMismatch {
			t.Fatalf("violation code = %s, want %s", violation.Code, ViolationSignalKindIntentMismatch)
		}
		if violation.Message != `signalKind "risk-exit" expects exit intent, got OPEN_LONG` {
			t.Fatalf("violation message = %q", violation.Message)
		}
	})

	t.Run("empty signal is ignored", func(t *testing.T) {
		if violation, ok := CheckSignalKindContract("order-empty-signal", "", OrderIntentOpenLong); ok {
			t.Fatalf("expected no violation, got %+v", violation)
		}
	})

	t.Run("matching signal and intent passes", func(t *testing.T) {
		if violation, ok := CheckSignalKindContract("order-risk-exit", " risk-exit ", OrderIntentCloseShort); ok {
			t.Fatalf("expected no violation, got %+v", violation)
		}
	})
}

func TestTradingConstraintReduceOnlyConstraint(t *testing.T) {
	t.Run("reduce-only exit without position fails", func(t *testing.T) {
		order := Order{ID: "reduce-only-no-position", Side: "SELL", ReduceOnly: true}
		violation, ok := CheckReduceOnlyConstraint(order, OrderIntentCloseLong, false)
		if !ok {
			t.Fatal("expected reduceOnly constraint violation")
		}
		if violation.Code != ViolationReduceOnlyWithoutPosition {
			t.Fatalf("violation code = %s, want %s", violation.Code, ViolationReduceOnlyWithoutPosition)
		}
		if violation.Message != "CLOSE_LONG has no matching virtual position" {
			t.Fatalf("violation message = %q", violation.Message)
		}
	})

	t.Run("metadata closePosition without position fails", func(t *testing.T) {
		order := Order{
			ID:       "close-position-no-position",
			Side:     "BUY",
			Metadata: map[string]any{"closePosition": true},
		}
		if violation, ok := CheckReduceOnlyConstraint(order, OrderIntentCloseShort, false); !ok {
			t.Fatal("expected closePosition constraint violation")
		} else if violation.Code != ViolationReduceOnlyWithoutPosition {
			t.Fatalf("violation code = %s, want %s", violation.Code, ViolationReduceOnlyWithoutPosition)
		}
	})

	t.Run("non reduce-only entry is ignored", func(t *testing.T) {
		order := Order{ID: "entry", Side: "BUY"}
		if violation, ok := CheckReduceOnlyConstraint(order, OrderIntentOpenLong, false); ok {
			t.Fatalf("expected no violation, got %+v", violation)
		}
	})

	t.Run("matching position passes", func(t *testing.T) {
		order := Order{ID: "reduce-only-has-position", Side: "SELL", ReduceOnly: true}
		if violation, ok := CheckReduceOnlyConstraint(order, OrderIntentCloseLong, true); ok {
			t.Fatalf("expected no violation, got %+v", violation)
		}
	})
}
