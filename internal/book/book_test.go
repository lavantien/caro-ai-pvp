package book

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func specKey(b *rules.Board) [config.BookKeyBytes]byte {
	var k [config.BookKeyBytes]byte
	for i, w := range b.Red {
		binary.LittleEndian.PutUint64(k[i*8:], w)
	}
	for i, w := range b.Blue {
		binary.LittleEndian.PutUint64(k[(config.BoardWordsPerColor+i)*8:], w)
	}
	return k
}

func specEntry(key [config.BookKeyBytes]byte, move rules.Move, visits uint32, source uint8) []byte {
	e := make([]byte, config.BookEntryBytes)
	copy(e, key[:])
	binary.LittleEndian.PutUint16(e[config.BookKeyBytes:], uint16(move))
	binary.LittleEndian.PutUint32(e[config.BookKeyBytes+config.BookMoveBytes:], visits)
	e[config.BookKeyBytes+config.BookMoveBytes+config.BookVisitsBytes] = source
	return e
}

func specMetaLine(version int) []byte {
	return []byte(`{"version":` + strconv.Itoa(version) + `,"max_ply":16,"generator":"spec","corpus_games":` + strconv.Itoa(config.BookCorpusGames) + `,"visit_floor":` + strconv.Itoa(config.BookVisitFloor) + `,"created":"20261018-000000"}` + "\n")
}

func specBook(meta []byte, entries ...[]byte) []byte {
	out := []byte(config.BookMagicLine)
	out = append(out, meta...)
	for _, e := range entries {
		out = append(out, e...)
	}
	return out
}

func mustCell(t *testing.T, name string) rules.Cell {
	t.Helper()
	c, err := rules.ParseCell(name)
	if err != nil {
		t.Fatalf("parse cell %s: %v", name, err)
	}
	return c
}

func playedBoard(t *testing.T, names ...string) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, n := range names {
		c := mustCell(t, n)
		if !b.IsLegal(c) {
			t.Fatalf("move %s illegal on the board under construction", n)
		}
		b.Make(c)
	}
	return b
}

func mustLoad(t *testing.T, data []byte) *Book {
	t.Helper()
	bk, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Load of a spec-valid book failed: %v", err)
	}
	if bk == nil {
		t.Fatalf("Load returned nil book with nil error")
	}
	return bk
}

func TestLoadAcceptsValidBook(t *testing.T) {
	b0 := playedBoard(t)
	b1 := playedBoard(t, "A1")
	b2 := playedBoard(t, "A1", "P16", "H8")
	type row struct {
		board  *rules.Board
		move   rules.Move
		visits uint32
		source uint8
	}
	rows := []row{
		{b0, rules.Move(mustCell(t, "H8")), 300, config.BookSourceMajority},
		{b1, rules.Move(mustCell(t, "P16")), 42, config.BookSourceSolver},
		{b2, rules.Move(mustCell(t, "B2")), 300, config.BookSourceMajority},
	}
	type entry struct {
		key [config.BookKeyBytes]byte
		raw []byte
	}
	entries := make([]entry, 0, len(rows))
	for _, r := range rows {
		k := specKey(r.board)
		entries = append(entries, entry{key: k, raw: specEntry(k, r.move, r.visits, r.source)})
	}
	slices.SortFunc(entries, func(a, b entry) int {
		return bytes.Compare(a.key[:], b.key[:])
	})
	raw := make([][]byte, 0, len(entries))
	for _, e := range entries {
		raw = append(raw, e.raw)
	}
	bk := mustLoad(t, specBook(specMetaLine(config.BookFormatVersion), raw...))
	if bk.Len() != len(rows) {
		t.Errorf("Len = %d, want %d", bk.Len(), len(rows))
	}
	if bk.MaxPly != 16 || bk.Generator != "spec" || bk.CorpusGames != config.BookCorpusGames || bk.VisitFloor != config.BookVisitFloor || bk.Created != "20261018-000000" {
		t.Errorf("meta = ply %d gen %q corpus %d floor %d created %q", bk.MaxPly, bk.Generator, bk.CorpusGames, bk.VisitFloor, bk.Created)
	}
	for _, r := range rows {
		mv, ok := bk.Probe(r.board)
		if !ok {
			t.Errorf("probe of a loaded board missed: %d stones", r.board.MoveCount)
			continue
		}
		if mv != r.move {
			t.Errorf("probe move = %d, want %d", uint16(mv), uint16(r.move))
		}
		if r.visits < uint32(config.BookVisitFloor) {
			t.Errorf("test row visits %d below the config floor %d", r.visits, config.BookVisitFloor)
		}
	}
	miss := playedBoard(t, "A1", "P16", "D4")
	if mv, ok := bk.Probe(miss); ok {
		t.Errorf("probe of an unbooked board hit with move %d", uint16(mv))
	}
}

