package config

import (
	"fmt"
	"math"
	"testing"
)

func TestBoardMatchesSpec(t *testing.T) {
	if BoardSize != 16 {
		t.Errorf("BoardSize = %d, want 16 (16x16 board, A1 to P16)", BoardSize)
	}
	if BoardStride != 16 {
		t.Errorf("BoardStride = %d, want 16 (uniform bitboard shifts)", BoardStride)
	}
	if BoardCells != BoardSize*BoardStride {
		t.Errorf("BoardCells = %d, want Size*Stride = %d", BoardCells, BoardSize*BoardStride)
	}
	if BoardCells != 1<<8 {
		t.Errorf("BoardCells = %d, want 256", BoardCells)
	}
	if BoardCells&(BoardCells-1) != 0 {
		t.Errorf("BoardCells = %d, want a power of two for masking", BoardCells)
	}
	if BoardWordsPerColor != 4 || BoardWordsPerColor*64 != BoardCells {
		t.Errorf("BoardWordsPerColor = %d, want 4 covering %d cells", BoardWordsPerColor, BoardCells)
	}
	if WinLength != 5 {
		t.Errorf("WinLength = %d, want exactly 5, overlines never win", WinLength)
	}
	if OpeningChebyshevMin != 3 {
		t.Errorf("OpeningChebyshevMin = %d, want 3 (max(|dx|,|dy|) >= 3 for red's second move)", OpeningChebyshevMin)
	}
	if CrossCheckSize != 8 || CrossCheckSize >= BoardSize {
		t.Errorf("CrossCheckSize = %d, want 8, strictly below BoardSize", CrossCheckSize)
	}
}

func TestZobristSeed(t *testing.T) {
	if ZobristSeed != 0x9E3779B97F4A7C15 {
		t.Errorf("ZobristSeed = %#x, want the fixed splitmix64 golden ratio constant", ZobristSeed)
	}
}

func TestTimeControlsMatchSpec(t *testing.T) {
	want := [][2]int{{1, 0}, {2, 1}, {3, 2}}
	if len(TimeControls) != len(want) {
		t.Fatalf("len(TimeControls) = %d, want %d", len(TimeControls), len(want))
	}
	for i, tc := range TimeControls {
		if tc.InitialSec != want[i][0] || tc.IncrementSec != want[i][1] {
			t.Errorf("TimeControls[%d] = %d+%d, want %d+%d", i, tc.InitialSec, tc.IncrementSec, want[i][0], want[i][1])
		}
		if tc.InitialSec <= 0 {
			t.Errorf("TimeControls[%d].InitialSec = %d, must be positive", i, tc.InitialSec)
		}
		if tc.IncrementSec < 0 {
			t.Errorf("TimeControls[%d].IncrementSec = %d, must be non-negative", i, tc.IncrementSec)
		}
	}
}

func TestClockLawConstants(t *testing.T) {
	if ClockExpectedMovesPerSide < 10 || ClockExpectedMovesPerSide > 60 {
		t.Errorf("ClockExpectedMovesPerSide = %d, want within [10, 60] to track real game length", ClockExpectedMovesPerSide)
	}
	if ClockIncrementShare <= 0 || ClockIncrementShare > 1 {
		t.Errorf("ClockIncrementShare = %v, want in (0, 1]", ClockIncrementShare)
	}
	if ClockPIDClampFraction <= 0 || ClockPIDClampFraction >= 1 {
		t.Errorf("ClockPIDClampFraction = %v, want in (0, 1)", ClockPIDClampFraction)
	}
	if ClockPIDIntegClampMs <= 0 {
		t.Errorf("ClockPIDIntegClampMs = %v, must be positive", ClockPIDIntegClampMs)
	}
	if len(ClockPID) != len(TimeControls) {
		t.Fatalf("len(ClockPID) = %d, want %d, one gain set per time control", len(ClockPID), len(TimeControls))
	}
	for i, g := range ClockPID {
		if g.Kp < 0 || g.Ki < 0 || g.Kd < 0 {
			t.Errorf("ClockPID[%d] = %+v, gains must be non-negative", i, g)
		}
	}
	if SearchMinMoveTimeMs <= 0 || SearchSafetyMarginMs <= 0 {
		t.Errorf("floor %d and reserve %d must both be positive", SearchMinMoveTimeMs, SearchSafetyMarginMs)
	}
	if SearchClockQuantumMs < 15 || SearchClockQuantumMs > 17 {
		t.Errorf("SearchClockQuantumMs = %d, want the 64 Hz Windows tick ceiling 16", SearchClockQuantumMs)
	}
	if SearchMinMoveTimeMs >= SearchClockQuantumMs {
		t.Errorf("floor %d must sit under one clock quantum %d so the minimum grant keeps soft-stop protection", SearchMinMoveTimeMs, SearchClockQuantumMs)
	}
	if MutateMaxParallel < 1 || MutateMaxParallel > 64 {
		t.Errorf("MutateMaxParallel = %d, want a sane bound in [1, 64]", MutateMaxParallel)
	}
}

