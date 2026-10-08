package config

import "fmt"

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
	InitialMin   int
	IncrementSec int
}

var TimeControls = [...]TimeControl{{InitialMin: 1}, {InitialMin: 2, IncrementSec: 1}, {InitialMin: 3, IncrementSec: 2}, {InitialMin: 10, IncrementSec: 5}}

func TCIndex(initialMin, incrementSec int) (int, bool) {
	for i := range TimeControls {
		if TimeControls[i].InitialMin == initialMin && TimeControls[i].IncrementSec == incrementSec {
			return i, true
		}
	}
	return 0, false
}

const (
	ClockExpectedMovesPerSide = 30
	ClockIncrementShare       = 0.5
	ClockPIDClampFraction     = 0.25
	ClockPIDIntegClampMs      = 500.0
)

type PIDGains struct {
	Kp, Ki, Kd float64
}

var ClockPID = [len(TimeControls)]PIDGains{
	{Kp: 0.02, Ki: 0.01, Kd: 0.005},
	{Kp: 0.03, Ki: 0.015, Kd: 0.005},
	{Kp: 0.03, Ki: 0.015, Kd: 0.005},
	{Kp: 0.03, Ki: 0.015, Kd: 0.005},
}

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
	Ponder  bool
}

var (
	TierEasy   = Tier{Name: "easy", Cores: 1, TTBytes: 32 << 20, VCF: false, VCT: false}
	TierMedium = Tier{Name: "medium", Cores: 2, TTBytes: 128 << 20, VCF: true, VCT: false}
	TierHard   = Tier{Name: "hard", Cores: 4, TTBytes: 1 << 30, VCF: true, VCT: true}
	TierMaster = Tier{Name: "master", Cores: 8, TTBytes: 2 << 30, VCF: true, VCT: true, Ponder: true}
	Tiers      = [...]Tier{TierEasy, TierMedium, TierHard, TierMaster}
)

func BotAccountName(i int) string { return "AI " + Tiers[i].Name }
func TierIndex(t Tier) (int, bool) {
	for i := range Tiers {
		if Tiers[i] == t {
			return i, true
		}
	}
	return 0, false
}

const (
	MaxCoresPerInstance       = 8
	MaxRAMPerInstanceBytes    = 16 << 30
	TournamentParallelMatches = 2
	MachineCores              = 16
)
const (
	PonderAdoptFraction   = 0.5
	PonderStableIters     = 3
	PonderScoreDropMargin = 300
	PonderDepthHistory    = 8
)
const (
	TournamentStartRating     = 1000
	TournamentSeriesLogFormat = "run%d_s%d_%s-vs-%s.txt"
	TournamentRunDirFormat    = "20060102-150405"
	TournamentSummaryName     = "summary.txt"
	TournamentDriveBeatSec    = 10
	TournamentDriveStaleSec   = 90
	TournamentLegacyLabel     = "legacy"
)

var TournamentLogRoot = "logs/tourny"

const (
	TournamentNameMaxBytes      = 32
	TournamentStartRatingAbsMax = 4000
)
const InstancesPerTier = 2

var TournamentMaxParticipants = InstancesPerTier * len(Tiers)

type TierInstance struct {
	Name string
	Tier string
}

func DefaultRoster() []TierInstance {
	out := make([]TierInstance, 0, TournamentMaxParticipants)
	for i := range Tiers {
		for k := 1; k <= InstancesPerTier; k++ {
			out = append(out, TierInstance{Name: fmt.Sprintf("%s-%d", Tiers[i].Name, k), Tier: Tiers[i].Name})
		}
	}
	return out
}

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
const EvalTempo = 1000
const (
	PatternWeightNone        = 0
	PatternWeightOpenTwo     = 100
	PatternWeightBrokenThree = 500
	PatternWeightThree       = 1200
	PatternWeightFour        = 4000
	PatternWeightOpenFour    = 15000
)
const (
	SearchMaxPly            = 64
	SearchMaxMovesPerPly    = BoardCells
	SearchNodeCheckInterval = 2048
	SearchSoftStopFraction  = 0.65
	SearchSafetyMarginMs    = 120
	SearchMinMoveTimeMs     = 10
	SearchClockQuantumMs    = 16
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

var CorePackages = [...]string{"internal/rules", "internal/engine", "internal/clock"}

const (
	MutatePackages            = "internal/rules,internal/engine,internal/clock"
	MutateTimeoutMs           = 30_000
	MutateMinTimeoutMs        = 2_000
	MutateTestTimeoutSlack    = 1_000
	MutateMaxParallel         = 16
	MutateRemoveRetryAttempts = 5
	MutateRemoveRetryDelayMs  = 200
)
const (
	PatternWindowLen      = 2*WinLength - 1
	PatternCellBits       = 2
	PatternTableIndexBits = PatternCellBits * PatternWindowLen
	PatternTableEntries   = 1 << PatternTableIndexBits
	PatternDirections     = 4
	PatternClassCount     = 6
)
const (
	PatternStateEmpty uint8 = iota
	PatternStateOwn
	PatternStateOpp
	PatternStateOff
)
const (
	PatternClassNone uint8 = iota
	PatternClassOpenTwo
	PatternClassBrokenThree
	PatternClassThree
	PatternClassFour
	PatternClassOpenFour
)

var PatternDirs = [PatternDirections][2]int{{0, 1}, {1, 0}, {1, 1}, {1, -1}}
var PatternClassWeights = [PatternClassCount]int{PatternWeightNone, PatternWeightOpenTwo, PatternWeightBrokenThree, PatternWeightThree, PatternWeightFour, PatternWeightOpenFour}

const (
	SearchRingRadius      = 2
	SearchExtensionMaxPly = 16
	SearchHistoryMax      = 1 << 20
	SearchHashFullSample  = 1024
	SearchEmptyBoardCell  = BoardStride*(BoardSize/2-1) + BoardSize/2 - 1
)
const (
	SearchOrderTT      = 1<<31 - 1
	SearchOrderKiller1 = 1 << 29
	SearchOrderKiller2 = 1<<29 - 1
)
const (
	SearchWorkerParkDelayMs = 5
)
const (
	SolverMaxPly            = SearchMaxPly
	SolverNodeCheckInterval = SearchNodeCheckInterval
	SolverTTBits            = 18
	SolverCandidateRadius   = 4
	SolverNodeBudget        = 1 << 20
	SolverBudgetShare       = 0.5
	SolverMinGrantMs        = 80
)
const (
	Argon2Time               = 2
	Argon2MemoryKiB          = 64 * 1024
	Argon2Parallelism        = 1
	Argon2SaltBytes          = 16
	Argon2KeyBytes           = 32
	SessionTokenBytes        = 32
	SessionTTLHours          = 24 * 30
	UsernameMaxBytes         = 32
	AdminName                = "admin"
	AdminPassword            = "1234qwerasdfzxcv"
	SQLiteSchemaVersion      = 9
	SQLiteBusyTimeoutMs      = 5000
	SQLiteJournalWAL         = "wal"
	SQLiteSyncNormal         = "normal"
	SQLiteTxLockImmediate    = "immediate"
	WriteQueueDepth          = 256
	WriteQueueApplyTimeoutMs = 30_000
	HubSubscriberBuffer      = 64
	HistoryPreviewTurns      = 8
	PagePollMs               = 5000
)