func TestLoadAcceptsEmptyBook(t *testing.T) {
	bk := mustLoad(t, specBook(specMetaLine(config.BookFormatVersion)))
	if bk.Len() != 0 {
		t.Errorf("Len = %d, want 0", bk.Len())
	}
	if _, ok := bk.Probe(playedBoard(t)); ok {
		t.Error("probe of an empty book hit")
	}
}

func TestLoadRejectsBadMagic(t *testing.T) {
	valid := specBook(specMetaLine(config.BookFormatVersion))
	cases := []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"short", []byte("CAROBOOK")},
		{"no newline", []byte(config.BookFileMagic)},
		{"wrong version tag", []byte("CAROBOOK2\n" + string(valid[len(config.BookMagicLine):]))},
		{"case bent", bytes.ToLower(valid)},
		{"crlf", []byte(config.BookFileMagic + "\r\n" + string(valid[len(config.BookMagicLine):]))},
	}
	for _, c := range cases {
		bk, err := Load(bytes.NewReader(c.data))
		if err == nil || !strings.Contains(err.Error(), "magic") {
			t.Errorf("%s: err = %v, want a magic fault", c.name, err)
		}
		if bk != nil {
			t.Errorf("%s: book non-nil on fault", c.name)
		}
	}
}

func TestLoadRejectsBadMeta(t *testing.T) {
	cases := []struct {
		name     string
		meta     []byte
		faultSub string
	}{
		{"absent", nil, "meta"},
		{"no newline", []byte(`{"version":1}`), "meta"},
		{"not json", []byte("{nope\n"), "meta"},
		{"version 2", specMetaLine(2), "version"},
		{"version 0", specMetaLine(0), "version"},
		{"version huge", specMetaLine(1 << 30), "version"},
		{"version as string", []byte(`{"version":"1"}` + "\n"), "meta"},
	}
	for _, c := range cases {
		bk, err := Load(bytes.NewReader(specBook(c.meta)))
		if err == nil || !strings.Contains(err.Error(), c.faultSub) {
			t.Errorf("%s: err = %v, want a %q fault", c.name, err, c.faultSub)
		}
		if bk != nil {
			t.Errorf("%s: book non-nil on fault", c.name)
		}
	}
}

func TestLoadRejectsTruncatedEntryRegion(t *testing.T) {
	full := specEntry([config.BookKeyBytes]byte{}, rules.Move(0), uint32(config.BookVisitFloor), config.BookSourceMajority)
	for _, n := range []int{1, config.BookEntryBytes - 1, config.BookEntryBytes + 1, 2*config.BookEntryBytes - 1, 2*config.BookEntryBytes + 3} {
		partial := make([]byte, n)
		copy(partial, append(append([]byte{}, full...), full...))
		bk, err := Load(bytes.NewReader(specBook(specMetaLine(config.BookFormatVersion), partial)))
		if err == nil || !strings.Contains(err.Error(), "multiple") {
			t.Errorf("region of %d bytes: err = %v, want an entry region multiple fault", n, err)
		}
		if bk != nil {
			t.Errorf("region of %d bytes: book non-nil on fault", n)
		}
	}
}

