package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func lineRecs(t *testing.T, cells ...int) []posRec {
	t.Helper()
	b := rules.NewBoard()
	var recs []posRec
	for _, c := range cells {
		recs = append(recs, posRec{key: keyOf(b), move: rules.Move(c)})
		b.Make(rules.Cell(c))
	}
	return recs
}

func renderGame(idx int, recs []posRec, illegal int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "G %d\n", idx)
	for _, r := range recs {
		fmt.Fprintf(&b, "P %s %d\n", hexKey(r.key[:]), r.move)
	}
	fmt.Fprintf(&b, "E %d %d\n", len(recs), illegal)
	return b.String()
}

func writeShard(t *testing.T, path, tier string, tc, target int, games ...[]posRec) {
	t.Helper()
	clean := seedHeader(t, path, corpusFingerprint(tier, tc, target))
	for i, recs := range games {
		next, err := appendGame(path, clean, i, recs, 0)
		if err != nil {
			t.Fatalf("appendGame %d on %s: %v", i, path, err)
		}
		clean = next
	}
}

func writeRaw(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readAll(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestFlagWasSet(t *testing.T) {
	fs := flag.NewFlagSet("merge", flag.ContinueOnError)
	fs.String("out", "default", "")
	if flagWasSet(fs, "out") {
		t.Fatalf("unset flag reported as set")
	}
	if err := fs.Parse([]string{"-out", "merged.txt"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !flagWasSet(fs, "out") {
		t.Fatalf("explicit -out not detected")
	}
}

func TestParseCorpusFileContracts(t *testing.T) {
	dir := t.TempDir()
	if _, err := parseCorpusFile(filepath.Join(dir, "absent.txt")); err == nil {
		t.Errorf("missing file parsed without error")
	}
	zero := filepath.Join(dir, "zero.txt")
	writeRaw(t, zero, "")
	cf, err := parseCorpusFile(zero)
	if err != nil {
		t.Fatalf("zero-byte file: %v", err)
	}
	if cf.fp != "" || cf.games != 0 || len(cf.blocks) != 0 || cf.torn {
		t.Errorf("zero-byte parse = %+v, want the empty scan", cf)
	}
	stub := filepath.Join(dir, "stub.txt")
	writeShard(t, stub, config.TierEasy.Name, 0, 2)
	cf, err = parseCorpusFile(stub)
	if err != nil {
		t.Fatalf("header-only shard: %v", err)
	}
	if cf.fp != corpusFingerprint(config.TierEasy.Name, 0, 2) || cf.games != 0 || cf.clean != int64(len(cf.fp)+1) {
		t.Errorf("header-only parse = %+v, want only the fingerprint", cf)
	}
	broken := filepath.Join(dir, "broken.txt")
	writeRaw(t, broken, corpusFingerprint(config.TierEasy.Name, 0, 2)+"\nG 1\nE 0 0\n")
	if _, err := parseCorpusFile(broken); err == nil || !strings.Contains(err.Error(), "breaks the recorded sequence") {
		t.Errorf("sequence break error = %v, want the sequence refusal", err)
	}
	torn := filepath.Join(dir, "torn.txt")
	writeShard(t, torn, config.TierEasy.Name, 0, 2, lineRecs(t, 119))
	f, err := os.OpenFile(torn, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open torn: %v", err)
	}
	if _, err := f.WriteString("G 1\nP " + strings.Repeat("a", 2*config.BookKeyBytes) + " 9\n"); err != nil {
		t.Fatalf("tear: %v", err)
	}
	f.Close()
	cf, err = parseCorpusFile(torn)
	if err != nil {
		t.Fatalf("torn shard parse: %v", err)
	}
	if !cf.torn || cf.games != 1 {
		t.Errorf("torn parse = %+v, want 1 complete game flagged torn", cf)
	}
}

func TestMergeConcatsShardBlocksInOrder(t *testing.T) {
	dir := t.TempDir()
	s1 := filepath.Join(dir, "s1.txt")
	s2 := filepath.Join(dir, "s2.txt")
	a := lineRecs(t, 119, 7)
	b := lineRecs(t, 120, 8)
	c := lineRecs(t, 119, 9)
	d := lineRecs(t, 120, 7)
	writeShard(t, s1, config.TierEasy.Name, 0, 4, a, b)
	writeShard(t, s2, config.TierEasy.Name, 0, 4, c, d)
	out := filepath.Join(dir, "merged.txt")
	var progress bytes.Buffer
	if err := runMerge(mergeOpts{shards: []string{s1, s2}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 4}, &progress); err != nil {
		t.Fatalf("runMerge: %v", err)
	}
	want := corpusFingerprint(config.TierEasy.Name, 0, 4) + "\n" +
		renderGame(0, a, 0) + renderGame(1, b, 0) + renderGame(2, c, 0) + renderGame(3, d, 0)
	if got := readAll(t, out); got != want {
		t.Fatalf("merged file = %q, want %q", got, want)
	}
	sc, err := loadCorpus(out)
	if err != nil {
		t.Fatalf("loadCorpus on merged output: %v", err)
	}
	if sc.games != 4 || sc.torn || sc.fp != corpusFingerprint(config.TierEasy.Name, 0, 4) {
		t.Fatalf("merged scan = %+v, want 4 games under the target fingerprint", sc)
	}
	log := progress.String()
	for _, wantLine := range []string{
		fmt.Sprintf("shard %s: 2 games", s1),
		fmt.Sprintf("shard %s: 2 games", s2),
		"merge done: 4 games from 2 shards (0 duplicates skipped)",
	} {
		if !strings.Contains(log, wantLine) {
			t.Errorf("merge log %q misses %q", log, wantLine)
		}
	}
}

func TestMergePreservesIllegalCounts(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "s.txt")
	recs := lineRecs(t, 119, 7)
	clean := seedHeader(t, shard, corpusFingerprint(config.TierEasy.Name, 0, 1))
	if _, err := appendGame(shard, clean, 0, recs, 2); err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	out := filepath.Join(dir, "m.txt")
	if err := runMerge(mergeOpts{shards: []string{shard}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 1}, io.Discard); err != nil {
		t.Fatalf("runMerge: %v", err)
	}
	if got := readAll(t, out); !strings.HasSuffix(got, fmt.Sprintf("E %d 2\n", len(recs))) {
		t.Fatalf("merged tail = %q, want the illegal count preserved", got)
	}
}

func TestMergeRefusesFingerprintMismatch(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name              string
		shardTier         string
		shardTC, shardTgt int
		runTier           string
		runTC, runGames   int
		field, got, want  string
	}{
		{"tier", config.TierMaster.Name, 0, 4, config.TierEasy.Name, 0, 4, "tier", config.TierMaster.Name, config.TierEasy.Name},
		{"tc", config.TierEasy.Name, 1, 4, config.TierEasy.Name, 0, 4, "tc", "1", "0"},
	}
	for _, c := range cases {
		shard := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".txt")
		writeShard(t, shard, c.shardTier, c.shardTC, c.shardTgt, lineRecs(t, 119), lineRecs(t, 120))
		out := filepath.Join(dir, "m.txt")
		err := runMerge(mergeOpts{shards: []string{shard}, outPath: out, tier: c.runTier, tc: c.runTC, games: c.runGames}, io.Discard)
		if err == nil {
			t.Errorf("%s: merge accepted a mismatching shard", c.name)
			continue
		}
		for _, part := range []string{filepath.Base(shard), c.field, c.field + "=" + c.got, c.field + "=" + c.want} {
			if !strings.Contains(err.Error(), part) {
				t.Errorf("%s: error %q misses %q", c.name, err.Error(), part)
			}
		}
	}
}

func TestMergeMixedTargetsUnifyAtTheRequestedTarget(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "legacy.txt")
	segB := filepath.Join(dir, "book-corpus-b.txt")
	segC := filepath.Join(dir, "book-corpus-c.txt")
	segD := filepath.Join(dir, "book-corpus-d.txt")
	games := [][]posRec{
		lineRecs(t, 119, 7),
		lineRecs(t, 120, 8),
		lineRecs(t, 119, 9),
		lineRecs(t, 120, 7),
		lineRecs(t, 102, 5),
	}
	writeShard(t, legacy, config.TierEasy.Name, 0, 8, games...)
	writeShard(t, segB, config.TierEasy.Name, 0, 2, lineRecs(t, 105, 6), lineRecs(t, 150, 4))
	writeShard(t, segC, config.TierEasy.Name, 0, 2, lineRecs(t, 153, 3), lineRecs(t, 116, 21))
	writeShard(t, segD, config.TierEasy.Name, 0, 2, lineRecs(t, 123, 22), lineRecs(t, 87, 23))
	out := filepath.Join(dir, "merged.txt")
	var progress bytes.Buffer
	if err := runMerge(mergeOpts{shards: []string{legacy, segB, segC, segD}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 8}, &progress); err != nil {
		t.Fatalf("runMerge over mixed targets: %v", err)
	}
	var want strings.Builder
	fmt.Fprintf(&want, "%s\n", corpusFingerprint(config.TierEasy.Name, 0, 8))
	for i, recs := range append(append([][]posRec{}, games...), lineRecs(t, 105, 6), lineRecs(t, 150, 4), lineRecs(t, 153, 3), lineRecs(t, 116, 21), lineRecs(t, 123, 22), lineRecs(t, 87, 23)) {
		want.WriteString(renderGame(i, recs, 0))
	}
	if got := readAll(t, out); got != want.String() {
		t.Fatalf("mixed-target merge = %q, want %q", got, want.String())
	}
	if !strings.Contains(progress.String(), "merge done: 11 games from 4 shards (0 duplicates skipped)") {
		t.Errorf("log %q misses the unified summary", progress.String())
	}
}

