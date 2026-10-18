package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

var seats = buildSeats()

func buildSeats() map[string]config.Tier {
	perturb := func(base config.Tier, name string, edit func(*config.Tier)) config.Tier {
		t := base
		t.Name = name
		edit(&t)
		return t
	}
	m := map[string]config.Tier{
		"easy":                 config.TierEasy,
		"medium":               config.TierMedium,
		"hard":                 config.TierHard,
		"master":               config.TierMaster,
		"grandmaster":          config.TierGrandmaster,
		"easy-vcf":             perturb(config.TierEasy, "easy-vcf", func(t *config.Tier) { t.VCF = true }),
		"easy-tt32m":           perturb(config.TierEasy, "easy-tt32m", func(t *config.Tier) { t.TTBytes = 32 << 20 }),
		"medium-vct":           perturb(config.TierMedium, "medium-vct", func(t *config.Tier) { t.VCT = true }),
		"medium-tt256m":        perturb(config.TierMedium, "medium-tt256m", func(t *config.Tier) { t.TTBytes = 256 << 20 }),
		"hard-novct":           perturb(config.TierHard, "hard-novct", func(t *config.Tier) { t.VCT = false }),
		"hard-nosolver":        perturb(config.TierHard, "hard-nosolver", func(t *config.Tier) { t.VCF = false; t.VCT = false }),
		"hard-tt256m":          perturb(config.TierHard, "hard-tt256m", func(t *config.Tier) { t.TTBytes = 256 << 20 }),
		"hard-tt512m":          perturb(config.TierHard, "hard-tt512m", func(t *config.Tier) { t.TTBytes = 512 << 20 }),
		"hard-tt1g":            perturb(config.TierHard, "hard-tt1g", func(t *config.Tier) { t.TTBytes = 1 << 30 }),
		"master-novct":         perturb(config.TierMaster, "master-novct", func(t *config.Tier) { t.VCT = false }),
		"master-noponder":      perturb(config.TierMaster, "master-noponder", func(t *config.Tier) { t.PonderThreads = 0 }),
		"master-nobook":        perturb(config.TierMaster, "master-nobook", func(t *config.Tier) { t.Book = false }),
		"grandmaster-noponder": perturb(config.TierGrandmaster, "grandmaster-noponder", func(t *config.Tier) { t.PonderThreads = 0 }),
		"grandmaster-nobook":   perturb(config.TierGrandmaster, "grandmaster-nobook", func(t *config.Tier) { t.Book = false }),
	}
	return m
}

type tally struct {
	name                                   string
	wins, losses, draws, redWins, blueWins int
	games                                  int
	ponderArms, ponderAdopts               int
}

type seatState struct {
	label   string
	tier    config.Tier
	eng     server.Searcher
	clock   *clock.GameClock
	ring    server.GrantDepthRing
	tally   *tally
	armed   bool
	predict rules.Move
	base    int
}

func main() {
	tcIdx := flag.Int("tc", 0, "time control index into config.TimeControls")
	games := flag.Int("games", 12, "games per unordered seat pair")
	seatsArg := flag.String("seats", "", "comma-separated seat labels, default all")
	flag.Parse()

	labels := seatLabels(*seatsArg)
	tc := config.TimeControls[*tcIdx]
	fmt.Printf("arena tc %d+%d seats %v games %d\n", tc.InitialMin, tc.IncrementSec, labels, *games)

	results := map[string]*tally{}
	for _, label := range labels {
		results[label] = &tally{name: label}
	}
	gameNo := 0
	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			a, b := labels[i], labels[j]
			for g := 0; g < *games; g++ {
				gameNo++
				red, blue := a, b
				if g%2 == 1 {
					red, blue = blue, red
				}
				fmt.Printf("game %d: %s (red) vs %s (blue), tc %d+%d\n", gameNo, red, blue, tc.InitialMin, tc.IncrementSec)
				winner, moves := play(results[red], results[blue], *tcIdx, gameNo)
				record(results, red, blue, winner, moves)
				outcome := "draw"
				if winner != "" {
					outcome = winner
				}
				fmt.Printf("game %d: %s (red) vs %s, winner %s over %d moves\n", gameNo, red, blue, outcome, moves)
			}
			fmt.Printf("pair done: %s vs %s\n", a, b)
		}
	}

	fmt.Println("results (wins-losses-draws over games, red/blue win split, ponder adopts/arms)")
	for _, label := range labels {
		r := results[label]
		fmt.Printf("%-22s %d-%d-%d over %d, red wins %d blue wins %d, ponder %d/%d\n",
			label, r.wins, r.losses, r.draws, r.games, r.redWins, r.blueWins, r.ponderAdopts, r.ponderArms)
	}
}

func seatLabels(arg string) []string {
	if arg == "" {
		return rosterOrder
	}
	var out []string
	for _, name := range splitComma(arg) {
		if _, ok := seats[name]; !ok {
			fmt.Fprintf(os.Stderr, "arena: unknown seat %q\n", name)
			os.Exit(1)
		}
		out = append(out, name)
	}
	return out
}

