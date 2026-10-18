package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

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
visits_min %d
visits_median %s
visits_mean %s
visits_max %d
solver_nodes_total %d
solver_nodes_max %d
`,
		st.CorpusGames, st.Positions, st.FloorPassing, st.Entries, st.Majority, st.Solver,
		st.SolverVCF, st.SolverVCT, st.Trunks,
		formatFloat(st.PositionsPerGame), st.DiversityAlarm, st.VisitsMin,
		formatFloat(st.VisitsMedian), formatFloat(st.VisitsMean), st.VisitsMax,
		st.SolverNodesTotal, st.SolverNodesMax)
}

func formatFloat(v float64) string {
	return fmt.Sprintf("%g", v)
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
	proofs := osc.proofs
	st.CorpusGames = sc.games
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
		if int(e.move) >= config.BoardCells || b.At(rules.Cell(e.move)) != rules.Empty {
			return st, fmt.Errorf("bookgen: entry move %d is occupied or off board in its own key", e.move)
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
	if err := os.WriteFile(o.outPath, buildArtifact(entries, meta), 0o644); err != nil {
		return st, fmt.Errorf("bookgen: write artifact %s: %w", o.outPath, err)
	}
	if err := os.WriteFile(o.statsPath, []byte(renderStats(st)), 0o644); err != nil {
		return st, fmt.Errorf("bookgen: write stats %s: %w", o.statsPath, err)
	}
	fmt.Fprintf(progress, "emit done: %d entries (%d majority, %d solver) to %s, stats at %s\n",
		st.Entries, st.Majority, st.Solver, o.outPath, o.statsPath)
	return st, nil
}