func TestMergeSegmentsOnlySetAccepted(t *testing.T) {
	dir := t.TempDir()
	segs := []string{filepath.Join(dir, "b.txt"), filepath.Join(dir, "c.txt"), filepath.Join(dir, "d.txt")}
	for i, seg := range segs {
		recs := [][]posRec{lineRecs(t, 119+i, 7), lineRecs(t, 120+i, 8), lineRecs(t, 121+i, 9)}
		writeShard(t, seg, config.TierEasy.Name, 0, 3, recs...)
	}
	out := filepath.Join(dir, "m.txt")
	if err := runMerge(mergeOpts{shards: segs, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 9}, io.Discard); err != nil {
		t.Fatalf("runMerge over segments only: %v", err)
	}
	sc, err := loadCorpus(out)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	if sc.games != 9 || sc.fp != corpusFingerprint(config.TierEasy.Name, 0, 9) {
		t.Fatalf("segments-only scan = %d games under %q, want 9 under the requested target", sc.games, sc.fp)
	}
}

func TestMergeRefusesShardGamesAboveTarget(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "s.txt")
	writeShard(t, shard, config.TierEasy.Name, 0, 5, lineRecs(t, 119), lineRecs(t, 120))
	out := filepath.Join(dir, "m.txt")
	err := runMerge(mergeOpts{shards: []string{shard}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 4}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), filepath.Base(shard)) || !strings.Contains(err.Error(), "games=5") || !strings.Contains(err.Error(), "target 4") {
		t.Fatalf("above-target shard error = %v, want the wrong-generation refusal naming shard value and target", err)
	}
	if _, serr := os.Stat(out); !os.IsNotExist(serr) {
		t.Fatalf("refused merge still wrote %s", out)
	}
}