func TestLoadRejectsBrokenOrder(t *testing.T) {
	low := specEntry([config.BookKeyBytes]byte{}, rules.Move(0), uint32(config.BookVisitFloor), config.BookSourceMajority)
	var highKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(highKey[:], 1)
	high := specEntry(highKey, rules.Move(1), uint32(config.BookVisitFloor), config.BookSourceMajority)
	cases := []struct {
		name    string
		entries [][]byte
	}{
		{"descending", [][]byte{high, low}},
		{"duplicate", [][]byte{low, low}},
	}
	for _, c := range cases {
		bk, err := Load(bytes.NewReader(specBook(specMetaLine(config.BookFormatVersion), c.entries...)))
		if err == nil || !strings.Contains(err.Error(), "ascending") {
			t.Errorf("%s: err = %v, want a strict ascending fault", c.name, err)
		}
		if bk != nil {
			t.Errorf("%s: book non-nil on fault", c.name)
		}
	}
}

func TestLoadRejectsOutOfBoundsMove(t *testing.T) {
	for _, mv := range []rules.Move{rules.Move(config.BoardCells), rules.Move(config.BoardCells + 44), ^rules.Move(0)} {
		e := specEntry([config.BookKeyBytes]byte{}, mv, uint32(config.BookVisitFloor), config.BookSourceMajority)
		bk, err := Load(bytes.NewReader(specBook(specMetaLine(config.BookFormatVersion), e)))
		if err == nil || !strings.Contains(err.Error(), "outside") {
			t.Errorf("move %d: err = %v, want a bounds fault", uint16(mv), err)
		}
		if bk != nil {
			t.Errorf("move %d: book non-nil on fault", uint16(mv))
		}
	}
}

func TestLoadRejectsOversizedLines(t *testing.T) {
	fatMagic := bytes.Repeat([]byte("X"), config.BookMetaLineMaxBytes+1)
	magicData := append(fatMagic, '\n')
	bk, err := Load(bytes.NewReader(magicData))
	if err == nil || !strings.Contains(err.Error(), "oversized") || !strings.Contains(err.Error(), "magic") {
		t.Errorf("oversized magic: err = %v, want an oversized magic fault", err)
	}
	if bk != nil {
		t.Error("oversized magic: book non-nil on fault")
	}
	fatMeta := []byte(`{"version":1,"generator":"` + strings.Repeat("A", config.BookMetaLineMaxBytes) + `"}` + "\n")
	bk, err = Load(bytes.NewReader(specBook(fatMeta)))
	if err == nil || !strings.Contains(err.Error(), "oversized") || !strings.Contains(err.Error(), "meta") {
		t.Errorf("oversized meta: err = %v, want an oversized meta fault", err)
	}
	if bk != nil {
		t.Error("oversized meta: book non-nil on fault")
	}
}

func TestLoadAcceptsThousandEntryBook(t *testing.T) {
	const entries = 1000
	raw := make([][]byte, 0, entries)
	for i := 1; i <= entries; i++ {
		var k [config.BookKeyBytes]byte
		k[62] = byte(i >> 8)
		k[63] = byte(i)
		raw = append(raw, specEntry(k, rules.Move(i%8), uint32(config.BookVisitFloor), config.BookSourceMajority))
	}
	bk := mustLoad(t, specBook(specMetaLine(config.BookFormatVersion), raw...))
	if bk.Len() != entries {
		t.Fatalf("Len = %d, want %d", bk.Len(), entries)
	}
	b := rules.NewBoard()
	b.Blue[3] = 0x0102 << 48
	if mv, ok := bk.Probe(b); !ok || int(mv) != 513%8 {
		t.Errorf("probe of entry 513 = (%d, %t), want (%d, true)", uint16(mv), ok, 513%8)
	}
}

