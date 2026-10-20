package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func seedCorpusCounts(t *testing.T, dir string, perPos []map[rules.Move]int) string {
	t.Helper()
	path := filepath.Join(dir, "corpus.txt")
	clean := seedHeader(t, path, corpusFingerprint(config.TierEasy.Name, 0, len(perPos)))
	for i, counts := range perPos {
		var recs []posRec
		for mv, n := range counts {
			var key [config.BookKeyBytes]byte
			binary.LittleEndian.PutUint64(key[:], uint64(i+1))
			for range n {
				recs = append(recs, posRec{key: key, move: mv})
			}
		}
		next, err := appendGame(path, clean, i, recs, 0)
		if err != nil {
			t.Fatalf("appendGame %d: %v", i, err)
		}
		clean = next
	}
	return path
}

func decodeEntry(t *testing.T, data []byte, i int) bookEntry {
	t.Helper()
	off := len(config.BookMagicLine)
	firstMeta := bytes.IndexByte(data[off:], '\n')
	metaEnd := off + firstMeta + 1
	base := metaEnd + i*config.BookEntryBytes
	if base+config.BookEntryBytes > len(data) {
		t.Fatalf("entry %d out of range", i)
	}
	var e bookEntry
	copy(e.key[:], data[base:base+config.BookKeyBytes])
	e.move = rules.Move(binary.LittleEndian.Uint16(data[base+config.BookKeyBytes:]))
	e.visits = binary.LittleEndian.Uint32(data[base+config.BookKeyBytes+config.BookMoveBytes:])
	e.source = data[base+config.BookKeyBytes+config.BookMoveBytes+config.BookVisitsBytes]
	for _, b := range data[base+config.BookKeyBytes+config.BookMoveBytes+config.BookVisitsBytes+config.BookSourceBytes : base+config.BookEntryBytes] {
		if b != 0 {
			t.Fatalf("entry %d carries a nonzero reserved byte", i)
		}
	}
	return e
}

