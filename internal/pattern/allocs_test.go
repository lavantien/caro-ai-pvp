package pattern

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestZeroAllocs(t *testing.T) {
	b := seededBoard(t, 99, false)
	cell := rules.Cell(config.BoardSize / 2 * config.BoardStride)
	idx := Index(b, cell, 0, rules.Red)
	w := Unpack(idx)

	if n := testing.AllocsPerRun(100, func() { _ = Lookup(0, idx) }); n != 0 {
		t.Fatalf("Lookup: %v allocs, want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() { _ = Index(b, cell, 2, rules.Blue) }); n != 0 {
		t.Fatalf("Index: %v allocs, want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() { _ = Pack(w) }); n != 0 {
		t.Fatalf("Pack: %v allocs, want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() { _ = Unpack(idx) }); n != 0 {
		t.Fatalf("Unpack: %v allocs, want 0", n)
	}
	if n := testing.AllocsPerRun(3, func() { buildTables(&benchSink) }); n != 0 {
		t.Fatalf("buildTables: %v allocs, want 0", n)
	}
}
