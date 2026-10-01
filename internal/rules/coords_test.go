package rules

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestCellNameKnown(t *testing.T) {
	cases := []struct {
		cell Cell
		name string
	}{
		{0, "A1"},
		{1, "B1"},
		{config.BoardStride - 1, "P1"},
		{config.BoardStride, "A2"},
		{config.BoardStride + 1, "B2"},
		{9 * config.BoardStride, "A10"},
		{15*config.BoardStride + 15, "P16"},
	}
	for _, tc := range cases {
		got, err := CellName(tc.cell)
		if err != nil {
			t.Fatalf("CellName(%d): %v", tc.cell, err)
		}
		if got != tc.name {
			t.Fatalf("CellName(%d) = %q, want %q", tc.cell, got, tc.name)
		}
	}
}

func TestCellNameRoundTrip(t *testing.T) {
	for cell := range config.BoardCells {
		name, err := CellName(Cell(cell))
		if err != nil {
			t.Fatalf("CellName(%d): %v", cell, err)
		}
		wantLen := 2
		if cell/config.BoardStride >= 9 {
			wantLen = 3
		}
		if len(name) != wantLen {
			t.Fatalf("CellName(%d) = %q: want %d chars", cell, name, wantLen)
		}
		back, err := ParseCell(name)
		if err != nil {
			t.Fatalf("ParseCell(%q): %v", name, err)
		}
		if back != Cell(cell) {
			t.Fatalf("round trip %q: got cell %d, want %d", name, back, cell)
		}
	}
}

func TestParseCellKnown(t *testing.T) {
	cases := []struct {
		name string
		cell Cell
	}{
		{"A1", 0},
		{"P1", config.BoardStride - 1},
		{"A2", config.BoardStride},
		{"A10", 9 * config.BoardStride},
		{"P16", config.BoardCells - 1},
		{"H8", 7*config.BoardStride + 7},
	}
	for _, tc := range cases {
		got, err := ParseCell(tc.name)
		if err != nil {
			t.Fatalf("ParseCell(%q): %v", tc.name, err)
		}
		if got != tc.cell {
			t.Fatalf("ParseCell(%q) = %d, want %d", tc.name, got, tc.cell)
		}
	}
}

func TestParseCellInvalid(t *testing.T) {
	for _, name := range []string{
		"", "A", "A123", "A16x",
		"Q1", "Z16", "a1", "p1", "@1",
		"A1b", "A-1", "A 1", "A+1",
		"A0", "A00", "A01", "P00",
		"A17", "P17", "A99",
	} {
		if _, err := ParseCell(name); err == nil {
			t.Fatalf("ParseCell(%q): want error", name)
		}
	}
}

func TestCellNameInvalid(t *testing.T) {
	for _, cell := range []Cell{config.BoardCells, config.BoardCells + 1, 65535} {
		if _, err := CellName(cell); err == nil {
			t.Fatalf("CellName(%d): want error", cell)
		}
	}
}