func TestMergeRefusesAliasedFingerprintField(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "alias.txt")
	k := lineRecs(t, 119)[0].key
	writeRaw(t, shard, fmt.Sprintf("F corpus v%d seat=1 tc=0 games=1\nG 0\nP %s 7\nE 1 0\n", config.BookSidecarVersion, hexKey(k[:])))
	out := filepath.Join(dir, "m.txt")
	err := runMerge(mergeOpts{shards: []string{shard}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 1}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "tier=") || !strings.Contains(err.Error(), filepath.Base(shard)) {
		t.Fatalf("aliased field error = %v, want the tier refusal naming the shard", err)
	}
}

func TestMergeRefusesTotalBelowGamesFloor(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "s.txt")
	writeShard(t, shard, config.TierEasy.Name, 0, 3, lineRecs(t, 119), lineRecs(t, 120))
	out := filepath.Join(dir, "m.txt")
	err := runMerge(mergeOpts{shards: []string{shard}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 3}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "kept 2") || !strings.Contains(err.Error(), "floor 3") {
		t.Fatalf("below-floor error = %v, want the kept count and floor named", err)
	}
	if _, serr := os.Stat(out); !os.IsNotExist(serr) {
		t.Fatalf("refused merge still wrote %s", out)
	}
}

func TestMergeAcceptsHeaderOnlyShard(t *testing.T) {
	dir := t.TempDir()
	stub := filepath.Join(dir, "stub.txt")
	full := filepath.Join(dir, "full.txt")
	a := lineRecs(t, 119)
	b := lineRecs(t, 120)
	writeShard(t, stub, config.TierEasy.Name, 0, 2)
	writeShard(t, full, config.TierEasy.Name, 0, 2, a, b)
	out := filepath.Join(dir, "m.txt")
	var progress bytes.Buffer
	if err := runMerge(mergeOpts{shards: []string{stub, full}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 2}, &progress); err != nil {
		t.Fatalf("runMerge over a header-only stub: %v", err)
	}
	want := corpusFingerprint(config.TierEasy.Name, 0, 2) + "\n" + renderGame(0, a, 0) + renderGame(1, b, 0)
	if got := readAll(t, out); got != want {
		t.Fatalf("merged with stub = %q, want %q", got, want)
	}
	if !strings.Contains(progress.String(), fmt.Sprintf("shard %s: 0 games", stub)) {
		t.Errorf("log %q misses the stub game count", progress.String())
	}
}

