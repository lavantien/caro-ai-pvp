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

// Clock law constants, all consumed by internal/clock. The budget for the
// next move spreads the spendable remainder over the expected moves left,
// adds a share of the increment, and lets a PID controller shape the result
// against the planned drain trajectory. The floor and the reserve bound
// every grant, so a drained clock still funds minimum moves and the tank
// never reaches zero while a grant is outstanding.
const (
	// ClockExpectedMovesPerSide anchors the drain trajectory and the spread
	// divisor: a decisive 16x16 caro game runs roughly 20 to 40 moves per
	// side, the PID absorbs the estimation error.
	ClockExpectedMovesPerSide = 30
	// ClockIncrementShare is the fraction of the per-move increment added to
	// the spread, the rest banks against long endgames.
	ClockIncrementShare = 0.5
	// ClockPIDClampFraction bounds the PID correction relative to the
	// feedforward spread of the same move.
	ClockPIDClampFraction = 0.25
	// ClockPIDIntegClampMs bounds the integral term against windup.
	ClockPIDIntegClampMs = 500.0
)

// PIDGains carries one time control's controller tuning. Gains scale a
// millisecond trajectory error into a millisecond budget correction.
type PIDGains struct {
	Kp, Ki, Kd float64
}

// ClockPID is indexed like TimeControls: one gain set per supported time
// control, tuned against the feedforward baseline by the clock benchmarks.
var ClockPID = [len(TimeControls)]PIDGains{
	{Kp: 0.02, Ki: 0.01, Kd: 0.005},
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

// EvalTempo is the side-to-move bonus in milliunits: one unblocked tempo,
// the anchor that fixes the meaning of the 1.0 base unit.
const EvalTempo = 1000

// Pattern class weights in milliunits. Forcing classes price the forcing
// depth from the pattern taxonomy: each class converts into the next one
// with a single own move, so values step up steeply and trading structure
// for a forcing threat always nets positive under search. OpenTwo is quiet
// shape worth a tenth of a tempo, BrokenThree forces a reply within two
// moves, Three forces one now, Four costs the opponent the whole move that
// blocks it, OpenFour wins on the spot. Bound check: PatternDirections *
// BoardCells * PatternWeightOpenFour stays two orders below the mate band,
// so a leaf score can never masquerade as a mate score.
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
	// SearchClockQuantumMs is the ceiling of one monotonic-clock tick on
	// the coarsest supported host (the 64 Hz Windows default timer): a
	// zero time.Since reading can hide up to this much real elapsed time.
	SearchClockQuantumMs = 16
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

// CorePackages is the correctness-critical set the covergate enforces at
// QualityCoverageCoreMin: the rules, the engine, and the time manager whose
// never-flag law is a product invariant.
var CorePackages = [...]string{"internal/rules", "internal/engine", "internal/clock"}

const (
	MutatePackages         = "internal/rules,internal/engine,internal/clock"
	MutateTimeoutMs        = 30_000
	MutateMinTimeoutMs     = 2_000
	MutateTestTimeoutSlack = 1_000
	// MutateMaxParallel bounds -parallel: every worker owns a full module
	// copy, and worker counts far past the core count only multiply temp
	// copies while suites thrash.
	MutateMaxParallel = 16
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
	// SearchRingRadius bounds candidate cells to the Chebyshev neighborhood of
	// existing stones. Radius 1 already contains every win-in-1 cell of either
	// color (any completing stone of an exact 5 sits on the line adjacent to a
	// stone of that run), radius 2 additionally keeps quiet developing moves.
	SearchRingRadius = 2
	// SearchExtensionMaxPly caps threat extensions along one root-to-leaf path.
	SearchExtensionMaxPly = 16
	// SearchHistoryMax caps history heuristic entries; on overflow all entries
	// halve. It sits far below the killer ordering layer.
	SearchHistoryMax = 1 << 20
	// SearchHashFullSample is the slot sample size behind the hash-full
	// permille statistic of direct-mapped tables.
	SearchHashFullSample = 1024
	// SearchEmptyBoardCell is the played cell when the board has no stones:
	// all first moves are symmetric, the center dominates every other cell.
	SearchEmptyBoardCell = BoardStride*(BoardSize/2-1) + BoardSize/2 - 1
)

const (
	// Move ordering layers, strictly ordered: TT move, then killers, then the
	// static threat plus history score which stays below SearchOrderKiller2.
	SearchOrderTT      = 1<<31 - 1
	SearchOrderKiller1 = 1 << 29
	SearchOrderKiller2 = 1<<29 - 1
)

const (
	// SearchWorkerParkDelayMs is how long an idle SMP worker spins with
	// yields before parking on the wake channel: back-to-back searches
	// (benchmarks, later ponder) keep workers hot at zero allocation, while
	// idle instances park and stop burning a core.
	SearchWorkerParkDelayMs = 5
)

const (
	// SolverMaxPly bounds one VCF/VCT recursion: the deepest forced line the
	// solver can certify. It sizes the per-ply stacks and the PV.
	SolverMaxPly = SearchMaxPly
	// SolverNodeCheckInterval is the deadline poll cadence of both solvers.
	SolverNodeCheckInterval = SearchNodeCheckInterval
	// SolverTTBits sizes one solver's direct-mapped proof memo,
	// 1 << SolverTTBits entries of a uint64 key plus a uint8 verdict.
	SolverTTBits = 18
	// SolverCandidateRadius is the Chebyshev dilation around a side's stones
	// bounding every cell that can complete or block a five: any completing
	// stone shares its five with four stones at line distance at most
	// WinLength-1.
	SolverCandidateRadius = 4
	// SolverNodeBudget is the default per-Solve node budget for engine
	// wiring; callers stay free to pass smaller budgets.
	SolverNodeBudget = 1 << 20
)
