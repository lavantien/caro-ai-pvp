package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const (
	defaultTierA  = "medium"
	defaultTierB  = "medium"
	defaultTC     = 1
	defaultBO     = config.SeriesBO3
	defaultSeed   = 1
	defaultMovCap = 60

	unitK = 1_000
	unitM = 1_000_000

	permillePerPercent = 10
	displayScoreDiv    = config.EvalMilliUnit / 100

	reasonWin  = "WIN"
	reasonDraw = "DRAW"
	reasonCap  = "CAP"
)

var mateScoreFence = config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep

var seatNames = [2]string{"A", "B"}

type seat struct {
	label string
	tier  config.Tier
	eng   *engine.SMP
	clock *clock.GameClock
	moves int
	wins  int
}

func tierByName(name string) (config.Tier, bool) {
	for _, t := range config.Tiers {
		if t.Name == name {
			return t, true
		}
	}
	return config.Tier{}, false
}

type overrunLog struct {
	n   int
	sum time.Duration
	max time.Duration
}

func (l *overrunLog) add(d time.Duration) {
	l.n++
	l.sum += d
	if d > l.max {
		l.max = d
	}
}

func (l *overrunLog) mean() time.Duration {
	if l.n == 0 {
		return 0
	}
	return l.sum / time.Duration(l.n)
}

func compact(n uint64) string {
	switch {
	case n >= unitM:
		return strconv.FormatFloat(float64(n)/unitM, 'f', -1, 64) + "m"
	case n >= unitK:
		return strconv.FormatFloat(float64(n)/unitK, 'f', -1, 64) + "k"
	}
	return strconv.FormatUint(n, 10)
}

func scoreStr(score int) string {
	if score >= mateScoreFence {
		return fmt.Sprintf("M%d", (config.EvalMateMax-score)/config.EvalMateScoreStep)
	}
	if score <= -mateScoreFence {
		return fmt.Sprintf("-M%d", (config.EvalMateMax+score)/config.EvalMateScoreStep)
	}
	centi := score / displayScoreDiv
	if centi == 0 {
		return "0"
	}
	return fmt.Sprintf("%+d", centi)
}

func colorName(c rules.Color) string {
	if c == rules.Red {
		return "Red"
	}
	return "Blue"
}

func mustCellName(c rules.Cell) string {
	name, err := rules.CellName(c)
	if err != nil {
		panic(err)
	}
	return name
}

func chebyshevDist(a, b rules.Cell) int {
	dr := int(a/config.BoardStride) - int(b/config.BoardStride)
	dc := int(a%config.BoardStride) - int(b%config.BoardStride)
	dr = max(dr, -dr)
	dc = max(dc, -dc)
	return max(dr, dc)
}

func botLine(turn int, mover rules.Color, cell rules.Cell, s *engine.SearchStats, granted time.Duration) string {
	return fmt.Sprintf(config.BotLogFormat,
		turn, colorName(mover), mustCellName(cell),
		s.Depth, compact(s.Nodes), compact(s.Nps),
		float64(s.EBFMilli)/config.EvalMilliUnit,
		s.TTHitPermille/permillePerPercent,
		s.HashFullPermille/permillePerPercent,
		s.FirstMoveFailHighPermille/permillePerPercent,
		scoreStr(s.Score), s.Threads,
		float64(s.ElapsedNs)/float64(time.Second),
		float64(granted)/float64(time.Second),
		"",
		string(s.AppendPV(make([]byte, 0, s.PVLen*4))))
}

func playGame(n int, seats []*seat, redIdx, tcIdx, movcap int, over *overrunLog) (int, string) {
	b := rules.NewBoard()
	for _, s := range seats {
		s.clock = clock.NewGameClock(tcIdx)
		s.moves = 0
	}
	var redFirst rules.Cell
	for turn := 1; ; turn++ {
		idx := redIdx
		if b.Side != rules.Red {
			idx = 1 - redIdx
		}
		mover := seats[idx]

		granted := mover.clock.Budget()
		start := time.Now()
		mv, stats := mover.eng.Search(b, engine.NewFixedBudget(granted))
		elapsed := time.Since(start)
		over.add(elapsed - granted)

		cell := rules.Cell(mv)
		if !b.IsLegal(cell) {
			panic(fmt.Sprintf("game %d move %d: seat %s returned illegal move %d (%q)",
				n, turn, mover.label, mv, mustCellName(cell)))
		}
		if stats.Nodes == 0 {
			panic(fmt.Sprintf("game %d move %d: seat %s searched 0 nodes", n, turn, mover.label))
		}
		if turn == 3 && chebyshevDist(cell, redFirst) < config.OpeningChebyshevMin {
			panic(fmt.Sprintf("game %d move 3: opening Chebyshev violation, %s within %d of red's first stone %s",
				n, mustCellName(cell), config.OpeningChebyshevMin, mustCellName(redFirst)))
		}

		mover.clock.Commit(elapsed)
		for _, s := range seats {
			if rem := s.clock.Remaining(); rem < 0 {
				panic(fmt.Sprintf("game %d move %d: seat %s clock remaining %s below zero", n, turn, s.label, rem))
			}
		}

		if turn == 1 {
			redFirst = cell
		}
		color := b.Side
		b.Make(cell)
		mover.moves++
		fmt.Println(botLine(turn, color, cell, &stats, granted))

		if b.FastLastMoveWin(color, cell) {
			return idx, reasonWin
		}
		if b.IsFull() {
			return -1, reasonDraw
		}
		if mover.moves >= movcap {
			return -1, reasonCap
		}
	}
}

