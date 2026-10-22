package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type treeStats struct {
	Canonical      int
	Emitted        int
	SolverProofs   int
	StableFrozen   int
	BoundFrozen    int
	LossFrozen     int
	FrontierAtStop int
	FrontierHeld   int
	TotalNodes     uint64
	ReverifyFlips  int
	PriorsGames    int
}

func (bt *bookTree) emitEntries() ([]bookEntry, treeStats, error) {
	st := treeStats{PriorsGames: bt.priors.games, TotalNodes: bt.spent, FrontierAtStop: len(bt.frontier), FrontierHeld: bt.frontierHeld()}
	for _, f := range bt.log.frozen {
		switch f.reason {
		case "quiet":
			st.StableFrozen++
		case "bound":
			st.BoundFrozen++
		case "loss":
			st.LossFrozen++
		}
	}
	st.ReverifyFlips = bt.flipCount()
	type ownerPlan struct {
		move   rules.Move
		visits uint32
		source uint8
	}
	plans := make(map[[config.BookKeyBytes]byte]ownerPlan)
	merge := func(key [config.BookKeyBytes]byte, p ownerPlan) error {
		old, ok := plans[key]
		if ok && old.source == config.BookSourceSolver && p.source != config.BookSourceSolver {
			return nil
		}
		if ok && p.source == config.BookSourceSolver && old.source != config.BookSourceSolver {
			plans[key] = p
			return nil
		}
		if ok && old != p {
			return fmt.Errorf("bookgen: canonical node %s holds conflicting emission plans", hexKey(key[:])[:12])
		}
		plans[key] = p
		return nil
	}
	for key, recs := range bt.log.analyses {
		if len(recs) == 0 {
			continue
		}
		b, err := boardFromKey(key)
		if err != nil {
			return nil, st, fmt.Errorf("bookgen: emit position: %w", err)
		}
		if b.Side != rules.Red {
			continue
		}
		if f, frozen := bt.log.frozen[key]; frozen && f.reason == "loss" {
			continue
		}
		latest := recs[len(recs)-1]
		if s, ok := bt.log.solver[key]; ok {
			if err := merge(key, ownerPlan{move: s.move, visits: uint32(config.BookTreeVisitScale), source: config.BookSourceSolver}); err != nil {
				return nil, st, err
			}
			continue
		}
		gap := 4 * bt.params.prune
		if ex, ok := bt.log.expansions[key]; ok {
			gap = ex.bestGap
		}
		visits := visitWeight(gap, bt.params.prune)
		if err := merge(key, ownerPlan{move: latest.best, visits: visits, source: config.BookSourceMajority}); err != nil {
			return nil, st, err
		}
	}
	for key := range bt.log.solver {
		if _, analyzed := plans[key]; analyzed {
			continue
		}
		b, err := boardFromKey(key)
		if err != nil {
			return nil, st, fmt.Errorf("bookgen: emit proof position: %w", err)
		}
		if b.Side != rules.Red {
			continue
		}
		s := bt.log.solver[key]
		if err := merge(key, ownerPlan{move: s.move, visits: uint32(config.BookTreeVisitScale), source: config.BookSourceSolver}); err != nil {
			return nil, st, err
		}
	}
	entries := make([]bookEntry, 0, len(plans)*dihedralOrder)
	for key, p := range plans {
		st.Canonical++
		if p.source == config.BookSourceSolver {
			st.SolverProofs++
		}
		for _, variant := range variantPairs(key) {
			b, err := boardFromKey(variant.key)
			if err != nil {
				return nil, st, fmt.Errorf("bookgen: emit variant: %w", err)
			}
			mv := rules.Move(transformCell(variant.transform, int(p.move)))
			if !b.IsLegal(rules.Cell(mv)) {
				return nil, st, fmt.Errorf("bookgen: emitted move %d illegal in variant %s", mv, hexKey(variant.key[:])[:12])
			}
			entries = append(entries, bookEntry{key: variant.key, move: mv, visits: p.visits, source: p.source})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return keyBefore(entries[i].key, entries[j].key) })
	for i := 1; i < len(entries); i++ {
		if entries[i].key == entries[i-1].key {
			return nil, st, fmt.Errorf("bookgen: duplicate emitted key %s", hexKey(entries[i].key[:])[:12])
		}
	}
	st.Emitted = len(entries)
	return entries, st, nil
}

func visitWeight(gap, margin int) uint32 {
	capped := gap
	if capped > 4*margin {
		capped = 4 * margin
	}
	visits := config.BookTreeVisitScale * capped / (4 * margin)
	if visits < 1 {
		visits = 1
	}
	return uint32(visits)
}

type keyTransform struct {
	key       [config.BookKeyBytes]byte
	transform int
}

func variantPairs(k [config.BookKeyBytes]byte) []keyTransform {
	seen := make(map[[config.BookKeyBytes]byte]struct{}, dihedralOrder)
	out := make([]keyTransform, 0, dihedralOrder)
	for t := range dihedral {
		v := transformKey(t, k)
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, keyTransform{key: v, transform: t})
	}
	sort.Slice(out, func(i, j int) bool { return keyBefore(out[i].key, out[j].key) })
	return out
}

func renderTreeStats(st treeStats) string {
	return fmt.Sprintf(`generator %s
canonical_entries %d
emitted_entries %d
solver_proofs %d
stable_frozen %d
bound_frozen %d
loss_frozen %d
frontier_at_stop %d
frontier_held %d
total_nodes %d
reverify_flips %d
priors_games %d
`, config.BookTreeGeneratorTag, st.Canonical, st.Emitted, st.SolverProofs, st.StableFrozen, st.BoundFrozen,
		st.LossFrozen, st.FrontierAtStop, st.FrontierHeld, st.TotalNodes, st.ReverifyFlips, st.PriorsGames)
}

func (bt *bookTree) writeArtifact(outPath, statsPath, created string) (treeStats, error) {
	entries, st, err := bt.emitEntries()
	if err != nil {
		return st, err
	}
	if len(entries) == 0 {
		return st, fmt.Errorf("bookgen: tree emit refused a zero-entry artifact to %s", outPath)
	}
	meta := bookMetaOut{
		Version:     config.BookFormatVersion,
		MaxPly:      bt.plyBound,
		Generator:   config.BookTreeGeneratorTag,
		CorpusGames: bt.priors.games,
		VisitFloor:  1,
		Created:     created,
	}
	if err := writeFileAtomic(outPath, buildArtifact(entries, meta)); err != nil {
		return st, err
	}
	if err := writeFileAtomic(statsPath, []byte(renderTreeStats(st))); err != nil {
		return st, err
	}
	return st, nil
}

func treeStoppedReason(bt *bookTree) string {
	if bt.stopped == "" {
		if len(bt.frontier) == 0 {
			return "frontier empty"
		}
		return "budget spent"
	}
	return bt.stopped
}

func treeSummary(bt *bookTree, st treeStats) string {
	var b strings.Builder
	fmt.Fprintf(&b, "tree done: %d canonical, %d emitted, %d solver proofs, %d stable, %d bound, %d loss, stop %s\n",
		st.Canonical, st.Emitted, st.SolverProofs, st.StableFrozen, st.BoundFrozen, st.LossFrozen, treeStoppedReason(bt))
	return b.String()
}
