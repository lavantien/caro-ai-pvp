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

// TimeControl is one clock shape in the conventional m+i notation: the
// initial bank in whole minutes, the per-move increment in seconds, so 1+0
// is one minute, not one second.
type TimeControl struct {
	InitialMin   int
	IncrementSec int
}

var TimeControls = [...]TimeControl{{InitialMin: 1}, {InitialMin: 2, IncrementSec: 1}, {InitialMin: 3, IncrementSec: 2}, {InitialMin: 10, IncrementSec: 5}}

// TCIndex resolves a time control by its clock shape, so callers name "3+2"
// instead of hardcoding table positions.
func TCIndex(initialMin, incrementSec int) (int, bool) {
	for i := range TimeControls {
		if TimeControls[i].InitialMin == initialMin && TimeControls[i].IncrementSec == incrementSec {
			return i, true
		}
	}
	return 0, false
}

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
	TierMedium = Tier{Name: "medium", Cores: 2, TTBytes: 32 << 20, VCF: true, VCT: false}
	TierHard   = Tier{Name: "hard", Cores: 4, TTBytes: 128 << 20, VCF: true, VCT: true}
	Tiers      = [...]Tier{TierEasy, TierMedium, TierHard}
)

// Bot seat accounts of human-vs-bot matches. Every tier owns one reserved
// users row (seeded by schema v4 without an explicit id, because a fixed
// reserved band would hijack SQLite's max-plus-one rowid assignment: a
// negative band turns the first real account of a fresh database negative,
// a high one pushes every later account onto it) so a bot pairing's series
// and games rows satisfy the users foreign keys and history and playback
// render the seat through the plain users join as "AI <tier>". The rows
// carry empty salt and hash: real accounts always carry argon2 material, so
// the emptiness doubles as the seat marker the store resolves bot accounts
// by, and password verification rejects the rows on length before any
// derivation, which keeps the seats permanently unloginable and their names
// reserved: registering "AI easy" reports bad credentials like any wrong
// password.

// BotAccountName is the reserved username of tier index i, the same
// "AI <tier>" word the live room page renders for a bot seat.
func BotAccountName(i int) string { return "AI " + Tiers[i].Name }

// TierIndex resolves a tier value's index in Tiers; false when unknown.
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
	// MachineCores is the machine-wide core budget for live searches, not a
	// per-run allowance: one run books TournamentParallelMatches rooms, one
	// search at a time each (bot turns alternate), at the largest tier core
	// count, and the per-run budget check plus the run gate (one ongoing run
	// at a time) hold the whole budget together. The spec offers 8x2 cores
	// in theory but pins the conductor to less, so a run refuses
	// parallelism whose worst case exceeds this.
	MachineCores = 8
)

// Tournament constants, all consumed by internal/tourney.
// TournamentStartRating is the per-run seed rating of Scenario 2: the setup
// lets the operator pick it, the full run of Implication 2.4 pins 1000.
// TournamentSeriesLogFormat names one series' txt log file inside one run's
// folder: run id, series id, then the two display names.
const (
	TournamentStartRating     = 1000
	TournamentSeriesLogFormat = "run%d_s%d_%s-vs-%s.txt"
	// TournamentRunDirFormat is the timestamp prefix of one tournament's
	// own log folder, followed by the run label.
	TournamentRunDirFormat = "20060102-150405"
	// TournamentSummaryName is the completion summary inside one run's
	// folder: the final rating table with every participant's series and
	// game record.
	TournamentSummaryName = "summary.txt"
	// TournamentDriveBeatSec and TournamentDriveStaleSec are the drive
	// lease's heartbeat interval and freshness window: the live drive
	// re-stamps its claim on the run row every beat, a claim whose last beat
	// sits inside the stale window refuses every other drive, and one older
	// than it is a dead process's claim, free to take over. The window sits
	// far above the beat so a loaded machine's skipped ticks never read as a
	// dead drive. The lease columns live on the run row (schema v8).
	TournamentDriveBeatSec  = 10
	TournamentDriveStaleSec = 90
	// TournamentLegacyLabel is the reserved label of pre-v7 runs whose
	// original driver label was never persisted: the v8 migration stamps it
	// on the ongoing rows v7 had defaulted to 'ui', and CreateRun and Resume
	// both refuse it, so the one shape that cannot name its own log folder
	// closes loudly instead of splitting its evidence across two.
	TournamentLegacyLabel = "legacy"
)

// TournamentLogRoot is the root directory holding one folder per
// tournament run, each named by TournamentRunDirFormat plus the driver's
// label. A package var like TimeControls so tests point it at a t.TempDir
// without touching the process working directory.
var TournamentLogRoot = "logs/tourny"

