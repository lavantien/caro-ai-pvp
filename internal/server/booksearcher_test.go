package server

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type bookRow struct {
	moves []string
	reply string
}

type deadlineRecorder struct {
	searcher
	dl engine.Deadline
}

func (r *deadlineRecorder) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	r.dl = dl
	return r.searcher.Search(b, dl)
}

var bookProbeTier = config.Tier{Name: "book-probe", Cores: 1, Book: true, BookMaxPly: 8}

func bookBoard(t *testing.T, names ...string) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, n := range names {
		c, err := rules.ParseCell(n)
		if err != nil {
			t.Fatalf("parse cell %s: %v", n, err)
		}
		if !b.IsLegal(c) {
			t.Fatalf("move %s illegal on the board under construction", n)
		}
		b.Make(c)
	}
	return b
}

func bookKeyOf(b *rules.Board) [config.BookKeyBytes]byte {
	var k [config.BookKeyBytes]byte
	for i, w := range b.Red {
		binary.LittleEndian.PutUint64(k[i*8:], w)
	}
	for i, w := range b.Blue {
		binary.LittleEndian.PutUint64(k[(config.BoardWordsPerColor+i)*8:], w)
	}
	return k
}

func buildBookArtifact(t *testing.T, rows []bookRow) []byte {
	t.Helper()
	type keyed struct {
		key [config.BookKeyBytes]byte
		raw []byte
	}
	entries := make([]keyed, 0, len(rows))
	seen := map[[config.BookKeyBytes]byte]string{}
	for _, r := range rows {
		k := bookKeyOf(bookBoard(t, r.moves...))
		if prev, dup := seen[k]; dup {
			t.Fatalf("duplicate artifact position after %v: replies %s then %s", r.moves, prev, r.reply)
		}
		seen[k] = r.reply
		e := make([]byte, config.BookEntryBytes)
		copy(e, k[:])
		binary.LittleEndian.PutUint16(e[config.BookKeyBytes:], uint16(mustMove(t, r.reply)))
		binary.LittleEndian.PutUint32(e[config.BookKeyBytes+config.BookMoveBytes:], uint32(config.BookVisitFloor))
		e[config.BookKeyBytes+config.BookMoveBytes+config.BookVisitsBytes] = config.BookSourceMajority
		entries = append(entries, keyed{key: k, raw: e})
	}
	slices.SortFunc(entries, func(a, b keyed) int { return bytes.Compare(a.key[:], b.key[:]) })
	out := []byte(config.BookMagicLine)
	meta := `{"version":` + strconv.Itoa(config.BookFormatVersion) + `,"max_ply":16,"generator":"spec","corpus_games":` +
		strconv.Itoa(config.BookCorpusGames) + `,"visit_floor":` + strconv.Itoa(config.BookVisitFloor) + `,"created":"20261018-000000"}` + "\n"
	out = append(out, meta...)
	for _, e := range entries {
		out = append(out, e.raw...)
	}
	return out
}

func stageBookArtifact(t *testing.T, data []byte) {
	t.Helper()
	saved := config.BookArtifactPath
	path := filepath.Join(t.TempDir(), "book.carobook")
	if data != nil {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("stage artifact: %v", err)
		}
	}
	config.BookArtifactPath = path
	t.Cleanup(func() { config.BookArtifactPath = saved })
}

func TestBookWiringWrapsTheBookTiers(t *testing.T) {
	stageBookArtifact(t, buildBookArtifact(t, []bookRow{{reply: "H8"}}))
	for _, tier := range []config.Tier{config.TierEasy, config.TierMedium, config.TierHard} {
		s := newBotSearcher(tier)
		if _, wrapped := s.(*bookSearcher); wrapped {
			t.Errorf("%s wiring = %T, want no book wrapper below master", tier.Name, s)
		}
		s.Close()
	}
	m := newBotSearcher(config.TierMaster)
	wrapped, ok := m.(*bookSearcher)
	if !ok {
		m.Close()
		t.Fatalf("master wiring = %T, want the book wrapper outermost", m)
	}
	if wrapped.maxPly != config.TierMaster.BookMaxPly {
		t.Errorf("master wrapper bound = %d, want the tier's %d", wrapped.maxPly, config.TierMaster.BookMaxPly)
	}
	solved, ok := wrapped.inner.(*solverSearcher)
	if !ok {
		t.Errorf("master inner wiring = %T, want the solver wrapper under the book", wrapped.inner)
	} else if _, pooled := solved.inner.(tierSearcher); !pooled {
		t.Errorf("master solver inner wiring = %T, want the tiered pool", solved.inner)
	}
	if wrapped.book == nil || wrapped.book.Len() != 1 {
		t.Errorf("master wrapper book = (%v entries), want the staged artifact loaded", wrapped.book)
	}
	m.Close()
}