func TestMergeRefusesZeroByteShard(t *testing.T) {
	dir := t.TempDir()
	zero := filepath.Join(dir, "zero.txt")
	full := filepath.Join(dir, "full.txt")
	writeRaw(t, zero, "")
	writeShard(t, full, config.TierEasy.Name, 0, 1, lineRecs(t, 119))
	out := filepath.Join(dir, "m.txt")
	err := runMerge(mergeOpts{shards: []string{zero, full}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 1}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), filepath.Base(zero)) || !strings.Contains(err.Error(), "fingerprint") {
		t.Fatalf("zero-byte shard error = %v, want the fingerprint refusal naming the file", err)
	}
}

func TestMergeRefusesTornShard(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "torn.txt")
	writeShard(t, shard, config.TierEasy.Name, 0, 1, lineRecs(t, 119))
	f, err := os.OpenFile(shard, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteString("G 1\n"); err != nil {
		t.Fatalf("tear: %v", err)
	}
	f.Close()
	out := filepath.Join(dir, "m.txt")
	err = runMerge(mergeOpts{shards: []string{shard}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 1}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), filepath.Base(shard)) || !strings.Contains(err.Error(), "torn") {
		t.Fatalf("torn shard error = %v, want the torn refusal naming the file", err)
	}
}

func TestMergeSingleShardPassthroughIsByteIdentical(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "s.txt")
	writeShard(t, shard, config.TierEasy.Name, 0, 2, lineRecs(t, 119, 7), lineRecs(t, 120, 8))
	out := filepath.Join(dir, "m.txt")
	if err := runMerge(mergeOpts{shards: []string{shard}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 2}, io.Discard); err != nil {
		t.Fatalf("runMerge: %v", err)
	}
	if readAll(t, out) != readAll(t, shard) {
		t.Fatalf("single-shard passthrough rewrote bytes")
	}
}

