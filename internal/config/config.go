package config

const (
	BoardSize           = 16
	BoardStride         = 16
	BoardCells          = 256
	BoardWordsPerColor  = 4
	WinLength           = 5
	OpeningChebyshevMin = 3
	CrossCheckSize      = 8
)

const ZobristSeed uint64 = 0x9E3779B97F4A7C15

type TimeControl struct {
	InitialSec   int
	IncrementSec int
}

var TimeControls = [...]TimeControl{{InitialSec: 1}, {InitialSec: 2, IncrementSec: 1}, {InitialSec: 3, IncrementSec: 2}}

const (
	SeriesBO3  = 3
	SeriesBO5  = 5
	SeriesBO7  = 7
	SeriesBO11 = 11
)

var SeriesLengths = [...]int{SeriesBO3, SeriesBO5, SeriesBO7, SeriesBO11}

type Tier struct {
	Name    string
	Cores   int
	TTBytes int64
	VCF     bool
	VCT     bool
}

var (
	TierEasy   = Tier{Name: "easy", Cores: 1, TTBytes: 0, VCF: false, VCT: false}
	TierMedium = Tier{Name: "medium", Cores: 2, TTBytes: 256 << 20, VCF: true, VCT: false}
	TierHard   = Tier{Name: "hard", Cores: 4, TTBytes: 1 << 30, VCF: true, VCT: true}
	Tiers      = [...]Tier{TierEasy, TierMedium, TierHard}
)

const (
	MaxCoresPerInstance       = 8
	MaxRAMPerInstanceBytes    = 16 << 30
	TournamentParallelMatches = 2
)

const (
	HTTPPort  = 38063
	DebugPort = 38069
)

const (
	RatingDelta       = 30
	RatingDivisorFull = 3000
	RatingDecayBase   = 500
	RatingDecaySlope  = 2.5
	RatingDecayMin    = 500
	RatingDecayMax    = 3000
	RatingStart       = 0
)

const (
	EvalMilliUnit     = 1000
	EvalMateMax       = 1 << 30
	EvalMateScoreStep = 16
)

const (
	SearchMaxPly            = 64
	SearchMaxMovesPerPly    = BoardCells
	SearchNodeCheckInterval = 2048
	SearchSoftStopFraction  = 0.65
	SearchSafetyMarginMs    = 120
	SearchMinMoveTimeMs     = 10
)

const (
	BotLogFormat    = "M%d, %s, %s, d=%d, n=%s, nps=%s, ebf=%.1f, tt=%d%%, hf=%d%%, fh1=%d%%, s=%s, thr=%d, t=%.2f, alloc=%.2f%s, pv=%s"
	BotLogTagVCF    = ", [VCF]"
	BotLogTagVCT    = ", [VCT]"
	BotLogTagPonder = ", [PONDER]"
)

const (
	QualityCoverageOverallMin = 95.0
	QualityCoverageCoreMin    = 100.0
)

const (
	MutatePackages         = "internal/rules,internal/engine"
	MutateTimeoutMs        = 30_000
	MutateMinTimeoutMs     = 2_000
	MutateTestTimeoutSlack = 1_000
)