func runEmitFor(t *testing.T, perPos []map[rules.Move]int, proofs map[int]proofRec, floor int) (string, string, []byte, emitStats) {
	t.Helper()
	dir := t.TempDir()
	corpus := seedCorpusCounts(t, dir, perPos)
	overlay := filepath.Join(dir, "overlay.txt")
	if len(proofs) > 0 {
		seedHeader(t, overlay, overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, len(perPos)), config.BookVisitFloor, config.BookSolverNodeBudget))
	}
	for i, p := range proofs {
		var key [config.BookKeyBytes]byte
		binary.LittleEndian.PutUint64(key[:], uint64(i+1))
		line := "N " + hexKey(key[:]) + "\n"
		if p.move != 0 || p.nodes != 0 || p.kind != "" {
			line = "P " + hexKey(key[:]) + " " + strconv.Itoa(int(p.move)) + " " + strconv.FormatUint(p.nodes, 10) + " " + p.kind + "\n"
		}
		f, err := os.OpenFile(overlay, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			t.Fatalf("open overlay: %v", err)
		}
		if _, err := f.WriteString(line); err != nil {
			t.Fatalf("write overlay: %v", err)
		}
		f.Close()
	}
	out := filepath.Join(dir, "book.carobook")
	statsPath := filepath.Join(dir, "stats.txt")
	st, err := emitRun(emitOpts{
		corpusPath:  corpus,
		overlayPath: overlay,
		outPath:     out,
		statsPath:   statsPath,
		floor:       floor,
		created:     "2026-10-18T00:00:00Z",
	}, io.Discard)
	if err != nil {
		t.Fatalf("emitRun: %v", err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	return out, statsPath, data, st
}

func TestEmitAppliesTheHybridSourceLaw(t *testing.T) {
	perPos := []map[rules.Move]int{
		{100: 21, 7: 1},
		{50: config.BookVisitFloor},
		{60: config.BookVisitFloor - 1},
	}
	proofs := map[int]proofRec{0: {move: 7, nodes: 5000, kind: "vct"}}
	_, _, data, st := runEmitFor(t, perPos, proofs, config.BookVisitFloor)
	if st.Entries != 2 {
		t.Fatalf("stats entries = %d, want the 2 floor-passing positions", st.Entries)
	}
	first := decodeEntry(t, data, 0)
	second := decodeEntry(t, data, 1)
	var keyA, keyB [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(keyA[:], 1)
	binary.LittleEndian.PutUint64(keyB[:], 2)
	var solverEntry, majorityEntry bookEntry
	if first.key == keyA {
		solverEntry, majorityEntry = first, second
	} else {
		solverEntry, majorityEntry = second, first
	}
	if solverEntry.key != keyA {
		t.Fatalf("floor-failing or wrong positions emitted")
	}
	if solverEntry.move != 7 || solverEntry.source != config.BookSourceSolver || solverEntry.visits != 1 {
		t.Errorf("solver entry = move %d source %d visits %d, want the proof move 7 source %d visits 1",
			solverEntry.move, solverEntry.source, solverEntry.visits, config.BookSourceSolver)
	}
	if majorityEntry.key != keyB || majorityEntry.move != 50 || majorityEntry.source != config.BookSourceMajority || majorityEntry.visits != config.BookVisitFloor {
		t.Errorf("majority entry = key match %t move %d source %d visits %d, want move 50 source %d visits %d",
			majorityEntry.key == keyB, majorityEntry.move, majorityEntry.source, majorityEntry.visits,
			config.BookSourceMajority, config.BookVisitFloor)
	}
	if st.Solver != 1 || st.Majority != 1 || st.SolverVCT != 1 || st.SolverVCF != 0 {
		t.Errorf("stats split = solver %d majority %d vcf %d vct %d, want 1/1/0/1", st.Solver, st.Majority, st.SolverVCF, st.SolverVCT)
	}
	if st.SolverNodesTotal != 5000 || st.SolverNodesMax != 5000 {
		t.Errorf("solver node stats = total %d max %d, want 5000/5000", st.SolverNodesTotal, st.SolverNodesMax)
	}
}

func TestEmitMetaAndMagicMatchContract(t *testing.T) {
	perPos := []map[rules.Move]int{{9: config.BookVisitFloor}}
	_, _, data, _ := runEmitFor(t, perPos, nil, config.BookVisitFloor)
	if !bytes.HasPrefix(data, []byte(config.BookMagicLine)) {
		t.Fatalf("artifact misses the magic line")
	}
	rest := data[len(config.BookMagicLine):]
	nl := bytes.IndexByte(rest, '\n')
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(rest[:nl], &fields); err != nil {
		t.Fatalf("meta line is not JSON: %v", err)
	}
	wantKeys := map[string]string{
		"version":      strconv.Itoa(config.BookFormatVersion),
		"max_ply":      strconv.Itoa(config.BookMaxPly),
		"generator":    "\"" + config.BookGeneratorTag + "\"",
		"corpus_games": "1",
		"visit_floor":  strconv.Itoa(config.BookVisitFloor),
		"created":      "\"2026-10-18T00:00:00Z\"",
	}
	if len(fields) != len(config.BookMetaFields) {
		t.Errorf("meta carries %d fields, want the contract's %d", len(fields), len(config.BookMetaFields))
	}
	for k, want := range wantKeys {
		got, ok := fields[k]
		if !ok || string(got) != want {
			t.Errorf("meta %s = %s, want %s", k, string(got), want)
		}
	}
}

func TestEmitArtifactLoadsAndProbesBack(t *testing.T) {
	dir := t.TempDir()
	b1 := rules.NewBoard()
	for _, name := range []string{"H8", "P8", "H9"} {
		c, _ := rules.ParseCell(name)
		b1.Make(c)
	}
	b2 := rules.NewBoard()
	for _, name := range []string{"H8", "J9", "H9"} {
		c, _ := rules.ParseCell(name)
		b2.Make(c)
	}
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 2))
	next, err := appendGame(corpus, clean, 0, repeat(posRec{key: keyOf(b1), move: 42}, config.BookVisitFloor), 0)
	if err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	clean = next
	next, err = appendGame(corpus, clean, 1, repeat(posRec{key: keyOf(b2), move: 43}, config.BookVisitFloor), 0)
	if err != nil {
		t.Fatalf("appendGame 2: %v", err)
	}
	overlay := filepath.Join(dir, "o.txt")
	out := filepath.Join(dir, "b.carobook")
	if _, err := emitRun(emitOpts{
		corpusPath: corpus, overlayPath: overlay, outPath: out,
		statsPath: filepath.Join(dir, "s.txt"), floor: config.BookVisitFloor,
		created: "2026-10-18T00:00:00Z",
	}, io.Discard); err != nil {
		t.Fatalf("emitRun: %v", err)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open artifact: %v", err)
	}
	defer func() { _ = f.Close() }()
	bk, err := book.Load(f)
	if err != nil {
		t.Fatalf("book.Load rejected the emitted artifact: %v", err)
	}
	if bk.Len() != 2 {
		t.Fatalf("artifact holds %d entries, want 2", bk.Len())
	}
	for board, want := range map[*rules.Board]rules.Move{b1: 42, b2: 43} {
		got, ok := bk.Probe(board)
		if !ok || got != want {
			t.Errorf("probe = %d/%t, want exactly %d", got, ok, want)
		}
	}
	other := rules.NewBoard()
	other.Make(rules.Cell(0))
	if _, ok := bk.Probe(other); ok {
		t.Errorf("probe hit an unlisted position")
	}
}

