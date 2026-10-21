package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func validateCorpusArgs(games, tcIdx int, tierName string) error {
	if games < 1 {
		return fmt.Errorf("bookgen: -games %d must be at least 1", games)
	}
	if tcIdx < 0 || tcIdx >= len(config.TimeControls) {
		return fmt.Errorf("bookgen: -tc %d outside config.TimeControls", tcIdx)
	}
	if _, err := seatTier(tierName); err != nil {
		return err
	}
	return nil
}

func validateFloor(floor int) error {
	if floor < 1 {
		return fmt.Errorf("bookgen: -floor %d must be at least 1", floor)
	}
	return nil
}

func validateReadSidecar(path, flagName string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("bookgen: -%s %s: %w", flagName, path, err)
	}
	if fi.IsDir() || fi.Size() == 0 {
		return fmt.Errorf("bookgen: -%s %s is empty or not a regular file", flagName, path)
	}
	return nil
}

func flagWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func main() {
	phase := flag.String("phase", "", "corpus, overlay, emit, merge, or stats")
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
		fatal(validateCorpusArgs(*games, *tcIdx, *tierName))
		lock, err := acquireSidecarLock(*corpusPath, "corpus")
		if err != nil {
			fatal(err)
		}
		defer lock.Release()
		tier, err := seatTier(*tierName)
		if err != nil {
			fatal(err)
		}
		play := func(idx, redSeat int) gameResult {
			return playCorpusGame(tier, *tcIdx, idx, redSeat, os.Stdout)
		}
		fatal(runCorpus(corpusOpts{path: *corpusPath, tier: *tierName, tc: *tcIdx, games: *games}, play, os.Stdout))
	case "overlay":
		fatal(validateFloor(*floor))
		fatal(validateReadSidecar(*corpusPath, "corpus"))
		lock, err := acquireSidecarLock(*corpusPath, "overlay")
		if err != nil {
			fatal(err)
		}
		defer lock.Release()
		fatal(overlayRun(overlayOpts{corpusPath: *corpusPath, outPath: *overlayPath, floor: *floor, budget: *budget}, solverProber(*budget), os.Stdout))
	case "merge":
		fatal(validateCorpusArgs(*games, *tcIdx, *tierName))
		if !flagWasSet(flag.CommandLine, "out") {
			fatal(fmt.Errorf("bookgen: -phase merge requires an explicit -out path for the merged corpus"))
		}
		if flag.NArg() == 0 {
			fatal(fmt.Errorf("bookgen: -phase merge wants the shard paths as positional arguments"))
		}
		lock, err := acquireSidecarLock(*outPath, "merge")
		if err != nil {
			fatal(err)
		}
		defer lock.Release()
		fatal(runMerge(mergeOpts{shards: flag.Args(), outPath: *outPath, tier: *tierName, tc: *tcIdx, games: *games}, os.Stdout))
	case "stats":
		fatal(validateFloor(*floor))
		if flag.NArg() == 0 {
			fatal(fmt.Errorf("bookgen: -phase stats wants the corpus paths as positional arguments"))
		}
		_, err := runStats(flag.Args(), *floor, os.Stdout)
		fatal(err)
	case "emit":
		fatal(validateFloor(*floor))
		fatal(validateReadSidecar(*corpusPath, "corpus"))
		fatal(validateReadSidecar(*overlayPath, "overlay"))
		lock, err := acquireSidecarLock(*corpusPath, "emit")
		if err != nil {
			fatal(err)
		}
		defer lock.Release()
		_, err = emitRun(emitOpts{
			corpusPath:  *corpusPath,
			overlayPath: *overlayPath,
			outPath:     *outPath,
			statsPath:   *statsPath,
			floor:       *floor,
			created:     time.Now().UTC().Format(time.RFC3339),
		}, os.Stdout)
		fatal(err)
	default:
		if *phase == "" {
			flag.Usage()
			os.Exit(1)
		}
		fatal(fmt.Errorf("bookgen: -phase %q must be corpus, overlay, emit, merge, or stats", *phase))
	}
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}
}
