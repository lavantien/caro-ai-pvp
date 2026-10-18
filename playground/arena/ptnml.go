package main

import "fmt"

const pairBuckets = 5

type Result int

const (
	Loss Result = iota
	Draw
	Win
)

var resultWords = [3]string{"loss", "draw", "win"}

const (
	PairTwoWins = iota
	PairWinDraw
	PairTie
	PairLossDraw
	PairTwoLosses
)

var pairScores = [pairBuckets]string{"2-0", "1.5-0.5", "1-1", "0.5-1.5", "0-0"}

type Ptnml struct {
	buckets      [pairBuckets]int
	pairs        int
	unpaired     int
	unpairedHalf int
	pending      Result
	open         bool
}

func pairBucket(a, b Result) int {
	return PairTwoLosses - int(a) - int(b)
}

func (p *Ptnml) AddGame(r Result) (int, bool) {
	if p.open {
		p.open = false
		bucket := pairBucket(p.pending, r)
		p.buckets[bucket]++
		p.pairs++
		return bucket, true
	}
	p.pending = r
	p.open = true
	return 0, false
}

func (p *Ptnml) Finish() (Result, bool) {
	if !p.open {
		return 0, false
	}
	p.open = false
	p.unpaired++
	p.unpairedHalf += int(p.pending)
	return p.pending, true
}

func (p Ptnml) Games() int {
	return p.pairs*2 + p.unpaired
}

func (p Ptnml) HalfPoints() int {
	return p.pairHalfPoints() + p.unpairedHalf
}

func (p Ptnml) pairHalfPoints() int {
	return 4*p.buckets[PairTwoWins] + 3*p.buckets[PairWinDraw] +
		2*p.buckets[PairTie] + p.buckets[PairLossDraw]
}

func (p Ptnml) Summary() string {
	return fmt.Sprintf("%d-%d-%d-%d-%d over %d pairs, %d unpaired",
		p.buckets[PairTwoWins], p.buckets[PairWinDraw], p.buckets[PairTie],
		p.buckets[PairLossDraw], p.buckets[PairTwoLosses], p.pairs, p.unpaired)
}