func main() {
	tierA := flag.String("tierA", defaultTierA, "seat A tier, a name from config.Tiers")
	tierB := flag.String("tierB", defaultTierB, "seat B tier, a name from config.Tiers")
	tc := flag.Int("tc", defaultTC, "time control index into config.TimeControls")
	bo := flag.Int("bo", defaultBO, "best-of series length, odd")
	seed := flag.Int("seed", defaultSeed, "series seed")
	movcap := flag.Int("movcap", defaultMovCap, "per-side move cap, a game ending there is a CAP")
	flag.Parse()

	fail := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "series: "+format+"\n", args...)
		os.Exit(2)
	}
	if *tc < 0 || *tc >= len(config.TimeControls) {
		fail("tc %d out of range 0..%d", *tc, len(config.TimeControls)-1)
	}
	if *bo < 1 || *bo%2 == 0 {
		fail("bo %d must be a positive odd number", *bo)
	}
	if *movcap < 1 {
		fail("movcap %d must be positive", *movcap)
	}

	seats := make([]*seat, 2)
	for i, name := range [2]string{*tierA, *tierB} {
		t, ok := tierByName(name)
		if !ok {
			fail("tier %q is not a name in config.Tiers", name)
		}
		seats[i] = &seat{label: seatNames[i], tier: t, eng: engine.NewTiered(t)}
	}
	defer seats[1].eng.Close()
	defer seats[0].eng.Close()

	tcDef := config.TimeControls[*tc]
	majority := *bo/2 + 1
	fmt.Printf("series harness: A=%s B=%s tc=%d (%d+%d) bo=%d majority=%d seed=%d movcap=%d\n",
		*tierA, *tierB, *tc, tcDef.InitialMin, tcDef.IncrementSec, *bo, majority, *seed, *movcap)

	wall := time.Now()
	var over overrunLog
	draws, caps := 0, 0
	redIdx := 0
	lastWinner := -1
	for g := 1; g <= *bo; g++ {
		if lastWinner >= 0 && redIdx != 1-lastWinner {
			fail("game %d: red is seat %d, the loser of game %d (seat %d) must take red", g, redIdx, g-1, 1-lastWinner)
		}
		winner, reason := playGame(g, seats, redIdx, *tc, *movcap, &over)
		if winner >= 0 {
			seats[winner].wins++
			lastWinner = winner
		} else if reason == reasonCap {
			caps++
			lastWinner = -1
		} else {
			draws++
			lastWinner = -1
		}
		wtag := "none"
		if winner >= 0 {
			wtag = seats[winner].label
		}
		fmt.Printf("game %d: red=%s winner=%s reason=%s moves A=%d B=%d clock A=%s B=%s\n",
			g, seats[redIdx].label, wtag, reason,
			seats[0].moves, seats[1].moves,
			seats[0].clock.Remaining(), seats[1].clock.Remaining())
		if winner >= 0 {
			redIdx = 1 - winner
		} else {
			redIdx = 1 - redIdx
		}
		if seats[0].wins == majority || seats[1].wins == majority {
			break
		}
	}

	result := "TIE"
	switch {
	case seats[0].wins > seats[1].wins:
		result = "A wins the series"
	case seats[1].wins > seats[0].wins:
		result = "B wins the series"
	}
	fmt.Printf("series tally: A=%d B=%d draws=%d caps=%d, %s\n",
		seats[0].wins, seats[1].wins, draws, caps, result)
	fmt.Printf("total wall time: %s\n", time.Since(wall))
	fmt.Printf("engine overrun past grant: turns=%d max=%s mean=%s\n", over.n, over.max, over.mean())
}
