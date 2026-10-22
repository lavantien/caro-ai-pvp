package main

import (
	"strings"
	"testing"
)

const arenaFixture = `arena tc 3+2 seats [master master-novct] games 3
g1 M1, Red, H8, d=0, n=0
g1 M2, Blue, H6, d=10, n=40
g1 M3, Red, H5, d=11, n=33
g1 M4, Blue, G6, d=9, n=36
game 1: master (red) vs master-novct, winner master over 42 moves
g1 ponder master m3 adopt
g1 ponder master-novct m4 reject
game 2: master-novct (red) vs master, winner draw over 256 moves
g2 ponder master m5 miss
g3 M1, Red, H8, d=0, n=0
g3 M2, Blue, H6, d=10, n=40
game 3: master (red) vs master-novct, winner master-novct over 51 moves
`

func TestParseArenaFields(t *testing.T) {
	af, err := parseArena(strings.NewReader(arenaFixture), "x.log")
	if err != nil {
		t.Fatal(err)
	}
	if len(af.Games) != 3 {
		t.Fatalf("games = %d, want 3", len(af.Games))
	}
	g1 := af.Games[0]
	if g1.TC != "3+2" || g1.Red != "master" || g1.Blue != "master-novct" || g1.Winner != "master" || g1.Moves != 42 {
		t.Fatalf("g1 = %+v", g1)
	}
	if af.Games[1].Winner != "draw" || af.Games[1].Moves != 256 {
		t.Fatalf("g2 = %+v", af.Games[1])
	}
	if af.Games[2].Winner != "master-novct" {
		t.Fatalf("g3 = %+v", af.Games[2])
	}
}

func TestParseArenaMLineCapture(t *testing.T) {
	af, err := parseArena(strings.NewReader(arenaFixture), "x.log")
	if err != nil {
		t.Fatal(err)
	}
	g1 := af.Games[0]
	if !g1.HasSeq || g1.Opening != "H8 H6 H5" || g1.Seq != "H8 H6 H5 G6" {
		t.Fatalf("g1 seq capture = %+v", g1)
	}
	g3 := af.Games[2]
	if !g3.HasSeq || g3.Opening != "H8 H6" || g3.Seq != "H8 H6" {
		t.Fatalf("g3 short seq = %+v", g3)
	}
	if af.Games[1].HasSeq {
		t.Fatalf("g2 must have no seq")
	}
}

func TestParseArenaPonderCounts(t *testing.T) {
	af, err := parseArena(strings.NewReader(arenaFixture), "x.log")
	if err != nil {
		t.Fatal(err)
	}
	m := af.Ponder["master"]
	if m == nil || m.Arms != 2 || m.Adopt != 1 || m.Reject != 0 || m.Miss != 1 {
		t.Fatalf("master ponder = %+v", m)
	}
	n := af.Ponder["master-novct"]
	if n == nil || n.Arms != 1 || n.Reject != 1 {
		t.Fatalf("master-novct ponder = %+v", n)
	}
}

func TestBucketOf(t *testing.T) {
	cases := []struct {
		red, blue, want string
	}{
		{"master", "master", bucketIdentical},
		{"master", "master-novct", bucketSameBase},
		{"grandmaster", "grandmaster-noponder", bucketSameBase},
		{"master", "hard", bucketCross},
		{"easy", "medium", bucketCross},
		{"hard-tt1g", "hard", bucketSameBase},
	}
	for _, c := range cases {
		if got := bucketOf(c.red, c.blue); got != c.want {
			t.Errorf("bucketOf(%q, %q) = %q, want %q", c.red, c.blue, got, c.want)
		}
	}
}
