package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

var errKeyHex = errors.New("bookgen: key hex is not 64 bytes")

type posRec struct {
	key  [config.BookKeyBytes]byte
	move rules.Move
}

type gameCall struct {
	idx     int
	redSeat int
}

type gameResult struct {
	records []posRec
	moves   int
	winner  string
}

type gamePlayer func(idx, redSeat int) gameResult

type corpusOpts struct {
	path  string
	tier  string
	tc    int
	games int
}

type corpusScan struct {
	games    int
	clean    int64
	torn     bool
	recs     []posRec
	fp       string
	openings map[rules.Move]int
}

var corpusZeroKey = keyOf(rules.NewBoard())

func checkTrunkDiversity(games, openings int) error {
	if games < 2 {
		return nil
	}
	want := games
	if want > len(config.BookOpeningCells) {
		want = len(config.BookOpeningCells)
	}
	if openings < want {
		return fmt.Errorf("bookgen: diversity alarm: %d distinct opening moves over %d games, the trunk collapsed", openings, games)
	}
	return nil
}

func corpusFingerprint(tier string, tc, games int) string {
	return fmt.Sprintf("F corpus v%d tier=%s tc=%d games=%d", config.BookSidecarVersion, tier, tc, games)
}

func overlayFingerprint(floor, budget int) string {
	return fmt.Sprintf("F overlay v%d floor=%d budget=%d", config.BookSidecarVersion, floor, budget)
}

func parseSidecarFingerprint(fields []string, kind string, pairs int) bool {
	if len(fields) != 3+pairs {
		return false
	}
	if fields[0] != "F" || fields[1] != kind || fields[2] != fmt.Sprintf("v%d", config.BookSidecarVersion) {
		return false
	}
	for _, f := range fields[3:] {
		eq := strings.IndexByte(f, '=')
		if eq <= 0 || eq == len(f)-1 {
			return false
		}
		if key, val := f[:eq], f[eq+1:]; key != "tier" {
			for _, ch := range val {
				if ch < '0' || ch > '9' {
					return false
				}
			}
		}
	}
	return true
}

func writeSidecarHeader(path, fp string) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("bookgen: write header %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	n, err := fmt.Fprintf(f, "%s\n", fp)
	if err != nil {
		return 0, fmt.Errorf("bookgen: write header %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		return 0, fmt.Errorf("bookgen: sync header %s: %w", path, err)
	}
	return int64(n), nil
}

func keyOf(b *rules.Board) [config.BookKeyBytes]byte {
	var k [config.BookKeyBytes]byte
	for i, w := range b.Red {
		binary.LittleEndian.PutUint64(k[i*8:], w)
	}
	for i, w := range b.Blue {
		binary.LittleEndian.PutUint64(k[(config.BoardWordsPerColor+i)*8:], w)
	}
	return k
}

func hexKey(k []byte) string { return hex.EncodeToString(k) }

func keyFromHex(s string) ([config.BookKeyBytes]byte, error) {
	var k [config.BookKeyBytes]byte
	raw, err := hex.DecodeString(s)
	if err != nil || len(raw) != config.BookKeyBytes {
		return k, errKeyHex
	}
	copy(k[:], raw)
	return k, nil
}

func boardFromKey(key [config.BookKeyBytes]byte) (*rules.Board, error) {
	var red, blue [config.BoardWordsPerColor]uint64
	for i := range red {
		if binary.LittleEndian.Uint64(key[i*8:])&binary.LittleEndian.Uint64(key[(config.BoardWordsPerColor+i)*8:]) != 0 {
			return nil, fmt.Errorf("bookgen: key overlaps red and blue on one cell")
		}
		red[i] = binary.LittleEndian.Uint64(key[i*8:])
		blue[i] = binary.LittleEndian.Uint64(key[(config.BoardWordsPerColor+i)*8:])
	}
	redCells := cellsOf(red)
	blueCells := cellsOf(blue)
	if len(blueCells) > len(redCells) || len(redCells) > len(blueCells)+1 {
		return nil, fmt.Errorf("bookgen: key parity red %d blue %d impossible with red first", len(redCells), len(blueCells))
	}
	b := rules.NewBoard()
	for r := range redCells {
		b.Make(rules.Cell(redCells[r]))
		if r < len(blueCells) {
			b.Make(rules.Cell(blueCells[r]))
		}
	}
	return b, nil
}

func cellsOf(words [config.BoardWordsPerColor]uint64) []int {
	var out []int
	for w, bits := range words {
		for bits != 0 {
			bit := bits & (-bits)
			out = append(out, w*64+bitLen64(bit-1))
			bits ^= bit
		}
	}
	sort.Ints(out)
	return out
}

func bitLen64(v uint64) int {
	n := 0
	for v != 0 {
		v >>= 1
		n++
	}
	return n
}

func loadCorpus(path string) (corpusScan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return corpusScan{}, nil
		}
		return corpusScan{}, fmt.Errorf("bookgen: read corpus %s: %w", path, err)
	}
	sc := corpusScan{}
	end := len(data)
	if end > 0 && data[end-1] != '\n' {
		sc.torn = true
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			end = i + 1
		} else {
			end = 0
		}
	}
	lineno := 0
	if len(data) > 0 {
		header := data
		if end > 0 {
			header = data[:end]
		}
		nl := bytes.IndexByte(header, '\n')
		if nl < 0 {
			nl = len(header)
		}
		if !parseSidecarFingerprint(strings.Fields(string(header[:nl])), "corpus", 3) {
			return sc, fmt.Errorf("bookgen: corpus %s line 1: missing or malformed fingerprint", path)
		}
		sc.fp = string(header[:nl])
		sc.clean = int64(nl + 1)
		lineno = 1
	}
	var block []posRec
	inBlock := false
	off := 0
	if sc.clean > 0 {
		off = int(sc.clean)
	}
	fail := func(reason string) error {
		return fmt.Errorf("bookgen: corpus %s line %d: %s", path, lineno, reason)
	}
	for off < end {
		nl := off + bytes.IndexByte(data[off:end], '\n')
		line := data[off:nl]
		off = nl + 1
		lineno++
		if len(line) == 0 {
			if off >= end {
				break
			}
			return sc, fail("empty line")
		}
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			return sc, fail("blank line")
		}
		switch fields[0] {
		case "G":
			if inBlock {
				return sc, fail("game header inside a game block")
			}
			if len(fields) != 2 {
				return sc, fail("game header wants one index")
			}
			idx, err := strconv.Atoi(fields[1])
			if err != nil || idx < 0 {
				return sc, fail("game header index")
			}
			if idx != sc.games {
				return sc, fail(fmt.Sprintf("game index %d breaks the recorded sequence at %d", idx, sc.games))
			}
			inBlock = true
			block = block[:0]
		case "P":
			if !inBlock {
				return sc, fail("position record outside a game block")
			}
			if len(fields) != 3 {
				return sc, fail("position record wants key and move")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return sc, fail("position key")
			}
			mv, err := strconv.Atoi(fields[2])
			if err != nil || mv < 0 || mv >= config.BoardCells {
				return sc, fail("position move")
			}
			block = append(block, posRec{key: k, move: rules.Move(mv)})
		case "E":
			if !inBlock {
				return sc, fail("block end outside a game block")
			}
			if len(fields) != 2 {
				return sc, fail("block end wants the record count")
			}
			n, err := strconv.Atoi(fields[1])
			if err != nil || n != len(block) {
				return sc, fail(fmt.Sprintf("block end count %d over %d records", n, len(block)))
			}
			sc.recs = append(sc.recs, block...)
			if len(block) > 0 && block[0].key == corpusZeroKey {
				if sc.openings == nil {
					sc.openings = make(map[rules.Move]int)
				}
				sc.openings[block[0].move]++
			}
			sc.games++
			sc.clean = int64(off)
			inBlock = false
			block = block[:0]
		default:
			return sc, fail(fmt.Sprintf("unknown line kind %q", fields[0]))
		}
	}
	if inBlock {
		sc.torn = true
	}
	return sc, nil
}

