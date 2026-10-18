package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func singleStoneKey(t *testing.T, cell int) [config.BookKeyBytes]byte {
	t.Helper()
	b := rules.NewBoard()
	b.Make(rules.Cell(cell))
	return keyOf(b)
}

func scriptedPlayer(t *testing.T, seen *[]gameCall) gamePlayer {
	t.Helper()
	return func(idx, redSeat int) gameResult {
		*seen = append(*seen, gameCall{idx: idx, redSeat: redSeat})
		return gameResult{
			records: []posRec{{key: corpusZeroKey, move: openingMove(idx)}},
			moves:   3,
			winner:  "red",
		}
	}
}

func collapsedPlayer(t *testing.T, seen *[]gameCall) gamePlayer {
	t.Helper()
	return func(idx, redSeat int) gameResult {
		*seen = append(*seen, gameCall{idx: idx, redSeat: redSeat})
		return gameResult{
			records: []posRec{{key: corpusZeroKey, move: openingMove(0)}},
			moves:   3,
			winner:  "red",
		}
	}
}

func TestOpeningMoveRotatesDistinctTrunks(t *testing.T) {
	seen := map[rules.Move]int{}
	for i := 0; i < 3*len(config.BookOpeningCells); i++ {
		mv := openingMove(i)
		if mv != rules.Move(config.BookOpeningCells[i%len(config.BookOpeningCells)]) {
			t.Fatalf("openingMove(%d) = %d, want cell %d", i, mv, config.BookOpeningCells[i%len(config.BookOpeningCells)])
		}
		seen[mv]++
	}
	if len(seen) != len(config.BookOpeningCells) {
		t.Fatalf("rotation covered %d distinct cells, want %d", len(seen), len(config.BookOpeningCells))
	}
	if openingMove(0) == openingMove(1) {
		t.Fatalf("consecutive games share one trunk")
	}
}

func TestCorpusRunFailsOnCollapsedTrunks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.txt")
	var seen []gameCall
	if err := runCorpus(easyCorpusOpts(path, 3), collapsedPlayer(t, &seen), io.Discard); err == nil {
		t.Fatalf("runCorpus accepted a collapsed trunk corpus of %d games", len(seen))
	}
}

func seedHeader(t *testing.T, path, fp string) int64 {
	t.Helper()
	clean, err := writeSidecarHeader(path, fp)
	if err != nil {
		t.Fatalf("writeSidecarHeader %s: %v", path, err)
	}
	return clean
}

func easyCorpusOpts(path string, games int) corpusOpts {
	return corpusOpts{path: path, tier: config.TierEasy.Name, tc: 0, games: games}
}

func TestCorpusBlockFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.txt")
	recs := []posRec{
		{key: singleStoneKey(t, 0), move: rules.Move(7)},
		{key: singleStoneKey(t, 1), move: rules.Move(200)},
	}
	if _, err := appendGame(path, 0, 4, recs, 0); err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	k0 := singleStoneKey(t, 0)
	k1 := singleStoneKey(t, 1)
	want := fmt.Sprintf("G 4\nP %s 7\nP %s 200\nE 2 0\n", hexKey(k0[:]), hexKey(k1[:]))
	if string(data) != want {
		t.Errorf("corpus block = %q, want %q", string(data), want)
	}
}

func TestCorpusScanCountsOnlyCompleteBlocks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.txt")
	clean0 := seedHeader(t, path, corpusFingerprint(config.TierEasy.Name, 0, 2))
	if _, err := appendGame(path, clean0, 0, []posRec{{key: singleStoneKey(t, 0), move: 1}}, 0); err != nil {
		t.Fatalf("appendGame 0: %v", err)
	}
	sc, err := loadCorpus(path)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	if sc.games != 1 || sc.torn || len(sc.recs) != 1 {
		t.Fatalf("scan after one block = %+v, want 1 complete game", sc)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open for tear: %v", err)
	}
	if _, err := f.WriteString("G 1\nP " + strings.Repeat("a", 2*config.BookKeyBytes) + " 9\n"); err != nil {
		t.Fatalf("tear tail: %v", err)
	}
	f.Close()
	sc, err = loadCorpus(path)
	if err != nil {
		t.Fatalf("loadCorpus torn: %v", err)
	}
	if sc.games != 1 || !sc.torn {
		t.Fatalf("scan of torn tail = %+v, want 1 game flagged torn", sc)
	}
	k0 := singleStoneKey(t, 0)
	block := len("G 0\nP " + hexKey(k0[:]) + " 1\nE 1 0\n")
	if sc.clean != clean0+int64(block) {
		t.Errorf("clean offset = %d, want the end of the last complete block at %d", sc.clean, clean0+int64(block))
	}
}