var rosterOrder = []string{"easy", "easy-vcf", "medium", "medium-vct", "hard", "hard-novct", "hard-nosolver"}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if part := s[start:i]; part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func newSeatState(t *tally, tcIdx int) *seatState {
	tier := seats[t.name]
	return &seatState{label: t.name, tier: tier, eng: server.NewBotSearcher(tier), clock: clock.NewGameClock(tcIdx), tally: t}
}

func play(redT, blueT *tally, tcIdx, gameNo int) (string, int) {
	st := map[rules.Color]*seatState{
		rules.Red:  newSeatState(redT, tcIdx),
		rules.Blue: newSeatState(blueT, tcIdx),
	}
	defer st[rules.Red].eng.Close()
	defer st[rules.Blue].eng.Close()

	var legalBuf [config.BoardCells]rules.Move
	b := rules.NewBoard()
	var lastMove rules.Move
	turnStart := time.Now()
	for {
		side := b.Side
		s := st[side]
		budget := s.clock.Budget()
		mv, stats, tag, outcome := collectPonder(s, b, lastMove, budget, turnStart)
		if outcome != "" {
			if outcome == "adopt" {
				s.tally.ponderAdopts++
			}
			emitPonderOutcome(os.Stdout, gameNo, b.MoveCount+1, s.label, outcome)
		}
		if outcome != "adopt" {
			mv, stats, tag = s.eng.Search(b, engine.NewFixedBudget(budget))
			if tag == "" {
				s.ring.Append(int64(budget), stats.Depth)
			}
		}
		emitMLine(os.Stdout, gameNo, b.MoveCount+1, side, mv, &stats, tag)
		if !b.IsLegal(rules.Cell(mv)) {
			n := b.LegalMoves(legalBuf[:])
			mv = legalBuf[0]
			fmt.Fprintf(os.Stderr, "arena: %s returned illegal %v, fell back (legal count %d)\n",
				s.label, cellNameOf(mv), n)
		}
		cell := rules.Cell(mv)
		b.Make(cell)
		s.clock.Commit(time.Since(turnStart))
		turnStart = time.Now()
		lastMove = mv
		if b.FastLastMoveWin(side, cell) {
			return s.label, b.MoveCount
		}
		if b.IsFull() {
			return "", b.MoveCount
		}
		armPonder(s, b, stats)
	}
}

func collectPonder(s *seatState, b *rules.Board, lastMove rules.Move, budget time.Duration, turnStart time.Time) (rules.Move, engine.SearchStats, string, string) {
	if !s.armed {
		return 0, engine.SearchStats{}, "", ""
	}
	s.armed = false
	pmv, pst, ptag := s.eng.StopPonder()
	hit := b.MoveCount == s.base+1 && lastMove == s.predict
	adopted := hit && server.AdoptPonder(pst, ptag, pst.ElapsedNs, int64(budget), s.ring, int64(budget))
	if outcome := ponderOutcome(true, hit, adopted); outcome != "adopt" {
		return 0, engine.SearchStats{}, "", outcome
	}
	st, tag := server.ComposePonderStats(pst, ptag, turnStart, int64(budget))
	return pmv, st, tag, "adopt"
}

func ponderOutcome(armed, hit, adopted bool) string {
	if !armed {
		return ""
	}
	if !hit {
		return "miss"
	}
	if adopted {
		return "adopt"
	}
	return "reject"
}

func emitMLine(w io.Writer, gameNo, moveNo int, side rules.Color, mv rules.Move, st *engine.SearchStats, tag string) {
	fmt.Fprintf(w, "g%d %s\n", gameNo, server.MLine(moveNo, side, mv, st, tag))
}

func emitPonderOutcome(w io.Writer, gameNo, moveNo int, seat, outcome string) {
	fmt.Fprintf(w, "g%d ponder %s m%d %s\n", gameNo, seat, moveNo, outcome)
}

func armPonder(s *seatState, b *rules.Board, stats engine.SearchStats) {
	if s.tier.PonderThreads < 1 || stats.PVLen < 2 {
		return
	}
	pb := *b
	if !pb.IsLegal(rules.Cell(stats.PV[1])) {
		return
	}
	s.base = pb.MoveCount
	s.predict = stats.PV[1]
	pb.Make(rules.Cell(s.predict))
	s.eng.StartPonder(&pb)
	s.armed = true
	s.tally.ponderArms++
}

func record(results map[string]*tally, redLabel, blueLabel, winner string, moves int) {
	results[redLabel].games++
	results[blueLabel].games++
	switch {
	case winner == "":
		results[redLabel].draws++
		results[blueLabel].draws++
	case winner == redLabel:
		results[redLabel].wins++
		results[blueLabel].losses++
		results[redLabel].redWins++
	case winner == blueLabel:
		results[blueLabel].wins++
		results[redLabel].losses++
		results[blueLabel].blueWins++
	}
}

func cellNameOf(m rules.Move) string {
	return string(appendCellName(nil, m))
}

func appendCellName(dst []byte, m rules.Move) []byte {
	col := int(m) % config.BoardStride
	row := int(m) / config.BoardStride
	dst = append(dst, byte('A'+col))
	if row >= 10 {
		dst = append(dst, byte('0'+row/10))
	}
	dst = append(dst, byte('0'+row%10))
	return dst
}