// Tournament setup-screen bounds of Scenario 2. TournamentNameMaxBytes
// bounds one seat's display name like UsernameMaxBytes bounds accounts.
// TournamentStartRatingAbsMax bounds the operator's start-rating input: two
// seats at opposite bounds put the law's worst delta at
// RatingDelta * 10^(2*bound/RatingDecayMin), which stays inside int64.
const (
	TournamentNameMaxBytes      = 32
	TournamentStartRatingAbsMax = 4000
)

// InstancesPerTier is the default roster shape of the setup screen and the
// Implication 2.4 full run: two instances of every tier. A package var's
// companion, TournamentMaxParticipants, bounds the setup form's roster (a
// pairing needs at least 2).
const InstancesPerTier = 2

// TournamentMaxParticipants is the setup form's roster ceiling.
var TournamentMaxParticipants = InstancesPerTier * len(Tiers)

// TierInstance is one seat of the default roster: the display name and its
// tier.
type TierInstance struct {
	Name string
	Tier string
}

// DefaultRoster spells the two-per-tier default roster, the single source
// the setup form prefills from and the headless drivers run.
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
	// MutateRemoveRetryAttempts and MutateRemoveRetryDelayMs bound the temp
	// cleanup retry: on Windows a directory handle can outlive its process
	// by milliseconds, and a single RemoveAll racing that teardown leaves an
	// empty directory shell behind.
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
	// SolverBudgetShare is the fraction of one move's grant the tier
	// searcher may burn on its solver passes before the standard search
	// takes the rest: a miss must still fund a real search.
	SolverBudgetShare = 0.5
	// SolverMinGrantMs is the smallest grant the solver passes may draw
	// on. Below it they skip and the standard search holds the whole
	// grant: on a drained clock's 10ms move floor, two compounding
	// shares left hard's inner search a 2.5ms slice that finished no
	// iteration, 497 zero-node moves in the corrected-clock 1+0 smoke,
	// while medium's single 5ms slice starved only a tenth of its floor
	// moves. The value sits where the at-gate inner slice
	// (1-SolverBudgetShare)^2 * gate clears one clock quantum, so the
	// soft stop's zero-reading law protects the smallest slice the
	// passes still leave behind.
	SolverMinGrantMs = 80
)

// Server constants, all consumed by internal/server. Storage is embedded
// SQLite in WAL mode behind the single-writer message queue; auth is
// server-side argon2id with opaque random session tokens; the stats hub
// fans every room's bot-log lines out to subscribers in publish order.
const (
	// Argon2 parameters per the OWASP password storage cheat sheet profile
	// for a laptop-class host: 64 MiB, 2 passes, parallelism 1, 16-byte
	// salt, 32-byte derived key.
	Argon2Time        = 2
	Argon2MemoryKiB   = 64 * 1024
	Argon2Parallelism = 1
	Argon2SaltBytes   = 16
	Argon2KeyBytes    = 32

	SessionTokenBytes = 32
	SessionTTLHours   = 24 * 30

	// UsernameMaxBytes is the login form's username ceiling.
	UsernameMaxBytes = 32

	// AdminName and AdminPassword seed the one account allowed to start
	// tournaments and close runs (migration v6). The demo deployment's
	// shared credential; the gate is a UI control, not a secret.
	AdminName     = "admin"
	AdminPassword = "1234qwerasdfzxcv"

	// SQLiteSchemaVersion is the number of landed migration scripts; the
	// server package enforces the equality at startup.
	SQLiteSchemaVersion = 8
	SQLiteBusyTimeoutMs = 5000
	SQLiteJournalWAL    = "wal"
	SQLiteSyncNormal    = "normal"
	// SQLiteTxLockImmediate is the begin mode of every transaction: the
	// write lock is taken up front so concurrent read-then-write units wait
	// on the busy timeout instead of failing the snapshot upgrade.
	SQLiteTxLockImmediate = "immediate"

	// WriteQueueDepth bounds the pending mutation queue. Every write
	// persists: the queue blocks producers rather than dropping, so the
	// depth only controls how far gameplay may run ahead of the disk.
	WriteQueueDepth = 256
	// HubSubscriberBuffer is the per-subscriber event slot count of the
	// stats pipeline; a consumer this far behind is too slow to watch live.
	HubSubscriberBuffer = 64

	// HistoryPreviewTurns is the move-history preview length before the
	// ellipsis in the match history tab.
	HistoryPreviewTurns = 8

	// PagePollMs is the htmx partial poll cadence the live shell pages ride:
	// the rooms grid and the tournament run board.
	PagePollMs = 5000
)
