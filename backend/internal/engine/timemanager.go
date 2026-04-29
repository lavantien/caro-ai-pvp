package engine

type TimeAllocation struct {
	SoftBoundMs int64
	HardBoundMs int64
	OptimalMs   int64
}

func AllocateTime(timeRemainingMs int64, incrementMs int64, moveNumber int) TimeAllocation {
	var phaseDivisor float64 = 25.0
	if moveNumber > 25 {
		phaseDivisor = 30.0
	}

	baseMs := float64(timeRemainingMs) / phaseDivisor
	incContrib := float64(incrementMs) * 0.6

	optimal := int64(baseMs + incContrib)
	if optimal < 300 {
		optimal = 300
	}

	maxTime := int64(float64(timeRemainingMs) * 0.4)
	if optimal > maxTime {
		optimal = maxTime
	}

	hardBound := int64(float64(optimal) * 1.3)
	buffer := int64(float64(timeRemainingMs) * 0.01)
	if buffer < 100 {
		buffer = 100
	}
	hardBound += buffer
	if hardBound > timeRemainingMs-50 {
		hardBound = timeRemainingMs - 50
	}

	softBound := int64(float64(optimal) * 0.8)

	return TimeAllocation{
		SoftBoundMs: softBound,
		HardBoundMs: hardBound,
		OptimalMs:   optimal,
	}
}