func appendGame(path string, clean int64, idx int, recs []posRec) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return 0, fmt.Errorf("bookgen: open corpus %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Truncate(clean); err != nil {
		return 0, fmt.Errorf("bookgen: truncate corpus %s to %d: %w", path, clean, err)
	}
	if _, err := f.Seek(clean, io.SeekStart); err != nil {
		return 0, fmt.Errorf("bookgen: seek corpus %s: %w", path, err)
	}
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "G %d\n", idx)
	for _, r := range recs {
		fmt.Fprintf(&buf, "P %s %d\n", hexKey(r.key[:]), r.move)
	}
	fmt.Fprintf(&buf, "E %d\n", len(recs))
	if _, err := f.Write(buf.Bytes()); err != nil {
		return 0, fmt.Errorf("bookgen: append game %d: %w", idx, err)
	}
	if err := f.Sync(); err != nil {
		return 0, fmt.Errorf("bookgen: sync corpus %s: %w", path, err)
	}
	return clean + int64(buf.Len()), nil
}

func runCorpus(o corpusOpts, play gamePlayer, progress io.Writer) error {
	sc, err := loadCorpus(o.path)
	if err != nil {
		return err
	}
	want := corpusFingerprint(o.tier, o.tc, o.games)
	if sc.fp == "" {
		clean, err := writeSidecarHeader(o.path, want)
		if err != nil {
			return err
		}
		sc.clean = clean
	} else if sc.fp != want {
		return fmt.Errorf("bookgen: corpus %s holds fingerprint %q, run config wants %q", o.path, sc.fp, want)
	}
	if err := checkTrunkDiversity(sc.games, len(sc.openings)); err != nil {
		return err
	}
	if sc.games >= o.games {
		fmt.Fprintf(progress, "corpus complete: %d games, %d distinct openings recorded at %s\n", sc.games, len(sc.openings), o.path)
		return nil
	}
	for i := sc.games; i < o.games; i++ {
		redSeat := i % 2
		res := play(i, redSeat)
		clean, err := appendGame(o.path, sc.clean, i, res.records)
		if err != nil {
			return err
		}
		sc.clean = clean
		sc.games = i + 1
		if len(res.records) > 0 && res.records[0].key == corpusZeroKey {
			if sc.openings == nil {
				sc.openings = make(map[rules.Move]int)
			}
			sc.openings[res.records[0].move]++
		}
		if err := checkTrunkDiversity(sc.games, len(sc.openings)); err != nil {
			return err
		}
		fmt.Fprintf(progress, "game %d/%d red seat %d winner %s moves %d positions %d openings %d\n",
			i+1, o.games, redSeat+1, res.winner, res.moves, len(res.records), len(sc.openings))
	}
	return nil
}