func repeat(r posRec, n int) []posRec {
	out := make([]posRec, n)
	for i := range out {
		out[i] = r
	}
	return out
}

func TestEmitStatsFileCarriesTheSummary(t *testing.T) {
	perPos := []map[rules.Move]int{
		{10: config.BookVisitFloor + 4},
		{11: config.BookVisitFloor},
		{12: 1},
	}
	_, statsPath, _, _ := runEmitFor(t, perPos, nil, config.BookVisitFloor)
	data, err := os.ReadFile(statsPath)
	if err != nil {
		t.Fatalf("read stats: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := map[string]string{
		"corpus_games":       "3",
		"positions":          "3",
		"floor_passing":      "2",
		"entries":            "2",
		"entries_majority":   "2",
		"entries_solver":     "0",
		"solver_vcf":         "0",
		"solver_vct":         "0",
		"visits_min":         strconv.Itoa(config.BookVisitFloor),
		"visits_max":         strconv.Itoa(config.BookVisitFloor + 4),
		"visits_median":      strconv.Itoa(config.BookVisitFloor + 2),
		"solver_nodes_total": "0",
		"solver_nodes_max":   "0",
	}
	for _, l := range lines {
		parts := strings.Fields(l)
		if len(parts) != 2 {
			t.Fatalf("stats line %q is not key value", l)
		}
		if want[parts[0]] != "" && parts[1] != want[parts[0]] {
			t.Errorf("stats %s = %s, want %s", parts[0], parts[1], want[parts[0]])
		}
	}
	if len(lines) < len(want) {
		t.Errorf("stats file holds %d lines, want at least %d", len(lines), len(want))
	}
}

func TestEmitStatsReportTrunkDiversity(t *testing.T) {
	dir := t.TempDir()
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 2))
	rotated := []posRec{
		{key: corpusZeroKey, move: openingMove(0)},
		{key: corpusZeroKey, move: openingMove(1)},
	}
	next, err := appendGame(corpus, clean, 0, rotated[:1], 0)
	if err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	if _, err := appendGame(corpus, next, 1, rotated[1:], 0); err != nil {
		t.Fatalf("appendGame 2: %v", err)
	}
	statsPath := filepath.Join(dir, "s.txt")
	st, err := emitRun(emitOpts{
		corpusPath: corpus, overlayPath: filepath.Join(dir, "o.txt"),
		outPath: filepath.Join(dir, "b.carobook"), statsPath: statsPath,
		floor: 1, created: "2026-10-18T00:00:00Z",
	}, io.Discard)
	if err != nil {
		t.Fatalf("emitRun rotated: %v", err)
	}
	if st.Trunks != 2 || st.DiversityAlarm {
		t.Fatalf("rotated stats = trunks %d alarm %t, want 2 trunks and no alarm", st.Trunks, st.DiversityAlarm)
	}
	collapsed := filepath.Join(dir, "c2.txt")
	clean = seedHeader(t, collapsed, corpusFingerprint(config.TierEasy.Name, 0, 2))
	cnext, err := appendGame(collapsed, clean, 0, rotated[:1], 1)
	if err != nil {
		t.Fatalf("appendGame collapsed 1: %v", err)
	}
	if _, err := appendGame(collapsed, cnext, 1, rotated[:1], 1); err != nil {
		t.Fatalf("appendGame collapsed 2: %v", err)
	}
	st, err = emitRun(emitOpts{
		corpusPath: collapsed, overlayPath: filepath.Join(dir, "o.txt"),
		outPath: filepath.Join(dir, "b2.carobook"), statsPath: filepath.Join(dir, "s2.txt"),
		floor: 1, created: "2026-10-18T00:00:00Z",
	}, io.Discard)
	if err != nil {
		t.Fatalf("emitRun collapsed: %v", err)
	}
	if st.Trunks != 1 || !st.DiversityAlarm {
		t.Fatalf("collapsed stats = trunks %d alarm %t, want 1 trunk and the alarm", st.Trunks, st.DiversityAlarm)
	}
	if st.IllegalFallbacks != 2 {
		t.Fatalf("collapsed stats illegal fallbacks = %d, want the 2 recorded per game", st.IllegalFallbacks)
	}
	data, err := os.ReadFile(filepath.Join(dir, "s2.txt"))
	if err != nil {
		t.Fatalf("read stats: %v", err)
	}
	if !strings.Contains(string(data), "diversity_alarm true") {
		t.Fatalf("stats file misses the alarm line: %q", data)
	}
}

