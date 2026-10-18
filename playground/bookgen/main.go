package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func main() {
	phase := flag.String("phase", "", "corpus, overlay, or emit")
	games := flag.Int("games", config.BookCorpusGames, "corpus game target")
	tcIdx := flag.Int("tc", config.BookCorpusTCIndex, "time control index into config.TimeControls")
	tierName := flag.String("tier", config.TierMaster.Name, "corpus seat tier")
	corpusPath := flag.String("corpus", config.BookCorpusPath, "append-only corpus file")
	overlayPath := flag.String("overlay", config.BookOverlayPath, "solver overlay file")
	floor := flag.Int("floor", config.BookVisitFloor, "majority visit floor")
	budget := flag.Int("solver-budget", config.BookSolverNodeBudget, "solver node budget per pass")
	outPath := flag.String("out", config.BookArtifactPath, "CAROBOOK1 artifact path")
	statsPath := flag.String("stats", config.BookStatsPath, "stats artifact path")
	flag.Parse()

	switch *phase {
	case "corpus":
		if *tcIdx < 0 || *tcIdx >= len(config.TimeControls) {
			fatal(fmt.Errorf("bookgen: -tc %d outside config.TimeControls", *tcIdx))
		}
		tier, err := seatTier(*tierName)
		if err != nil {
			fatal(err)
		}
		play := func(idx, redSeat int) gameResult {
			return playCorpusGame(tier, *tcIdx, idx, redSeat, os.Stdout)
		}
		fatal(runCorpus(corpusOpts{path: *corpusPath, tier: *tierName, tc: *tcIdx, games: *games}, play, os.Stdout))
	case "overlay":
		fatal(overlayRun(overlayOpts{corpusPath: *corpusPath, outPath: *overlayPath, floor: *floor, budget: *budget}, solverProber(*budget), os.Stdout))
	case "emit":
		_, err := emitRun(emitOpts{
			corpusPath:  *corpusPath,
			overlayPath: *overlayPath,
			outPath:     *outPath,
			statsPath:   *statsPath,
			floor:       *floor,
			created:     time.Now().UTC().Format(time.RFC3339),
		}, os.Stdout)
		fatal(err)
	default:
		fatal(fmt.Errorf("bookgen: -phase %q must be corpus, overlay, or emit", *phase))
	}
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
