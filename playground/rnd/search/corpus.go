package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/bits"
	"math/rand"
	"os"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

type corpusGame struct {
	index int
	keys  [][config.BookKeyBytes]byte
	moves []rules.Move
}

func keyOfBoard(b *rules.Board) [config.BookKeyBytes]byte {
	var k [config.BookKeyBytes]byte
	for i, w := range b.Red {
		binary.LittleEndian.PutUint64(k[i*8:], w)
	}
	for i, w := range b.Blue {
		binary.LittleEndian.PutUint64(k[(config.BoardWordsPerColor+i)*8:], w)
	}
	return k
}

func cellsOf(words [config.BoardWordsPerColor]uint64) []int {
	var out []int
	for w, ws := range words {
		for ws != 0 {
			bit := ws & (-ws)
			out = append(out, w*64+bits.TrailingZeros64(bit))
			ws ^= bit
		}
	}
	return out
}

func boardFromKey(key [config.BookKeyBytes]byte) (*rules.Board, error) {
	var red, blue [config.BoardWordsPerColor]uint64
	for i := range red {
		r := binary.LittleEndian.Uint64(key[i*8:])
		bl := binary.LittleEndian.Uint64(key[(config.BoardWordsPerColor+i)*8:])
		if r&bl != 0 {
			return nil, fmt.Errorf("key overlaps red and blue on one cell")
		}
		red[i] = r
		blue[i] = bl
	}
	redCells := cellsOf(red)
	blueCells := cellsOf(blue)
	if len(blueCells) > len(redCells) || len(redCells) > len(blueCells)+1 {
		return nil, fmt.Errorf("key parity red %d blue %d impossible with red first", len(redCells), len(blueCells))
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

func loadCorpus(paths string) ([]corpusGame, error) {
	var games []corpusGame
	for _, path := range strings.Split(paths, ",") {
		gs, err := loadCorpusFile(path)
		if err != nil {
			return nil, err
		}
		for i := range gs {
			gs[i].index = len(games)
			games = append(games, gs[i])
		}
	}
	return games, nil
}

func loadCorpusFile(path string) ([]corpusGame, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var games []corpusGame
	var cur *corpusGame
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		switch fields[0] {
		case "F":
		case "G":
			if len(fields) != 2 {
				return nil, fmt.Errorf("bad G line %q", line)
			}
			idx, err := strconv.Atoi(fields[1])
			if err != nil {
				return nil, err
			}
			games = append(games, corpusGame{index: idx})
			cur = &games[len(games)-1]
		case "P":
			if cur == nil || len(fields) != 3 {
				return nil, fmt.Errorf("bad P line %q", line)
			}
			raw, err := hex.DecodeString(fields[1])
			if err != nil || len(raw) != config.BookKeyBytes {
				return nil, fmt.Errorf("bad key hex in %q", line)
			}
			var k [config.BookKeyBytes]byte
			copy(k[:], raw)
			mv, err := strconv.Atoi(fields[2])
			if err != nil {
				return nil, err
			}
			cur.keys = append(cur.keys, k)
			cur.moves = append(cur.moves, rules.Move(mv))
		case "E":
			cur = nil
		default:
			return nil, fmt.Errorf("unknown line kind %q", fields[0])
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return games, nil
}

type benchPos struct {
	game  int
	ply   int
	board *rules.Board
}

func pickPositions(games []corpusGame, want int) []benchPos {
	var out []benchPos
	for _, g := range games {
		for _, ply := range []int{positionPickA, positionPickB} {
			if ply >= len(g.keys) {
				continue
			}
			b, err := boardFromKey(g.keys[ply])
			if err != nil {
				continue
			}
			out = append(out, benchPos{game: g.index, ply: ply, board: b})
		}
		if len(out) >= want {
			break
		}
	}
	return out
}

func tacticalPositions(games []corpusGame, want int) []benchPos {
	rng := rand.New(rand.NewSource(mctsSeed))
	probe := vcf.New(vcf.KindVCF)
	var out []benchPos
	for _, g := range games {
		for _, start := range []int{positionPickA, positionPickB} {
			if start >= len(g.keys) {
				continue
			}
			b, err := boardFromKey(g.keys[start])
			if err != nil {
				continue
			}
			for range tacticalCap {
				if b.IsFull() {
					break
				}
				cands := freeInRegion(b, dilateBB(bb(b.Full), candRadiusSearch))
				if len(cands) == 0 {
					break
				}
				if hasFour(b, b.Side) {
					var stats vcf.SolverStats
					found := probe.Solve(b, solveBudget, nil, &stats)
					if (found && stats.Plies >= 5) || (!found && stats.Nodes >= 500) {
						cp := *b
						out = append(out, benchPos{game: g.index, ply: start + b.MoveCount, board: &cp})
						break
					}
				}
				var cell rules.Cell
				if rng.Intn(2) == 0 {
					cell = greedyCell(b, cands, b.Side, rng)
				} else {
					cell = cands[rng.Intn(len(cands))]
				}
				b.Make(cell)
			}
			if len(out) >= want {
				return out
			}
		}
	}
	return out
}

func hasFour(b *rules.Board, mover rules.Color) bool {
	for _, ok := range tableFours(b, mover) {
		if ok {
			return true
		}
	}
	return false
}

func greedyCell(b *rules.Board, cands []rules.Cell, mover rules.Color, rng *rand.Rand) rules.Cell {
	reach := dirReachBB(b, mover)
	best, bestScore := cands[0], -1
	for _, c := range cands {
		s := int(threatScore(b, c, mover, &reach, config.PatternClassBrokenThree))
		if s > bestScore {
			bestScore, best = s, c
		}
	}
	return best
}
