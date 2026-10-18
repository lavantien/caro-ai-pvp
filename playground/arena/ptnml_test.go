package main

import "testing"

func TestPairBucketDefinitionTable(t *testing.T) {
	cases := []struct {
		a, b Result
		want int
		why  string
	}{
		{Win, Win, PairTwoWins, "two wins"},
		{Win, Draw, PairWinDraw, "one win and one draw"},
		{Draw, Win, PairWinDraw, "one draw and one win"},
		{Win, Loss, PairTie, "one win and one loss"},
		{Loss, Win, PairTie, "one loss and one win"},
		{Draw, Draw, PairTie, "two draws"},
		{Draw, Loss, PairLossDraw, "one draw and one loss"},
		{Loss, Draw, PairLossDraw, "one loss and one draw"},
		{Loss, Loss, PairTwoLosses, "two losses"},
	}
	for _, c := range cases {
		if got := pairBucket(c.a, c.b); got != c.want {
			t.Errorf("pairBucket(%d, %d) = %d, want %d (%s)", c.a, c.b, got, c.want, c.why)
		}
	}
}

func TestPairScoresOrderMatchesBuckets(t *testing.T) {
	want := [pairBuckets]string{"2-0", "1.5-0.5", "1-1", "0.5-1.5", "0-2"}
	if pairScores != want {
		t.Errorf("pairScores = %v, want %v", pairScores, want)
	}
}

func TestAddGamePairsConsecutiveGames(t *testing.T) {
	var p Ptnml
	if _, closed := p.AddGame(Win); closed {
		t.Errorf("first game closed a pair")
	}
	bucket, closed := p.AddGame(Win)
	if !closed || bucket != PairTwoWins {
		t.Errorf("second game closed = %v bucket = %d, want true %d", closed, bucket, PairTwoWins)
	}
	if _, closed = p.AddGame(Loss); closed {
		t.Errorf("third game closed a pair")
	}
	if r, had := p.Finish(); !had || r != Loss {
		t.Errorf("Finish = (%d, %v), want (%d, true)", r, had, Loss)
	}
	if got := p.Games(); got != 3 {
		t.Errorf("Games = %d, want 3", got)
	}
	if got := p.pairs; got != 1 {
		t.Errorf("pairs = %d, want 1", got)
	}
	if got := p.unpaired; got != 1 {
		t.Errorf("unpaired = %d, want 1", got)
	}
	if p.buckets[PairTwoWins] != 1 {
		t.Errorf("two-wins bucket = %d, want 1", p.buckets[PairTwoWins])
	}
}

func TestAddGameEmptyStream(t *testing.T) {
	var p Ptnml
	if _, had := p.Finish(); had {
		t.Errorf("Finish on an empty stream reported a pending game")
	}
	want := "0-0-0-0-0 over 0 pairs, 0 unpaired"
	if got := p.Summary(); got != want {
		t.Errorf("Summary = %q, want %q", got, want)
	}
	if p.Games() != 0 || p.HalfPoints() != 0 {
		t.Errorf("Games = %d HalfPoints = %d, want 0 0", p.Games(), p.HalfPoints())
	}
}

func TestAddGameSingleGameUnpaired(t *testing.T) {
	var p Ptnml
	p.AddGame(Draw)
	p.Finish()
	if p.pairs != 0 || p.unpaired != 1 {
		t.Errorf("pairs = %d unpaired = %d, want 0 1", p.pairs, p.unpaired)
	}
	if p.HalfPoints() != 1 {
		t.Errorf("HalfPoints = %d, want the single draw's 1", p.HalfPoints())
	}
	if got := p.Summary(); got != "0-0-0-0-0 over 0 pairs, 1 unpaired" {
		t.Errorf("Summary = %q", got)
	}
}

func TestNoDrawDegenerateStreams(t *testing.T) {
	var p Ptnml
	for _, r := range []Result{Win, Loss, Win, Win, Loss, Loss} {
		p.AddGame(r)
	}
	p.Finish()
	want := "1-0-1-0-1 over 3 pairs, 0 unpaired"
	if got := p.Summary(); got != want {
		t.Errorf("decisive-only Summary = %q, want %q", got, want)
	}
	if p.buckets[PairWinDraw] != 0 || p.buckets[PairLossDraw] != 0 {
		t.Errorf("draw buckets filled without draws: %v", p.buckets)
	}
}

func TestHalfPointsAcrossAllBuckets(t *testing.T) {
	var p Ptnml
	for _, r := range []Result{Win, Win, Win, Draw, Win, Loss, Draw, Draw, Loss, Draw, Loss, Loss, Draw, Loss, Win, Draw, Loss} {
		p.AddGame(r)
	}
	p.Finish()
	if p.pairs != 8 || p.unpaired != 1 {
		t.Fatalf("pairs = %d unpaired = %d, want 8 1", p.pairs, p.unpaired)
	}
	want := 2*5 + 6
	if got := p.HalfPoints(); got != want {
		t.Errorf("HalfPoints = %d, want %d", got, want)
	}
	if got := p.pairHalfPoints(); got != want {
		t.Errorf("pairHalfPoints = %d, want %d", got, want)
	}
}

func TestFinishIdempotentThenNewPair(t *testing.T) {
	var p Ptnml
	p.AddGame(Win)
	p.Finish()
	p.Finish()
	if p.unpaired != 1 {
		t.Errorf("double Finish counted unpaired %d times", p.unpaired)
	}
	bucket, closed := p.AddGame(Win)
	if closed || bucket != 0 {
		t.Errorf("game after Finish = (%d, %v), want a fresh pending game", bucket, closed)
	}
	if _, closed = p.AddGame(Win); !closed {
		t.Errorf("second game after Finish did not close the fresh pair")
	}
	if p.pairs != 1 || p.unpaired != 1 {
		t.Errorf("pairs = %d unpaired = %d, want 1 1", p.pairs, p.unpaired)
	}
}

func TestUnpairedResultWord(t *testing.T) {
	want := [3]string{"loss", "draw", "win"}
	if resultWords != want {
		t.Errorf("resultWords = %v, want %v", resultWords, want)
	}
}
