package domain

const (
	ResearchBaselineDir2ZeroInitial = true
	ResearchBaselineZeroInitialMode = "reentry_window"
	ResearchBaselineMaxTradesPerBar = 2
)

func ResearchBaselineReentrySizeSchedule() []float64 {
	return []float64{0.20, 0.10}
}
