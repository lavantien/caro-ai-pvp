package server

import (
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestMateClaimPliesTable(t *testing.T) {
	for _, row := range [...]struct {
		score int
		want  int
	}{
		{config.EvalMateMax - config.EvalMateScoreStep, 1},
		{config.EvalMateMax - 3*config.EvalMateScoreStep, 3},
		{config.EvalMateMax - 8*config.EvalMateScoreStep, 8},
		{config.EvalMateFloor, config.SearchMaxPly},
	} {
		if got := mateClaimPlies(row.score); got != row.want {
			t.Errorf("mateClaimPlies(%d) = %d, want %d", row.score, got, row.want)
		}
	}
	if got := mateClaimPlies(config.EvalMateFloor - config.EvalMateScoreStep); got != config.SearchMaxPly+1 {
		t.Errorf("below floor claim = %d, want the lattice arithmetic one past the fence, unreachable through the gate", got)
	}
}

func blueFourBoard(t testing.TB) *rules.Board {
	return anchorBoard(t, rules.Red, x(0, 1), x(0, 2), x(0, 3), x(0, 4), r(3, -3), r(3, -2))
}

func TestOpponentRefutesFastKillOverSlowClaim(t *testing.T) {
	b := blueFourBoard(t)
	claim := config.EvalMateMax - 5*config.EvalMateScoreStep
	slow := rules.Cell(mustCellT(t, "E5"))
	if !b.IsLegal(slow) {
		t.Fatalf("candidate %v illegal on the fixture", cellName(slow))
	}
	if !opponentRefutes(b, rules.Move(slow), claim, config.RefAuditNodeBudget, nil) {
		t.Fatal("opponent one ply kill must refute a mate in 5 claim")
	}
	if opponentRefutes(b, rules.Move(slow), config.EvalMateMax-config.EvalMateScoreStep, config.RefAuditNodeBudget, nil) {
		t.Fatal("a mate in 1 claim ties the one ply kill and must keep the claim")
	}
}

func TestOpponentRefutesPassesAWonBoard(t *testing.T) {
	b := openFourBoard(t)
	win := rules.Move(mustCellT(t, "I9"))
	if opponentRefutes(b, win, config.EvalMateMax-config.EvalMateScoreStep, config.RefAuditNodeBudget, nil) {
		t.Fatal("the winning move itself must never be refuted")
	}
}

type refAuditBot struct {
	mu        sync.Mutex
	searches  int
	excluded  int
	lastExcl  rules.Move
	move      rules.Move
	stats     engine.SearchStats
	exclMove  rules.Move
	exclStats engine.SearchStats
}

func (b *refAuditBot) Search(_ *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.mu.Lock()
	b.searches++
	b.mu.Unlock()
	return b.move, b.stats, ""
}

func (b *refAuditBot) SearchExcluded(_ *rules.Board, _ engine.Deadline, exclude rules.Move) (rules.Move, engine.SearchStats) {
	b.mu.Lock()
	b.excluded++
	b.lastExcl = exclude
	b.mu.Unlock()
	return b.exclMove, b.exclStats
}

func (b *refAuditBot) StartPonder(*rules.Board) {}

func (b *refAuditBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *refAuditBot) Close() {}

func (b *refAuditBot) counts() (int, int, rules.Move) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.searches, b.excluded, b.lastExcl
}

func TestSolverSearcherAuditDemotesRefutedClaim(t *testing.T) {
	b := blueFourBoard(t)
	inner := &refAuditBot{}
	inner.move = rules.Move(mustCellT(t, "E5"))
	inner.stats = engine.SearchStats{Score: config.EvalMateMax - 5*config.EvalMateScoreStep, Depth: 4}
	inner.exclMove = rules.Move(mustCellT(t, "F6"))
	inner.exclStats = engine.SearchStats{Score: -200, Depth: 3}
	s := &solverSearcher{inner: inner, cores: 1}
	grant := 2 * time.Second
	mv, st, tag := s.Search(b, engine.NewFixedBudget(grant))
	s.Close()
	searches, excluded, lastExcl := inner.counts()
	if searches != 1 || excluded != 1 {
		t.Fatalf("inner searches %d excluded %d, want 1 and the single demotion re-search", searches, excluded)
	}
	if lastExcl != inner.move {
		t.Fatalf("re-search excluded %d, want the refuted candidate %d", lastExcl, inner.move)
	}
	if mv != inner.exclMove || tag != "" {
		t.Fatalf("demoted answer = %d tag %q, want the honest re-search move with no tag", mv, tag)
	}
	if st.Score != -200 || st.AllocNs != int64(grant) {
		t.Fatalf("honest stats score %d alloc %d, want the re-search score and the grant billed", st.Score, st.AllocNs)
	}
}

func TestSolverSearcherAuditPassesConfirmedWin(t *testing.T) {
	b := openFourBoard(t)
	inner := &refAuditBot{}
	inner.move = rules.Move(mustCellT(t, "I9"))
	inner.stats = engine.SearchStats{Score: config.EvalMateMax - config.EvalMateScoreStep, Depth: 4}
	s := &solverSearcher{inner: inner, cores: 1}
	mv, st, tag := s.Search(b, engine.NewFixedBudget(2*time.Second))
	s.Close()
	_, excluded, _ := inner.counts()
	if excluded != 0 {
		t.Fatalf("confirmed win re-searched, excluded = %d, want the audit to pass it", excluded)
	}
	if mv != inner.move || st.Score != inner.stats.Score || tag != "" {
		t.Fatalf("confirmed win answer = %d score %d tag %q, want the untouched claim", mv, st.Score, tag)
	}
}

func TestSolverSearcherAuditSkippedUnderFloorGrant(t *testing.T) {
	b := blueFourBoard(t)
	inner := &refAuditBot{}
	inner.move = rules.Move(mustCellT(t, "E5"))
	inner.stats = engine.SearchStats{Score: config.EvalMateMax - 5*config.EvalMateScoreStep, Depth: 4}
	s := &solverSearcher{inner: inner, cores: 1}
	grant := time.Duration(config.SolverMinGrantMs-10) * time.Millisecond
	mv, _, tag := s.Search(b, engine.NewFixedBudget(grant))
	s.Close()
	_, excluded, _ := inner.counts()
	if excluded != 0 {
		t.Fatalf("floor grant audited, excluded = %d, want the skip", excluded)
	}
	if mv != inner.move || tag != "" {
		t.Fatalf("floor grant answer = %d tag %q, want the claim passed through", mv, tag)
	}
}