func TestCorpusLoadRejectsCorruptLines(t *testing.T) {
	dir := t.TempDir()
	k3 := singleStoneKey(t, 3)
	fp := corpusFingerprint(config.TierEasy.Name, 0, 1) + "\n"
	cases := []struct {
		name, body string
	}{
		{"record outside a game", fp + "P " + hexKey(k3[:]) + " 5\n"},
		{"game header inside a game", fp + "G 0\nG 1\n"},
		{"bad key hex", fp + "G 0\nP zz 5\nE 1 0\n"},
		{"short key", fp + "G 0\nP abcd 5\nE 1 0\n"},
		{"bad move", fp + "G 0\nP " + hexKey(k3[:]) + " x\nE 1 0\n"},
		{"move off board", fp + "G 0\nP " + hexKey(k3[:]) + " 256\nE 1 0\n"},
		{"unknown line", fp + "Q 0\n"},
		{"count mismatch", fp + "G 0\nE 2 0\n"},
		{"block end missing illegal count", fp + "G 0\nE 1\n"},
	}
	for _, c := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".txt")
		if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", c.name, err)
		}
		if _, err := loadCorpus(path); err == nil {
			t.Errorf("%s: loadCorpus accepted %q", c.name, c.body)
		}
	}
}

func TestCorpusRunResumesFromRecordedGames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.txt")
	var seen []gameCall
	play := scriptedPlayer(t, &seen)
	if err := runCorpus(easyCorpusOpts(path, 5), play, io.Discard); err != nil {
		t.Fatalf("runCorpus 5: %v", err)
	}
	if len(seen) != 5 {
		t.Fatalf("first run played %d games, want 5", len(seen))
	}
	if err := runCorpus(easyCorpusOpts(path, 5), play, io.Discard); err != nil {
		t.Fatalf("runCorpus rerun: %v", err)
	}
	wantIdx := []int{0, 1, 2, 3, 4}
	wantRed := []int{0, 1, 0, 1, 0}
	for i, call := range seen {
		if call.idx != wantIdx[i] {
			t.Errorf("game %d: played index %d, want %d", i, call.idx, wantIdx[i])
		}
		if call.redSeat != wantRed[i] {
			t.Errorf("game %d: red seat %d, want %d", i, call.redSeat, wantRed[i])
		}
	}
	sc, err := loadCorpus(path)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	if sc.games != 5 || sc.torn {
		t.Fatalf("final scan = %+v, want 5 complete games", sc)
	}
	if len(sc.recs) != 5 {
		t.Fatalf("final records = %d, want 5", len(sc.recs))
	}
}

