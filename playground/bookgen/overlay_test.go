package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func writeCorpusWithVisits(t *testing.T, path string, visits map[[config.BookKeyBytes]byte]int, baseMove int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir corpus dir: %v", err)
	}
	idx := 0
	clean := seedHeader(t, path, corpusFingerprint(config.TierEasy.Name, 0, len(visits)))
	for k, n := range visits {
		recs := make([]posRec, n)
		for i := range recs {
			recs[i] = posRec{key: k, move: rules.Move(baseMove)}
		}
		next, err := appendGame(path, clean, idx, recs, 0)
		if err != nil {
			t.Fatalf("appendGame %d: %v", idx, err)
		}
		clean = next
		idx++
	}
}

func TestMajorityPicksMostPlayedSmallestOnTie(t *testing.T) {
	mv, n := majorityOf(map[rules.Move]int{5: 3, 12: 3, 7: 1})
	if mv != 5 || n != 3 {
		t.Errorf("majority of tie = %d/%d, want smallest tied move 5 at 3", mv, n)
	}
	mv, n = majorityOf(map[rules.Move]int{7: 2, 3: 2})
	if mv != 3 || n != 2 {
		t.Errorf("majority of pair tie = %d/%d, want 3 at 2", mv, n)
	}
	mv, n = majorityOf(map[rules.Move]int{200: 1})
	if mv != 200 || n != 1 {
		t.Errorf("majority of single = %d/%d, want 200 at 1", mv, n)
	}
}

func TestOverlayMergesVisitsAcrossGames(t *testing.T) {
	k := singleStoneKey(t, 9)
	dir := t.TempDir()
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 3))
	for g := range 3 {
		next, err := appendGame(corpus, clean, g, []posRec{{key: k, move: 42}}, 0)
		if err != nil {
			t.Fatalf("appendGame %d: %v", g, err)
		}
		clean = next
	}
	sc, err := loadCorpus(corpus)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	agg := aggregate(sc.recs)
	if n := agg[k][42]; n != 3 {
		t.Errorf("merged visits for the transposed key = %d, want 3", n)
	}
}

func TestOverlayRecordsOnlyFloorPassingProofs(t *testing.T) {
	dir := t.TempDir()
	strong := singleStoneKey(t, 4)
	weak := singleStoneKey(t, 5)
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{
		strong: config.BookVisitFloor,
		weak:   config.BookVisitFloor - 1,
	}, 200)
	out := filepath.Join(dir, "o.txt")
	var probed []int
	probe := func(b *rules.Board) (rules.Move, uint64, string, bool) {
		k := keyOf(b)
		probed = append(probed, int(k[0]))
		return 11, 42, "vcf", true
	}
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, probe, io.Discard); err != nil {
		t.Fatalf("overlayRun: %v", err)
	}
	if len(probed) != 1 {
		t.Fatalf("probed %d positions, want only the floor-passing one", len(probed))
	}
	osc, err := readOverlay(out)
	if err != nil {
		t.Fatalf("readOverlay: %v", err)
	}
	if len(osc.done) != 1 {
		t.Errorf("processed set = %d keys, want 1", len(osc.done))
	}
	p, ok := osc.proofs[strong]
	if !ok {
		t.Fatalf("no proof recorded for the floor-passing position")
	}
	if p.move != 11 || p.nodes != 42 || p.kind != "vcf" {
		t.Errorf("proof = %+v, want move 11 nodes 42 kind vcf", p)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}
	want := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 2), config.BookVisitFloor, 0) + "\nP " + hexKey(strong[:]) + " 11 42 vcf\n"
	if string(data) != want {
		t.Errorf("overlay file = %q, want %q", string(data), want)
	}
}

