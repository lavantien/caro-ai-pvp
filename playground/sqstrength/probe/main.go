package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

type position struct {
	Leg   string   `json:"leg"`
	Game  int      `json:"game"`
	Move  int      `json:"move"`
	Pool  string   `json:"pool"`
	Cell  string   `json:"cell"`
	Score int      `json:"score"`
	Depth int      `json:"depth"`
	Tag   string   `json:"tag"`
	Moves []string `json:"moves"`
}

type positionFile struct {
	Positions []position `json:"positions"`
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "sqdiscrim: "+format+"\n", args...)
	os.Exit(1)
}

func readProgress(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	body := strings.TrimSpace(string(data))
	parts := strings.SplitN(body, "/", 2)
	if len(parts) != 2 {
		fail("progress file %s holds %q, want n/total", path, body)
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		fail("progress file %s holds %q: %v", path, body, err)
	}
	return n
}

func writeProgress(path string, done, total int) {
	if err := os.WriteFile(path, []byte(fmt.Sprintf("%d/%d\n", done, total)), 0o644); err != nil {
		fail("write progress %s: %v", path, err)
	}
}

func buildBoard(g *geom.Geometry, pos position) *rules.Board {
	b := rules.NewBoardFor(g)
	for _, name := range pos.Moves {
		cell, err := g.ParseCell(name)
		if err != nil {
			fail("position %s g%d m%d: cell %s: %v", pos.Leg, pos.Game, pos.Move, name, err)
		}
		if !b.IsLegal(rules.Cell(cell)) {
			fail("position %s g%d m%d: cell %s illegal at move %d", pos.Leg, pos.Game, pos.Move, name, b.MoveCount+1)
		}
		b.Make(rules.Cell(cell))
	}
	if b.MoveCount != pos.Move-1 {
		fail("position %s g%d m%d: rebuilt %d moves, want %d", pos.Leg, pos.Game, pos.Move, b.MoveCount, pos.Move-1)
	}
	if b.Side != rules.Blue {
		fail("position %s g%d m%d: side to move is %v, want Blue", pos.Leg, pos.Game, pos.Move, b.Side)
	}
	cell, err := g.ParseCell(pos.Cell)
	if err != nil || !b.IsLegal(rules.Cell(cell)) {
		fail("position %s g%d m%d: recorded in-game cell %s unusable", pos.Leg, pos.Game, pos.Move, pos.Cell)
	}
	return b
}

func scoreToken(score int) string {
	if score >= config.EvalMateFloor {
		return fmt.Sprintf("M%d", (config.EvalMateMax-score)/config.EvalMateScoreStep)
	}
	if score <= -config.EvalMateFloor {
		return fmt.Sprintf("-M%d", (config.EvalMateMax+score)/config.EvalMateScoreStep)
	}
	return fmt.Sprintf("%+d", score)
}

func tracePosition(w *bufio.Writer, g *geom.Geometry, idx int, pos position, budget time.Duration) (int, int, int) {
	b := buildBoard(g, pos)
	smp := engine.NewTieredEval(config.TierMaster, config.TierMaster.EvalWeights)
	defer smp.Close()
	mv, st := smp.Search(b, engine.NewFixedBudget(budget))
	if mv == rules.MoveNone || st.RootIters < 1 {
		fail("position %s g%d m%d: no iteration completed in %v", pos.Leg, pos.Game, pos.Move, budget)
	}
	n := min(config.PonderDepthHistory, st.RootIters)
	flip := 0
	minScore := 1 << 30
	for j := range n {
		depth := st.RootIters - n + 1 + j
		fmt.Fprintf(w, "p%02d %s g%d m%d iter d=%d s=%s cell=%s\n", idx, pos.Leg, pos.Game, pos.Move, depth, scoreToken(st.RootScores[j]), g.AppendName(nil, int(st.RootMoves[j])))
		if st.RootScores[j] < minScore {
			minScore = st.RootScores[j]
		}
		if flip == 0 && st.RootScores[j] < 0 {
			flip = depth
		}
	}
	line := server.MLineFor(g, pos.Move, rules.Blue, mv, &st, "")
	fmt.Fprintf(w, "p%02d %s g%d m%d %s\n", idx, pos.Leg, pos.Game, pos.Move, line)
	return st.RootIters, flip, minScore
}

func main() {
	positionsPath := flag.String("positions", "playground/sqstrength/out/discriminator-positions.json", "position list json")
	outPath := flag.String("out", "playground/sqstrength/out/discriminator-trace.log", "append target for the per-depth M-lines")
	progressPath := flag.String("progress", ".scratch/fix-sqdiscrim.progress", "resume counter n/total rewritten after each position")
	budgetSec := flag.Int("budgetSec", 60, "fixed per-position search budget in seconds")
	limit := flag.Int("limit", 0, "process at most n positions after the resume point, 0 means all")
	variant := flag.String("variant", "square", "board geometry variant")
	flag.Parse()

	g, err := geom.ByName(*variant)
	if err != nil {
		fail("geometry %s: %v", *variant, err)
	}
	raw, err := os.ReadFile(*positionsPath)
	if err != nil {
		fail("read %s: %v", *positionsPath, err)
	}
	var pf positionFile
	if err := json.Unmarshal(raw, &pf); err != nil {
		fail("parse %s: %v", *positionsPath, err)
	}
	total := len(pf.Positions)
	if total == 0 {
		fail("position list %s is empty", *positionsPath)
	}
	skip := readProgress(*progressPath)
	if skip > total {
		fail("progress %d exceeds position count %d", skip, total)
	}
	todo := total - skip
	if *limit > 0 && *limit < todo {
		todo = *limit
	}
	if err := os.MkdirAll(filepath.Dir(*outPath), 0o755); err != nil {
		fail("create out dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(*progressPath), 0o755); err != nil {
		fail("create progress dir: %v", err)
	}
	f, err := os.OpenFile(*outPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fail("open %s: %v", *outPath, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	budget := time.Duration(*budgetSec) * time.Second
	fmt.Fprintf(w, "probe start %s positions=%d budget=%v skip=%d limit=%d\n", time.Now().Format(time.RFC3339), total, budget, skip, todo)
	for i := skip; i < skip+todo; i++ {
		pos := pf.Positions[i]
		fmt.Fprintf(w, "pos %d/%d start %s g%d m%d pool=%s ingame=%d@%d %s\n", i+1, total, pos.Leg, pos.Game, pos.Move, pos.Pool, pos.Score, pos.Depth, pos.Tag)
		maxDepth, flip, minScore := tracePosition(w, g, i+1, pos, budget)
		fmt.Fprintf(w, "pos %d/%d done %s g%d m%d depths=1..%d flip=%d min=%d\n", i+1, total, pos.Leg, pos.Game, pos.Move, maxDepth, flip, minScore)
		w.Flush()
		writeProgress(*progressPath, i+1, total)
	}
	fmt.Fprintf(w, "probe done %s completed=%d\n", time.Now().Format(time.RFC3339), skip+todo)
	w.Flush()
}
