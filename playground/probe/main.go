// probe replays one logged game up to a move and re-searches the position
// under the logged budget on both the easy and hard tier engines, printing
// the Implication 1.5 stat fields. Built for the 2026-10-04 investigation:
// the same smoke32 series-9 position whose logged line reads d=0 n=5.7m at
// a 1.68s grant.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func main() {
	// The smoke32 s9 game 1 move column, in order.
	moves := []string{
		"H8", "H6", "H5", "G7", "F8", "G8", "G9", "I7", "H10", "E7",
		"H9", "H7", "F7", "K7", "J7", "F9", "H11", "H12", "I6", "D11",
		"C12",
	}
	upTo := 21
	budget := 1680 * time.Millisecond
	if len(os.Args) > 1 {
		n, err := strconv.Atoi(os.Args[1])
		if err != nil || n < 1 || n > len(moves) {
			fmt.Fprintf(os.Stderr, "probe: move count must be 1..%d\n", len(moves))
			os.Exit(1)
		}
		upTo = n
	}

	b := rules.NewBoard()
	for _, name := range moves[:upTo] {
		cell, err := rules.ParseCell(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "probe: cell %s: %v\n", name, err)
			os.Exit(1)
		}
		b.Make(cell)
	}

	for _, tier := range []config.Tier{config.TierEasy, config.TierHard} {
		var eng interface {
			Search(*rules.Board, engine.Deadline) (rules.Move, engine.SearchStats)
			Close()
		}
		if tier.Cores > 1 {
			eng = engine.NewTiered(tier)
		} else {
			eng = plainEngine{engine.New(tier.TTBytes)}
		}
		mv, st := eng.Search(b, engine.NewFixedBudget(budget))
		pv := make([]string, 0, st.PVLen)
		for i := range st.PVLen {
			pv = append(pv, cellName(st.PV[i]))
		}
		fmt.Printf("%s: d=%d n=%d nps=%d ebf=%.1f t=%.2f alloc=%.2f move=%s pv=%s\n",
			tier.Name, st.Depth, st.Nodes, st.Nps,
			float64(st.EBFMilli)/config.EvalMilliUnit,
			float64(st.ElapsedNs)/float64(time.Second),
			float64(st.AllocNs)/float64(time.Second),
			cellName(mv), strings.Join(pv, " "))
		eng.Close()
	}
}

type plainEngine struct{ e *engine.Engine }

func (p plainEngine) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats) {
	return p.e.Search(b, dl)
}
func (p plainEngine) Close() {}

func cellName(m rules.Move) string {
	return string(rune('A'+m%config.BoardStride)) + strconv.Itoa(int(m/config.BoardStride)+1)
}