func TestBookHitReturnsZeroCostTaggedLine(t *testing.T) {
	stageBookArtifact(t, buildBookArtifact(t, []bookRow{{reply: "H8"}}))
	s := newBotSearcher(bookProbeTier)
	defer s.Close()
	grant := 2 * time.Second
	dl := engine.NewFixedBudget(grant)
	mv, st, tag := s.Search(rules.NewBoard(), dl)
	if tag != config.BotLogTagBook {
		t.Fatalf("booked opening tag = %q, want %q", tag, config.BotLogTagBook)
	}
	if mv != mustMove(t, "H8") {
		t.Fatalf("booked opening move = %s, want the artifact's H8", cellName(rules.Cell(mv)))
	}
	if st.Depth != 0 || st.Nodes != 0 || st.Nps != 0 || st.EBFMilli != 0 || st.Score != 0 {
		t.Errorf("book hit stats carry cost %d/%d/%d/%d/%d, want the zero-cost law", st.Depth, st.Nodes, st.Nps, st.EBFMilli, st.Score)
	}
	if st.Threads != bookProbeTier.Cores {
		t.Errorf("book hit threads = %d, want the tier's %d", st.Threads, bookProbeTier.Cores)
	}
	if st.PVLen != 1 || st.PV[0] != mv {
		t.Errorf("book hit pv = %d slots starting %s, want the single move", st.PVLen, cellName(rules.Cell(st.PV[0])))
	}
	if st.AllocNs != int64(grant) {
		t.Fatalf("book hit alloc = %d, want the whole idempotent grant draw %d", st.AllocNs, grant)
	}
	if dl.Budget() != grant {
		t.Fatalf("grant after the book reply = %v, want the draw to stay idempotent at %v", dl.Budget(), grant)
	}
	if st.ElapsedNs < 0 {
		t.Fatalf("book hit elapsed = %d, want a measured non-negative duration", st.ElapsedNs)
	}
	want := "M2, Blue, H8, d=0, n=0, nps=0, ebf=0.0, tt=0%, hf=0%, fh1=0%, s=+0, thr=1, t=0.00, alloc=2.00, [BOOK], pv=H8"
	if line := MLine(2, rules.Blue, mv, &st, tag); line != want {
		t.Errorf("book M-line:\n got %q\nwant %q", line, want)
	}
}

func TestBookHitSkipsAndMissFallsThroughUnchanged(t *testing.T) {
	bk, err := book.Load(bytes.NewReader(buildBookArtifact(t, []bookRow{{reply: "H8"}})))
	if err != nil {
		t.Fatalf("load staged artifact: %v", err)
	}
	rec := &deadlineRecorder{searcher: &scriptedBot{script: movesOf(t, []string{"A1"}), stats: fakeStats()}}
	s := &bookSearcher{inner: rec, book: bk, maxPly: 8, cores: 2}
	dl := engine.NewFixedBudget(time.Second)
	mv, _, tag := s.Search(rules.NewBoard(), dl)
	if tag != config.BotLogTagBook || mv != mustMove(t, "H8") {
		t.Fatalf("book hit = (%s, %q), want the artifact's H8 tagged [BOOK]", cellName(rules.Cell(mv)), tag)
	}
	if rec.searcher.(*scriptedBot).searchCount() != 0 {
		t.Errorf("inward searches after a book hit = %d, want the probe to answer alone", rec.searcher.(*scriptedBot).searchCount())
	}
	miss := bookBoard(t, "D4")
	wantStats := fakeStats()
	mv, st, tag := s.Search(miss, dl)
	if tag != "" {
		t.Fatalf("unbooked position tag = %q, want the inward line's untagged pass-through", tag)
	}
	if mv != mustMove(t, "A1") {
		t.Fatalf("unbooked position move = %s, want the inward scripted A1", cellName(rules.Cell(mv)))
	}
	if st != wantStats {
		t.Errorf("miss stats = %+v, want the inward stats unchanged %+v", st, wantStats)
	}
	if rec.dl != dl {
		t.Errorf("miss deadline = %v, want the caller's deadline passed through untouched", rec.dl)
	}
	if rec.searcher.(*scriptedBot).searchCount() != 1 {
		t.Errorf("inward searches after a miss = %d, want exactly one", rec.searcher.(*scriptedBot).searchCount())
	}
}

