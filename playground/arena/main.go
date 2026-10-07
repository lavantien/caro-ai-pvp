// arena plays capability-perturbed tier engines against each other under
// the shipped clock law, for the strength-inversion research program: the
// question every run answers is what each tier resource (cores, table,
// VCF, VCT) actually contributes at a given time control. Seats are
// config.Tier values wired through server.NewBotSearcher, so the
// experiments measure the same code the rooms drive, never a copy. Games
// run serially: one pairing at a time on the whole machine, budgets from
// the real GameClock, engines fresh per game per the ephemerality law,
// red alternating per game per the bot-series law.
//
// Usage:
//
//	arena -tc 0 -games 12 -seats hard,hard-novct,medium,medium-vct
//	arena -tc 0 -games 40 -seats hard,hard          # self-play noise floor
//
// -tc indexes config.TimeControls (0 = 1+0, 1 = 2+1, 2 = 3+2, 3 = 10+5). Every
// unordered seat pair meets -games times. One line prints per finished
// game so a monitor can track progress, then a results table and the
// red-versus-blue win split.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// seats maps every experiment label to its tier: the three shipped tiers
// plus the perturbations the inversion research demands, solver added to
// the weaker tiers and stripped from the stronger one.
var seats = map[string]config.Tier{
	"easy":          config.TierEasy,
	"easy-vcf":      {Name: "easy-vcf", Cores: 1, VCF: true},
	"medium":        config.TierMedium,
	"medium-vct":    {Name: "medium-vct", Cores: 2, TTBytes: config.TierMedium.TTBytes, VCF: true, VCT: true},
	"hard":          config.TierHard,
	"hard-novct":    {Name: "hard-novct", Cores: 4, TTBytes: config.TierHard.TTBytes, VCF: true},
	"hard-nosolver": {Name: "hard-nosolver", Cores: 4, TTBytes: config.TierHard.TTBytes},
}

// tally is one seat's record over the whole run.
type tally struct {
	wins, losses, draws, redWins, blueWins int
	games                                  int
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
		results[label] = &tally{}
	}
	gameNo := 0
	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			a, b := labels[i], labels[j]
			for g := 0; g < *games; g++ {
				gameNo++
				// Red alternates per game; the first game of each pair
				// seats a on red.
				red, blue := a, b
				if g%2 == 1 {
					red, blue = blue, red
				}
				winner, moves := play(red, blue, *tcIdx)
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

	fmt.Println("results (wins-losses-draws over games, red/blue win split)")
	for _, label := range labels {
		r := results[label]
		fmt.Printf("%-14s %d-%d-%d over %d, red wins %d blue wins %d\n",
			label, r.wins, r.losses, r.draws, r.games, r.redWins, r.blueWins)
	}
}

// seatLabels resolves the -seats argument, defaulting to the full roster
// in seats-map order made stable by rosterOrder.
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

// rosterOrder pins the default round-robin order so runs are comparable.
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

// play drives one full game under the shipped clock law and returns the
// winning seat label (empty for a draw) and the move count. Engines are
// fresh per game; the safety net mirrors the room's: the engine contract
// guarantees legality, an illegal answer falls back to the first legal
// move rather than corrupting the experiment.
func play(redLabel, blueLabel string, tcIdx int) (string, int) {
	engines := map[rules.Color]server.Searcher{
		rules.Red:  server.NewBotSearcher(seats[redLabel]),
		rules.Blue: server.NewBotSearcher(seats[blueLabel]),
	}
	defer engines[rules.Red].Close()
	defer engines[rules.Blue].Close()
	labels := map[rules.Color]string{rules.Red: redLabel, rules.Blue: blueLabel}

	clocks := map[rules.Color]*clock.GameClock{
		rules.Red:  clock.NewGameClock(tcIdx),
		rules.Blue: clock.NewGameClock(tcIdx),
	}
	var legalBuf [config.BoardCells]rules.Move

	b := rules.NewBoard()
	turnStart := time.Now()
	for {
		side := b.Side
		budget := clocks[side].Budget()
		mv, _, _ := engines[side].Search(b, engine.NewFixedBudget(budget))
		if !b.IsLegal(rules.Cell(mv)) {
			n := b.LegalMoves(legalBuf[:])
			mv = legalBuf[0]
			fmt.Fprintf(os.Stderr, "arena: %s returned illegal %v, fell back (legal count %d)\n",
				labels[side], cellNameOf(mv), n)
		}
		cell := rules.Cell(mv)
		b.Make(cell)
		clocks[side].Commit(time.Since(turnStart))
		turnStart = time.Now()
		if b.FastLastMoveWin(side, cell) {
			return labels[side], b.MoveCount
		}
		if b.IsFull() {
			return "", b.MoveCount
		}
	}
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

// appendCellName mirrors the rules codec notation for the stderr fallback
// line without reaching into unexported server helpers.
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
