package main

import (
	"fmt"
	"io"
	"time"
)

type treeOpts struct {
	tier      string
	logPath   string
	outPath   string
	statsPath string
	priors    []string
	reverify  bool
	created   string
}

func validateTreeArgs(tierName string) error {
	if _, err := seatTier(tierName); err != nil {
		return err
	}
	return nil
}

func runTree(o treeOpts, analyst analystPort, solver solverPort, progress io.Writer) (treeStats, error) {
	st := treeStats{}
	if err := validateTreeArgs(o.tier); err != nil {
		return st, err
	}
	tier, _ := seatTier(o.tier)
	priors, err := loadPriors(o.priors)
	if err != nil {
		return st, err
	}
	lock, err := acquireSidecarLock(o.logPath, "tree")
	if err != nil {
		return st, err
	}
	defer lock.Release()
	params := paramsFromConfig(o.tier)
	analystFn := analyst
	if analystFn == nil {
		a := &engineAnalyst{}
		analystFn = a.analyze
	}
	solverFn := solver
	if solverFn == nil {
		solverFn = treeSolverPort
	}
	bt, err := newBookTree(params, tier.BookMaxPly, priors, analystFn, solverFn, o.logPath)
	if err != nil {
		return st, err
	}
	if err := bt.run(progress); err != nil {
		return st, err
	}
	if o.reverify {
		if err := bt.reverifyPass(progress); err != nil {
			return st, err
		}
	}
	st, err = bt.writeArtifact(o.outPath, o.statsPath, o.created)
	if err != nil {
		return st, err
	}
	fmt.Fprint(progress, treeSummary(bt, st))
	return st, nil
}

func treeOptsFromFlags(tier, logPath, outPath, statsPath string, priors []string, reverify bool) treeOpts {
	return treeOpts{
		tier:      tier,
		logPath:   logPath,
		outPath:   outPath,
		statsPath: statsPath,
		priors:    priors,
		reverify:  reverify,
		created:   time.Now().UTC().Format(time.RFC3339),
	}
}