func TestBookPlyBoundStopsProbingPastTheTierBound(t *testing.T) {
	opening := []string{"D4", "E5", "P16", "J10", "H12", "C9", "M4"}
	rows := []bookRow{
		{moves: opening, reply: "L7"},
		{moves: append(append([]string{}, opening...), "L7"), reply: "A1"},
	}
	stageBookArtifact(t, buildBookArtifact(t, rows))
	s := newBotSearcher(bookProbeTier)
	defer s.Close()
	mv, _, tag := s.Search(bookBoard(t, opening...), engine.NewFixedBudget(time.Second))
	if tag != config.BotLogTagBook || mv != mustMove(t, "L7") {
		t.Fatalf("ply-8 reply at 7 stones = (%s, %q), want the booked L7 hit", cellName(rules.Cell(mv)), tag)
	}
	past := bookBoard(t, append(append([]string{}, opening...), "L7")...)
	mv, st, tag := s.Search(past, engine.NewFixedBudget(50*time.Millisecond))
	if tag == config.BotLogTagBook {
		t.Fatalf("ply-9 position at 8 stones hit the book with %s past the tier bound %d", cellName(rules.Cell(mv)), bookProbeTier.BookMaxPly)
	}
	if st.Nodes == 0 {
		t.Error("past-bound stats show no search nodes, want the inward engine consulted")
	}
	bk, err := book.Load(bytes.NewReader(buildBookArtifact(t, rows)))
	if err != nil {
		t.Fatalf("reload staged artifact: %v", err)
	}
	wide := &bookSearcher{
		inner:  &scriptedBot{script: movesOf(t, []string{"B2"}), stats: fakeStats()},
		book:   bk,
		maxPly: config.TierGrandmaster.BookMaxPly,
		cores:  1,
	}
	mv, _, tag = wide.Search(past, engine.NewFixedBudget(time.Second))
	if tag != config.BotLogTagBook || mv != mustMove(t, "A1") {
		t.Fatalf("grandmaster-bound probe at 8 stones = (%s, %q), want the booked A1: the tier bound governs, not the artifact", cellName(rules.Cell(mv)), tag)
	}
}

func TestLowerTiersNeverProbeTheArtifact(t *testing.T) {
	stageBookArtifact(t, buildBookArtifact(t, []bookRow{{reply: "H8"}}))
	eng := newBotSearcher(config.TierEasy)
	defer eng.Close()
	mv, st, tag := eng.Search(rules.NewBoard(), engine.NewFixedBudget(250*time.Millisecond))
	if tag != "" {
		t.Fatalf("easy tag over a booked opening = %q, want the untagged plain search", tag)
	}
	if st.Nodes == 0 {
		t.Error("easy stats show no search nodes, want a real engine answer")
	}
	if !rules.NewBoard().IsLegal(rules.Cell(mv)) {
		t.Errorf("easy move %s illegal on the fresh board", cellName(rules.Cell(mv)))
	}
	for _, tier := range []config.Tier{config.TierMedium, config.TierHard} {
		s := newBotSearcher(tier)
		if _, wrapped := s.(*bookSearcher); wrapped {
			t.Errorf("%s wiring = %T over a staged artifact, want no book wrapper below master", tier.Name, s)
		}
		s.Close()
	}
}

func TestBookArtifactCorruptionFailsLoudlyAtConstruction(t *testing.T) {
	stageBookArtifact(t, []byte("CAROBOOK2\n"))
	var msg string
	func() {
		defer func() { msg, _ = recover().(string) }()
		s := newBotSearcher(bookProbeTier)
		s.Close()
	}()
	if msg == "" {
		t.Fatal("corrupt artifact constructed a searcher without failing loudly")
	}
	if !strings.Contains(msg, config.BookArtifactPath) || !strings.Contains(msg, "book") {
		t.Errorf("corrupt artifact panic = %q, want the artifact path and the book named", msg)
	}
}

func TestBookMissingArtifactMeansEveryProbeMisses(t *testing.T) {
	stageBookArtifact(t, nil)
	s := newBotSearcher(bookProbeTier)
	defer s.Close()
	mv, st, tag := s.Search(rules.NewBoard(), engine.NewFixedBudget(100*time.Millisecond))
	if tag != "" {
		t.Fatalf("missing-artifact tag = %q, want the untagged inward search", tag)
	}
	if st.Nodes == 0 {
		t.Error("missing-artifact stats show no search nodes, want the inward engine consulted")
	}
	if !rules.NewBoard().IsLegal(rules.Cell(mv)) {
		t.Errorf("missing-artifact move %s illegal", cellName(rules.Cell(mv)))
	}
}

