package service

import "testing"

func TestNormalizePaperSignalTimeframeAcceptsFifteenAndThirtyMinuteBars(t *testing.T) {
	for _, item := range []struct {
		input string
		want  string
	}{
		{input: "15min", want: "15m"},
		{input: "15m", want: "15m"},
		{input: "30min", want: "30m"},
		{input: "30m", want: "30m"},
	} {
		if got := normalizePaperSignalTimeframe(item.input); got != item.want {
			t.Fatalf("expected normalizePaperSignalTimeframe(%q)=%q, got %q", item.input, item.want, got)
		}
	}
}

func TestLiveSignalResolutionSupportsIntradayBaselineTemplates(t *testing.T) {
	for _, item := range []struct {
		input string
		want  string
	}{
		{input: "15m", want: "15"},
		{input: "30m", want: "30"},
	} {
		if got := liveSignalResolution(item.input); got != item.want {
			t.Fatalf("expected liveSignalResolution(%q)=%q, got %q", item.input, item.want, got)
		}
	}
}

func TestNormalizeBacktestParametersAcceptsIntradayBaselineTemplateTimeframes(t *testing.T) {
	for _, timeframe := range []string{"15m", "30m"} {
		normalized, err := NormalizeBacktestParameters(map[string]any{
			"signalTimeframe":     timeframe,
			"executionDataSource": "tick",
			"symbol":              "BTCUSDT",
		})
		if err != nil {
			t.Fatalf("expected %s to be accepted by NormalizeBacktestParameters: %v", timeframe, err)
		}
		if got := stringValue(normalized["signalTimeframe"]); got != timeframe {
			t.Fatalf("expected normalized signalTimeframe %s, got %s", timeframe, got)
		}
	}
}