func TestEmitIsAtomicOverThePreviousArtifact(t *testing.T) {
	dir := t.TempDir()
	k := singleStoneKey(t, 4)
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 1))
	if _, err := appendGame(corpus, clean, 0, repeat(posRec{key: k, move: 42}, config.BookVisitFloor), 0); err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	overlay := filepath.Join(dir, "o.txt")
	seedHeader(t, overlay, overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, config.BookSolverNodeBudget))
	out := filepath.Join(dir, "b.carobook")
	stats := filepath.Join(dir, "s.txt")
	opts := emitOpts{
		corpusPath: corpus, overlayPath: overlay, outPath: out, statsPath: stats,
		floor: config.BookVisitFloor, created: "2026-10-18T00:00:00Z",
	}
	if _, err := emitRun(opts, io.Discard); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	good, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	occupied := "P " + hexKey(k[:]) + " " + strconv.Itoa(int(4)) + " 5 vcf\n"
	if err := os.WriteFile(overlay, []byte(overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, config.BookSolverNodeBudget)+"\n"+occupied), 0o644); err != nil {
		t.Fatalf("poison overlay: %v", err)
	}
	if _, err := emitRun(opts, io.Discard); err == nil {
		t.Fatalf("second emit accepted an occupied proof move")
	}
	after, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("reread artifact: %v", err)
	}
	if !bytes.Equal(after, good) {
		t.Fatalf("failed emit replaced the artifact: %d bytes over %d", len(after), len(good))
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, "*.tmp*"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("emit left temp files %v (err %v), want none", leftovers, err)
	}
}

func TestEmitFailedArtifactPublishLeavesTheStatsUntouched(t *testing.T) {
	dir := t.TempDir()
	strong := singleStoneKey(t, 4)
	weak := singleStoneKey(t, 5)
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 2))
	next, err := appendGame(corpus, clean, 0, repeat(posRec{key: strong, move: 42}, config.BookVisitFloor+4), 0)
	if err != nil {
		t.Fatalf("appendGame 0: %v", err)
	}
	if _, err := appendGame(corpus, next, 1, repeat(posRec{key: weak, move: 43}, config.BookVisitFloor), 0); err != nil {
		t.Fatalf("appendGame 1: %v", err)
	}
	overlay := filepath.Join(dir, "o.txt")
	stats := filepath.Join(dir, "s.txt")
	first := emitOpts{
		corpusPath: corpus, overlayPath: overlay,
		outPath: filepath.Join(dir, "b.carobook"), statsPath: stats,
		floor: config.BookVisitFloor, created: "2026-10-18T00:00:00Z",
	}
	if _, err := emitRun(first, io.Discard); err != nil {
		t.Fatalf("first emit: %v", err)
	}
	good, err := os.ReadFile(stats)
	if err != nil {
		t.Fatalf("read stats: %v", err)
	}
	broken := first
	broken.floor = config.BookVisitFloor + 3
	broken.outPath = filepath.Join(dir, "gone", "b.carobook")
	if _, err := emitRun(broken, io.Discard); err == nil {
		t.Fatalf("emitRun published an artifact into a missing directory")
	}
	after, err := os.ReadFile(stats)
	if err != nil {
		t.Fatalf("reread stats: %v", err)
	}
	if !bytes.Equal(after, good) {
		t.Fatalf("failed artifact publish rewrote stats: %q over %q", after, good)
	}
}

func TestEmitRefusesAZeroEntryArtifact(t *testing.T) {
	dir := t.TempDir()
	k := singleStoneKey(t, 4)
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 1))
	if _, err := appendGame(corpus, clean, 0, repeat(posRec{key: k, move: 42}, config.BookVisitFloor), 0); err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	out := filepath.Join(dir, "b.carobook")
	if _, err := emitRun(emitOpts{
		corpusPath: corpus, overlayPath: filepath.Join(dir, "o.txt"),
		outPath: out, statsPath: filepath.Join(dir, "s.txt"),
		floor: config.BookVisitFloor * 10, created: "2026-10-18T00:00:00Z",
	}, io.Discard); err == nil {
		t.Fatalf("emitRun accepted a zero-entry artifact")
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("refused emit still touched the artifact path: %v", err)
	}
}

