package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type branchOpts struct {
	tier      string
	fromPath  string
	logPath   string
	outPath   string
	statsPath string
	seeds     []string
	created   string
}

type branchSeed struct {
	board *rules.Board
	key   [config.BookKeyBytes]byte
}

func parseBranchSeeds(specs []string) ([]branchSeed, error) {
	if len(specs) == 0 {
		return nil, fmt.Errorf("bookgen: -phase branch wants the blue target move lists as positional arguments")
	}
	seeds := make([]branchSeed, 0, len(specs))
	for _, spec := range specs {
		parts := strings.Split(spec, ",")
		b := rules.NewBoard()
		for _, p := range parts {
			name := strings.TrimSpace(p)
			if name == "" {
				return nil, fmt.Errorf("bookgen: seed %q holds an empty cell", spec)
			}
			c, err := rules.ParseCell(name)
			if err != nil {
				return nil, fmt.Errorf("bookgen: seed %q: %w", spec, err)
			}
			if !b.IsLegal(c) {
				return nil, fmt.Errorf("bookgen: seed %q: cell %s illegal on the board under construction", spec, name)
			}
			b.Make(c)
		}
		if b.Side != rules.Blue {
			return nil, fmt.Errorf("bookgen: seed %q leaves red to move at %d stones, the branch stores blue replies only", spec, len(parts))
		}
		seeds = append(seeds, branchSeed{board: b, key: canonicalBoardKey(b)})
	}
	return seeds, nil
}

func readLegacyTreeRecords(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("bookgen: read trunk log %s: %w", path, err)
	}
	text := strings.TrimSuffix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("bookgen: trunk log %s is empty", path)
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 3 || fields[0] != "F" || fields[1] != "tree" {
		return nil, fmt.Errorf("bookgen: trunk log %s line 1 is not a tree fingerprint", path)
	}
	records := make([]string, 0, len(lines)-1)
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[0] {
		case "X", "L", "N", "S", "F", "R":
			records = append(records, line)
		default:
			return nil, fmt.Errorf("bookgen: trunk log %s holds unknown record kind %q", path, f[0])
		}
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("bookgen: trunk log %s carries no records", path)
	}
	return records, nil
}

func seedBranchLog(o branchOpts, params treeParams, plyBound int, records []string) error {
	header := params.fingerprint(plyBound)
	if existing, err := os.ReadFile(o.logPath); err == nil {
		first := strings.SplitN(strings.ReplaceAll(string(existing), "\r\n", "\n"), "\n", 2)[0]
		if first == header {
			return nil
		}
		pairs := len(strings.Fields(header)) - 3
		ver, ok := parseSidecarFingerprint(strings.Fields(first), "tree", pairs)
		if ok || ver != "" {
			return fmt.Errorf("bookgen: branch log %s holds fingerprint %q, run wants %q", o.logPath, first, header)
		}
		if len(strings.Fields(first)) < 3 || strings.Fields(first)[1] != "tree" {
			return fmt.Errorf("bookgen: branch log %s line 1 is not a tree fingerprint", o.logPath)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("bookgen: read branch log %s: %w", o.logPath, err)
	}
	out := &strings.Builder{}
	fmt.Fprintf(out, "%s\n", header)
	for _, r := range records {
		fmt.Fprintf(out, "%s\n", r)
	}
	if err := writeFileAtomic(o.logPath, []byte(out.String())); err != nil {
		return err
	}
	return nil
}

func runBranch(o branchOpts, analyst analystPort, solver solverPort, progress io.Writer) (treeStats, error) {
	st := treeStats{}
	tier, err := seatTier(o.tier)
	if err != nil {
		return st, err
	}
	seeds, err := parseBranchSeeds(o.seeds)
	if err != nil {
		return st, err
	}
	records, err := readLegacyTreeRecords(o.fromPath)
	if err != nil {
		return st, err
	}
	lock, err := acquireSidecarLock(o.logPath, "tree")
	if err != nil {
		return st, err
	}
	defer lock.Release()
	params := paramsFromConfig(o.tier)
	if err := seedBranchLog(o, params, tier.BookMaxPly, records); err != nil {
		return st, err
	}
	analystFn := analyst
	if analystFn == nil {
		a := &engineAnalyst{}
		analystFn = a.analyze
	}
	solverFn := solver
	if solverFn == nil {
		solverFn = treeSolverPort
	}
	bt, err := newBookTree(params, tier.BookMaxPly, priorsTable{}, analystFn, solverFn, o.logPath)
	if err != nil {
		return st, err
	}
	for _, s := range seeds {
		if len(bt.log.analyses[s.key]) > 0 {
			fmt.Fprintf(progress, "branch target %s already analyzed, skipping\n", hexKey(s.key[:])[:12])
			continue
		}
		if err := bt.solverPass(s.key, s.board); err != nil {
			return st, err
		}
		if _, frozen := bt.log.frozen[s.key]; frozen {
			continue
		}
		base := bt.params.critical
		if _, _, err := bt.analyze(s.key, s.board, base, bt.params.opponentK); err != nil {
			return st, err
		}
		if _, _, err := bt.tryFreeze(s.key, s.board, base); err != nil {
			return st, err
		}
	}
	bt.stopped = "branch targets done"
	st, err = bt.writeArtifact(o.outPath, o.statsPath, o.created)
	if err != nil {
		return st, err
	}
	fmt.Fprint(progress, treeSummary(bt, st))
	return st, nil
}

func branchOptsFromFlags(tier, fromPath, logPath, outPath, statsPath string, seeds []string) branchOpts {
	return branchOpts{
		tier:      tier,
		fromPath:  fromPath,
		logPath:   logPath,
		outPath:   outPath,
		statsPath: statsPath,
		seeds:     seeds,
		created:   time.Now().UTC().Format(time.RFC3339),
	}
}
