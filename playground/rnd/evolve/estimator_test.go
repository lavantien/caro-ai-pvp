package main

import (
	"math"
	"testing"
)

func TestSwissPairsValid(t *testing.T) {
	pairs := swissPairs(12, 4, newRng(6))
	if len(pairs) == 0 {
		t.Fatal("no pairs")
	}
	degree := make([]int, 12)
	for _, p := range pairs {
		if p[0] >= p[1] {
			t.Fatalf("unordered pair %v", p)
		}
		if p[0] < 0 || p[1] >= 12 {
			t.Fatalf("out of range %v", p)
		}
		degree[p[0]]++
		degree[p[1]]++
	}
	for i, d := range degree {
		if d == 0 {
			t.Fatalf("isolated node %d", i)
		}
	}
}

func TestSprtBoundsSigns(t *testing.T) {
	lo, hi := sprtBounds()
	if lo >= 0 || hi <= 0 {
		t.Fatalf("bounds %v %v", lo, hi)
	}
}

func TestPlayPairSprtStopsFast(t *testing.T) {
	g := newGameObj(6, marginStrong, 9)
	lo, hi := g.Lo(), g.Hi()
	strong := make([]float64, 6)
	weak := make([]float64, 6)
	for i := range strong {
		strong[i] = lo[i] + 0.5*(hi[i]-lo[i])
		weak[i] = hi[i]
	}
	strong[0] = 0
	rng := newRng(10)
	alwaysWin := make([]float64, sprtMaxGames)
	r := playPairSprt(g, strong, weak, alwaysWin, rng)
	if r.games >= sprtMaxGames {
		t.Fatalf("dominant pair burned %v games", r.games)
	}
	if r.wins != r.games {
		t.Fatalf("u=0 lost a dominated game: %v of %v", r.wins, r.games)
	}
	alwaysLose := make([]float64, sprtMaxGames)
	for i := range alwaysLose {
		alwaysLose[i] = 1
	}
	r2 := playPairSprt(g, weak, strong, alwaysLose, rng)
	if r2.games >= sprtMaxGames {
		t.Fatalf("dominated pair burned %v games", r2.games)
	}
	if r2.wins != 0 {
		t.Fatalf("u=1 won a dominated game")
	}
}

func TestPlayPairFixedBudget(t *testing.T) {
	g := newGameObj(6, marginMid, 12)
	x := make([]float64, 6)
	r := playPairFixed(g, x, append([]float64{}, x...), nil, newRng(1))
	if r.games != gamesPerCandidate {
		t.Fatalf("fixed games %v", r.games)
	}
	if r.wins < 0 || r.wins > r.games {
		t.Fatal("wins out of range")
	}
}

func TestBtFitRecoversOrdering(t *testing.T) {
	n := 5
	strengths := []float64{2, 1, 0, -1, -2}
	res := map[[2]int]pairResult{}
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			p := sigmoidStable(strengths[i] - strengths[j])
			games := 60.0
			w := p * games
			res[[2]int{i, j}] = pairResult{wins: w, games: games}
		}
	}
	s := btFit(n, res)
	for i := 0; i+1 < n; i++ {
		if s[i] <= s[i+1] {
			t.Fatalf("ordering lost %v", s)
		}
	}
	if math.Abs(s[0]-s[n-1]) < 1 {
		t.Fatalf("spread collapsed %v", s)
	}
}

func TestRankGenerationImprovesPool(t *testing.T) {
	c := gameCfg("shade+btsprt")
	c.Estimator = "btsprt"
	o := newGameObj(6, marginStrong, 21)
	e := &engine{cfg: c, obj: o, rng: newRng(21)}
	e.lo, e.hi = o.Lo(), o.Hi()
	s := newShadeState(c.Pop, o.Dim(), e.lo, e.hi)
	pop := make([][]float64, c.Pop)
	for i := range pop {
		pop[i] = uniformIn(e.rng, e.lo, e.hi)
	}
	for gen := 0; gen < 12; gen++ {
		pop, _, _ = e.rankGeneration(pop, s)
	}
	losses := make([]float64, len(pop))
	for i, x := range pop {
		losses[i] = o.True(x)
	}
	med := medianFloat(losses)
	if med > 0.5 {
		t.Fatalf("median true loss after evolution %v", med)
	}
	if o.Evals() > c.Budget {
		t.Fatal("budget blown")
	}
}
