package main

import (
	"strings"
	"testing"
)

const corpusFixture = `F corpus v2 tier=master tc=3 games=1500
G 0
P 00000000000000000000000000000000 119
P 00000000000000000000000000000001 119
P 00000000000000000000000000000002 87
E 3 0
G 1
P 00000000000000000000000000000000 119
P 00000000000000000000000000000001 104
P 00000000000000000000000000000003 87
P 00000000000000000000000000000004 88
E 4 0
`

func TestParseCorpusOnceGames(t *testing.T) {
	cs := &corpusStats{Keys: map[string]*keyAgg{}, PlyUniq: map[int]int{}, PlyVisits: map[int]int{}}
	cf, err := parseCorpusOnce(strings.NewReader(corpusFixture), "corpus.txt", cs)
	if err != nil {
		t.Fatal(err)
	}
	if cf.Fingerprint != "corpus v2 tier=master tc=3 games=1500" {
		t.Fatalf("fingerprint = %q", cf.Fingerprint)
	}
	if len(cf.Games) != 2 || cf.Games[0].Plies != 3 || cf.Games[1].Plies != 4 {
		t.Fatalf("games = %+v", cf.Games)
	}
}

func TestParseCorpusOnceAggregates(t *testing.T) {
	cs := &corpusStats{Keys: map[string]*keyAgg{}, PlyUniq: map[int]int{}, PlyVisits: map[int]int{}}
	if _, err := parseCorpusOnce(strings.NewReader(corpusFixture), "corpus.txt", cs); err != nil {
		t.Fatal(err)
	}
	if cs.TotalP != 7 {
		t.Fatalf("totalP = %d, want 7", cs.TotalP)
	}
	if len(cs.Keys) != 5 {
		t.Fatalf("keys = %d, want 5", len(cs.Keys))
	}
	root := cs.Keys["00000000000000000000000000000000"]
	if root.Visits != 2 || len(root.Moves) != 1 || root.Moves["119"] != 2 || len(root.Games) != 2 {
		t.Fatalf("root agg = %+v", root)
	}
	if cs.PlyUniq[0] != 1 || cs.PlyVisits[0] != 2 {
		t.Fatalf("ply 0 uniq %d visits %d", cs.PlyUniq[0], cs.PlyVisits[0])
	}
	if cs.MaxPly != 3 {
		t.Fatalf("maxPly = %d, want 3", cs.MaxPly)
	}
}

func TestTopCounts(t *testing.T) {
	a := &keyAgg{Moves: map[string]int{"a": 3, "b": 7, "c": 1}}
	counts := a.topCounts()
	if len(counts) != 3 || counts[0] != 7 || counts[1] != 3 || counts[2] != 1 {
		t.Fatalf("counts = %v", counts)
	}
}
