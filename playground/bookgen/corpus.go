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
	games int
}

type corpusScan struct {
	games int
	clean int64
	torn  bool
	recs  []posRec
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
	var block []posRec
	inBlock := false
	off := 0
	fail := func(line int, reason string) error {
		return fmt.Errorf("bookgen: corpus %s line %d: %s", path, line, reason)
	}
	for len(data) > 0 {
		nl := bytes.IndexByte(data, '\n')
		var line []byte
		if nl < 0 {
			line, data = data, nil
		} else {
			line, data = data[:nl], data[nl+1:]
		}
		off += len(line) + 1
		lineno := sc.games + len(block) + 1
		if len(line) == 0 {
			return sc, fail(lineno, "empty line")
		}
		fields := strings.Fields(string(line))
		switch fields[0] {
		case "G":
			if inBlock {
				return sc, fail(lineno, "game header inside a game block")
			}
			if len(fields) != 2 {
				return sc, fail(lineno, "game header wants one index")
			}
			idx, err := strconv.Atoi(fields[1])
			if err != nil || idx < 0 {
				return sc, fail(lineno, "game header index")
			}
			if idx != sc.games {
				return sc, fail(lineno, fmt.Sprintf("game index %d breaks the recorded sequence at %d", idx, sc.games))
			}
			inBlock = true
			block = block[:0]
		case "P":
			if !inBlock {
				return sc, fail(lineno, "position record outside a game block")
			}
			if len(fields) != 3 {
				return sc, fail(lineno, "position record wants key and move")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return sc, fail(lineno, "position key")
			}
			mv, err := strconv.Atoi(fields[2])
			if err != nil || mv < 0 || mv >= config.BoardCells {
				return sc, fail(lineno, "position move")
			}
			block = append(block, posRec{key: k, move: rules.Move(mv)})
		case "E":
			if !inBlock {
				return sc, fail(lineno, "block end outside a game block")
			}
			if len(fields) != 2 {
				return sc, fail(lineno, "block end wants the record count")
			}
			n, err := strconv.Atoi(fields[1])
			if err != nil || n != len(block) {
				return sc, fail(lineno, fmt.Sprintf("block end count %d over %d records", n, len(block)))
			}
			sc.recs = append(sc.recs, block...)
			sc.games++
			sc.clean = int64(off)
			inBlock = false
			block = block[:0]
		default:
			return sc, fail(lineno, fmt.Sprintf("unknown line kind %q", fields[0]))
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
	if sc.games >= o.games {
		fmt.Fprintf(progress, "corpus complete: %d games recorded at %s\n", sc.games, o.path)
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
		fmt.Fprintf(progress, "game %d/%d red seat %d winner %s moves %d positions %d\n",
			i+1, o.games, redSeat+1, res.winner, res.moves, len(res.records))
	}
	return nil
}