func TestLoadRejectsOccupiedMove(t *testing.T) {
	var redKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(redKey[:], uint64(1)<<17)
	var blueKey [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(blueKey[config.BoardWordsPerColor*8:], uint64(1)<<17)
	cases := map[string][]byte{
		"red plane":  specEntry(redKey, rules.Move(17), uint32(config.BookVisitFloor), config.BookSourceMajority),
		"blue plane": specEntry(blueKey, rules.Move(17), uint32(config.BookVisitFloor), config.BookSourceMajority),
	}
	for name, e := range cases {
		bk, err := Load(bytes.NewReader(specBook(specMetaLine(config.BookFormatVersion), e)))
		if err == nil || !strings.Contains(err.Error(), "occupied") {
			t.Errorf("%s: err = %v, want an occupied-move fault", name, err)
		}
		if bk != nil {
			t.Errorf("%s: book non-nil on fault", name)
		}
	}
}

func TestLoadRejectsBadSource(t *testing.T) {
	for _, src := range []uint8{0, 3, 255} {
		e := specEntry([config.BookKeyBytes]byte{}, 0, uint32(config.BookVisitFloor), src)
		bk, err := Load(bytes.NewReader(specBook(specMetaLine(config.BookFormatVersion), e)))
		if err == nil || !strings.Contains(err.Error(), "source") {
			t.Errorf("source %d: err = %v, want a source fault", src, err)
		}
		if bk != nil {
			t.Errorf("source %d: book non-nil on fault", src)
		}
	}
}

func TestLoadRejectsNonzeroReserved(t *testing.T) {
	e := specEntry([config.BookKeyBytes]byte{}, 0, uint32(config.BookVisitFloor), config.BookSourceMajority)
	e[config.BookEntryBytes-1] = 7
	bk, err := Load(bytes.NewReader(specBook(specMetaLine(config.BookFormatVersion), e)))
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Errorf("err = %v, want a reserved-bytes fault", err)
	}
	if bk != nil {
		t.Error("book non-nil on fault")
	}
}

func TestLoadRejectsMetaSchemaFaults(t *testing.T) {
	base := string(specMetaLine(config.BookFormatVersion))
	versionField := `"version":` + strconv.Itoa(config.BookFormatVersion)
	corpusField := `"corpus_games":` + strconv.Itoa(config.BookCorpusGames)
	floorField := `"visit_floor":` + strconv.Itoa(config.BookVisitFloor)
	cases := []struct {
		name     string
		meta     string
		faultSub string
	}{
		{"missing version", strings.Replace(base, versionField+",", "", 1), "misses field"},
		{"missing created", strings.Replace(base, `,"created":"20261018-000000"`, "", 1), "misses field"},
		{"case bent", strings.Replace(base, `"version":`, `"Version":`, 1), "misses field"},
		{"unknown field", strings.Replace(base, "{", `{"extra":1,`, 1), "outside the contract"},
		{"negative max_ply", strings.Replace(base, `"max_ply":16`, `"max_ply":-16`, 1), "negative"},
		{"negative corpus_games", strings.Replace(base, corpusField, `"corpus_games":-1`, 1), "negative"},
		{"negative visit_floor", strings.Replace(base, floorField, `"visit_floor":-1`, 1), "negative"},
		{"version wrong type", strings.Replace(base, versionField, `"version":"x"`, 1), "decode"},
		{"duplicate max_ply", strings.Replace(base, `"max_ply":16`, `"max_ply":16,"max_ply":17`, 1), "repeats"},
		{"duplicate version", strings.Replace(base, versionField+",", versionField+","+versionField+",", 1), "repeats"},
		{"duplicate created", strings.Replace(base, `"created":"20261018-000000"`, `"created":"20261018-000000","created":"x"`, 1), "repeats"},
		{"escaped duplicate version", strings.Replace(base, versionField+",", versionField+`,"version":2,`, 1), "repeats"},
		{"content after the object", strings.Replace(base, "}\n", "} junk\n", 1), "meta"},
	}
	for _, c := range cases {
		bk, err := Load(bytes.NewReader(specBook([]byte(c.meta))))
		if err == nil || !strings.Contains(err.Error(), c.faultSub) {
			t.Errorf("%s: err = %v, want a %q fault", c.name, err, c.faultSub)
		}
		if bk != nil {
			t.Errorf("%s: book non-nil on fault", c.name)
		}
	}
}

