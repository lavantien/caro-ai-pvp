package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func writePriorsCorpus(t *testing.T, games ...[]posRec) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "priors-corpus.txt")
	fp := corpusFingerprint(config.TierMaster.Name, 3, len(games))
	if _, err := writeSidecarHeader(path, fp); err != nil {
		t.Fatal(err)
	}
	clean := int64(len(fp) + 1)
	for i, recs := range games {
		next, err := appendGame(path, clean, i, recs, 0)
		if err != nil {
			t.Fatal(err)
		}
		clean = next
	}
	return path
}

func priorsRec(b *rules.Board, mv rules.Move) posRec {
	return posRec{key: keyOf(b), move: mv}
}

func TestLoadPriorsCanonicalizesAndCounts(t *testing.T) {
	a := boardWithMoves(t, rules.Cell(119), rules.Cell(72))
	b := boardWithMoves(t, rules.Cell(120), rules.Cell(71))
	if canonicalBoardKey(a) != canonicalBoardKey(b) {
		t.Fatal("fixture pair lost symmetry under the canonical key")
	}
	path := writePriorsCorpus(t,
		[]posRec{priorsRec(a, rules.Move(50)), priorsRec(b, rules.Move(0))},
		[]posRec{priorsRec(a, rules.Move(50))},
	)
	pt, err := loadPriors([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if pt.games != 2 {
		t.Errorf("priors games = %d, want 2", pt.games)
	}
	ckA, mvA := canonicalEntry(keyOf(a), rules.Move(50))
	ckB, mvB := canonicalEntry(keyOf(b), rules.Move(0))
	if ckA != ckB {
		t.Fatal("symmetric fixture pair must canonicalize to one key")
	}
	if got := pt.bonus(ckA, mvA); got != 2 {
		t.Errorf("canonical-frame move count = %d, want 2", got)
	}
	if got := pt.bonus(ckA, mvB); got != 1 {
		t.Errorf("variant-frame move mapped into canonical frame = %d, want 1", got)
	}
	if got := pt.bonus(ckA, rules.Move(50)); got != 0 {
		t.Errorf("raw variant-frame cell must not count in the canonical frame, got %d", got)
	}
	if got := pt.bonus(ckA, rules.Move(9)); got != 0 {
		t.Errorf("unplayed move bonus = %d, want 0", got)
	}
}

func TestLoadPriorsMissingFileIsFatal(t *testing.T) {
	if _, err := loadPriors([]string{filepath.Join(t.TempDir(), "absent.txt")}); err == nil {
		t.Fatal("missing prior file must error loudly")
	}
}

func TestRankMovesAppliesPriorsTieBreak(t *testing.T) {
	a := boardWithMoves(t, rules.Cell(119), rules.Cell(72))
	path := writePriorsCorpus(t, []posRec{priorsRec(a, rules.Move(33))})
	pt, err := loadPriors([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	canon, playedMv := canonicalEntry(keyOf(a), rules.Move(33))
	rm := []rankedMove{{Move: 10, Score: 100}, {Move: playedMv, Score: 100}, {Move: 2, Score: 200}}
	got := rankedByScoreMoves(rm, canon, pt)
	if len(got) != 3 || got[0].Move != 2 || got[1].Move != playedMv || got[2].Move != 10 {
		t.Errorf("ranked order = %v, want score desc then prior bonus then cell", got)
	}
	if rm[0].Move != 10 || rm[1].Move != playedMv {
		t.Error("input slice must not be reordered in place")
	}
}

func TestLoadPriorsEmptyTable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	pt, err := loadPriors([]string{path})
	if err != nil {
		t.Fatal(err)
	}
	if pt.games != 0 || len(pt.counts) != 0 {
		t.Errorf("empty corpus loaded games %d counts %d, want zeros", pt.games, len(pt.counts))
	}
}
