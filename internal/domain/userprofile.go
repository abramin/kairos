package domain

type UserProfile struct {
	ID                     string
	BufferPct              float64
	WeightDeadlinePressure float64
	WeightBehindPace       float64
	WeightSpacing          float64
	WeightVariation        float64
	DefaultMaxSlices       int
}
