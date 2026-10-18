package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

type prober func(b *rules.Board) (rules.Move, uint64, string, bool)

type overlayOpts struct {
	corpusPath string
	outPath    string
	floor      int
	budget     int
}

type proofRec struct {
	move  rules.Move
	nodes uint64
	kind  string
}

func solverProber(budget int) prober {
	fours := vcf.New(vcf.KindVCF)
	threes := vcf.New(vcf.KindVCT)
	return func(b *rules.Board) (rules.Move, uint64, string, bool) {
		var out vcf.SolverStats
		if fours.Solve(b, budget, nil, &out) {
			return rules.Move(out.PV[0]), out.Nodes, "vcf", true
		}
		if threes.Solve(b, budget, nil, &out) {
			return rules.Move(out.PV[0]), out.Nodes, "vct", true
		}
		return 0, out.Nodes, "", false
	}
}

func aggregate(recs []posRec) map[[config.BookKeyBytes]byte]map[rules.Move]int {
	agg := make(map[[config.BookKeyBytes]byte]map[rules.Move]int)
	for _, r := range recs {
		counts, ok := agg[r.key]
		if !ok {
			counts = make(map[rules.Move]int)
			agg[r.key] = counts
		}
		counts[r.move]++
	}
	return agg
}

func majorityOf(counts map[rules.Move]int) (rules.Move, int) {
	best := rules.Move(config.BoardCells)
	bestN := -1
	for mv, n := range counts {
		if n > bestN || n == bestN && int(mv) < int(best) {
			best, bestN = mv, n
		}
	}
	return best, bestN
}

func readOverlay(path string) (map[[config.BookKeyBytes]byte]proofRec, map[[config.BookKeyBytes]byte]struct{}, error) {
	proofs := make(map[[config.BookKeyBytes]byte]proofRec)
	done := make(map[[config.BookKeyBytes]byte]struct{})
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return proofs, done, nil
		}
		return nil, nil, fmt.Errorf("bookgen: read overlay %s: %w", path, err)
	}
	lineno := 0
	for _, line := range bytes.Split(data, []byte("\n")) {
		lineno++
		if len(line) == 0 {
			continue
		}
		fields := strings.Fields(string(line))
		fail := func(reason string) error {
			return fmt.Errorf("bookgen: overlay %s line %d: %s", path, lineno, reason)
		}
		switch fields[0] {
		case "P":
			if len(fields) != 5 {
				return nil, nil, fail("proof line wants key move nodes kind")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return nil, nil, fail("proof key")
			}
			mv, err := strconv.Atoi(fields[2])
			if err != nil || mv < 0 || mv >= config.BoardCells {
				return nil, nil, fail("proof move")
			}
			nodes, err := strconv.ParseUint(fields[3], 10, 64)
			if err != nil {
				return nil, nil, fail("proof nodes")
			}
			if fields[4] != "vcf" && fields[4] != "vct" {
				return nil, nil, fail("proof kind")
			}
			proofs[k] = proofRec{move: rules.Move(mv), nodes: nodes, kind: fields[4]}
			done[k] = struct{}{}
		case "N":
			if len(fields) != 2 {
				return nil, nil, fail("no-proof line wants the key")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return nil, nil, fail("no-proof key")
			}
			done[k] = struct{}{}
		default:
			return nil, nil, fail(fmt.Sprintf("unknown line kind %q", fields[0]))
		}
	}
	return proofs, done, nil
}

func floorPassingKeys(agg map[[config.BookKeyBytes]byte]map[rules.Move]int, floor int) [][config.BookKeyBytes]byte {
	keys := make([][config.BookKeyBytes]byte, 0, len(agg))
	for k, counts := range agg {
		if _, n := majorityOf(counts); n >= floor {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return bytes.Compare(keys[i][:], keys[j][:]) < 0 })
	return keys
}

func overlayRun(o overlayOpts, probe prober, progress io.Writer) error {
	sc, err := loadCorpus(o.corpusPath)
	if err != nil {
		return err
	}
	agg := aggregate(sc.recs)
	keys := floorPassingKeys(agg, o.floor)
	proofs, done, err := readOverlay(o.outPath)
	if err != nil {
		return err
	}
	pending := keys[:0]
	for _, k := range keys {
		if _, ok := done[k]; !ok {
			pending = append(pending, k)
		}
	}
	if len(pending) == 0 {
		fmt.Fprintf(progress, "overlay complete: %d positions processed at %s (%d proofs)\n", len(done), o.outPath, len(proofs))
		return nil
	}
	f, err := os.OpenFile(o.outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("bookgen: open overlay %s: %w", o.outPath, err)
	}
	defer func() { _ = f.Close() }()
	for i, k := range pending {
		b, err := boardFromKey(k)
		if err != nil {
			return fmt.Errorf("bookgen: overlay position: %w", err)
		}
		mv, nodes, kind, ok := probe(b)
		line := fmt.Sprintf("N %s\n", hexKey(k[:]))
		if ok {
			line = fmt.Sprintf("P %s %d %d %s\n", hexKey(k[:]), mv, nodes, kind)
		}
		if _, err := f.WriteString(line); err != nil {
			return fmt.Errorf("bookgen: append overlay verdict: %w", err)
		}
		if err := f.Sync(); err != nil {
			return fmt.Errorf("bookgen: sync overlay %s: %w", o.outPath, err)
		}
		outcome := "no proof"
		if ok {
			proofs[k] = proofRec{move: mv, nodes: nodes, kind: kind}
			outcome = fmt.Sprintf("proof %s move %d nodes %d", kind, mv, nodes)
		}
		fmt.Fprintf(progress, "position %d/%d key %s... %s\n", i+1, len(pending), hexKey(k[:])[:12], outcome)
	}
	fmt.Fprintf(progress, "overlay done: %d positions, %d proofs\n", len(pending), len(proofs))
	return nil
}