func TestSeriesMatchSpec(t *testing.T) {
	want := []int{3, 5, 7, 11}
	if len(SeriesLengths) != len(want) {
		t.Fatalf("len(SeriesLengths) = %d, want %d", len(SeriesLengths), len(want))
	}
	for i, n := range SeriesLengths {
		if n != want[i] {
			t.Errorf("SeriesLengths[%d] = %d, want %d", i, n, want[i])
		}
		if n%2 == 0 {
			t.Errorf("SeriesLengths[%d] = %d, must be odd so a majority wins", i, n)
		}
		if i > 0 && n <= SeriesLengths[i-1] {
			t.Errorf("SeriesLengths[%d] = %d, must ascend", i, n)
		}
	}
	if SeriesBO3 != 3 || SeriesBO5 != 5 || SeriesBO7 != 7 || SeriesBO11 != 11 {
		t.Errorf("series constants = %d/%d/%d/%d, want 3/5/7/11", SeriesBO3, SeriesBO5, SeriesBO7, SeriesBO11)
	}
}

func TestTiersMatchSpec(t *testing.T) {
	cases := []struct {
		want     Tier
		tiersPtr *Tier
	}{
		{Tier{Name: "easy", Cores: 1, TTBytes: 0, VCF: false, VCT: false}, &TierEasy},
		{Tier{Name: "medium", Cores: 2, TTBytes: 256 << 20, VCF: true, VCT: false}, &TierMedium},
		{Tier{Name: "hard", Cores: 4, TTBytes: 1 << 30, VCF: true, VCT: true}, &TierHard},
	}
	for _, c := range cases {
		if *c.tiersPtr != c.want {
			t.Errorf("tier = %+v, want %+v", *c.tiersPtr, c.want)
		}
	}
	if len(Tiers) != 3 || Tiers[0] != TierEasy || Tiers[1] != TierMedium || Tiers[2] != TierHard {
		t.Errorf("Tiers = %+v, want [easy medium hard]", Tiers)
	}
	for i, tier := range Tiers {
		if tier.Cores > MaxCoresPerInstance {
			t.Errorf("%s cores %d exceeds MaxCoresPerInstance %d", tier.Name, tier.Cores, MaxCoresPerInstance)
		}
		if tier.TTBytes > MaxRAMPerInstanceBytes {
			t.Errorf("%s tt %d exceeds MaxRAMPerInstanceBytes %d", tier.Name, tier.TTBytes, MaxRAMPerInstanceBytes)
		}
		if i > 0 {
			prev := Tiers[i-1]
			if tier.Cores < prev.Cores || tier.TTBytes < prev.TTBytes {
				t.Errorf("tier resources regress at %s", tier.Name)
			}
			if prev.VCF && !tier.VCF || prev.VCT && !tier.VCT {
				t.Errorf("tier capabilities regress at %s", tier.Name)
			}
		}
	}
}

func TestResourceCapsMatchHardwareBudget(t *testing.T) {
	if MaxCoresPerInstance != 8 {
		t.Errorf("MaxCoresPerInstance = %d, want 8 (half of the 16 core machine)", MaxCoresPerInstance)
	}
	if MaxRAMPerInstanceBytes != 16<<30 {
		t.Errorf("MaxRAMPerInstanceBytes = %d, want 16GiB (half of 32GB)", MaxRAMPerInstanceBytes)
	}
	if TournamentParallelMatches != 2 {
		t.Errorf("TournamentParallelMatches = %d, want 2", TournamentParallelMatches)
	}
	if TournamentParallelMatches*TierHard.Cores > 16 {
		t.Errorf("%d parallel hard matches need %d cores, machine has 16", TournamentParallelMatches, TournamentParallelMatches*TierHard.Cores)
	}
	if TournamentParallelMatches*TierHard.TTBytes > 32<<30 {
		t.Errorf("%d parallel hard matches need %d bytes, machine has 32GiB", TournamentParallelMatches, TournamentParallelMatches*TierHard.TTBytes)
	}
}

