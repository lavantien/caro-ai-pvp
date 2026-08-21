package domain

const (
	BoardSize   = 16
	WinLength   = 5
	MaxMoves    = BoardSize * BoardSize
	OpenRuleMin = 3

	Infinity  = 100_000
	MaxEval   = 25_000

	MaxConcurrentGames       = 4
	HeapHardLimitBytes int64 = 2 * 1024 * 1024 * 1024
	AbandonedTimeoutMinutes  = 30

	DefaultTTSizeMB    = 1024
	MaxVCFCacheEntries = 10_000
	VCFTimeFraction    = 0.20

	MaxSearchRadius       = 2
	TimeCheckInterval     = 16
	AbsoluteMaxDepth      = 50
	AspirationWindowSize  = 1_500
	MaxAspirationAttempts = 3
	NullMoveMinDepth      = 4
	NullMoveReduction     = 2
	MaxQuiescenceDepth    = 4

	LMRMinDepth       = 3
	LMRFullDepthMoves = 4

	WinScore = 30_000

	// TT configuration
	TTShardCount = 16

	// VCF solver
	VCFSearchDepth = 12

	// Time management
	TimePhaseDivisorEarly   = 25.0
	TimePhaseDivisorLate    = 30.0
	TimePhaseSwitchMove     = 25
	TimeIncContribFactor    = 0.6
	TimeMinOptimalMs  int64 = 300
	TimeMaxFraction         = 0.4
	TimeHardBoundMultiplier = 1.3
	TimeBufferFraction      = 0.01
	TimeMinBufferMs   int64 = 100
	TimeReserveMs     int64 = 50
	TimeSoftBoundFraction   = 0.8

	// Pondering (L5 background search on the predicted reply)
	PonderTimeCapMs         int64 = 30_000
	PonderMinCompletedDepth int   = 1
	PonderAdoptionFraction        = 0.5
)
