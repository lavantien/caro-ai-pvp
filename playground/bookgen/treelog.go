package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type treeParams struct {
	tier        string
	plyBound    int
	opponentK   int
	leaf        int
	critical    int
	reverify    int
	critMargin  int
	stableIters int
	stableMarg  int
	lopsided    int
	prune       int
	total       int
	solver      int
}

func paramsFromConfig(tier string) treeParams {
	return treeParams{
		tier:        tier,
		opponentK:   config.BookTreeOpponentK,
		leaf:        config.BookTreeLeafNodes,
		critical:    config.BookTreeCriticalNodes,
		reverify:    config.BookTreeReverifyNodes,
		critMargin:  config.BookTreeCriticalMarginMilli,
		stableIters: config.BookTreeStableIters,
		stableMarg:  config.BookTreeStableMarginMilli,
		lopsided:    config.BookTreeLopsidedMilli,
		prune:       config.BookTreePruneMarginMilli,
		total:       config.BookTreeTotalNodes,
		solver:      config.BookSolverNodeBudget,
	}
}

func (p treeParams) fingerprint(plyBound int) string {
	return fmt.Sprintf("F tree v%d tier=%s ply=%d k=%d leaf=%d critical=%d reverify=%d critmargin=%d stable=%d stablemargin=%d lopsided=%d prune=%d total=%d solver=%d",
		config.BookSidecarVersion, p.tier, plyBound, p.opponentK, p.leaf, p.critical, p.reverify, p.critMargin,
		p.stableIters, p.stableMarg, p.lopsided, p.prune, p.total, p.solver)
}

type childRec struct {
	move  rules.Move
	child [config.BookKeyBytes]byte
}

type treeExpansion struct {
	ply      int
	own      bool
	best     rules.Move
	children []childRec
	bestGap  int
}

type leafRec struct {
	budget int
	score  int
	best   rules.Move
	nodes  uint64
}

type solverRec struct {
	move  rules.Move
	kind  string
	pvLen int
	lose  bool
}

type freezeRec struct {
	reason string
	score  int
}

type reverifyRec struct {
	key   [config.BookKeyBytes]byte
	smp   int
	delta int
	flip  bool
}

type treeLog struct {
	fp         string
	clean      int64
	torn       bool
	spent      uint64
	expansions map[[config.BookKeyBytes]byte]*treeExpansion
	analyses   map[[config.BookKeyBytes]byte][]leafRec
	solver     map[[config.BookKeyBytes]byte]solverRec
	solverDone map[[config.BookKeyBytes]byte]struct{}
	frozen     map[[config.BookKeyBytes]byte]freezeRec
	reverify   []reverifyRec
}

func newTreeLog() treeLog {
	return treeLog{
		expansions: make(map[[config.BookKeyBytes]byte]*treeExpansion),
		analyses:   make(map[[config.BookKeyBytes]byte][]leafRec),
		solver:     make(map[[config.BookKeyBytes]byte]solverRec),
		solverDone: make(map[[config.BookKeyBytes]byte]struct{}),
		frozen:     make(map[[config.BookKeyBytes]byte]freezeRec),
	}
}