func TestCorpusRunTruncatesTornTailAndSkipsDoneGames(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corpus.txt")
	clean0 := seedHeader(t, path, corpusFingerprint(config.TierEasy.Name, 0, 2))
	if _, err := appendGame(path, clean0, 0, []posRec{{key: corpusZeroKey, move: openingMove(0)}, {key: singleStoneKey(t, 0), move: 1}}, 0); err != nil {
		t.Fatalf("seed block: %v", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open tear: %v", err)
	}
	tear := "G 1\nP " + strings.Repeat("0", 2*config.BookKeyBytes) + " 4\n"
	if _, err := f.WriteString(tear); err != nil {
		t.Fatalf("tear: %v", err)
	}
	f.Close()
	var seen []gameCall
	if err := runCorpus(easyCorpusOpts(path, 2), scriptedPlayer(t, &seen), io.Discard); err != nil {
		t.Fatalf("runCorpus: %v", err)
	}
	if len(seen) != 1 || seen[0].idx != 1 {
		t.Fatalf("resume played %+v, want only game 1", seen)
	}
	sc, err := loadCorpus(path)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	if sc.games != 2 || sc.torn {
		t.Fatalf("post-resume scan = %+v, want 2 complete games and no torn tail", sc)
	}
	var progress bytes.Buffer
	if err := runCorpus(easyCorpusOpts(path, 2), scriptedPlayer(t, &seen), &progress); err != nil {
		t.Fatalf("rerun complete corpus: %v", err)
	}
	if len(seen) != 1 || !strings.Contains(progress.String(), "corpus complete: 2 games") {
		t.Errorf("complete corpus replayed games or missed the no-op report: played %d, said %q", len(seen), progress.String())
	}
}

func TestCorpusMissingFileScansEmpty(t *testing.T) {
	sc, err := loadCorpus(filepath.Join(t.TempDir(), "absent.txt"))
	if err != nil {
		t.Fatalf("loadCorpus on missing file: %v", err)
	}
	if sc.games != 0 || sc.torn || sc.clean != 0 || len(sc.recs) != 0 {
		t.Errorf("missing file scan = %+v, want the empty scan", sc)
	}
}

func TestCorpusUnterminatedTailIsDroppedNotParsed(t *testing.T) {
	dir := t.TempDir()
	k0 := singleStoneKey(t, 0)
	fp := corpusFingerprint(config.TierEasy.Name, 0, 1) + "\n"
	cases := []struct {
		name string
		body string
	}{
		{"tear mid record", fp + "G 0\nP " + hexKey(k0[:]) + " 9\nP " + strings.Repeat("a", 2*config.BookKeyBytes)},
		{"tear after end without newline", fp + "G 0\nP " + hexKey(k0[:]) + " 1\nE 1"},
	}
	for _, c := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".txt")
		if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", c.name, err)
		}
		sc, err := loadCorpus(path)
		if err != nil {
			t.Fatalf("%s: loadCorpus: %v", c.name, err)
		}
		if sc.games != 0 || !sc.torn || sc.clean != int64(len(fp)) || len(sc.recs) != 0 {
			t.Fatalf("%s: scan = %+v, want the game replayed from the header", c.name, sc)
		}
		var seen []gameCall
		if err := runCorpus(easyCorpusOpts(path, 1), scriptedPlayer(t, &seen), io.Discard); err != nil {
			t.Fatalf("%s: resume: %v", c.name, err)
		}
		if len(seen) != 1 || seen[0].idx != 0 {
			t.Fatalf("%s: resume played %+v, want game 0 replayed", c.name, seen)
		}
		sc, err = loadCorpus(path)
		if err != nil {
			t.Fatalf("%s: reread: %v", c.name, err)
		}
		if sc.games != 1 || sc.torn {
			t.Fatalf("%s: resumed file = %+v, want 1 complete game and no tail", c.name, sc)
		}
	}
}

func TestCorpusTearAfterCompleteGameKeepsEarlierGames(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "c.txt")
	k0 := singleStoneKey(t, 0)
	clean0 := seedHeader(t, path, corpusFingerprint(config.TierEasy.Name, 0, 2))
	clean, err := appendGame(path, clean0, 0, []posRec{{key: corpusZeroKey, move: openingMove(0)}, {key: k0, move: 1}}, 0)
	if err != nil {
		t.Fatalf("appendGame: %v", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := f.WriteString("P " + strings.Repeat("b", 2*config.BookKeyBytes)); err != nil {
		t.Fatalf("tear: %v", err)
	}
	f.Close()
	sc, err := loadCorpus(path)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	if sc.games != 1 || !sc.torn || sc.clean != clean {
		t.Fatalf("scan = %+v, want 1 game, torn tail, clean at %d", sc, clean)
	}
	var seen []gameCall
	if err := runCorpus(easyCorpusOpts(path, 2), scriptedPlayer(t, &seen), io.Discard); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if len(seen) != 1 || seen[0].idx != 1 {
		t.Fatalf("resume played %+v, want only game 1", seen)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if bytes.Contains(data, []byte(strings.Repeat("b", 8))) || bytes.Contains(data, []byte{0}) {
		t.Fatalf("resumed file keeps torn or nul bytes: %q", data)
	}
}

func TestCorpusToleratesExactlyOneTrailingBlankLine(t *testing.T) {
	dir := t.TempDir()
	k0 := singleStoneKey(t, 0)
	fp := corpusFingerprint(config.TierEasy.Name, 0, 1) + "\n"
	base := fp + "G 0\nP " + hexKey(k0[:]) + " 1\nE 1 0\n"
	cases := []struct {
		name string
		body string
		ok   bool
	}{
		{"clean file", base, true},
		{"one trailing blank", base + "\n", true},
		{"two trailing blanks", base + "\n\n", false},
	}
	for _, c := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".txt")
		if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", c.name, err)
		}
		sc, err := loadCorpus(path)
		if c.ok && (err != nil || sc.games != 1 || sc.torn) {
			t.Errorf("%s: scan = %+v err %v, want 1 clean game", c.name, sc, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s: loadCorpus accepted %q", c.name, c.body)
		}
	}
}

