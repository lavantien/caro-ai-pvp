package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

type neverDL struct{}

func (neverDL) Exceeded() bool { return false }
func (neverDL) Stop()          {}

type wallDL struct{ until time.Time }

func (d wallDL) Exceeded() bool { return time.Now().After(d.until) }
func (wallDL) Stop()            {}

type moveRec struct {
	color string
	cell  string
}

type flipRow struct {
	arm          string
	geometry     string
	game         int
	result       string
	masterSeat   string
	killerMove   int
	killerCell   string
	killerTag    string
	masterMove   int
	masterCell   string
	masterDepth  int
	masterTT     int
	recordedMate int
}

type gameKey struct {
	arm  string
	game int
}

func readCsv(path string) [][]string {
	f, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		panic(err)
	}
	return rows
}

func atoi(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return n
}

func loadFlips() []flipRow {
	rows := readCsv("../out/flips.csv")
	out := make([]flipRow, 0, len(rows)-1)
	for _, r := range rows[1:] {
		out = append(out, flipRow{
			arm: r[0], geometry: r[1], game: atoi(r[2]), result: r[3], masterSeat: r[4],
			killerMove: atoi(r[6]), killerCell: r[7], killerTag: r[9],
			masterMove: atoi(r[13]), masterCell: r[14], masterDepth: atoi(r[15]),
			masterTT: atoi(r[17]), recordedMate: atoi(r[11]),
		})
	}
	return out
}

func loadMoves() map[gameKey][]moveRec {
	rows := readCsv("../out/claim_games_moves.csv")
	m := make(map[gameKey][]moveRec)
	for _, r := range rows[1:] {
		k := gameKey{r[0], atoi(r[1])}
		m[k] = append(m[k], moveRec{color: r[4], cell: r[5]})
	}
	return m
}

func nameToCell(g *geom.Geometry, name string) rules.Cell {
	if g.Name != "hex" {
		cell, err := g.ParseCell(name)
		if err != nil {
			panic(err)
		}
		return rules.Cell(cell)
	}
	col := int(name[0]) - 'A'
	row := 0
	for _, ch := range name[1:] {
		if ch < '0' || ch > '9' {
			panic("bad hex log name " + name)
		}
		row = row*10 + int(ch-'0')
	}
	return rules.Cell((row-1)*16 + col)
}

func boardAt(g *geom.Geometry, recs []moveRec, upTo int) *rules.Board {
	b := rules.NewBoardFor(g)
	for i := 0; i < upTo && i < len(recs); i++ {
		b.Make(nameToCell(g, recs[i].cell))
	}
	return b
}

var mateFloor = config.EvalMateMax - config.SearchMaxPly*config.EvalMateScoreStep

type cfgRec struct {
	tt      int64
	threads int
}

func probeConfigs(flips []flipRow, moves map[gameKey][]moveRec) {
	f, err := os.Create("../out/replay_config.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"arm", "game", "result", "seat", "tt_mib", "threads", "score",
		"blind", "depth", "nodes_m", "tt_hit", "hash_full", "gt_kind", "gt_plies"})
	cfgs := []cfgRec{
		{0, 1}, {0, 6},
		{32 << 20, 1}, {32 << 20, 6},
		{128 << 20, 1}, {128 << 20, 4}, {128 << 20, 6},
		{768 << 20, 1}, {768 << 20, 2}, {768 << 20, 4}, {768 << 20, 6},
	}
	for _, fr := range flips {
		g, err := geom.ByName(fr.geometry)
		if err != nil {
			panic(err)
		}
		recs := moves[gameKey{fr.arm, fr.game}]
		b := boardAt(g, recs, fr.masterMove-1)
		after := *b
		after.Make(nameToCell(g, fr.masterCell))
		gtKind, gtPlies := groundTruth(&after)
		for _, c := range cfgs {
			t := config.Tier{Cores: c.threads, TTBytes: c.tt}
			s := engine.NewTieredEval(t, nil)
			st := searchAt(s, b, 300*time.Millisecond, 8)
			s.Close()
			w.Write([]string{
				fr.arm, strconv.Itoa(fr.game), fr.result, fr.masterSeat,
				strconv.FormatInt(c.tt>>20, 10), strconv.Itoa(c.threads),
				strconv.Itoa(st.Score),
				strconv.FormatBool(st.Score >= mateFloor),
				strconv.Itoa(st.Depth),
				fmt.Sprintf("%.2f", float64(st.Nodes)/1e6),
				strconv.Itoa(st.TTHitPermille),
				strconv.Itoa(st.HashFullPermille),
				gtKind, strconv.Itoa(gtPlies),
			})
			w.Flush()
		}
		fmt.Printf("config sweep done %s g%d gt %s %d\n", fr.arm, fr.game, gtKind, gtPlies)
	}
}