func parseTreeLog(path string) (treeLog, error) {
	tl := newTreeLog()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return tl, nil
		}
		return tl, fmt.Errorf("bookgen: read tree log %s: %w", path, err)
	}
	end := len(data)
	if end > 0 && data[end-1] != '\n' {
		tl.torn = true
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			end = i + 1
		} else {
			end = 0
		}
	}
	off := 0
	lineno := 1
	if end > 0 {
		nl := bytes.IndexByte(data[:end], '\n')
		if nl < 0 {
			tl.torn = true
		} else {
			if err := sidecarHeaderError("tree", path, string(data[:nl]), 13); err != nil {
				return tl, err
			}
			tl.fp = string(data[:nl])
			tl.clean = int64(nl + 1)
			off = nl + 1
		}
	}
	fail := func(reason string) error {
		return fmt.Errorf("bookgen: tree log %s line %d: %s", path, lineno, reason)
	}
	for off < end {
		nl := off + bytes.IndexByte(data[off:end], '\n')
		line := data[off:nl]
		off = nl + 1
		lineno++
		fields := strings.Fields(string(line))
		if len(fields) == 0 {
			return tl, fail("blank line")
		}
		switch fields[0] {
		case "X":
			if len(fields) < 6 {
				return tl, fail("expansion wants key ply own gap children")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return tl, fail("expansion key")
			}
			ply, err := strconv.Atoi(fields[2])
			if err != nil || ply < 0 {
				return tl, fail("expansion ply")
			}
			own := fields[3] == "own"
			if fields[3] != "own" && fields[3] != "opp" {
				return tl, fail("expansion mover")
			}
			gap, err := strconv.Atoi(fields[4])
			if err != nil || gap < 0 {
				return tl, fail("expansion gap")
			}
			ex := &treeExpansion{ply: ply, own: own, bestGap: gap}
			for _, pair := range fields[5:] {
				mv, ck, perr := parseChildPair(pair)
				if perr != nil {
					return tl, fail("expansion child " + perr.Error())
				}
				ex.children = append(ex.children, childRec{move: mv, child: ck})
			}
			if len(ex.children) == 0 {
				return tl, fail("expansion without children")
			}
			ex.best = ex.children[0].move
			tl.expansions[k] = ex
			tl.clean = int64(off)
		case "L":
			if len(fields) != 6 {
				return tl, fail("leaf wants key budget score best nodes")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return tl, fail("leaf key")
			}
			budget, err := strconv.Atoi(fields[2])
			if err != nil || budget <= 0 {
				return tl, fail("leaf budget")
			}
			score, err := strconv.Atoi(fields[3])
			if err != nil {
				return tl, fail("leaf score")
			}
			best, err := strconv.Atoi(fields[4])
			if err != nil || best < 0 || best >= config.BoardCells {
				return tl, fail("leaf best")
			}
			nodes, err := strconv.ParseUint(fields[5], 10, 64)
			if err != nil {
				return tl, fail("leaf nodes")
			}
			tl.spent += nodes
			tl.analyses[k] = append(tl.analyses[k], leafRec{budget: budget, score: score, best: rules.Move(best), nodes: nodes})
			tl.clean = int64(off)
		case "S":
			if len(fields) != 6 {
				return tl, fail("proof wants key kind move nodes pvlen")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return tl, fail("proof key")
			}
			kind := fields[2]
			if kind != "vcf" && kind != "vct" {
				return tl, fail("proof kind")
			}
			mv, err := strconv.Atoi(fields[3])
			if err != nil || mv < 0 || mv >= config.BoardCells {
				return tl, fail("proof move")
			}
			nodes, err := strconv.ParseUint(fields[4], 10, 64)
			if err != nil {
				return tl, fail("proof nodes")
			}
			pvLen, err := strconv.Atoi(fields[5])
			if err != nil || pvLen < 1 {
				return tl, fail("proof pvlen")
			}
			tl.spent += nodes
			tl.solver[k] = solverRec{move: rules.Move(mv), kind: kind, pvLen: pvLen}
			tl.solverDone[k] = struct{}{}
			tl.clean = int64(off)
		case "N":
			if len(fields) != 3 {
				return tl, fail("no-proof wants key nodes")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return tl, fail("no-proof key")
			}
			nodes, err := strconv.ParseUint(fields[2], 10, 64)
			if err != nil {
				return tl, fail("no-proof nodes")
			}
			tl.spent += nodes
			tl.solverDone[k] = struct{}{}
			tl.clean = int64(off)
		case "F":
			if len(fields) != 4 {
				return tl, fail("freeze wants key reason score")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return tl, fail("freeze key")
			}
			reason := fields[2]
			if reason != "quiet" && reason != "win" && reason != "loss" && reason != "bound" && reason != "backed" {
				return tl, fail("freeze reason")
			}
			score, err := strconv.Atoi(fields[3])
			if err != nil {
				return tl, fail("freeze score")
			}
			tl.frozen[k] = freezeRec{reason: reason, score: score}
			tl.clean = int64(off)
		case "R":
			if len(fields) != 5 {
				return tl, fail("reverify wants key smp delta flip")
			}
			k, err := keyFromHex(fields[1])
			if err != nil {
				return tl, fail("reverify key")
			}
			smp, err := strconv.Atoi(fields[2])
			if err != nil {
				return tl, fail("reverify smp score")
			}
			delta, err := strconv.Atoi(fields[3])
			if err != nil {
				return tl, fail("reverify delta")
			}
			tl.reverify = append(tl.reverify, reverifyRec{key: k, smp: smp, delta: delta, flip: fields[4] == "flip"})
			tl.clean = int64(off)
		default:
			return tl, fail(fmt.Sprintf("unknown line kind %q", fields[0]))
		}
	}
	return tl, nil
}