func TestOverlayResumeSkipsProcessedPositions(t *testing.T) {
	dir := t.TempDir()
	keys := [][config.BookKeyBytes]byte{singleStoneKey(t, 1), singleStoneKey(t, 2), singleStoneKey(t, 3)}
	corpus := filepath.Join(dir, "c.txt")
	visits := map[[config.BookKeyBytes]byte]int{}
	for _, k := range keys {
		visits[k] = config.BookVisitFloor
	}
	writeCorpusWithVisits(t, corpus, visits, 200)
	out := filepath.Join(dir, "o.txt")
	calls := 0
	flaky := func(b *rules.Board) (rules.Move, uint64, string, bool) {
		calls++
		if calls == 1 {
			return 7, 9, "vct", true
		}
		return 0, 100, "", false
	}
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, flaky, io.Discard); err != nil {
		t.Fatalf("overlayRun first: %v", err)
	}
	if calls != 3 {
		t.Fatalf("first overlay probed %d positions, want 3", calls)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}
	if lines := strings.Count(string(data), "\n"); lines != 4 {
		t.Fatalf("overlay file holds %d lines, want 4 (fingerprint, 1 proof, 2 no-proof)", lines)
	}
	always := func(b *rules.Board) (rules.Move, uint64, string, bool) {
		calls++
		return 8, 8, "vcf", true
	}
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, always, io.Discard); err != nil {
		t.Fatalf("overlayRun resume: %v", err)
	}
	if calls != 3 {
		t.Errorf("resume probed again, calls = %d, want to stay at 3", calls)
	}
	osc, err := readOverlay(out)
	if err != nil {
		t.Fatalf("readOverlay: %v", err)
	}
	if p := osc.proofs[keysSortedAscending(t, keys)[0]]; p.move != 7 || p.kind != "vct" {
		t.Errorf("resume rewrote the first proof: %+v", p)
	}
	if len(osc.proofs) != 1 {
		t.Errorf("resume promoted no-proof lines, proofs = %d, want 1", len(osc.proofs))
	}
}

func keysSortedAscending(t *testing.T, keys [][config.BookKeyBytes]byte) [][config.BookKeyBytes]byte {
	t.Helper()
	out := append([][config.BookKeyBytes]byte(nil), keys...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && keyLess(out[j], out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func keyLess(a, b [config.BookKeyBytes]byte) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}

func forcedWinBoard(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, name := range []string{"H4", "A1", "H5", "A2", "H6", "A3", "H7", "A16"} {
		c, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		b.Make(c)
	}
	return b
}

func TestSolverProberPinsACompletingMove(t *testing.T) {
	b := forcedWinBoard(t)
	probe := solverProber(config.BookSolverNodeBudget)
	mv, nodes, kind, ok := probe(b)
	if !ok {
		t.Fatalf("solver prober found no forced win on a four-in-a-row board")
	}
	if kind != "vcf" {
		t.Errorf("proof kind = %q, want vcf before vct", kind)
	}
	if int(mv) >= config.BoardCells {
		t.Fatalf("proof move %d outside the board", mv)
	}
	if b.At(rules.Cell(mv)) != rules.Empty {
		t.Fatalf("proof move %d lands on an occupied cell", mv)
	}
	after := *b
	after.Make(rules.Cell(mv))
	if !after.FastLastMoveWin(rules.Red, rules.Cell(mv)) {
		t.Errorf("proof move %d does not complete five, want a win-in-1 cell", mv)
	}
	if nodes == 0 {
		t.Errorf("proof nodes = 0, want the recorded node count")
	}
}

func TestOverlayProvesAndRecordsTheRealSolverVerdict(t *testing.T) {
	dir := t.TempDir()
	k := keyOf(forcedWinBoard(t))
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k: config.BookVisitFloor}, 200)
	out := filepath.Join(dir, "o.txt")
	var progress strings.Builder
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor, budget: config.BookSolverNodeBudget}, solverProber(config.BookSolverNodeBudget), &progress); err != nil {
		t.Fatalf("overlayRun: %v", err)
	}
	osc, err := readOverlay(out)
	if err != nil {
		t.Fatalf("readOverlay: %v", err)
	}
	p, ok := osc.proofs[k]
	if !ok {
		t.Fatalf("the forced-win position carries no proof")
	}
	after := *forcedWinBoard(t)
	after.Make(rules.Cell(p.move))
	if !after.FastLastMoveWin(rules.Red, rules.Cell(p.move)) {
		t.Errorf("recorded proof move %d does not win, kind %s nodes %d", p.move, p.kind, p.nodes)
	}
	if !strings.Contains(progress.String(), "proof") {
		t.Errorf("progress log misses the proof line: %q", progress.String())
	}
}

func TestOverlayQuietPositionExhaustsBudgetWithoutProof(t *testing.T) {
	dir := t.TempDir()
	start := keyOf(rules.NewBoard())
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{start: config.BookVisitFloor}, 119)
	out := filepath.Join(dir, "o.txt")
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor, budget: 1 << 10}, solverProber(1<<10), io.Discard); err != nil {
		t.Fatalf("overlayRun: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}
	want := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, 1<<10) + "\nN " + hexKey(start[:]) + "\n"
	if string(data) != want {
		t.Errorf("overlay file = %q, want %q", string(data), want)
	}
}