func TestTournamentConstants(t *testing.T) {
	if TournamentStartRating != 1000 {
		t.Errorf("TournamentStartRating = %d, want 1000 per the Implication 2.4 full run", TournamentStartRating)
	}
	if TournamentLogDir != "tourney-logs" {
		t.Errorf("TournamentLogDir = %q, want %q", TournamentLogDir, "tourney-logs")
	}
	// The format must carry run id, series id, and both display names in
	// order, and end in .txt so the artifact stays greppable.
	name := fmt.Sprintf(TournamentSeriesLogFormat, 7, 3, "hard-1", "easy-2")
	if name != "run7_s3_hard-1-vs-easy-2.txt" {
		t.Errorf("TournamentSeriesLogFormat renders %q", name)
	}
}

func TestPortsSpec(t *testing.T) {
	for name, p := range map[string]int{"HTTPPort": HTTPPort, "DebugPort": DebugPort} {
		if p <= 10000 || p > 65535 {
			t.Errorf("%s = %d, want an uncommon port in (10000, 65535]", name, p)
		}
	}
	if HTTPPort == DebugPort {
		t.Errorf("HTTPPort and DebugPort collide on %d", HTTPPort)
	}
}

func TestRatingConstantsMatchSpec(t *testing.T) {
	if RatingDelta != 30 {
		t.Errorf("RatingDelta = %d, want 30 per win or loss", RatingDelta)
	}
	if RatingDivisorFull != 3000 {
		t.Errorf("RatingDivisorFull = %d, want 3000", RatingDivisorFull)
	}
	if RatingDecayBase != 500 {
		t.Errorf("RatingDecayBase = %d, want 500", RatingDecayBase)
	}
	if RatingDecaySlope != 2.5 {
		t.Errorf("RatingDecaySlope = %v, want 2.5", RatingDecaySlope)
	}
	if RatingDecayMin != 500 || RatingDecayMax != 3000 {
		t.Errorf("decay bounds = %d..%d, want 500..3000", RatingDecayMin, RatingDecayMax)
	}
	if RatingStart != 0 {
		t.Errorf("RatingStart = %d, want 0 (can go negative)", RatingStart)
	}
	if RatingDecayMin > RatingDecayBase || RatingDecayBase > RatingDecayMax {
		t.Errorf("decay ordering broken: %d %d %d", RatingDecayMin, RatingDecayBase, RatingDecayMax)
	}
	bands := []struct {
		loserRating int
		divisor     float64
	}{
		{0, 500}, {200, 1000}, {400, 1500}, {600, 2000}, {800, 2500}, {1001, 3000},
	}
	for _, b := range bands {
		d := float64(RatingDecayBase) + RatingDecaySlope*float64(b.loserRating)
		if d > float64(RatingDecayMax) {
			d = float64(RatingDecayMax)
		}
		if d != b.divisor {
			t.Errorf("closed form at loser=%d gives %v, want band %v", b.loserRating, d, b.divisor)
		}
	}
}

func TestEvalConstants(t *testing.T) {
	if EvalMilliUnit != 1000 {
		t.Errorf("EvalMilliUnit = %d, want 1000 (1.0 base unit as integer)", EvalMilliUnit)
	}
	if EvalMateMax <= 1<<20 {
		t.Errorf("EvalMateMax = %d, must sit above any milliunit score", EvalMateMax)
	}
	if EvalMateMax >= math.MaxInt32 {
		t.Errorf("EvalMateMax = %d, must fit int32 with its negation", EvalMateMax)
	}
	if EvalMateScoreStep <= 0 {
		t.Errorf("EvalMateScoreStep = %d, must be positive", EvalMateScoreStep)
	}
	worst := int64(BoardCells) * 2 * EvalMateScoreStep
	if int64(EvalMateMax)-worst <= 1<<20 {
		t.Errorf("mate-in-%d plies at step %d collides with normal scores", BoardCells*2, EvalMateScoreStep)
	}
}

