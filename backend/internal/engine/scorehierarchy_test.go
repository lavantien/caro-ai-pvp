package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScoreHierarchy(t *testing.T) {
	assert.Greater(t, domain.Infinity, domain.WinScore,
		"Infinity must exceed WinScore")
	assert.Greater(t, domain.WinScore, domain.MaxEval,
		"WinScore must exceed MaxEval")
	assert.Greater(t, domain.MaxEval, int(flex4WinBonus),
		"MaxEval must exceed flex4WinBonus")
}

func TestAspirationWindowGomokuScale(t *testing.T) {
	assert.GreaterOrEqual(t, domain.AspirationWindowSize, int(flex3Score),
		"aspiration window should cover at least a flex3 swing")
}

func TestFiveScoreEqualsWinScore(t *testing.T) {
	assert.Equal(t, domain.WinScore, int(fiveScore),
		"fiveScore must equal WinScore so static eval and search agree on win value")
}

func TestMaxCorrectedEvalEqualsMaxEval(t *testing.T) {
	assert.Equal(t, domain.MaxEval, int(maxCorrectedEval),
		"maxCorrectedEval must equal MaxEval")
}

func TestFiveScoreBoundedByWinScore(t *testing.T) {
	assert.LessOrEqual(t, fiveScore, domain.WinScore,
		"fiveScore must never exceed WinScore")
}

func TestEvalClampedBelowWinScore(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	sb := NewSearchBoard(b)
	score := Evaluate(&sb, domain.PlayerRed)
	assert.Less(t, score, domain.WinScore,
		"static eval for 4-in-a-row must stay below WinScore")
}

func TestSearchNoGhostScores(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    20,
		TimeLimitMs: 1,
		Goroutines:  1,
	}

	_, _, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.NotEqual(t, -60_000, stats.SearchScore,
		"search must not return -60k ghost score on timeout")
	assert.NotEqual(t, 60_000, stats.SearchScore,
		"search must not return +60k ghost score on timeout")
}

func TestMateInOneBeatsMateInThree(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    4,
		TimeLimitMs: 5000,
		Goroutines:  1,
	}

	_, _, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.GreaterOrEqual(t, stats.SearchScore, domain.WinScore-domain.AbsoluteMaxDepth,
		"found winning move should score in mate range")
	assert.Less(t, stats.SearchScore, domain.WinScore,
		"mate score must be less than WinScore (ply penalty applied)")
}

func TestTTMateScoreRoundTrip(t *testing.T) {
	plyStored := 3
	mateScore := domain.WinScore - plyStored

	stored := adjustMateScoreForStore(mateScore, plyStored)
	assert.GreaterOrEqual(t, stored, domain.WinScore,
		"stored positive mate score should be >= WinScore")

	// Retrieve at ply 5
	plyRetrieve := 5
	retrieved := adjustMateScoreForRetrieve(stored, plyRetrieve)
	assert.Equal(t, domain.WinScore-plyRetrieve, retrieved,
		"round-trip at ply %d should give WinScore-%d", plyRetrieve, plyRetrieve)

	// Same position retrieved at ply 1 should score higher (closer to root)
	retrievedEarly := adjustMateScoreForRetrieve(stored, 1)
	assert.Greater(t, retrievedEarly, retrieved,
		"mate retrieved at ply 1 should score higher than at ply 5")
	assert.Equal(t, domain.WinScore-1, retrievedEarly)
}

func TestTTNonMateScoreUnchanged(t *testing.T) {
	normalScore := 5000
	assert.Equal(t, normalScore, adjustMateScoreForStore(normalScore, 3))
	assert.Equal(t, normalScore, adjustMateScoreForRetrieve(normalScore, 3))
}

func TestAbortPreservesPreviousDepth(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	b = b.PlaceStone(10, 10, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    20,
		TimeLimitMs: 5,
		Goroutines:  1,
	}

	_, _, stats := SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())
	assert.Greater(t, stats.DepthAchieved, 0,
		"should complete at least depth 1 before timeout")
	assert.Less(t, stats.SearchScore, domain.WinScore,
		"score from completed depth should be < WinScore for a 4-in-a-row position")
}

func TestAbortDoesNotPoisonTT(t *testing.T) {
	// Position where search will timeout mid-depth with an incomplete score
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 9, domain.PlayerBlue)

	tt := NewTranspositionTable(1)
	heuristics := NewSearchHeuristics()
	opts := SearchConfig{
		MaxDepth:    20,
		TimeLimitMs: 1,
		Goroutines:  1,
	}

	SearchPosition(b, domain.PlayerRed, opts, tt, heuristics, context.Background())

	// After search, check TT entries for poisoned scores
	// We can't iterate TT entries directly, but we can verify no Infinity scores
	// by doing a second search with the same TT and checking results
	opts2 := SearchConfig{
		MaxDepth:    6,
		TimeLimitMs: 5000,
		Goroutines:  1,
	}
	tt.ResetStats()
	_, _, stats2 := SearchPosition(b, domain.PlayerRed, opts2, tt, heuristics, context.Background())
	// If TT was poisoned with -Infinity, the second search might return it
	assert.Greater(t, stats2.SearchScore, -domain.WinScore,
		"second search should not inherit poisoned TT score")
}