func TestLoadThroughCallerBufio(t *testing.T) {
	valid := specBook(specMetaLine(config.BookFormatVersion), specEntry([config.BookKeyBytes]byte{}, 0, uint32(config.BookVisitFloor), config.BookSourceMajority))
	fatMeta := []byte(config.BookMagicLine + `{"version":` + strconv.Itoa(config.BookFormatVersion) + `,"generator":"` + strings.Repeat("A", 2000) + `"}` + "\n")
	for _, size := range []int{16, 90, 512, 4096} {
		bk, err := Load(bufio.NewReaderSize(bytes.NewReader(valid), size))
		if err != nil || bk == nil || bk.Len() != 1 {
			t.Errorf("buffer %d: valid book load = (%d entries, %v), want 1 entry, nil error", size, bk.Len(), err)
		}
		bk, err = Load(bufio.NewReaderSize(bytes.NewReader(fatMeta), size))
		if err == nil || !strings.Contains(err.Error(), "oversized") {
			t.Errorf("buffer %d: oversized meta err = %v, want the cap to hold regardless of the caller buffer", size, err)
		}
		if bk != nil {
			t.Errorf("buffer %d: book non-nil on fault", size)
		}
	}
}

func TestProbeIsOrderIndependentAndIgnoresHash(t *testing.T) {
	var k [config.BookKeyBytes]byte
	binary.LittleEndian.PutUint64(k[:], 1)
	want := rules.Move(mustCell(t, "B2"))
	data := specBook(specMetaLine(config.BookFormatVersion), specEntry(k, want, uint32(config.BookVisitFloor), config.BookSourceMajority))
	bk := mustLoad(t, data)
	forward := playedBoard(t, "A1")
	backward := rules.NewBoard()
	backward.Red[0] |= 1
	if mv, ok := bk.Probe(forward); !ok || mv != want {
		t.Errorf("forward probe = (%d, %t), want (%d, true)", uint16(mv), ok, uint16(want))
	}
	if mv, ok := bk.Probe(backward); !ok || mv != want {
		t.Errorf("stone-mask probe = (%d, %t), want (%d, true): the key is the mask, not the move order", uint16(mv), ok, uint16(want))
	}
	before := forward.Hash
	forward.Hash = 0xDEADBEEF
	if mv, ok := bk.Probe(forward); !ok || mv != want {
		t.Errorf("probe after hash corruption = (%d, %t), want (%d, true): keys are not zobrist", uint16(mv), ok, uint16(want))
	}
	forward.Hash = before
}

func TestProbeNilBookMisses(t *testing.T) {
	var bk *Book
	if mv, ok := bk.Probe(playedBoard(t)); ok {
		t.Errorf("nil book probe = (%d, true), want a miss", uint16(mv))
	}
	loaded := mustLoad(t, specBook(specMetaLine(config.BookFormatVersion)))
	if mv, ok := loaded.Probe(nil); ok {
		t.Errorf("nil board probe = (%d, true), want a miss", uint16(mv))
	}
}

func TestProbeAllocsPerCall(t *testing.T) {
	entries := make([][]byte, 0, 64)
	for i := 1; i <= 64; i++ {
		var k [config.BookKeyBytes]byte
		binary.LittleEndian.PutUint64(k[:], uint64(i))
		entries = append(entries, specEntry(k, rules.Move(i), uint32(config.BookVisitFloor), config.BookSourceMajority))
	}
	bk := mustLoad(t, specBook(specMetaLine(config.BookFormatVersion), entries...))
	b := playedBoard(t, "B1")
	if _, ok := bk.Probe(b); !ok {
		t.Fatalf("probe of booked board missed before the alloc measurement")
	}
	if n := testing.AllocsPerRun(200, func() {
		_, _ = bk.Probe(b)
	}); n != 0 {
		t.Fatalf("probe: %v allocs, want 0", n)
	}
}