func TestMergeRerunOverMergedOutputIsByteStable(t *testing.T) {
	dir := t.TempDir()
	s1 := filepath.Join(dir, "s1.txt")
	s2 := filepath.Join(dir, "s2.txt")
	a := lineRecs(t, 119, 7)
	b := lineRecs(t, 120, 8)
	c := lineRecs(t, 119, 9)
	d := lineRecs(t, 120, 7)
	writeShard(t, s1, config.TierEasy.Name, 0, 4, a, b)
	writeShard(t, s2, config.TierEasy.Name, 0, 4, c, d)
	m1 := filepath.Join(dir, "m1.txt")
	if err := runMerge(mergeOpts{shards: []string{s1, s2}, outPath: m1, tier: config.TierEasy.Name, tc: 0, games: 4}, io.Discard); err != nil {
		t.Fatalf("first merge: %v", err)
	}
	m2 := filepath.Join(dir, "m2.txt")
	if err := runMerge(mergeOpts{shards: []string{m1}, outPath: m2, tier: config.TierEasy.Name, tc: 0, games: 4}, io.Discard); err != nil {
		t.Fatalf("re-merge: %v", err)
	}
	want := corpusFingerprint(config.TierEasy.Name, 0, 4) + "\n" +
		renderGame(0, a, 0) + renderGame(1, b, 0) + renderGame(2, c, 0) + renderGame(3, d, 0)
	if got := readAll(t, m2); got != want || got != readAll(t, m1) {
		t.Fatalf("re-merged output drifted from the first merge or the pinned render")
	}
}

func TestMergeSkipsDuplicateGamesWithLogCount(t *testing.T) {
	dir := t.TempDir()
	s1 := filepath.Join(dir, "s1.txt")
	s2 := filepath.Join(dir, "s2.txt")
	a := lineRecs(t, 119, 7, 8)
	b := lineRecs(t, 120, 7)
	c := lineRecs(t, 120, 8)
	writeShard(t, s1, config.TierEasy.Name, 0, 3, a, b)
	c1, err := parseCorpusFile(s1)
	if err != nil {
		t.Fatalf("parse s1: %v", err)
	}
	clean := seedHeader(t, s2, corpusFingerprint(config.TierEasy.Name, 0, 3))
	next, err := appendGame(s2, clean, 0, a, 0)
	if err != nil {
		t.Fatalf("append dup game: %v", err)
	}
	if _, err := appendGame(s2, next, 1, c, 0); err != nil {
		t.Fatalf("append game 1: %v", err)
	}
	c2, err := parseCorpusFile(s2)
	if err != nil {
		t.Fatalf("parse s2: %v", err)
	}
	if c1.blocks[0].sig != c2.blocks[0].sig {
		t.Fatalf("identical game content produced different signatures")
	}
	out := filepath.Join(dir, "m.txt")
	var progress bytes.Buffer
	if err := runMerge(mergeOpts{shards: []string{s1, s2}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 3}, &progress); err != nil {
		t.Fatalf("runMerge: %v", err)
	}
	want := corpusFingerprint(config.TierEasy.Name, 0, 3) + "\n" +
		renderGame(0, a, 0) + renderGame(1, b, 0) + renderGame(2, c, 0)
	if got := readAll(t, out); got != want {
		t.Fatalf("deduped merge = %q, want %q", got, want)
	}
	log := progress.String()
	if !strings.Contains(log, fmt.Sprintf("duplicate game skipped: %s game 0 repeats %s game 0", s2, s1)) {
		t.Errorf("log %q misses the per-duplicate skip line", log)
	}
	if !strings.Contains(log, "merge done: 3 games from 2 shards (1 duplicates skipped)") {
		t.Errorf("log %q misses the duplicate count", log)
	}
}