func TestCorpusRejectsBlankAndWhitespaceLinesWithoutPanicking(t *testing.T) {
	dir := t.TempDir()
	k0 := singleStoneKey(t, 0)
	fp := corpusFingerprint(config.TierEasy.Name, 0, 1) + "\n"
	cases := []struct {
		name string
		body string
	}{
		{"whitespace line", fp + "G 0\n \nP " + hexKey(k0[:]) + " 1\nE 1 0\n"},
		{"bare cr line", fp + "G 0\n\r\nP " + hexKey(k0[:]) + " 1\nE 1 0\n"},
		{"tab line", fp + "\t\n"},
	}
	for _, c := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".txt")
		if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", c.name, err)
		}
		_, err := loadCorpus(path)
		if err == nil {
			t.Errorf("%s: loadCorpus accepted %q", c.name, c.body)
		}
	}
}

func TestCorpusErrorsCarryPhysicalLineNumbers(t *testing.T) {
	dir := t.TempDir()
	k0 := singleStoneKey(t, 0)
	fp := corpusFingerprint(config.TierEasy.Name, 0, 1) + "\n"
	body := fp + "G 0\nP " + hexKey(k0[:]) + " 1\nP " + hexKey(k0[:]) + " 2\nQ 9\n"
	path := filepath.Join(dir, "c.txt")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := loadCorpus(path)
	if err == nil || !strings.Contains(err.Error(), "line 5") {
		t.Fatalf("error = %v, want line 5 named", err)
	}
}

func TestCorpusRunWritesAndVerifiesTheFingerprint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.txt")
	var seen []gameCall
	opts := corpusOpts{path: path, games: 2, tier: config.TierEasy.Name, tc: 0}
	if err := runCorpus(opts, scriptedPlayer(t, &seen), io.Discard); err != nil {
		t.Fatalf("runCorpus: %v", err)
	}
	sc, err := loadCorpus(path)
	if err != nil {
		t.Fatalf("loadCorpus: %v", err)
	}
	want := corpusFingerprint(config.TierEasy.Name, 0, 2)
	if sc.fp != want {
		t.Fatalf("fingerprint = %q, want %q", sc.fp, want)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(want+"\n")) {
		t.Fatalf("file does not open with the fingerprint line: %q", data)
	}
	seen = nil
	if err := runCorpus(opts, scriptedPlayer(t, &seen), io.Discard); err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if len(seen) != 0 {
		t.Fatalf("matching rerun replayed %d games", len(seen))
	}
	for _, changed := range []corpusOpts{
		{path: path, games: 3, tier: config.TierEasy.Name, tc: 0},
		{path: path, games: 2, tier: config.TierMaster.Name, tc: 0},
		{path: path, games: 2, tier: config.TierEasy.Name, tc: 1},
	} {
		if err := runCorpus(changed, scriptedPlayer(t, &seen), io.Discard); err == nil {
			t.Errorf("runCorpus accepted a changed config %+v over fingerprint %q", changed, want)
		}
	}
}

func TestCorpusRejectsMissingOrMalformedFingerprint(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		body string
	}{
		{"no fingerprint", "G 0\n"},
		{"wrong kind", "F overlay v1 floor=1 budget=1\n"},
		{"wrong version", "F corpus v2 tier=easy tc=0 games=1\n"},
		{"missing games field", "F corpus v1 tier=easy tc=0\n"},
		{"bad tc value", "F corpus v1 tier=easy tc=x games=1\n"},
	}
	for _, c := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "-")+".txt")
		if err := os.WriteFile(path, []byte(c.body), 0o644); err != nil {
			t.Fatalf("write %s: %v", c.name, err)
		}
		if _, err := loadCorpus(path); err == nil {
			t.Errorf("%s: loadCorpus accepted %q", c.name, c.body)
		}
	}
}