func parseChildPair(pair string) (rules.Move, [config.BookKeyBytes]byte, error) {
	mvHex, keyHex, ok := strings.Cut(pair, ":")
	if !ok {
		return 0, [config.BookKeyBytes]byte{}, fmt.Errorf("wants move:childkey")
	}
	mv, err := strconv.Atoi(mvHex)
	if err != nil || mv < 0 || mv >= config.BoardCells {
		return 0, [config.BookKeyBytes]byte{}, fmt.Errorf("child move %q", mvHex)
	}
	k, err := keyFromHex(keyHex)
	if err != nil {
		return 0, [config.BookKeyBytes]byte{}, fmt.Errorf("child key")
	}
	return rules.Move(mv), k, nil
}

func appendTreeLine(path string, clean int64, line string) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return 0, fmt.Errorf("bookgen: open tree log %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Truncate(clean); err != nil {
		return 0, fmt.Errorf("bookgen: truncate tree log %s to %d: %w", path, clean, err)
	}
	if _, err := f.Seek(clean, io.SeekStart); err != nil {
		return 0, fmt.Errorf("bookgen: seek tree log %s: %w", path, err)
	}
	if _, err := f.WriteString(line); err != nil {
		return 0, fmt.Errorf("bookgen: append tree log %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		return 0, fmt.Errorf("bookgen: sync tree log %s: %w", path, err)
	}
	return clean + int64(len(line)), nil
}

func formatExpansion(k [config.BookKeyBytes]byte, ex treeExpansion) string {
	var b strings.Builder
	fmt.Fprintf(&b, "X %s %d", hexKey(k[:]), ex.ply)
	mover := "opp"
	if ex.own {
		mover = "own"
	}
	fmt.Fprintf(&b, " %s %d", mover, ex.bestGap)
	for _, c := range ex.children {
		fmt.Fprintf(&b, " %d:%s", c.move, hexKey(c.child[:]))
	}
	fmt.Fprintf(&b, "\n")
	return b.String()
}

func formatLeaf(k [config.BookKeyBytes]byte, r leafRec) string {
	return fmt.Sprintf("L %s %d %d %d %d\n", hexKey(k[:]), r.budget, r.score, r.best, r.nodes)
}

func formatSolver(k [config.BookKeyBytes]byte, mv rules.Move, kind string, nodes uint64, pvLen int) string {
	return fmt.Sprintf("S %s %s %d %d %d\n", hexKey(k[:]), kind, mv, nodes, pvLen)
}

func formatNoProof(k [config.BookKeyBytes]byte, nodes uint64) string {
	return fmt.Sprintf("N %s %d\n", hexKey(k[:]), nodes)
}

func formatFreeze(k [config.BookKeyBytes]byte, reason string, score int) string {
	return fmt.Sprintf("F %s %s %d\n", hexKey(k[:]), reason, score)
}

func formatReverify(r reverifyRec) string {
	flip := "hold"
	if r.flip {
		flip = "flip"
	}
	return fmt.Sprintf("R %s %d %d %s\n", hexKey(r.key[:]), r.smp, r.delta, flip)
}
