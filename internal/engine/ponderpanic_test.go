package engine

import (
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type panickingPonderDeadline struct{}

func (panickingPonderDeadline) Exceeded() bool { panic("engine: injected ponder panic") }
func (panickingPonderDeadline) Stop()          {}
func (panickingPonderDeadline) Reset()         {}

func TestEngineStartPonderRecoversInjectedPanic(t *testing.T) {
	e := New(testTTBytes)
	defer e.Close()
	e.ponderDL = panickingPonderDeadline{}
	e.StartPonder(midgameBoard(t))
	mv, stats := e.StopPonder()
	if stats.PanickedWorkers != 1 {
		t.Fatalf("recovered ponder panics = %d, want 1", stats.PanickedWorkers)
	}
	if !strings.Contains(stats.PanicNote, "injected ponder panic") {
		t.Fatalf("panic note = %q, want the injected panic value", stats.PanicNote)
	}
	if mv != moveNone {
		t.Fatalf("ponder move after the recovered panic = %d, want moveNone", mv)
	}
	if mv2, stats2 := e.StopPonder(); mv2 != 0 || stats2 != (SearchStats{}) {
		t.Fatalf("second StopPonder = %d %+v, want zero values", mv2, stats2)
	}
	b := midgameBoard(t)
	mv3, stats3 := e.Search(b, cappedNodeBudget(50_000))
	if !b.IsLegal(rules.Cell(mv3)) {
		t.Fatalf("post-panic search move %d illegal", mv3)
	}
	if stats3.PanickedWorkers != 0 {
		t.Fatalf("post-panic search reported %d panicked workers, want 0", stats3.PanickedWorkers)
	}
}