func TestOverlayTornTailIsDroppedAndTruncatedOnResume(t *testing.T) {
	dir := t.TempDir()
	strong := singleStoneKey(t, 4)
	weak := singleStoneKey(t, 6)
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{
		strong: config.BookVisitFloor,
		weak:   config.BookVisitFloor,
	}, 200)
	out := filepath.Join(dir, "o.txt")
	fp := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 2), config.BookVisitFloor, 0)
	good := "P " + hexKey(strong[:]) + " 11 42 vcf\n"
	tear := "N " + strings.Repeat("c", 2*config.BookKeyBytes-4)
	if err := os.WriteFile(out, []byte(fp+"\n"+good+tear), 0o644); err != nil {
		t.Fatalf("seed overlay: %v", err)
	}
	osc, err := readOverlay(out)
	if err != nil {
		t.Fatalf("readOverlay torn: %v", err)
	}
	cleanLine := int64(len(fp) + 1 + len(good))
	if len(osc.proofs) != 1 || len(osc.done) != 1 || !osc.torn || osc.clean != cleanLine {
		t.Fatalf("torn scan = %d proofs %d done clean %d torn %t, want the good line only at %d", len(osc.proofs), len(osc.done), osc.clean, osc.torn, cleanLine)
	}
	calls := 0
	probe := func(b *rules.Board) (rules.Move, uint64, string, bool) {
		calls++
		return 8, 8, "vct", true
	}
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, probe, io.Discard); err != nil {
		t.Fatalf("overlayRun resume: %v", err)
	}
	if calls != 1 {
		t.Fatalf("resume probed %d positions, want only the torn key re-probed", calls)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reread overlay: %v", err)
	}
	want := fp + "\n" + good + "P " + hexKey(weak[:]) + " 8 8 vct\n"
	if string(data) != want {
		t.Fatalf("overlay after resume = %q, want %q", string(data), want)
	}
}

func TestOverlaySkipsBlankLinesAndRejectsMalformed(t *testing.T) {
	dir := t.TempDir()
	k := singleStoneKey(t, 4)
	fp := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, 0)
	good := "P " + hexKey(k[:]) + " 11 42 vcf\n"
	path := filepath.Join(dir, "o.txt")
	if err := os.WriteFile(path, []byte(fp+"\n"+good+"\n"+good), 0o644); err != nil {
		t.Fatalf("write blank-separated overlay: %v", err)
	}
	osc, err := readOverlay(path)
	if err != nil {
		t.Fatalf("readOverlay: %v", err)
	}
	if len(osc.proofs) != 1 || len(osc.done) != 1 || osc.torn || osc.clean != int64(len(fp)+1+len(good)*2+1) {
		t.Fatalf("scan = %d/%d clean %d torn %t, want both good lines", len(osc.proofs), len(osc.done), osc.clean, osc.torn)
	}
	if err := os.WriteFile(path, []byte(fp+"\nQ x\n"), 0o644); err != nil {
		t.Fatalf("write malformed: %v", err)
	}
	if _, err := readOverlay(path); err == nil {
		t.Errorf("readOverlay accepted an unknown line kind")
	}
}

func TestOverlayRecordsNoProofForAnIllegalProofMove(t *testing.T) {
	dir := t.TempDir()
	anchored := rules.NewBoard()
	anchored.Make(rules.Cell(119))
	anchored.Make(rules.Cell(0))
	k := keyOf(anchored)
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k: config.BookVisitFloor}, 200)
	out := filepath.Join(dir, "o.txt")
	probe := func(b *rules.Board) (rules.Move, uint64, string, bool) {
		return 135, 5, "vcf", true
	}
	var progress strings.Builder
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, probe, &progress); err != nil {
		t.Fatalf("overlayRun: %v", err)
	}
	if !strings.Contains(progress.String(), "proof move 135 illegal in its position") {
		t.Fatalf("progress misses the refused proof: %q", progress.String())
	}
	osc, err := readOverlay(out)
	if err != nil {
		t.Fatalf("readOverlay: %v", err)
	}
	if len(osc.proofs) != 0 {
		t.Fatalf("illegal proof recorded: %+v", osc.proofs)
	}
	if _, ok := osc.done[k]; !ok {
		t.Fatalf("the position was not marked processed, resume would re-probe it")
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read overlay: %v", err)
	}
	want := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, 0) + "\nN " + hexKey(k[:]) + "\n"
	if string(data) != want {
		t.Fatalf("overlay file = %q, want the no-proof verdict %q", string(data), want)
	}
}