func TestEmitRejectsOpeningRuleViolation(t *testing.T) {
	dir := t.TempDir()
	b := rules.NewBoard()
	b.Make(rules.Cell(119))
	b.Make(rules.Cell(0))
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 1))
	if _, err := appendGame(corpus, clean, 0, repeat(posRec{key: keyOf(b), move: 135}, config.BookVisitFloor), 0); err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	_, err := emitRun(emitOpts{
		corpusPath: corpus, overlayPath: filepath.Join(dir, "o.txt"),
		outPath: filepath.Join(dir, "b.carobook"), statsPath: filepath.Join(dir, "s.txt"),
		floor: config.BookVisitFloor, created: "2026-10-18T00:00:00Z",
	}, io.Discard)
	if err == nil {
		t.Fatalf("emitRun accepted move 135 one row from the lone red anchor, inside the opening exclusion")
	}
}

func TestEmitSolverVisitsCarryTheCorpusCountEvenAtZero(t *testing.T) {
	perPos := []map[rules.Move]int{{100: config.BookVisitFloor}}
	proofs := map[int]proofRec{0: {move: 7, nodes: 42, kind: "vcf"}}
	out, _, data, st := runEmitFor(t, perPos, proofs, config.BookVisitFloor)
	if st.Solver != 1 {
		t.Fatalf("stats solver = %d, want the proof entry", st.Solver)
	}
	entry := decodeEntry(t, data, 0)
	if entry.source != config.BookSourceSolver || entry.move != 7 {
		t.Fatalf("solver entry = move %d source %d, want the proof move 7", entry.move, entry.source)
	}
	if entry.visits != 0 {
		t.Fatalf("solver entry visits = %d, want the corpus count of the proof move, zero when never played", entry.visits)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("open artifact: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := book.Load(f); err != nil {
		t.Fatalf("book.Load rejected a zero-visit solver entry: %v", err)
	}
}

func TestEmitRefusesStaleOverlayProofs(t *testing.T) {
	dir := t.TempDir()
	k := singleStoneKey(t, 4)
	corpus := filepath.Join(dir, "c.txt")
	writeCorpusWithVisits(t, corpus, map[[config.BookKeyBytes]byte]int{k: config.BookVisitFloor}, 200)
	overlay := filepath.Join(dir, "o.txt")
	legacy := fmt.Sprintf("F overlay v%d floor=%d budget=%d", config.BookSidecarVersion, config.BookVisitFloor, config.BookSolverNodeBudget)
	otherCorpus := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 2), config.BookVisitFloor, config.BookSolverNodeBudget)
	staleLines := []string{legacy, otherCorpus}
	for _, stale := range staleLines {
		if err := os.WriteFile(overlay, []byte(stale+"\nP "+hexKey(k[:])+" 7 5 vcf\n"), 0o644); err != nil {
			t.Fatalf("write stale overlay: %v", err)
		}
		_, err := emitRun(emitOpts{
			corpusPath: corpus, overlayPath: overlay,
			outPath: filepath.Join(dir, "b.carobook"), statsPath: filepath.Join(dir, "s.txt"),
			floor: config.BookVisitFloor, created: "2026-10-18T00:00:00Z",
		}, io.Discard)
		if err == nil {
			t.Fatalf("emitRun applied the stale overlay %q", stale)
		}
	}
}

func TestEmitRejectsOccupiedProofMove(t *testing.T) {
	dir := t.TempDir()
	b := rules.NewBoard()
	b.Make(rules.Cell(30))
	corpus := filepath.Join(dir, "c.txt")
	clean := seedHeader(t, corpus, corpusFingerprint(config.TierEasy.Name, 0, 1))
	if _, err := appendGame(corpus, clean, 0, repeat(posRec{key: keyOf(b), move: 42}, config.BookVisitFloor), 0); err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	overlay := filepath.Join(dir, "o.txt")
	occupied := rules.Move(30)
	kb := keyOf(b)
	line := overlayFingerprint(corpusFingerprint(config.TierEasy.Name, 0, 1), config.BookVisitFloor, config.BookSolverNodeBudget) + "\nP " + hexKey(kb[:]) + " " + strconv.Itoa(int(occupied)) + " 5 vcf\n"
	if err := os.WriteFile(overlay, []byte(line), 0o644); err != nil {
		t.Fatalf("write overlay: %v", err)
	}
	_, err := emitRun(emitOpts{
		corpusPath: corpus, overlayPath: overlay,
		outPath: filepath.Join(dir, "b.carobook"), statsPath: filepath.Join(dir, "s.txt"),
		floor: config.BookVisitFloor, created: "2026-10-18T00:00:00Z",
	}, io.Discard)
	if err == nil {
		t.Fatalf("emitRun accepted a proof move occupied in its own key")
	}
}
