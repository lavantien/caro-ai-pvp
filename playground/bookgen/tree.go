package main

import (
	"fmt"
	"io"
	"sort"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type analystPort func(b *rules.Board, budget int, k int) (leafRec, []rankedMove)

type solverPort func(b *rules.Board, budget int) (rules.Move, uint64, string, int, bool)

type frontierEntry struct {
	key      [config.BookKeyBytes]byte
	ply      int
	critical bool
	attempts int
}

type bookTree struct {
	params   treeParams
	plyBound int
	priors   priorsTable
	analyst  analystPort
	solver   solverPort
	log      treeLog
	logPath  string
	clean    int64
	spent    uint64
	frontier []frontierEntry
	root     [config.BookKeyBytes]byte
	stopped  string
}

func mateScore(distance int) int {
	if distance < 1 {
		distance = 1
	}
	return config.EvalMateMax - distance*config.EvalMateScoreStep
}

func (bt *bookTree) append(line string) error {
	clean, err := appendTreeLine(bt.logPath, bt.clean, line)
	if err != nil {
		return err
	}
	bt.clean = clean
	return nil
}

func backUp(child int) int {
	switch {
	case child > config.EvalMateMax/2:
		return -child + config.EvalMateScoreStep
	case child < -config.EvalMateMax/2:
		return -child - config.EvalMateScoreStep
	default:
		return -child
	}
}

func newBookTree(params treeParams, plyBound int, priors priorsTable, analyst analystPort, solver solverPort, logPath string) (*bookTree, error) {
	tl, err := parseTreeLog(logPath)
	if err != nil {
		return nil, err
	}
	want := params.fingerprint(plyBound)
	if tl.fp == "" {
		clean, err := writeSidecarHeader(logPath, want)
		if err != nil {
			return nil, err
		}
		tl.clean = clean
	} else if tl.fp != want {
		return nil, fmt.Errorf("bookgen: tree log %s holds fingerprint %q, run config wants %q", logPath, tl.fp, want)
	}
	bt := &bookTree{
		params: params, plyBound: plyBound, priors: priors, analyst: analyst, solver: solver,
		log: tl, logPath: logPath, clean: tl.clean, spent: tl.spent, root: canonicalKey(corpusZeroKey),
	}
	unfrozen := make([][config.BookKeyBytes]byte, 0, len(tl.solver))
	for k := range tl.solver {
		if _, ok := tl.frozen[k]; !ok {
			unfrozen = append(unfrozen, k)
		}
	}
	sort.Slice(unfrozen, func(i, j int) bool { return keyBefore(unfrozen[i], unfrozen[j]) })
	for _, k := range unfrozen {
		if err := bt.freeze(k, "win", mateScore(tl.solver[k].pvLen)); err != nil {
			return nil, err
		}
	}
	if _, done := bt.log.expansions[bt.root]; done {
		bt.collectChildren(bt.root)
	} else if _, frozen := bt.log.frozen[bt.root]; !frozen {
		bt.frontier = append(bt.frontier, frontierEntry{key: bt.root, ply: 0, critical: true})
	}
	return bt, nil
}

func (bt *bookTree) collectChildren(key [config.BookKeyBytes]byte) {
	ex, ok := bt.log.expansions[key]
	if !ok {
		return
	}
	for _, c := range ex.children {
		if _, done := bt.log.expansions[c.child]; done {
			bt.collectChildren(c.child)
			continue
		}
		if _, frozen := bt.log.frozen[c.child]; frozen {
			continue
		}
		if c.child == key {
			continue
		}
		bt.frontier = append(bt.frontier, frontierEntry{key: c.child, ply: ex.ply + 1, critical: ex.bestGap < bt.params.critMargin})
	}
}

func (bt *bookTree) nodeValue(key [config.BookKeyBytes]byte) (int, bool) {
	if f, ok := bt.log.frozen[key]; ok {
		return f.score, true
	}
	best := 0
	found := false
	if ex, ok := bt.log.expansions[key]; ok {
		for _, c := range ex.children {
			if v, ok := bt.nodeValue(c.child); ok {
				nv := backUp(v)
				if !found || nv > best {
					best, found = nv, true
				}
			}
		}
		if found {
			return best, true
		}
	}
	if recs := bt.log.analyses[key]; len(recs) > 0 {
		return recs[len(recs)-1].score, true
	}
	return 0, false
}

func (bt *bookTree) principalVariation() [][config.BookKeyBytes]byte {
	pv := make([][config.BookKeyBytes]byte, 0, bt.plyBound+2)
	node := bt.root
	for len(pv) <= bt.plyBound+1 {
		pv = append(pv, node)
		ex, ok := bt.log.expansions[node]
		if !ok {
			return pv
		}
		bestKey := [config.BookKeyBytes]byte{}
		best := 0
		found := false
		for _, c := range ex.children {
			if v, ok := bt.nodeValue(c.child); ok {
				nv := backUp(v)
				if !found || nv > best || (nv == best && keyBefore(c.child, bestKey)) {
					best, bestKey, found = nv, c.child, true
				}
			}
		}
		if !found {
			return pv
		}
		node = bestKey
	}
	return pv
}

func (bt *bookTree) sortFrontier(pv [][config.BookKeyBytes]byte) {
	pvSet := make(map[[config.BookKeyBytes]byte]struct{}, len(pv))
	for _, k := range pv {
		pvSet[k] = struct{}{}
	}
	sort.SliceStable(bt.frontier, func(i, j int) bool {
		a, b := bt.frontier[i], bt.frontier[j]
		_, aPV := pvSet[a.key]
		_, bPV := pvSet[b.key]
		if aPV != bPV {
			return aPV
		}
		aw := absInt(bt.provisional(a.key)) + a.attempts*bt.params.critMargin
		bw := absInt(bt.provisional(b.key)) + b.attempts*bt.params.critMargin
		if aw != bw {
			return aw < bw
		}
		if a.ply != b.ply {
			return a.ply < b.ply
		}
		return keyBefore(a.key, b.key)
	})
}

func (bt *bookTree) provisional(key [config.BookKeyBytes]byte) int {
	v, _ := bt.nodeValue(key)
	return v
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (bt *bookTree) classBudget(critical bool) int {
	if critical {
		return bt.params.critical
	}
	return bt.params.leaf
}

func (bt *bookTree) solverPass(key [config.BookKeyBytes]byte, b *rules.Board) error {
	if _, done := bt.log.solverDone[key]; done {
		return nil
	}
	mv, nodes, kind, pvLen, ok := bt.solver(b, bt.params.solver)
	bt.spent += nodes
	bt.log.solverDone[key] = struct{}{}
	if !ok {
		return bt.append(formatNoProof(key, nodes))
	}
	bt.log.solver[key] = solverRec{move: mv, kind: kind, pvLen: pvLen}
	if err := bt.append(formatSolver(key, mv, kind, nodes, pvLen)); err != nil {
		return err
	}
	return bt.freeze(key, "win", mateScore(pvLen))
}

func (bt *bookTree) analyze(key [config.BookKeyBytes]byte, b *rules.Board, budget, k int) (leafRec, []rankedMove, error) {
	rec, ranking := bt.analyst(b, budget, k)
	bt.spent += rec.nodes
	bt.log.analyses[key] = append(bt.log.analyses[key], rec)
	err := bt.append(formatLeaf(key, rec))
	return rec, ranking, err
}

func (bt *bookTree) tryFreeze(key [config.BookKeyBytes]byte, b *rules.Board, base int) (bool, []rankedMove, error) {
	ranking := []rankedMove(nil)
	for mult := 2; mult <= 1<<(bt.params.stableIters-1); mult *= 2 {
		dup := false
		for _, r := range bt.log.analyses[key] {
			if r.budget == base*mult {
				dup = true
			}
		}
		if dup {
			continue
		}
		_, rank, err := bt.analyze(key, b, base*mult, bt.params.opponentK)
		if err != nil {
			return false, ranking, err
		}
		ranking = rank
	}
	recs := bt.log.analyses[key]
	if len(recs) < bt.params.stableIters {
		return false, ranking, nil
	}
	lo, hi := recs[0].score, recs[0].score
	for _, r := range recs {
		if r.score < lo {
			lo = r.score
		}
		if r.score > hi {
			hi = r.score
		}
	}
	latest := recs[len(recs)-1].score
	if hi-lo <= bt.params.stableMarg && absInt(latest) >= bt.params.lopsided {
		return true, ranking, bt.freeze(key, "quiet", latest)
	}
	return false, ranking, nil
}

func (bt *bookTree) freeze(key [config.BookKeyBytes]byte, reason string, score int) error {
	bt.log.frozen[key] = freezeRec{reason: reason, score: score}
	if err := bt.append(formatFreeze(key, reason, score)); err != nil {
		return err
	}
	return bt.cascade()
}

func (bt *bookTree) cascade() error {
	changed := true
	for changed {
		changed = false
		for key, ex := range bt.log.expansions {
			if _, frozen := bt.log.frozen[key]; frozen {
				continue
			}
			all := true
			best := 0
			found := false
			for _, c := range ex.children {
				f, ok := bt.log.frozen[c.child]
				if !ok {
					all = false
					break
				}
				nv := backUp(f.score)
				if !found || nv > best {
					best, found = nv, true
				}
			}
			if all && found {
				reason := "backed"
				if best < -config.EvalMateMax/2 {
					reason = "loss"
				}
				if err := bt.freeze(key, reason, best); err != nil {
					return err
				}
				changed = true
			}
		}
	}
	return nil
}

func (bt *bookTree) expand(key [config.BookKeyBytes]byte, b *rules.Board, ply int, ranking []rankedMove) error {
	ranked := rankedByScoreMoves(ranking, key, bt.priors)
	var chosen []rankedMove
	if s, ok := bt.log.solver[key]; ok {
		if !b.IsLegal(rules.Cell(s.move)) {
			return fmt.Errorf("bookgen: solver proof move %d illegal in its position", s.move)
		}
		chosen = append(chosen, rankedMove{Move: s.move, Score: mateScore(s.pvLen)})
	} else if ply == 0 {
		openSet := make(map[rules.Move]struct{}, len(config.BookOpeningCells))
		for _, c := range config.BookOpeningCells {
			openSet[rules.Move(c)] = struct{}{}
		}
		for _, rm := range ranked {
			if _, ok := openSet[rm.Move]; ok {
				chosen = append(chosen, rm)
			}
		}
		if len(chosen) == 0 {
			return fmt.Errorf("bookgen: root analysis ranked no protocol opening cell")
		}
	} else if b.Side == rules.Blue {
		for i, rm := range ranked {
			if i >= bt.params.opponentK {
				break
			}
			chosen = append(chosen, rm)
		}
	} else {
		if len(ranked) == 0 {
			return fmt.Errorf("bookgen: analysis at ply %d returned no ranking", ply)
		}
		chosen = append(chosen, ranked[0])
	}
	if len(chosen) == 0 {
		return fmt.Errorf("bookgen: expansion at ply %d chose no move", ply)
	}
	gap := 0
	if len(chosen) >= 2 {
		gap = chosen[0].Score - chosen[1].Score
		if gap < 0 {
			gap = 0
		}
	}
	ex := treeExpansion{ply: ply, own: b.Side == rules.Red, best: chosen[0].Move, bestGap: gap}
	seen := make(map[[config.BookKeyBytes]byte]struct{}, len(chosen))
	for _, rm := range chosen {
		if !b.IsLegal(rules.Cell(rm.Move)) {
			return fmt.Errorf("bookgen: ranked move %d illegal in its position", rm.Move)
		}
		cb, err := boardFromKey(key)
		if err != nil {
			return fmt.Errorf("bookgen: expand position: %w", err)
		}
		cb.Make(rules.Cell(rm.Move))
		ck, _ := canonicalEntry(keyOf(cb), 0)
		if _, dup := seen[ck]; dup {
			continue
		}
		seen[ck] = struct{}{}
		ex.children = append(ex.children, childRec{move: rm.Move, child: ck})
	}
	bt.log.expansions[key] = &ex
	if err := bt.append(formatExpansion(key, ex)); err != nil {
		return err
	}
	for _, c := range ex.children {
		bt.frontier = append(bt.frontier, frontierEntry{
			key: c.child, ply: ply + 1, critical: gap < bt.params.critMargin,
		})
	}
	return nil
}

func (bt *bookTree) allHeld(pv [][config.BookKeyBytes]byte) bool {
	pvSet := make(map[[config.BookKeyBytes]byte]struct{}, len(pv)+4)
	for _, k := range pv {
		pvSet[k] = struct{}{}
	}
	if ex, ok := bt.log.expansions[pv[len(pv)-1]]; ok {
		for _, c := range ex.children {
			pvSet[c.child] = struct{}{}
		}
	}
	for _, e := range bt.frontier {
		if e.attempts == 0 {
			return false
		}
		if _, ok := pvSet[e.key]; ok {
			return false
		}
	}
	return true
}

func (bt *bookTree) over() bool {
	return bt.spent >= uint64(bt.params.total)
}

func (bt *bookTree) run(progress io.Writer) error {
	for bt.spent < uint64(bt.params.total) && len(bt.frontier) > 0 {
		pv := bt.principalVariation()
		bt.sortFrontier(pv)
		e := bt.frontier[0]
		bt.frontier = bt.frontier[1:]
		if _, frozen := bt.log.frozen[e.key]; frozen {
			continue
		}
		if _, done := bt.log.expansions[e.key]; done {
			continue
		}
		if e.ply >= 2 && e.attempts == 0 && len(pv) > 1 {
			if _, onPV := containsKey(pv, e.key); !onPV {
				e.attempts = 1
				bt.frontier = append(bt.frontier, e)
				if bt.allHeld(pv) {
					bt.stopped = "frontier held"
					return nil
				}
				continue
			}
		}
		b, err := boardFromKey(e.key)
		if err != nil {
			return fmt.Errorf("bookgen: tree position: %w", err)
		}
		if err := bt.solverPass(e.key, b); err != nil {
			return err
		}
		if _, frozen := bt.log.frozen[e.key]; frozen {
			continue
		}
		if bt.over() {
			bt.stopped = "budget spent"
			return nil
		}
		if e.ply == 0 {
			_, ranking, err := bt.analyze(e.key, b, bt.params.critical, config.SearchMaxMovesPerPly)
			if err != nil {
				return err
			}
			if err := bt.expand(e.key, b, 0, ranking); err != nil {
				return err
			}
			fmt.Fprintf(progress, "tree root expanded: %d openings, gap %d, spent %d\n", len(bt.log.expansions[bt.root].children), bt.log.expansions[bt.root].bestGap, bt.spent)
			continue
		}
		base := bt.classBudget(e.critical)
		rec, ranking, err := bt.analyze(e.key, b, base, bt.params.opponentK)
		if err != nil {
			return err
		}
		if e.ply >= bt.plyBound {
			if err := bt.freeze(e.key, "bound", rec.score); err != nil {
				return err
			}
			continue
		}
		frozen, rank, err := bt.tryFreeze(e.key, b, base)
		if err != nil {
			return err
		}
		if frozen {
			continue
		}
		if rank != nil {
			ranking = rank
		}
		if err := bt.expand(e.key, b, e.ply, ranking); err != nil {
			return err
		}
	}
	if len(bt.frontier) == 0 {
		bt.stopped = "frontier empty"
	} else {
		bt.stopped = "budget spent"
	}
	return nil
}

func containsKey(keys [][config.BookKeyBytes]byte, k [config.BookKeyBytes]byte) (int, bool) {
	for i, x := range keys {
		if x == k {
			return i, true
		}
	}
	return 0, false
}

func (bt *bookTree) frontierHeld() int {
	held := 0
	for _, e := range bt.frontier {
		if e.attempts > 0 {
			held++
		}
	}
	return held
}