func TestSearchConstants(t *testing.T) {
	if SearchMaxPly <= 0 || SearchMaxPly > BoardCells {
		t.Errorf("SearchMaxPly = %d, want in (0, %d]", SearchMaxPly, BoardCells)
	}
	if SearchMaxMovesPerPly < BoardCells {
		t.Errorf("SearchMaxMovesPerPly = %d, must hold worst case %d moves", SearchMaxMovesPerPly, BoardCells)
	}
	if SearchNodeCheckInterval < 1 {
		t.Errorf("SearchNodeCheckInterval = %d, must be at least 1", SearchNodeCheckInterval)
	}
	if SearchSoftStopFraction <= 0 || SearchSoftStopFraction >= 1 {
		t.Errorf("SearchSoftStopFraction = %v, want in (0, 1)", SearchSoftStopFraction)
	}
	if SearchSafetyMarginMs < 1 {
		t.Errorf("SearchSafetyMarginMs = %d, must be positive", SearchSafetyMarginMs)
	}
	if SearchMinMoveTimeMs < 1 {
		t.Errorf("SearchMinMoveTimeMs = %d, must be positive", SearchMinMoveTimeMs)
	}
	if SearchMinMoveTimeMs+SearchSafetyMarginMs >= 1000 {
		t.Errorf("min move time %d + safety %d must fit a 1+0 clock", SearchMinMoveTimeMs, SearchSafetyMarginMs)
	}
}

func TestEngineEvalConstants(t *testing.T) {
	if EvalTempo != EvalMilliUnit {
		t.Errorf("EvalTempo = %d, want %d: one unblocked tempo is the 1.0 anchor", EvalTempo, EvalMilliUnit)
	}
	w := PatternClassWeights
	for i := 1; i < PatternClassCount; i++ {
		if w[i] <= w[i-1] {
			t.Errorf("PatternClassWeights[%d] = %d must exceed %d: forcing depth strictly ascends", i, w[i], w[i-1])
		}
	}
	if w[0] != 0 {
		t.Errorf("PatternWeightNone = %d, want 0", w[0])
	}
	mateFloor := int64(EvalMateMax) - int64(SearchMaxPly)*int64(EvalMateScoreStep)
	maxWindowSum := int64(PatternDirections) * int64(BoardCells) * int64(PatternWeightOpenFour)
	if maxWindowSum*2 >= mateFloor {
		t.Errorf("max leaf score %d collides with the mate band below %d", maxWindowSum*2, mateFloor)
	}
}

func TestEngineSearchConstants(t *testing.T) {
	windowHalf := (PatternWindowLen - 1) / 2
	if SearchRingRadius < 1 || SearchRingRadius > windowHalf {
		t.Errorf("SearchRingRadius = %d, want in [1, %d]", SearchRingRadius, windowHalf)
	}
	if SearchExtensionMaxPly < 1 || SearchMaxPly+SearchExtensionMaxPly > 120 {
		t.Errorf("SearchExtensionMaxPly = %d leaves depth out of int8 tt range", SearchExtensionMaxPly)
	}
	if OpeningChebyshevMin > SearchRingRadius+1 {
		t.Errorf("OpeningChebyshevMin %d exceeds SearchRingRadius+1 %d: the opening filter can empty the candidate ring", OpeningChebyshevMin, SearchRingRadius+1)
	}
	if SearchHistoryMax >= SearchOrderKiller2 {
		t.Errorf("SearchHistoryMax %d must sit below SearchOrderKiller2 %d", SearchHistoryMax, SearchOrderKiller2)
	}
	if PatternWeightOpenFour*2*PatternDirections+SearchHistoryMax >= SearchOrderKiller2 {
		t.Errorf("static ordering score plus history can pass the killer layer")
	}
	if SearchOrderTT != math.MaxInt32 || SearchOrderKiller1 != SearchOrderKiller2+1 {
		t.Errorf("ordering layers must be strictly ordered with TT on top")
	}
	if SearchHashFullSample < 1 {
		t.Errorf("SearchHashFullSample = %d, must be positive", SearchHashFullSample)
	}
	want := BoardStride*(BoardSize/2-1) + BoardSize/2 - 1
	if SearchEmptyBoardCell != want {
		t.Errorf("SearchEmptyBoardCell = %d, want center cell %d", SearchEmptyBoardCell, want)
	}
}

