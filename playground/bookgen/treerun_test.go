package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func testTreeOpts(dir string) treeOpts {
	return treeOpts{
		tier:      config.TierMaster.Name,
		logPath:   filepath.Join(dir, "tree.txt"),
		outPath:   filepath.Join(dir, "book.carobook"),
		statsPath: filepath.Join(dir, "stats.txt"),
		created:   "2026-10-22T00:00:00Z",
	}
}

func TestRunTreeWithStubsEndsClean(t *testing.T) {
	dir := t.TempDir()
	o := testTreeOpts(dir)
	sa := &stubAnalyst{score: 5000, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	st, err := runTree(o, sa.analyze, (&stubSolver{nodes: 10}).probe, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if st.Emitted == 0 {
		t.Fatal("stub run emitted nothing")
	}
	if _, err := os.Stat(o.outPath); err != nil {
		t.Fatal(err)
	}
}

func TestRunTreeRefusesUnknownTier(t *testing.T) {
	o := testTreeOpts(t.TempDir())
	o.tier = "titanium"
	if _, err := runTree(o, (&stubAnalyst{}).analyze, (&stubSolver{}).probe, io.Discard); err == nil {
		t.Fatal("unknown tier must fail loudly")
	}
}

func TestRunTreeResumeSecondInvocationIsNoop(t *testing.T) {
	dir := t.TempDir()
	o := testTreeOpts(dir)
	sa := &stubAnalyst{score: 5000, rankings: map[int][]rankedMove{0: rootOpeningRanking()}}
	if _, err := runTree(o, sa.analyze, (&stubSolver{nodes: 10}).probe, io.Discard); err != nil {
		t.Fatal(err)
	}
	calls := sa.calls
	if _, err := runTree(o, sa.analyze, (&stubSolver{nodes: 10}).probe, io.Discard); err != nil {
		t.Fatal(err)
	}
	if sa.calls != calls {
		t.Errorf("second invocation ran %d extra analyses, want zero", sa.calls-calls)
	}
}

func TestRunTreeRealEngineSmokeArtifactProbes(t *testing.T) {
	dir := t.TempDir()
	p := paramsFromConfig(config.TierMaster.Name)
	p.leaf = 2048
	p.critical = 4096
	p.total = 400_000
	p.solver = 1 << 14
	path := filepath.Join(dir, "tree.txt")
	a := &engineAnalyst{}
	bt, err := newBookTree(p, 2, priorsTable{}, a.analyze, treeSolverPort, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := bt.run(io.Discard); err != nil {
		t.Fatal(err)
	}
	st, err := bt.writeArtifact(filepath.Join(dir, "book.carobook"), filepath.Join(dir, "stats.txt"), "2026-10-22T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if st.Emitted == 0 {
		t.Fatal("real-engine smoke emitted nothing")
	}
	f, err := os.Open(filepath.Join(dir, "book.carobook"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	bk, err := book.Load(f)
	if err != nil {
		t.Fatalf("artifact rejected by the loader: %v", err)
	}
	empty := rules.NewBoard()
	mv, ok := bk.Probe(empty)
	if !ok {
		t.Fatal("empty board must probe into the book")
	}
	if !empty.IsLegal(rules.Cell(mv)) {
		t.Fatalf("probed move %v illegal on the empty board", mv)
	}
	occupied := 0
	for _, c := range config.BookOpeningCells {
		if int(mv) == c {
			occupied++
		}
	}
	if occupied == 0 {
		t.Errorf("root move %d is not a protocol opening cell", mv)
	}
	deep := rules.NewBoard()
	for _, c := range []rules.Cell{119, 72, 10, 240, 130, 4, 200, 60} {
		deep.Make(c)
	}
	if deep.MoveCount != 8 {
		t.Fatalf("deep fixture made %d of 8 moves", deep.MoveCount)
	}
	if _, ok := bk.Probe(deep); ok {
		t.Error("past-bound position must not probe")
	}
}

func TestTreeSolverPortRealNoProofOnQuietBoard(t *testing.T) {
	b := rules.NewBoard()
	b.Make(rules.Cell(119))
	b.Make(rules.Cell(60))
	mv, nodes, kind, pvLen, ok := treeSolverPort(b, 1<<14)
	if ok {
		t.Fatalf("quiet two-stone board proved %s move %d pv %d, want no proof", kind, mv, pvLen)
	}
	if nodes == 0 {
		t.Error("solver must report spent nodes even without a proof")
	}
}

func TestTreeLogFingerprintPinsEngineVersion(t *testing.T) {
	p := paramsFromConfig(config.TierMaster.Name)
	if !strings.Contains(p.fingerprint(8), "leaf=1048576") {
		t.Error("fingerprint must pin the leaf budget")
	}
}