func TestRoomMasterOpensFromTheBookThenSearches(t *testing.T) {
	redScript := []string{"D4", "P16", "H12", "M4", "A1"}
	booked := []string{"E5", "J10", "C9", "L7"}
	rows := make([]bookRow, 0, len(booked))
	for i, reply := range booked {
		played := make([]string, 0, 2*i+1)
		for k := range i {
			played = append(played, redScript[k], booked[k])
		}
		played = append(played, redScript[i])
		rows = append(rows, bookRow{moves: played, reply: reply})
	}
	stageBookArtifact(t, buildBookArtifact(t, rows))
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierMaster)
	r.mu.Lock()
	r.budgetCap = time.Duration(12*config.SearchClockQuantumMs) * time.Millisecond
	r.mu.Unlock()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	for i, name := range redScript {
		waitFor(t, func() bool { return humanTurn(r, alice.ID) })
		if err := r.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
			t.Fatalf("human move %s: %v", name, err)
		}
		want := i + 1
		waitFor(t, func() bool {
			r.mu.Lock()
			defer r.mu.Unlock()
			return len(r.mlines) == want
		})
	}
	r.mu.Lock()
	lines := append([]GameStat(nil), r.mlines...)
	r.mu.Unlock()
	if len(lines) != len(booked)+1 {
		t.Fatalf("master M-lines = %d, want %d", len(lines), len(booked)+1)
	}
	for i, reply := range booked {
		line := lines[i]
		if line.MoveNo != 2*i+2 {
			t.Errorf("book line %d move number = %d, want %d", i, line.MoveNo, 2*i+2)
		}
		if prefix := fmt.Sprintf("M%d, Blue, %s, d=0, n=0, nps=0, ebf=0.0, tt=0%%, hf=0%%, fh1=0%%, s=+0, thr=%d, ", 2*i+2, reply, config.TierMaster.Cores); !strings.HasPrefix(line.Line, prefix) {
			t.Errorf("book line %d:\n got %q\nwant prefix %q", i, line.Line, prefix)
		}
		if tail := ", t=0.00, alloc=0.19, [BOOK], pv=" + reply; !strings.HasSuffix(line.Line, tail) {
			t.Errorf("book line %d tail:\n got %q\nwant %q", i, line.Line, tail)
		}
	}
	past := lines[len(booked)]
	if past.MoveNo != 2*len(booked)+2 {
		t.Errorf("past-bound move number = %d, want %d", past.MoveNo, 2*len(booked)+2)
	}
	if strings.Contains(past.Line, "[BOOK]") {
		t.Errorf("past-bound line still tagged [BOOK]: %q", past.Line)
	}
	if strings.Contains(past.Line, ", d=0, n=0, nps=0, ebf=0.0") {
		t.Errorf("past-bound line carries the zero-cost book shape: %q", past.Line)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

type lifecycleRecorder struct {
	searcher
	started, stopped, closed int
	ponderBoard              *rules.Board
}

func (r *lifecycleRecorder) StartPonder(b *rules.Board) {
	r.started++
	r.ponderBoard = b
	r.searcher.StartPonder(b)
}

func (r *lifecycleRecorder) StopPonder() (rules.Move, engine.SearchStats, string) {
	r.stopped++
	return r.searcher.StopPonder()
}

func (r *lifecycleRecorder) Close() {
	r.closed++
	r.searcher.Close()
}

func TestBookSearcherDelegatesLifecycleAndPlainDeadlines(t *testing.T) {
	bk, err := book.Load(bytes.NewReader(buildBookArtifact(t, []bookRow{{reply: "H8"}})))
	if err != nil {
		t.Fatalf("load staged artifact: %v", err)
	}
	rec := &lifecycleRecorder{searcher: &scriptedBot{script: movesOf(t, []string{"A1"}), stats: fakeStats()}}
	s := &bookSearcher{inner: rec, book: bk, maxPly: 8, cores: 2}
	b := rules.NewBoard()
	s.StartPonder(b)
	if rec.started != 1 || rec.ponderBoard != b {
		t.Fatalf("ponder delegation = %d starts, want the ponder board handed inward once", rec.started)
	}
	if _, _, _ = s.StopPonder(); rec.stopped != 1 {
		t.Fatalf("stop-ponder delegation = %d, want 1", rec.stopped)
	}
	s.Close()
	if rec.closed != 1 {
		t.Fatalf("close delegation = %d, want 1", rec.closed)
	}
	s2 := &bookSearcher{inner: &scriptedBot{script: movesOf(t, []string{"A1"})}, book: bk, maxPly: 8, cores: 2}
	_, st, tag := s2.Search(rules.NewBoard(), engine.NewStoppable())
	if tag != config.BotLogTagBook {
		t.Fatalf("plain-deadline hit tag = %q, want %q", tag, config.BotLogTagBook)
	}
	if st.AllocNs != 0 {
		t.Fatalf("plain-deadline hit alloc = %d, want 0: only a Budgeter deadline carries an alloc draw", st.AllocNs)
	}
}

func TestReadySurfacesCorruptBookArtifactWithoutResidue(t *testing.T) {
	stageBookArtifact(t, []byte("CAROBOOK2\n"))
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierMaster)
	err := r.Ready(alice.ID)
	if err == nil || !strings.Contains(err.Error(), "book") {
		t.Fatalf("ready over a corrupt artifact err = %v, want the construction failure surfaced", err)
	}
	if rerr := r.Ready(alice.ID); rerr != nil {
		t.Fatalf("repeat ready over the parked room err = %v, want the quiet state short-circuit", rerr)
	}
	r.Close()
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("core bookings after close = %d, want 0: the parked room must release on teardown", got)
	}
}