func TestBotLogFormatMatchesImplication15(t *testing.T) {
	cases := []struct {
		name string
		line string
		args []any
	}{
		{
			"healthy",
			"M24, Red, J9, d=14, n=6.25m, nps=2.5m, ebf=2.1, tt=38%, hf=45%, fh1=93%, s=+150, thr=4, t=2.50, alloc=2.50, pv=J9 K10 K9 L9 M8 L8",
			[]any{24, "Red", "J9", 14, "6.25m", "2.5m", 2.1, 38, 45, 93, "+150", 4, 2.50, 2.50, "", "J9 K10 K9 L9 M8 L8"},
		},
		{
			"vct hit",
			"M31, Blue, G7, d=9, n=45k, nps=1.2m, ebf=1.4, tt=18%, hf=60%, fh1=88%, s=M9, thr=4, t=0.03, alloc=3.00, [VCT], pv=G7 H7 G8 G6 G9 G10 F8 E9 I8",
			[]any{31, "Blue", "G7", 9, "45k", "1.2m", 1.4, 18, 60, 88, "M9", 4, 0.03, 3.00, BotLogTagVCT, "G7 H7 G8 G6 G9 G10 F8 E9 I8"},
		},
		{
			"ponder hit",
			"M12, Red, H10, d=16, n=18m, nps=2.8m, ebf=2.0, tt=44%, hf=78%, fh1=91%, s=-25, thr=4, t=0.01, alloc=4.00, [PONDER], pv=H10 I9 J8 K7 J10",
			[]any{12, "Red", "H10", 16, "18m", "2.8m", 2.0, 44, 78, 91, "-25", 4, 0.01, 4.00, BotLogTagPonder, "H10 I9 J8 K7 J10"},
		},
	}
	for _, c := range cases {
		if got := fmt.Sprintf(BotLogFormat, c.args...); got != c.line {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.line)
		}
	}
	if BotLogTagVCF != ", [VCF]" || BotLogTagVCT != ", [VCT]" || BotLogTagPonder != ", [PONDER]" {
		t.Errorf("bot log tags = %q %q %q", BotLogTagVCF, BotLogTagVCT, BotLogTagPonder)
	}
}

func TestQualityGates(t *testing.T) {
	if QualityCoverageOverallMin != 95 {
		t.Errorf("QualityCoverageOverallMin = %v, want 95", QualityCoverageOverallMin)
	}
	if QualityCoverageCoreMin != 100 {
		t.Errorf("QualityCoverageCoreMin = %v, want 100", QualityCoverageCoreMin)
	}
	if QualityCoverageCoreMin < QualityCoverageOverallMin {
		t.Errorf("core gate %v below overall gate %v", QualityCoverageCoreMin, QualityCoverageOverallMin)
	}
}

func TestServerConstants(t *testing.T) {
	if Argon2Time < 1 || Argon2MemoryKiB < 19456 || Argon2Parallelism < 1 {
		t.Errorf("argon2 profile t=%d m=%dKiB p=%d below the OWASP floor", Argon2Time, Argon2MemoryKiB, Argon2Parallelism)
	}
	if Argon2SaltBytes < 16 || Argon2KeyBytes < 32 {
		t.Errorf("argon2 salt %d or key %d too short", Argon2SaltBytes, Argon2KeyBytes)
	}
	if SessionTokenBytes < 32 {
		t.Errorf("SessionTokenBytes = %d, want >= 32 for an opaque token", SessionTokenBytes)
	}
	if SessionTTLHours < 1 {
		t.Errorf("SessionTTLHours = %d, must be positive", SessionTTLHours)
	}
	if UsernameMaxBytes < 1 || UsernameMaxBytes > 64 {
		t.Errorf("UsernameMaxBytes = %d, want in [1, 64]", UsernameMaxBytes)
	}
	if SQLiteSchemaVersion < 1 || SQLiteSchemaVersion > 3 {
		t.Errorf("SQLiteSchemaVersion = %d, want in [1, 3]: raise the ceiling with the next migration", SQLiteSchemaVersion)
	}
	if SQLiteBusyTimeoutMs < 1000 || SQLiteBusyTimeoutMs > 60000 {
		t.Errorf("SQLiteBusyTimeoutMs = %d, want in [1000, 60000]", SQLiteBusyTimeoutMs)
	}
	if SQLiteJournalWAL != "wal" || SQLiteSyncNormal != "normal" {
		t.Errorf("sqlite modes = %q/%q, want wal/normal", SQLiteJournalWAL, SQLiteSyncNormal)
	}
	if WriteQueueDepth < 1 {
		t.Errorf("WriteQueueDepth = %d, must be positive", WriteQueueDepth)
	}
	if HubSubscriberBuffer < 1 {
		t.Errorf("HubSubscriberBuffer = %d, must be positive", HubSubscriberBuffer)
	}
	if HistoryPreviewTurns != 8 {
		t.Errorf("HistoryPreviewTurns = %d, want 8 per the spec's move-history preview", HistoryPreviewTurns)
	}
}