func searchAt(s *engine.SMP, b *rules.Board, budget time.Duration, depth int) engine.SearchStats {
	mv, st := s.SearchDepth(b, engine.NewFixedBudget(budget), depth)
	if mv == 0 && st.Nodes == 0 {
		panic("search returned nothing")
	}
	return st
}

func groundTruth(b *rules.Board) (string, int) {
	var out vcf.SolverStats
	dl := wallDL{time.Now().Add(10 * time.Second)}
	s := vcf.New(vcf.KindVCF)
	if s.Solve(b, config.SolverNodeBudget*4, dl, &out) {
		return "VCF", out.Plies
	}
	s2 := vcf.New(vcf.KindVCT)
	if s2.Solve(b, config.SolverNodeBudget*4, wallDL{time.Now().Add(10 * time.Second)}, &out) {
		return "VCT", out.Plies
	}
	return "none", 0
}

func probeWarm(flips []flipRow, moves map[gameKey][]moveRec) {
	f, err := os.Create("../out/replay_warm.csv")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	w.Write([]string{"arm", "game", "seat", "warm_blind", "warm_score", "warm_depth",
		"warm_tt_hit", "warm_hash_full", "warm_solver_found", "cold_solver_found",
		"warm_vs_cold_solver_disagree", "game_moves"})
	byGame := make(map[gameKey][]flipRow)
	for _, fr := range flips {
		byGame[gameKey{fr.arm, fr.game}] = append(byGame[gameKey{fr.arm, fr.game}], fr)
	}
	for k, frs := range byGame {
		fr := frs[0]
		g, err := geom.ByName(fr.geometry)
		if err != nil {
			panic(err)
		}
		recs := moves[k]
		t := config.Tier{Cores: 6, TTBytes: 768 << 20}
		s := engine.NewTieredEval(t, nil)
		warmSolver := vcf.New(vcf.KindVCF)
		b := rules.NewBoardFor(g)
		var at *flipRow
		var warmSt engine.SearchStats
		warmFound := false
		for i := 0; i < len(recs); i++ {
			for _, cand := range frs {
				if cand.masterMove == i+1 {
					at = &cand
					warmSt = searchAt(s, b, 120*time.Millisecond, 6)
					mc := nameToCell(g, recs[i].cell)
					after := *b
					after.Make(mc)
					var out vcf.SolverStats
					warmFound = warmSolver.Solve(&after, config.SolverNodeBudget*4, wallDL{time.Now().Add(10 * time.Second)}, &out)
				}
			}
			cell := nameToCell(g, recs[i].cell)
			b.Make(cell)
		}
		s.Close()
		if at == nil {
			continue
		}
		recs2 := moves[k]
		b2 := boardAt(g, recs2, at.masterMove-1)
		mc := nameToCell(g, at.masterCell)
		b2.Make(mc)
		coldKind, _ := groundTruth(b2)
		w.Write([]string{
			at.arm, strconv.Itoa(at.game), at.masterSeat,
			strconv.FormatBool(warmSt.Score >= mateFloor),
			strconv.Itoa(warmSt.Score), strconv.Itoa(warmSt.Depth),
			strconv.Itoa(warmSt.TTHitPermille), strconv.Itoa(warmSt.HashFullPermille),
			strconv.FormatBool(warmFound), coldKind,
			strconv.FormatBool(warmFound != (coldKind != "none")),
			strconv.Itoa(len(recs)),
		})
		w.Flush()
		fmt.Printf("warm walk done %s g%d\n", at.arm, at.game)
	}
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: replay <configs|warm|all>")
		return
	}
	flips := loadFlips()
	moves := loadMoves()
	mode := os.Args[1]
	if mode == "debug" {
		debugReplay(flips, moves)
		return
	}
	if mode == "configs" || mode == "all" {
		probeConfigs(flips, moves)
	}
	if mode == "warm" || mode == "all" {
		probeWarm(flips, moves)
	}
	fmt.Println("done")
}

func debugReplay(flips []flipRow, moves map[gameKey][]moveRec) {
	fr := flips[0]
	g, err := geom.ByName(fr.geometry)
	if err != nil {
		panic(err)
	}
	recs := moves[gameKey{fr.arm, fr.game}]
	b := rules.NewBoardFor(g)
	seen := make(map[rules.Cell]int)
	for i := 0; i < len(recs); i++ {
		c := nameToCell(g, recs[i].cell)
		if prev, dup := seen[c]; dup {
			fmt.Printf("dup flat %d at move %d, first seen move %d, name %s\n", c, i+1, prev, recs[i].cell)
		}
		seen[c] = i + 1
		if !b.IsLegal(c) {
			fmt.Printf("illegal flat %d from name %s at move %d, at=%d occupied=%d\n", c, recs[i].cell, i+1, b.At(c), 0)
			continue
		}
		b.Make(c)
	}
	fmt.Printf("debug done arm %s game %d moves %d\n", fr.arm, fr.game, len(recs))
}