func TestKeyOfMatchesBookProbeGeometry(t *testing.T) {
	b := rules.NewBoard()
	for _, name := range []string{"H8", "P8", "H9", "P9"} {
		c, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		b.Make(c)
	}
	want := rules.Move(42)
	meta := bookMetaOut{
		Version: config.BookFormatVersion, MaxPly: config.BookMaxPly,
		Generator: config.BookGeneratorTag, CorpusGames: 1,
		VisitFloor: config.BookVisitFloor, Created: "2026-10-18T00:00:00Z",
	}
	entry := []bookEntry{{key: keyOf(b), move: want, visits: config.BookVisitFloor, source: config.BookSourceMajority}}
	f := bytes.NewReader(buildArtifact(entry, meta))
	bk, err := book.Load(f)
	if err != nil {
		t.Fatalf("book.Load over a keyOf-built artifact: %v", err)
	}
	got, ok := bk.Probe(b)
	if !ok || got != want {
		t.Fatalf("probe = %d/%t, want %d through the real book reader", got, ok, want)
	}
	other := rules.NewBoard()
	other.Make(rules.Cell(0))
	if _, ok := bk.Probe(other); ok {
		t.Fatalf("probe hit an unlisted position")
	}
}

func TestBoardFromKeyReplaysThePosition(t *testing.T) {
	ref := rules.NewBoard()
	for _, name := range []string{"H8", "P8", "H9", "P9", "H10"} {
		c, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ref.Make(c)
	}
	got, err := boardFromKey(keyOf(ref))
	if err != nil {
		t.Fatalf("boardFromKey: %v", err)
	}
	if got.Red != ref.Red || got.Blue != ref.Blue || got.Side != ref.Side || got.MoveCount != ref.MoveCount || got.Hash != ref.Hash {
		t.Errorf("replayed board mismatch: red %v/%v blue %v/%v side %d/%d moves %d/%d hash %#x/%#x",
			got.Red, ref.Red, got.Blue, ref.Blue, got.Side, ref.Side, got.MoveCount, ref.MoveCount, got.Hash, ref.Hash)
	}
	for _, name := range []string{"H8", "P8", "H9", "P9", "H10"} {
		c, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if got.At(c) != ref.At(c) {
			t.Errorf("cell %s = %d, want %d", name, got.At(c), ref.At(c))
		}
	}
}

func TestBoardFromKeyStartAndParity(t *testing.T) {
	empty, err := boardFromKey(keyOf(rules.NewBoard()))
	if err != nil {
		t.Fatalf("empty key: %v", err)
	}
	if empty.MoveCount != 0 || empty.Side != rules.Red {
		t.Errorf("empty key board = move %d side %d, want the start position", empty.MoveCount, empty.Side)
	}
	blueToMove := rules.NewBoard()
	for _, name := range []string{"H8", "P8", "H9"} {
		c, _ := rules.ParseCell(name)
		blueToMove.Make(c)
	}
	got, err := boardFromKey(keyOf(blueToMove))
	if err != nil {
		t.Fatalf("blue-to-move key: %v", err)
	}
	if got.Side != rules.Blue {
		t.Errorf("side to move = %d, want blue", got.Side)
	}
	var bad [config.BookKeyBytes]byte
	var blue [config.BoardWordsPerColor]uint64
	blue[0] = 1
	for i, w := range blue {
		bad[(config.BoardWordsPerColor+i)*8] = byte(w)
		bad[(config.BoardWordsPerColor+i)*8+1] = byte(w >> 8)
	}
	if _, err := boardFromKey(bad); err == nil {
		t.Errorf("boardFromKey accepted blue-ahead parity")
	}
}

func TestHexKeyRoundTrip(t *testing.T) {
	k := singleStoneKey(t, 77)
	s := hexKey(k[:])
	if len(s) != 2*config.BookKeyBytes {
		t.Fatalf("hex key length = %d, want %d", len(s), 2*config.BookKeyBytes)
	}
	back, err := keyFromHex(s)
	if err != nil {
		t.Fatalf("keyFromHex: %v", err)
	}
	if back != k {
		t.Errorf("hex round trip = %v, want %v", back, k)
	}
	if _, err := keyFromHex("ab"); !errors.Is(err, errKeyHex) {
		t.Errorf("short hex error = %v, want errKeyHex", err)
	}
}
