package main

import "testing"

func TestParseMLineGoldens(t *testing.T) {
	cases := []struct {
		name string
		line string
		want mLine
	}{
		{
			"healthy",
			"M24, Red, J9, d=14, n=6.25m, nps=2.5m, ebf=2.1, tt=38%, hf=45%, fh1=93%, s=+150, thr=4, t=2.50, alloc=2.50, pv=J9 K10 K9 L9 M8 L8",
			mLine{move: 24, side: "Red", cell: "J9", depth: 14, nodes: 6_250_000, nps: 2_500_000,
				ebf: 2.1, tt: 38, hf: 45, fh1: 93, score: "+150", thr: 4, t: 2.50, alloc: 2.50,
				tag: "", pv: "J9 K10 K9 L9 M8 L8"},
		},
		{
			"vct hit",
			"M31, Blue, G7, d=9, n=45k, nps=1.2m, ebf=1.4, tt=18%, hf=60%, fh1=88%, s=M9, thr=4, t=0.03, alloc=3.00, [VCT], pv=G7 H7 G8 G6 G9 G10 F8 E9 I8",
			mLine{move: 31, side: "Blue", cell: "G7", depth: 9, nodes: 45_000, nps: 1_200_000,
				ebf: 1.4, tt: 18, hf: 60, fh1: 88, score: "M9", thr: 4, t: 0.03, alloc: 3.00,
				tag: "[VCT]", pv: "G7 H7 G8 G6 G9 G10 F8 E9 I8"},
		},
		{
			"ponder hit",
			"M12, Red, H10, d=16, n=18m, nps=2.8m, ebf=2.0, tt=44%, hf=78%, fh1=91%, s=-25, thr=4, t=0.01, alloc=4.00, [PONDER], pv=H10 I9 J8 K7 J10",
			mLine{move: 12, side: "Red", cell: "H10", depth: 16, nodes: 18_000_000, nps: 2_800_000,
				ebf: 2.0, tt: 44, hf: 78, fh1: 91, score: "-25", thr: 4, t: 0.01, alloc: 4.00,
				tag: "[PONDER]", pv: "H10 I9 J8 K7 J10"},
		},
		{
			"book tag wide count",
			"M2, Red, H8, d=0, n=0, nps=0, ebf=0.0, tt=0%, hf=0%, fh1=0%, s=0, thr=8, t=0.00, alloc=0.00, [BOOK], pv=H8",
			mLine{move: 2, side: "Red", cell: "H8", depth: 0, nodes: 0, nps: 0,
				ebf: 0, tt: 0, hf: 0, fh1: 0, score: "0", thr: 8, t: 0, alloc: 0,
				tag: "[BOOK]", pv: "H8"},
		},
		{
			"empty pv",
			"M3, Blue, K7, d=2, n=2500m, nps=1.0m, ebf=1.9, tt=13%, hf=32%, fh1=80%, s=-M3, thr=1, t=0.16, alloc=1.00, pv=",
			mLine{move: 3, side: "Blue", cell: "K7", depth: 2, nodes: 2_500_000_000, nps: 1_000_000,
				ebf: 1.9, tt: 13, hf: 32, fh1: 80, score: "-M3", thr: 1, t: 0.16, alloc: 1.00,
				tag: "", pv: ""},
		},
	}
	for _, c := range cases {
		got, err := parseMLine(c.line)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if got != c.want {
			t.Errorf("%s:\n got %+v\nwant %+v", c.name, got, c.want)
		}
	}
}

func TestParseMLineRejects(t *testing.T) {
	for _, line := range []string{
		"",
		"garbage",
		"M24, Red, J9, d=14",
		"M24, Green, J9, d=14, n=1k, nps=1k, ebf=1.0, tt=1%, hf=1%, fh1=1%, s=1, thr=1, t=1.00, alloc=1.00, pv=A1",
		"M24, Red, J9, d=x, n=1k, nps=1k, ebf=1.0, tt=1%, hf=1%, fh1=1%, s=1, thr=1, t=1.00, alloc=1.00, pv=A1",
		"M24, Red, Q9, d=1, n=1k, nps=1k, ebf=1.0, tt=1%, hf=1%, fh1=1%, s=1, thr=1, t=1.00, alloc=1.00, pv=A1",
	} {
		if _, err := parseMLine(line); err == nil {
			t.Errorf("line %q parsed, want rejection", line)
		}
	}
}