func TestMergeKeepsGamesWhoseIllegalCountDiffers(t *testing.T) {
	dir := t.TempDir()
	s1 := filepath.Join(dir, "s1.txt")
	s2 := filepath.Join(dir, "s2.txt")
	a := lineRecs(t, 119, 7)
	clean := seedHeader(t, s1, corpusFingerprint(config.TierEasy.Name, 0, 2))
	if _, err := appendGame(s1, clean, 0, a, 0); err != nil {
		t.Fatalf("append s1: %v", err)
	}
	clean = seedHeader(t, s2, corpusFingerprint(config.TierEasy.Name, 0, 2))
	if _, err := appendGame(s2, clean, 0, a, 1); err != nil {
		t.Fatalf("append s2: %v", err)
	}
	out := filepath.Join(dir, "m.txt")
	var progress bytes.Buffer
	if err := runMerge(mergeOpts{shards: []string{s1, s2}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 2}, &progress); err != nil {
		t.Fatalf("runMerge: %v", err)
	}
	sc, err := loadCorpus(out)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	if sc.games != 2 || sc.illegal != 1 {
		t.Fatalf("merged scan = %+v, want both games kept with the illegal sum", sc)
	}
	if !strings.Contains(progress.String(), "0 duplicates skipped") {
		t.Errorf("log %q reports a duplicate where only the illegal count differs", progress.String())
	}
}

func TestMergeRefusesOutputOverlappingAShard(t *testing.T) {
	dir := t.TempDir()
	shard := filepath.Join(dir, "s.txt")
	writeShard(t, shard, config.TierEasy.Name, 0, 1, lineRecs(t, 119))
	err := runMerge(mergeOpts{shards: []string{shard}, outPath: shard, tier: config.TierEasy.Name, tc: 0, games: 1}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "also an input shard") {
		t.Fatalf("overlap error = %v, want the output-over-shard refusal", err)
	}
}

func TestMergeRefusesShardHeldByLiveMember(t *testing.T) {
	dir := t.TempDir()
	held := filepath.Join(dir, "held.txt")
	free := filepath.Join(dir, "free.txt")
	writeShard(t, held, config.TierEasy.Name, 0, 2, lineRecs(t, 119))
	writeShard(t, free, config.TierEasy.Name, 0, 2, lineRecs(t, 120))
	if err := os.WriteFile(lockPath(held, "corpus"), []byte("4242"), 0o644); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	out := filepath.Join(dir, "m.txt")
	err := runMerge(mergeOpts{shards: []string{held, free}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 2}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), filepath.Base(held)) || !strings.Contains(err.Error(), "4242") {
		t.Fatalf("held shard error = %v, want the refusal naming the shard and pid", err)
	}
	stale := filepath.Join(dir, "stale.txt")
	writeShard(t, stale, config.TierEasy.Name, 0, 2, lineRecs(t, 119))
	if err := os.WriteFile(lockPath(stale, "corpus"), []byte("1"), 0o644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	aged := time.Now().Add(-2 * time.Duration(config.BookLockStaleMs) * time.Millisecond)
	if err := os.Chtimes(lockPath(stale, "corpus"), aged, aged); err != nil {
		t.Fatalf("age lock: %v", err)
	}
	if err := runMerge(mergeOpts{shards: []string{stale, free}, outPath: out, tier: config.TierEasy.Name, tc: 0, games: 2}, io.Discard); err != nil {
		t.Fatalf("stale lock blocked the merge: %v", err)
	}
}

func wantPly(pairs map[int]plyCount) [config.BookMaxPly]plyCount {
	var plies [config.BookMaxPly]plyCount
	for p, c := range pairs {
		plies[p] = c
	}
	return plies
}

