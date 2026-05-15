package domain

import "time"

// PretouchEvent represents a detected pretouch breakout event for the
// ETH pretouch timing strategy.
type PretouchEvent struct {
	EventID           string    `json:"eventId"`
	Symbol            string    `json:"symbol"`
	Side              string    `json:"side"` // long / short
	TouchTime         time.Time `json:"touchTime"`
	TouchPrice        float64   `json:"touchPrice"`
	Level             float64   `json:"level"` // breakout level (prev_high_2 / prev_low_2)
	ATR               float64   `json:"atr"`
	TouchExtensionATR float64   `json:"touchExtensionAtr"`
	Speed300sATR      float64   `json:"speed300sAtr"`
	Eff300s           float64   `json:"eff300s"`
	PreTouchSeconds   float64   `json:"preTouchSeconds"`
	RoundtripCostATR  float64   `json:"roundtripCostAtr"`
	SignalBarStart    time.Time `json:"signalBarStart"`

	// Features for ML inference (Original_10_Features)
	Features map[string]float64 `json:"features,omitempty"`

	// ML inference results (filled by Go-native model inference)
	TimingRegime     string  `json:"timingRegime"` // skip / fast / slow
	RFProbability    float64 `json:"rfProbability"`
	SizingMultiplier float64 `json:"sizingMultiplier"` // clip(prob × 2, 0, 2)

	// Final sizing
	CostPenalty       float64 `json:"costPenalty"`       // 1.0 or 0.5
	FinalPositionSize float64 `json:"finalPositionSize"` // baseShare × multiplier × costPenalty
}
