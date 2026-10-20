package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type emitOpts struct {
	corpusPath  string
	overlayPath string
	outPath     string
	statsPath   string
	floor       int
	created     string
}

type bookEntry struct {
	key    [config.BookKeyBytes]byte
	move   rules.Move
	visits uint32
	source uint8
}

type bookMetaOut struct {
	Version     int    `json:"version"`
	MaxPly      int    `json:"max_ply"`
	Generator   string `json:"generator"`
	CorpusGames int    `json:"corpus_games"`
	VisitFloor  int    `json:"visit_floor"`
	Created     string `json:"created"`
}

type emitStats struct {
	CorpusGames      int
	Positions        int
	FloorPassing     int
	Entries          int
	Majority         int
	Solver           int
	SolverVCF        int
	SolverVCT        int
	Trunks           int
	PositionsPerGame float64
	DiversityAlarm   bool
	IllegalFallbacks int
	VisitsMin        uint32
	VisitsMedian     float64
	VisitsMean       float64
	VisitsMax        uint32
	SolverNodesTotal uint64
	SolverNodesMax   uint64
}

func buildArtifact(entries []bookEntry, meta bookMetaOut) []byte {
	metaLine, _ := json.Marshal(meta)
	out := make([]byte, 0, len(config.BookMagicLine)+len(metaLine)+1+len(entries)*config.BookEntryBytes)
	out = append(out, config.BookMagicLine...)
	out = append(out, metaLine...)
	out = append(out, '\n')
	for _, e := range entries {
		buf := make([]byte, config.BookEntryBytes)
		copy(buf, e.key[:])
		binary.LittleEndian.PutUint16(buf[config.BookKeyBytes:], uint16(e.move))
		binary.LittleEndian.PutUint32(buf[config.BookKeyBytes+config.BookMoveBytes:], e.visits)
		buf[config.BookKeyBytes+config.BookMoveBytes+config.BookVisitsBytes] = e.source
		out = append(out, buf...)
	}
	return out
}

func visitSummary(visits []uint32) (min, max uint32, median, mean float64) {
	if len(visits) == 0 {
		return 0, 0, 0, 0
	}
	sorted := append([]uint32(nil), visits...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	total := uint64(0)
	for _, v := range sorted {
		total += uint64(v)
	}
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		median = float64(sorted[mid])
	} else {
		median = (float64(sorted[mid-1]) + float64(sorted[mid])) / 2
	}
	return sorted[0], sorted[len(sorted)-1], median, float64(total) / float64(len(sorted))
}

func renderStats(st emitStats) string {
	return fmt.Sprintf(`corpus_games %d
positions %d
floor_passing %d
entries %d
entries_majority %d
entries_solver %d
solver_vcf %d
solver_vct %d
trunks %d
positions_per_game %s
diversity_alarm %t
illegal_fallbacks %d
visits_min %d
visits_median %s
visits_mean %s
visits_max %d
solver_nodes_total %d
solver_nodes_max %d
`,
		st.CorpusGames, st.Positions, st.FloorPassing, st.Entries, st.Majority, st.Solver,
		st.SolverVCF, st.SolverVCT, st.Trunks,
		formatFloat(st.PositionsPerGame), st.DiversityAlarm, st.IllegalFallbacks, st.VisitsMin,
		formatFloat(st.VisitsMedian), formatFloat(st.VisitsMean), st.VisitsMax,
		st.SolverNodesTotal, st.SolverNodesMax)
}

func formatFloat(v float64) string {
	return fmt.Sprintf("%g", v)
}

func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp")
	if err != nil {
		return fmt.Errorf("bookgen: temp for %s: %w", path, err)
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return fmt.Errorf("bookgen: write temp %s: %w", name, err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return fmt.Errorf("bookgen: sync temp %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("bookgen: close temp %s: %w", name, err)
	}
	if err := os.Rename(name, path); err != nil {
		_ = os.Remove(name)
		return fmt.Errorf("bookgen: publish %s over %s: %w", name, path, err)
	}
	return nil
}

func emitRun(o emitOpts, progress io.Writer) (emitStats, error) {
	var st emitStats
	sc, err := loadCorpus(o.corpusPath)
	if err != nil {
		return st, err
	}
	agg := aggregate(sc.recs)
	osc, err := readOverlay(o.overlayPath)
	if err != nil {
		return st, err
	}
	if osc.fp != "" {
		solver, hasSolver := fingerprintField(osc.fp, "solver")
		corpus, hasCorpus := fingerprintField(osc.fp, "corpus")
		if !hasSolver || !hasCorpus || corpus != corpusTag(sc.fp) || solver != strconv.Itoa(config.BookSolverIdentity) {
			return st, fmt.Errorf("bookgen: emit refused overlay %s pinned to corpus %q solver %q over corpus %q solver %d",
				o.overlayPath, corpus, solver, sc.fp, config.BookSolverIdentity)
		}
	}
	proofs := osc.proofs
	st.CorpusGames = sc.games
	st.IllegalFallbacks = sc.illegal
	st.Positions = len(agg)
	keys := floorPassingKeys(agg, o.floor)
	st.FloorPassing = len(keys)
	entries := make([]bookEntry, 0, len(keys))
	visits := make([]uint32, 0, len(keys))
	for _, k := range keys {
		counts := agg[k]
		mv, cnt := majorityOf(counts)
		e := bookEntry{key: k, move: mv, visits: uint32(cnt), source: config.BookSourceMajority}
		if p, ok := proofs[k]; ok {
			e.move = p.move
			e.visits = uint32(counts[p.move])
			e.source = config.BookSourceSolver
			st.Solver++
			if p.kind == "vcf" {
				st.SolverVCF++
			} else {
				st.SolverVCT++
			}
			st.SolverNodesTotal += p.nodes
			if p.nodes > st.SolverNodesMax {
				st.SolverNodesMax = p.nodes
			}
		} else {
			st.Majority++
		}
		b, err := boardFromKey(k)
		if err != nil {
			return st, fmt.Errorf("bookgen: emit position: %w", err)
		}
		if !b.IsLegal(rules.Cell(e.move)) {
			return st, fmt.Errorf("bookgen: entry move %d is not a legal move in its own key", e.move)
		}
		entries = append(entries, e)
		visits = append(visits, e.visits)
	}
	st.Entries = len(entries)
	st.Trunks = len(agg[corpusZeroKey])
	if st.CorpusGames > 0 {
		st.PositionsPerGame = float64(st.Positions) / float64(st.CorpusGames)
	}
	st.DiversityAlarm = checkTrunkDiversity(st.CorpusGames, st.Trunks) != nil
	st.VisitsMin, st.VisitsMax, st.VisitsMedian, st.VisitsMean = visitSummary(visits)
	meta := bookMetaOut{
		Version:     config.BookFormatVersion,
		MaxPly:      config.BookMaxPly,
		Generator:   config.BookGeneratorTag,
		CorpusGames: sc.games,
		VisitFloor:  o.floor,
		Created:     o.created,
	}
	if len(entries) == 0 {
		return st, fmt.Errorf("bookgen: emit refused to write a zero-entry artifact to %s", o.outPath)
	}
	if err := writeFileAtomic(o.outPath, buildArtifact(entries, meta)); err != nil {
		return st, err
	}
	if err := writeFileAtomic(o.statsPath, []byte(renderStats(st))); err != nil {
		return st, err
	}
	fmt.Fprintf(progress, "emit done: %d entries (%d majority, %d solver) to %s, stats at %s\n",
		st.Entries, st.Majority, st.Solver, o.outPath, o.statsPath)
	return st, nil
}