func TestStatsTablePinsSyntheticCounts(t *testing.T) {
	dir := t.TempDir()
	gameA := lineRecs(t, 119, 7, 8, 9)
	gameB := lineRecs(t, 119, 7, 8, 9)
	gameC := lineRecs(t, 120, 7, 8)
	abc := filepath.Join(dir, "abc.txt")
	ac := filepath.Join(dir, "ac.txt")
	b := filepath.Join(dir, "b.txt")
	aEmptyC := filepath.Join(dir, "a-empty-c.txt")
	writeShard(t, abc, config.TierEasy.Name, 0, 3, gameA, gameB, gameC)
	writeShard(t, ac, config.TierEasy.Name, 0, 3, gameA, gameC)
	writeShard(t, b, config.TierEasy.Name, 0, 3, gameB)
	writeShard(t, aEmptyC, config.TierEasy.Name, 0, 3, gameA, nil, gameC)
	splitPinned := wantPly(map[int]plyCount{0: {1, 1}, 1: {2, 1}, 2: {2, 1}, 3: {1, 1}})
	noPass := wantPly(map[int]plyCount{0: {1, 0}, 1: {2, 0}, 2: {2, 0}, 3: {1, 0}})
	cases := []struct {
		name  string
		paths []string
		floor int
		want  statsResult
	}{
		{"three games floor 2", []string{abc}, 2, statsResult{games: 3, unique: 6, trunks: 2, plies: splitPinned}},
		{"three games floor 3", []string{abc}, 3, statsResult{games: 3, unique: 6, trunks: 2, plies: noPass}},
		{"split files union", []string{ac, b}, 2, statsResult{games: 3, unique: 6, trunks: 2, plies: splitPinned}},
		{"empty game adds no mass", []string{aEmptyC}, 2, statsResult{games: 3, unique: 6, trunks: 2, plies: noPass}},
	}
	for _, c := range cases {
		res, err := runStats(c.paths, c.floor, io.Discard)
		if err != nil {
			t.Errorf("%s: runStats: %v", c.name, err)
			continue
		}
		if res != c.want {
			t.Errorf("%s: stats = %+v, want %+v", c.name, res, c.want)
		}
	}
}

func TestStatsPrintsPinnedLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.txt")
	writeShard(t, path, config.TierEasy.Name, 0, 3, lineRecs(t, 119, 7, 8, 9), lineRecs(t, 119, 7, 8, 9), lineRecs(t, 120, 7, 8))
	var progress bytes.Buffer
	if _, err := runStats([]string{path}, 2, &progress); err != nil {
		t.Fatalf("runStats: %v", err)
	}
	log := progress.String()
	for _, line := range []string{
		fmt.Sprintf("stats %s: 3 games", path),
		"stats done: games 3 unique 6 trunks 2 floor 2",
		"ply 0 unique 1 pass 1",
		"ply 1 unique 2 pass 1",
		"ply 15 unique 0 pass 0",
	} {
		if !strings.Contains(log, line+"\n") {
			t.Errorf("stats log %q misses line %q", log, line)
		}
	}
}

func TestStatsRefusesUnreadableCorpora(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "absent.txt")
	if _, err := runStats([]string{missing}, 2, io.Discard); err == nil {
		t.Errorf("missing corpus accepted")
	}
	zero := filepath.Join(dir, "zero.txt")
	writeRaw(t, zero, "")
	if _, err := runStats([]string{zero}, 2, io.Discard); err == nil || !strings.Contains(err.Error(), filepath.Base(zero)) {
		t.Errorf("zero-byte corpus error = %v, want the refusal naming the file", err)
	}
	torn := filepath.Join(dir, "torn.txt")
	writeShard(t, torn, config.TierEasy.Name, 0, 1, lineRecs(t, 119))
	f, err := os.OpenFile(torn, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open torn: %v", err)
	}
	if _, err := f.WriteString("G 1\n"); err != nil {
		t.Fatalf("tear: %v", err)
	}
	f.Close()
	if _, err := runStats([]string{torn}, 2, io.Discard); err == nil || !strings.Contains(err.Error(), "torn") {
		t.Errorf("torn corpus error = %v, want the torn refusal", err)
	}
}
