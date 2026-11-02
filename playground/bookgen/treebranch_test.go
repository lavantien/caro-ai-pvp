package main

import (
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func parseArtifactEntries(t *testing.T, path string) ([]bookEntry, error) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	body := data[len(config.BookMagicLine):]
	for end := 0; end < len(body); end++ {
		if body[end] == '\n' {
			body = body[end+1:]
			break
		}
	}
	out := make([]bookEntry, 0, len(body)/config.BookEntryBytes)
	for off := 0; off+config.BookEntryBytes <= len(body); off += config.BookEntryBytes {
		var e bookEntry
		copy(e.key[:], body[off:off+config.BookKeyBytes])
		e.move = rules.Move(binary.LittleEndian.Uint16(body[off+config.BookKeyBytes:]))
		e.visits = binary.LittleEndian.Uint32(body[off+config.BookKeyBytes+config.BookMoveBytes:])
		e.source = body[off+config.BookKeyBytes+config.BookMoveBytes+config.BookVisitsBytes]
		out = append(out, e)
	}
	return out, nil
}

func legacyTrunkPath(t *testing.T, records ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy-tree.txt")
	body := "F tree v2 tier=master ply=8 k=4 leaf=1048576 critical=8388608 reverify=16777216 critmargin=300 stable=3 stablemargin=150 lopsided=3000 prune=300 total=268435456 solver=16777216\n"
	body += strings.Join(records, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func trunkRecords(t *testing.T) []string {
	t.Helper()
	root := canonicalKey(corpusZeroKey)
	after := canonicalBoardKey(boardWithMoves(t, rules.Cell(136)))
	return []string{
		formatLeaf(root, leafRec{budget: 8388608, score: -1000, best: 136, nodes: 8388608}),
		formatExpansion(root, treeExpansion{ply: 0, own: true, best: 136, bestGap: 0, children: []childRec{{move: 136, child: after}}}),
		formatLeaf(after, leafRec{budget: 8388608, score: 200, best: 104, nodes: 8388608}),
	}
}

func branchOptsFor(t *testing.T, from string, seeds ...string) branchOpts {
	t.Helper()
	dir := t.TempDir()
	return branchOpts{
		tier:      config.TierMaster.Name,
		fromPath:  from,
		logPath:   filepath.Join(dir, "book-tree.txt"),
		outPath:   filepath.Join(dir, "book.carobook"),
		statsPath: filepath.Join(dir, "book-stats.txt"),
		seeds:     seeds,
		created:   "2026-11-03T00:00:00Z",
	}
}

func TestBranchSeedsLegacyTrunkAndEmitsBlueTargets(t *testing.T) {
	from := legacyTrunkPath(t, trunkRecords(t)...)
	o := branchOptsFor(t, from, "H8", "H8,H6,H5")
	sa := &stubAnalyst{score: 400}
	ss := &stubSolver{nodes: 10}
	st, err := runBranch(o, sa.analyze, ss.probe, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if st.Emitted == 0 {
		t.Fatal("branch emitted nothing")
	}
	raw, err := os.ReadFile(o.logPath)
	if err != nil {
		t.Fatal(err)
	}
	wantHeader := paramsFromConfig(config.TierMaster.Name).fingerprint(config.TierMaster.BookMaxPly)
	if !strings.HasPrefix(string(raw), wantHeader+"\n") {
		t.Fatalf("branched log header = %q, want the current fingerprint %q", strings.SplitN(string(raw), "\n", 2)[0], wantHeader)
	}
	for _, rec := range trunkRecords(t) {
		if !strings.Contains(string(raw), rec) {
			t.Fatalf("legacy record %q missing from the branched log", rec)
		}
	}
	f, err := os.Open(o.outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bk, err := book.Load(f)
	if err != nil {
		t.Fatalf("branched artifact rejected by the loader: %v", err)
	}
	afterOpen := boardWithMoves(t, rules.Cell(119))
	mv, ok := bk.Probe(afterOpen)
	if !ok {
		t.Fatal("blue move-2 position after H8 missed the book")
	}
	if !afterOpen.IsLegal(rules.Cell(mv)) {
		t.Fatalf("blue move-2 reply %d illegal", mv)
	}
	board4 := boardWithMoves(t, rules.Cell(119), rules.Cell(87), rules.Cell(71))
	mv, ok = bk.Probe(board4)
	if !ok {
		t.Fatal("blue move-4 discriminator board missed the book")
	}
	if !board4.IsLegal(rules.Cell(mv)) {
		t.Fatalf("blue move-4 reply %d illegal", mv)
	}
	root := rules.NewBoard()
	if mv, ok := bk.Probe(root); !ok || mv != rules.Move(136) {
		t.Fatalf("root probe = (%d, %v), want the legacy trunk's 136 preserved", mv, ok)
	}
	entries, err := parseArtifactEntries(t, o.outPath)
	if err != nil {
		t.Fatal(err)
	}
	blueKey := canonicalBoardKey(boardWithMoves(t, rules.Cell(119)))
	for _, e := range entries {
		if e.key != blueKey {
			continue
		}
		if e.source != config.BookSourceMajority {
			t.Errorf("blue entry source = %d, want majority", e.source)
		}
		if e.visits != uint32(config.BookTreeVisitScale) {
			t.Errorf("blue entry visits = %d, want the full unexpanded weight %d", e.visits, config.BookTreeVisitScale)
		}
	}
}

func TestBranchAnalyzesTargetsAtTheCriticalLadder(t *testing.T) {
	from := legacyTrunkPath(t, trunkRecords(t)...)
	o := branchOptsFor(t, from, "H8,I7,H5")
	sa := &stubAnalyst{score: 400}
	if _, err := runBranch(o, sa.analyze, (&stubSolver{nodes: 10}).probe, io.Discard); err != nil {
		t.Fatal(err)
	}
	key := canonicalBoardKey(boardWithMoves(t, rules.Cell(119), rules.Cell(104), rules.Cell(71)))
	tl, err := parseTreeLog(o.logPath)
	if err != nil {
		t.Fatal(err)
	}
	recs := tl.analyses[key]
	if len(recs) != 1 {
		t.Fatalf("target analyses = %d, want the single critical base under the plausibility gate", len(recs))
	}
	if recs[0].budget != config.BookTreeCriticalNodes {
		t.Fatalf("target budget = %d, want the critical base %d", recs[0].budget, config.BookTreeCriticalNodes)
	}
	if _, done := tl.solverDone[key]; !done {
		t.Error("target skipped the solver pass")
	}
}

func TestBranchResumeRunIsIdempotent(t *testing.T) {
	from := legacyTrunkPath(t, trunkRecords(t)...)
	o := branchOptsFor(t, from, "H8", "H8,H6,H5")
	sa := &stubAnalyst{score: 400}
	ss := &stubSolver{nodes: 10}
	if _, err := runBranch(o, sa.analyze, ss.probe, io.Discard); err != nil {
		t.Fatal(err)
	}
	calls, probes := sa.calls, ss.calls
	sizeFirst, err := os.Stat(o.logPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runBranch(o, sa.analyze, ss.probe, io.Discard); err != nil {
		t.Fatal(err)
	}
	if sa.calls != calls || ss.calls != probes {
		t.Fatalf("resume ran %d analyses and %d solver probes, want zero of each", sa.calls-calls, ss.calls-probes)
	}
	sizeSecond, err := os.Stat(o.logPath)
	if err != nil {
		t.Fatal(err)
	}
	if sizeSecond.Size() != sizeFirst.Size() {
		t.Fatal("resume appended records to a completed branch log")
	}
}

func TestBranchRejectsRedToMoveTarget(t *testing.T) {
	from := legacyTrunkPath(t, trunkRecords(t)...)
	o := branchOptsFor(t, from, "H8,H6")
	if _, err := runBranch(o, (&stubAnalyst{score: 400}).analyze, (&stubSolver{nodes: 10}).probe, io.Discard); err == nil {
		t.Fatal("red-to-move target must fail loudly, the branch serves blue replies only")
	}
}

func TestBranchRejectsBadSeedSpec(t *testing.T) {
	from := legacyTrunkPath(t, trunkRecords(t)...)
	for _, spec := range []string{"H8,ZZ9", "", "H8,,H6"} {
		o := branchOptsFor(t, from, spec)
		if _, err := runBranch(o, (&stubAnalyst{score: 400}).analyze, (&stubSolver{nodes: 10}).probe, io.Discard); err == nil {
			t.Fatalf("seed spec %q must fail loudly", spec)
		}
	}
}

func TestBranchRejectsForeignCurrentFormatLog(t *testing.T) {
	from := legacyTrunkPath(t, trunkRecords(t)...)
	o := branchOptsFor(t, from, "H8")
	p := tinyParams()
	p.total = 999
	if err := os.WriteFile(o.logPath, []byte(p.fingerprint(4)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := runBranch(o, (&stubAnalyst{score: 400}).analyze, (&stubSolver{nodes: 10}).probe, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("foreign current-format log err = %v, want a fingerprint refusal", err)
	}
	if raw, rerr := os.ReadFile(o.logPath); rerr != nil || !strings.HasPrefix(string(raw), p.fingerprint(4)) {
		t.Fatal("the refused branch clobbered the foreign log")
	}
}

func TestBranchRejectsSourceThatIsNotATreeLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-tree.txt")
	if err := os.WriteFile(path, []byte("F corpus v2 corpus=x\nL nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	o := branchOptsFor(t, path, "H8")
	if _, err := runBranch(o, (&stubAnalyst{score: 400}).analyze, (&stubSolver{nodes: 10}).probe, io.Discard); err == nil {
		t.Fatal("non-tree source must fail loudly")
	}
}

func TestBranchSkipsBlankLegacyLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy-tree.txt")
	body := "F tree v2 tier=master ply=8\n\n" + strings.Join(trunkRecords(t), "\n") + "\n   \n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	o := branchOptsFor(t, path, "H8")
	if _, err := runBranch(o, (&stubAnalyst{score: 400}).analyze, (&stubSolver{nodes: 10}).probe, io.Discard); err != nil {
		t.Fatalf("blank and space-only legacy lines must be skipped: %v", err)
	}
	f, err := os.Open(o.outPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	bk, err := book.Load(f)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bk.Probe(rules.NewBoard()); !ok {
		t.Fatal("trunk records lost around the blank lines")
	}
}
