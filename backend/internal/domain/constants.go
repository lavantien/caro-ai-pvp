package domain

const (
	BoardSize   = 16
	WinLength   = 5
	MaxMoves    = BoardSize * BoardSize
	OpenRuleMin = 3

	MaxConcurrentGames       = 4
	HeapHardLimitBytes int64 = 2 * 1024 * 1024 * 1024
	AbandonedTimeoutMinutes  = 30

	DefaultTTSizeMB    = 1024
	MaxVCFCacheEntries = 10_000
	VCFTimeFraction    = 0.20

	MaxSearchRadius       = 7
	MaxKillerMoves        = 2
	MaxKillerDepth        = 512
	TimeCheckInterval     = 16
	AbsoluteMaxDepth      = 50
	AspirationWindowSize  = 50
	MaxAspirationAttempts = 3
	NullMoveMinDepth      = 3
	NullMoveReduction     = 3
	MaxQuiescenceDepth    = 4
	ContinuationPlyCount  = 6

	FutilityMarginBase     = 300
	FutilityMarginPerDepth = 100
	FutilityMinDepth       = 3
	LMRMinDepth            = 3
	LMRFullDepthMoves      = 4
	PVSEnabledDepth        = 2

	WinScore = 30_000
)