func TestAncestorBookPathFindsTheArtifactFromTheExecutableTree(t *testing.T) {
	root := t.TempDir()
	touch := func(rel string) string {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("stage dirs for %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte("placeholder"), 0o644); err != nil {
			t.Fatalf("stage %s: %v", rel, err)
		}
		return p
	}
	saved := config.BookArtifactPath
	config.BookArtifactPath = filepath.Join("db", "book.carobook")
	t.Cleanup(func() { config.BookArtifactPath = saved })
	want := touch(filepath.Join("db", "book.carobook"))
	touch(filepath.Join("bin", "caro.exe"))
	if got, ok := ancestorBookPath(filepath.Join(root, "bin")); !ok || got != want {
		t.Fatalf("ancestor path from bin = (%q, %v), want (%q, true)", got, ok, want)
	}
	touch(filepath.Join("dist", "windows", "caro.exe"))
	if got, ok := ancestorBookPath(filepath.Join(root, "dist", "windows")); !ok || got != want {
		t.Fatalf("ancestor path from dist/windows = (%q, %v), want (%q, true)", got, ok, want)
	}
}

func TestGoBuildTempTreeDisablesTheExecutableAnchor(t *testing.T) {
	root := t.TempDir()
	exeDir := filepath.Join(root, ".gotmp", config.GoBuildTempDirPrefix+"001", "b001", "exe")
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatalf("stage go-build exe dir: %v", err)
	}
	if !isGoBuildTempTree(exeDir) {
		t.Fatal("go-build temp exe dir passed the anchor gate, want the anchor disabled")
	}
	bin := filepath.Join(root, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatalf("stage bin: %v", err)
	}
	if isGoBuildTempTree(bin) {
		t.Fatal("plain bin dir tripped the go-build gate, want the anchor enabled")
	}
}

func TestMissingBookArtifactWarnsOnceNamingTriedPaths(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	savedPath := config.BookArtifactPath
	t.Cleanup(func() { config.BookArtifactPath = savedPath })
	savedWarn := missingBookWarn
	missingBookWarn = new(sync.Once)
	t.Cleanup(func() { missingBookWarn = savedWarn })
	config.BookArtifactPath = filepath.Join("no-such-book-tree", "book.carobook")
	if bk := loadBotBook(); bk != nil {
		t.Fatalf("missing artifact loaded %d entries, want nil", bk.Len())
	}
	if bk := loadBotBook(); bk != nil {
		t.Fatalf("second missing artifact load = %d entries, want nil", bk.Len())
	}
	out := buf.String()
	if n := strings.Count(out, "book artifact missing"); n != 1 {
		t.Fatalf("missing-artifact warnings after two loads = %d, want exactly 1:\n%s", n, out)
	}
	abs, err := filepath.Abs(config.BookArtifactPath)
	if err != nil {
		t.Fatalf("resolve the tried absolute path: %v", err)
	}
	if !strings.Contains(out, abs) {
		t.Errorf("missing-artifact warning %q does not name the tried absolute path %s", out, abs)
	}
}
