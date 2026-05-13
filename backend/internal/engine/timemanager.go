package engine

import "caro-ai-pvp/internal/domain"

type TimeAllocation struct {
	SoftBoundMs int64
	HardBoundMs int64
	OptimalMs   int64
}

func AllocateTime(timeRemainingMs int64, incrementMs int64, moveNumber int) TimeAllocation {
	phaseDivisor := domain.TimePhaseDivisorEarly
	if moveNumber > domain.TimePhaseSwitchMove {
		phaseDivisor = domain.TimePhaseDivisorLate
	}

	baseMs := float64(timeRemainingMs) / phaseDivisor
	incContrib := float64(incrementMs) * domain.TimeIncContribFactor

	optimal := int64(baseMs + incContrib)
	if optimal < domain.TimeMinOptimalMs {
		optimal = domain.TimeMinOptimalMs
	}

	maxTime := int64(float64(timeRemainingMs) * domain.TimeMaxFraction)
	if optimal > maxTime {
		optimal = maxTime
	}

	hardBound := int64(float64(optimal) * domain.TimeHardBoundMultiplier)
	buffer := int64(float64(timeRemainingMs) * domain.TimeBufferFraction)
	if buffer < domain.TimeMinBufferMs {
		buffer = domain.TimeMinBufferMs
	}
	hardBound += buffer
	if hardBound > timeRemainingMs-domain.TimeReserveMs {
		hardBound = timeRemainingMs - domain.TimeReserveMs
	}

	softBound := int64(float64(optimal) * domain.TimeSoftBoundFraction)

	return TimeAllocation{
		SoftBoundMs: softBound,
		HardBoundMs: hardBound,
		OptimalMs:   optimal,
	}
}