func TestOverlayRecoversATornHeaderWithoutItsNewline(t *testing.T) {
	dir := t.TempDir()
	k := singleStoneKey(t, 4)
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k: config.BookVisitFloor}, 200)
	out := filepath.Join(dir, "o.txt")
	fp := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, 0)
	if err := os.WriteFile(out, []byte(fp), 0o644); err != nil {
		t.Fatalf("write torn header: %v", err)
	}
	probe := func(b *rules.Board) (rules.Move, uint64, string, bool) { return 8, 8, "vcf", true }
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, probe, io.Discard); err != nil {
		t.Fatalf("overlayRun over a header that lost its newline: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	want := fp + "\nP " + hexKey(k[:]) + " 8 8 vcf\n"
	if string(data) != want {
		t.Fatalf("recovered overlay = %q, want %q", data, want)
	}
	osc, err := readOverlay(out)
	if err != nil {
		t.Fatalf("readOverlay after recovery: %v", err)
	}
	if len(osc.done) != 1 || len(osc.proofs) != 1 || osc.torn || osc.fp != fp || osc.clean != int64(len(want)) {
		t.Fatalf("recovered overlay scan = %d proofs %d done clean %d torn %t, want the rewritten header at %d", len(osc.proofs), len(osc.done), osc.clean, osc.torn, len(want))
	}
	calls := 0
	reprobe := func(b *rules.Board) (rules.Move, uint64, string, bool) {
		calls++
		return 8, 8, "vcf", true
	}
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, reprobe, io.Discard); err != nil {
		t.Fatalf("rerun after recovery: %v", err)
	}
	if calls != 0 {
		t.Fatalf("recovered overlay re-probed %d positions", calls)
	}
}

func TestOverlayRefusesProofsFromADifferentCorpus(t *testing.T) {
	dir := t.TempDir()
	k1 := singleStoneKey(t, 4)
	k2 := singleStoneKey(t, 5)
	corpus := filepath.Join(dir, "c.txt")
	out := filepath.Join(dir, "o.txt")
	probe := func(b *rules.Board) (rules.Move, uint64, string, bool) { return 8, 8, "vcf", true }
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k1: config.BookVisitFloor}, 200)
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, probe, io.Discard); err != nil {
		t.Fatalf("overlayRun first: %v", err)
	}
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k1: config.BookVisitFloor, k2: config.BookVisitFloor}, 200)
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, probe, io.Discard); err == nil {
		t.Fatalf("overlayRun reused proofs pinned to a regrown corpus at the same floor and budget")
	}
}

func TestOverlayRefusesProofsFromASolverUpgrade(t *testing.T) {
	dir := t.TempDir()
	k := singleStoneKey(t, 4)
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k: config.BookVisitFloor}, 200)
	upgraded := fmt.Sprintf("F overlay v%d solver=%d corpus=%s floor=%d budget=%d",
		config.BookSidecarVersion, config.BookSolverIdentity+1, corpusTag(corpusFingerprint(config.TierEasy.Name, 0, 1)), config.BookVisitFloor, 0)
	out := filepath.Join(dir, "o.txt")
	if err := os.WriteFile(out, []byte(upgraded+"\nP "+hexKey(k[:])+" 8 8 vcf\n"), 0o644); err != nil {
		t.Fatalf("write upgraded overlay: %v", err)
	}
	probe := func(b *rules.Board) (rules.Move, uint64, string, bool) { return 8, 8, "vcf", true }
	if err := overlayRun(overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor}, probe, io.Discard); err == nil {
		t.Fatalf("overlayRun accepted proofs minted under a different solver identity")
	}
}

func TestOverlayResumeRejectsConfigChange(t *testing.T) {
	dir := t.TempDir()
	k := singleStoneKey(t, 4)
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k: config.BookVisitFloor}, 200)
	out := filepath.Join(dir, "o.txt")
	probe := func(b *rules.Board) (rules.Move, uint64, string, bool) { return 8, 8, "vcf", true }
	opts := overlayOpts{corpusPath: corpus, outPath: out, floor: config.BookVisitFloor, budget: config.BookSolverNodeBudget}
	if err := overlayRun(opts, probe, io.Discard); err != nil {
		t.Fatalf("overlayRun: %v", err)
	}
	opts.floor = config.BookVisitFloor + 1
	if err := overlayRun(opts, probe, io.Discard); err == nil {
		t.Errorf("overlayRun accepted a changed floor over the recorded fingerprint")
	}
	opts.floor = config.BookVisitFloor
	opts.budget = config.BookSolverNodeBudget / 2
	if err := overlayRun(opts, probe, io.Discard); err == nil {
		t.Errorf("overlayRun accepted a changed budget over the recorded fingerprint")
	}
}
