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

type overlayScan struct {
	proofs map[[config.BookKeyBytes]byte]proofRec
	done   map[[config.BookKeyBytes]byte]struct{}
	clean  int64
	torn   bool
	fp     string
}

func readOverlay(path string) (overlayScan, error) {
	sc := overlayScan{
		proofs: make(map[[config.BookKeyBytes]byte]proofRec),
		done:   make(map[[config.BookKeyBytes]byte]struct{}),
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sc, nil
		}
		return sc, fmt.Errorf("bookgen: read overlay %s: %w", path, err)
	}
	end := len(data)
	if end > 0 && data[end-1] != '\n' {
		sc.torn = true
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			end = i + 1
		} else {
			end = 0
		}
	}
	off := 0
	if len(data) > 0 {
		header := data
		if end > 0 {
			header = data[:end]
		}
		nl := bytes.IndexByte(header, '\n')
		if nl < 0 {
			nl = len(header)
		}
		if !parseSidecarFingerprint(strings.Fields(string(header[:nl])), "overlay", 2) {
			return sc, fmt.Errorf("bookgen: overlay %s line 1: missing or malformed fingerprint", path)
		}
		sc.fp = string(header[:nl])
		off = nl + 1
	}
	lineno := 1
	for off < end {
		nl := off + bytes.IndexByte(data[off:end], '\n')
		line := data[off:nl]
		off = nl + 1
		lineno++
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			continue
		}
		fail := func(reason string) error {
			return fmt.Errorf("bookgen: overlay %s line %d: %s", path, lineno, reason)
		}
		switch fields[0] {
		case "P":
			if len(fields) != 5 {
				return sc, fail("proof line wants key move nodes kind")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return sc, fail("proof key")
			}
			mv, err := strconv.Atoi(fields[2])
			if err != nil || mv < 0 || mv >= config.BoardCells {
				return sc, fail("proof move")
			}
			nodes, err := strconv.ParseUint(fields[3], 10, 64)
			if err != nil {
				return sc, fail("proof nodes")
			}
			if fields[4] != "vcf" && fields[4] != "vct" {
				return sc, fail("proof kind")
			}
			sc.proofs[k] = proofRec{move: rules.Move(mv), nodes: nodes, kind: fields[4]}
			sc.done[k] = struct{}{}
		case "N":
			if len(fields) != 2 {
				return sc, fail("no-proof line wants the key")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return sc, fail("no-proof key")
			}
			sc.done[k] = struct{}{}
		default:
			return sc, fail(fmt.Sprintf("unknown line kind %q", fields[0]))
		}
	}
	sc.clean = int64(end)
	return sc, nil
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
	osc, err := readOverlay(o.outPath)
	if err != nil {
		return err
	}
	want := overlayFingerprint(o.floor, o.budget)
	clean := osc.clean
	if osc.fp == "" {
		written, err := writeSidecarHeader(o.outPath, want)
		if err != nil {
			return err
		}
		clean = written
	} else if osc.fp != want {
		return fmt.Errorf("bookgen: overlay %s holds fingerprint %q, run config wants %q", o.outPath, osc.fp, want)
	}
	pending := keys[:0]
	for _, k := range keys {
		if _, ok := osc.done[k]; !ok {
			pending = append(pending, k)
		}
	}
	if len(pending) == 0 {
		fmt.Fprintf(progress, "overlay complete: %d positions processed at %s (%d proofs)\n", len(osc.done), o.outPath, len(osc.proofs))
		return nil
	}
	f, err := os.OpenFile(o.outPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("bookgen: open overlay %s: %w", o.outPath, err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Truncate(clean); err != nil {
		return fmt.Errorf("bookgen: truncate overlay %s to %d: %w", o.outPath, clean, err)
	}
	if _, err := f.Seek(clean, io.SeekStart); err != nil {
		return fmt.Errorf("bookgen: seek overlay %s: %w", o.outPath, err)
	}
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
			osc.proofs[k] = proofRec{move: mv, nodes: nodes, kind: kind}
			outcome = fmt.Sprintf("proof %s move %d nodes %d", kind, mv, nodes)
		}
		fmt.Fprintf(progress, "position %d/%d key %s... %s\n", i+1, len(pending), hexKey(k[:])[:12], outcome)
	}
	fmt.Fprintf(progress, "overlay done: %d positions, %d proofs\n", len(pending), len(osc.proofs))
	return nil
}
