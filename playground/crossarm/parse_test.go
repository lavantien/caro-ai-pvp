package main

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestParseMLine(t *testing.T) {
	cases := []struct {
		name string
		line string
		want mLine
	}{
		{
			name: "plain",
			line: "M2, Blue, H6, d=10, n=9.38m, nps=4.7m, ebf=5.0, tt=10%, hf=5%, fh1=92%, s=+300, thr=4, t=2.00, alloc=2.00, pv=H6 H5",
			want: mLine{move: 2, side: "Blue", depth: 10, nodes: 9380000, nps: 4700000, score: 300, t: 2, alloc: 2},
		},
		{
			name: "ponder tagged",
			line: "M12, Red, K8, d=12, n=9.09m, nps=5.97m, ebf=3.8, tt=16%, hf=3%, fh1=89%, s=M4, thr=2, t=0.00, alloc=4.00, [PONDER], pv=K8",
			want: mLine{move: 12, side: "Red", depth: 12, nodes: 9090000, nps: 5970000, score: config.EvalMateMax - 4*config.EvalMateScoreStep, t: 0, alloc: 4, tag: "[PONDER]"},
		},
		{
			name: "negative mate",
			line: "M9, Blue, I5, d=10, n=5.15m, nps=5.15m, ebf=4.7, tt=13%, hf=4%, fh1=94%, s=-M3, thr=1, t=1.00, alloc=2.00, pv=",
			want: mLine{move: 9, side: "Blue", depth: 10, nodes: 5150000, nps: 5150000, score: -(config.EvalMateMax - 3*config.EvalMateScoreStep), t: 1, alloc: 2},
		},
	}
	for _, c := range cases {
		got, ok := parseMLine(c.line)
		if !ok {
			t.Fatalf("%s: parse failed", c.name)
		}
		if got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
	bad := []string{
		"M2, Blue, H6, d=10, n=9.38m",
		"X2, Blue, H6, d=10, n=9.38m, nps=4.7m, ebf=5.0, tt=10%, hf=5%, fh1=92%, s=+300, thr=4, t=2.00, alloc=2.00, pv=",
		"M2, Blue, H6, d=10, n=9.38m, nps=4.7m, ebf=5.0, tt=10%, hf=5%, fh1=92%, s=+300, thr=4, t=2.00, alloc=2.00, [BOGUS], pv=",
		"M2, Green, H6, d=10, n=9.38m, nps=4.7m, ebf=5.0, tt=10%, hf=5%, fh1=92%, s=+300, thr=4, t=2.00, alloc=2.00, pv=",
	}
	for _, line := range bad {
		if _, ok := parseMLine(line); ok {
			t.Errorf("accepted bad line: %s", line)
		}
	}
}

func TestParseGameLine(t *testing.T) {
	cases := []struct {
		line string
		want gameLine
	}{
		{
			line: "game 1: hard-2 (red) beat easy-1 (blue), 41 moves, won by open 4",
			want: gameLine{game: 1, outcome: outcomeRed, moves: 41, wonBy: wonByOpenFour},
		},
		{
			line: "game 2: easy-1 (blue) beat hard-2 (red), 55 moves, won by double 4",
			want: gameLine{game: 2, outcome: outcomeBlue, moves: 55, wonBy: wonByDoubleFour},
		},
		{
			line: "game 3: hard-2 (red) vs easy-1 (blue), drawn at 120 moves",
			want: gameLine{game: 3, outcome: outcomeDraw, moves: 120},
		},
		{
			line: "game 2: hard-2 (red) beat easy-1 (blue), 41 moves",
			want: gameLine{game: 2, outcome: outcomeRed, moves: 41},
		},
	}
	for _, c := range cases {
		got, ok := parseGameLine(c.line)
		if !ok {
			t.Fatalf("parse failed: %s", c.line)
		}
		if got != c.want {
			t.Errorf("got %+v want %+v", got, c.want)
		}
	}
	if _, ok := parseGameLine("game 1: hard-2 (red) beat easy-1 (blue), won by open 4"); ok {
		t.Error("accepted game line without a move count")
	}
}

func TestParseVerdictAndHeader(t *testing.T) {
	v, ok := parseVerdictLine("series master-2 2-1")
	if !ok || v.winner != "master-2" || v.firstWins != 2 || v.secondWins != 1 {
		t.Errorf("got %+v ok %v", v, ok)
	}
	rn, rt, bn, bt, ok := parsePairingLine("pairing easy-2 (easy) vs hard-1 (hard)")
	if !ok || rn != "easy-2" || rt != "easy" || bn != "hard-1" || bt != "hard" {
		t.Errorf("got %s %s %s %s ok %v", rn, rt, bn, bt, ok)
	}
	initial, inc, bo, ok := parseTCLine("tc 3+2 bo3")
	if !ok || initial != 3 || inc != 2 || bo != 3 {
		t.Errorf("got %d %d %d ok %v", initial, inc, bo, ok)
	}
	if _, _, _, _, ok := parsePairingLine("pairing easy-2 easy vs hard-1 (hard)"); ok {
		t.Error("accepted pairing without a red tier")
	}
}
