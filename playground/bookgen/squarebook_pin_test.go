package main

import (
	"os"
	"sort"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/book"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type pinRow struct {
	red  []int
	blue []int
	move int
}

var squareRedTrunkRows = []pinRow{
	{red: nil, blue: nil, move: 119},
	{red: []int{136}, blue: []int{151}, move: 133},
	{red: []int{136}, blue: []int{134}, move: 101},
	{red: []int{136}, blue: []int{119}, move: 88},
}

var squareBlueTrunkRows = []pinRow{
	{red: []int{136}, blue: nil, move: 121},
	{red: []int{133, 136}, blue: []int{151}, move: 134},
}

var squareBlueTargetLines = [][]string{
	{"H8"},
	{"H8", "H6", "H5"},
	{"H8", "I7", "H5"},
}

func boardCells(t *testing.T, b *rules.Board, c rules.Color) []int {
	t.Helper()
	var out []int
	for i := 0; i < config.BoardCells; i++ {
		if b.At(rules.Cell(i)) == c {
			out = append(out, i)
		}
	}
	return out
}

func pinBoard(red, blue []int) *rules.Board {
	b := rules.NewBoard()
	for i := 0; i < len(red) || i < len(blue); i++ {
		if i < len(red) {
			b.Make(rules.Cell(red[i]))
		}
		if i < len(blue) {
			b.Make(rules.Cell(blue[i]))
		}
	}
	return b
}

func transformCells(t int, cells []int) []int {
	out := make([]int, 0, len(cells))
	for _, c := range cells {
		out = append(out, transformCell(t, c))
	}
	sort.Ints(out)
	return out
}

func playedCanonical(t *testing.T, b *rules.Board, mv rules.Move) [config.BookKeyBytes]byte {
	t.Helper()
	played := *b
	played.Make(rules.Cell(mv))
	return canonicalBoardKey(&played)
}

func loadSquareArtifact(t *testing.T) *book.Book {
	t.Helper()
	f, err := os.Open(`..\..\db\book.carobook`)
	if err != nil {
		t.Fatalf("open the square artifact: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	bk, err := book.Load(f)
	if err != nil {
		t.Fatalf("square artifact broke the CAROBOOK1 contract: %v", err)
	}
	return bk
}

func probePinned(t *testing.T, bk *book.Book, row pinRow) {
	t.Helper()
	canon := pinBoard(row.red, row.blue)
	mv, ok := bk.Probe(canon)
	if !ok {
		t.Fatalf("canonical trunk position red %v blue %v missed the book", row.red, row.blue)
	}
	if int(mv) != row.move {
		t.Fatalf("canonical trunk move red %v blue %v = %d, want %d", row.red, row.blue, int(mv), row.move)
	}
	want := playedCanonical(t, canon, mv)
	for ti := range dihedral {
		vb := pinBoard(transformCells(ti, row.red), transformCells(ti, row.blue))
		vmv, vok := bk.Probe(vb)
		if !vok {
			t.Fatalf("transform %d of red %v blue %v missed the book", ti, row.red, row.blue)
		}
		if !vb.IsLegal(rules.Cell(vmv)) {
			t.Fatalf("transform %d reply %d illegal on its board", ti, int(vmv))
		}
		if playedCanonical(t, vb, vmv) != want {
			t.Fatalf("transform %d reply %d disagrees with the canonical decision %d", ti, int(vmv), row.move)
		}
	}
}

func TestSquareArtifactPinsTheRedTrunk(t *testing.T) {
	bk := loadSquareArtifact(t)
	for _, row := range squareRedTrunkRows {
		probePinned(t, bk, row)
	}
}

func TestSquareArtifactCoversTheBlueTrunk(t *testing.T) {
	bk := loadSquareArtifact(t)
	if bk.MaxPly != config.TierMaster.BookMaxPly {
		t.Fatalf("artifact max_ply = %d, want the master bound %d", bk.MaxPly, config.TierMaster.BookMaxPly)
	}
	for _, row := range squareBlueTrunkRows {
		probePinned(t, bk, row)
	}
}

func TestSquareArtifactAnswersTheDiscriminatorBoards(t *testing.T) {
	bk := loadSquareArtifact(t)
	for _, names := range squareBlueTargetLines {
		line := make([]rules.Cell, 0, len(names))
		for _, name := range names {
			c, err := rules.ParseCell(name)
			if err != nil {
				t.Fatalf("parse cell %s: %v", name, err)
			}
			line = append(line, c)
		}
		b := rules.NewBoard()
		for _, c := range line {
			if !b.IsLegal(c) {
				t.Fatalf("cell %d illegal on the discriminator board under construction", int(c))
			}
			b.Make(c)
		}
		mv, ok := bk.Probe(b)
		if !ok {
			t.Fatalf("discriminator board %v missed the book", line)
		}
		if !b.IsLegal(rules.Cell(mv)) {
			t.Fatalf("discriminator reply %d illegal on %v", int(mv), line)
		}
		again, _ := bk.Probe(b)
		if again != mv {
			t.Fatalf("discriminator probe of %v returned %d then %d, probes must be deterministic", line, int(mv), int(again))
		}
		want := playedCanonical(t, b, mv)
		red, blue := boardCells(t, b, rules.Red), boardCells(t, b, rules.Blue)
		for ti := range dihedral {
			vb := pinBoard(transformCells(ti, red), transformCells(ti, blue))
			vmv, vok := bk.Probe(vb)
			if !vok {
				t.Fatalf("transform %d of %v missed the book", ti, line)
			}
			if playedCanonical(t, vb, vmv) != want {
				t.Fatalf("transform %d of %v replies %d, a different decision than %d", ti, line, int(vmv), int(mv))
			}
		}
	}
}

func TestSquareArtifactKeepsRedEntriesByteIdentical(t *testing.T) {
	entries, err := parseArtifactEntries(t, `..\..\db\book.carobook`)
	if err != nil {
		t.Fatal(err)
	}
	red, blue := 0, 0
	for _, e := range entries {
		b, err := boardFromKey(e.key)
		if err != nil {
			t.Fatalf("artifact entry decode: %v", err)
		}
		if b.Side == rules.Red {
			red++
		} else {
			blue++
		}
	}
	if red != 21 {
		t.Fatalf("red-side entries = %d, want the trunk's 21 unchanged", red)
	}
	if blue < 8 {
		t.Fatalf("blue-side entries = %d, want at least the move-2, move-4 trunk, and both discriminator boards", blue)
	}
	for _, e := range entries {
		if e.source != config.BookSourceMajority && e.source != config.BookSourceSolver {
			t.Fatalf("entry source %d is not a known source", e.source)
		}
	}
}
