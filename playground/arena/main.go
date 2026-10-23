package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

var seats = buildSeats()

const (
	winnerRed  = "red"
	winnerBlue = "blue"
	winnerDraw = "draw"
)

func perturb(base config.Tier, name string, edit func(*config.Tier)) config.Tier {
	t := base
	t.Name = name
	edit(&t)
	return t
}

func halveTierShape(t *config.Tier) {
	t.Cores = t.Cores / 2
	t.TTBytes = t.TTBytes / 2
}

func parseEvalWeights(s string) ([config.PatternClassCount]int, error) {
	var w [config.PatternClassCount]int
	parts := strings.Split(s, ",")
	if len(parts) != config.PatternClassCount-1 {
		return w, fmt.Errorf("want %d comma-separated weights for classes 1..%d, got %d parts",
			config.PatternClassCount-1, config.PatternClassCount-1, len(parts))
	}
	for i, part := range parts {
		v, err := strconv.Atoi(part)
		if err != nil {
			return w, fmt.Errorf("weight %d %q: %v", i+1, part, err)
		}
		if v < 0 {
			return w, fmt.Errorf("weight %d = %d, want non-negative", i+1, v)
		}
		w[i+1] = v
	}
	return w, nil
}

func evalWeightsLabel(base string, w *[config.PatternClassCount]int) string {
	parts := make([]string, config.PatternClassCount-1)
	for i := range parts {
		parts[i] = strconv.Itoa(w[i+1])
	}
	return base + "-w" + strings.Join(parts, ".")
}

func registerEvalWeightsSeat(base string, w [config.PatternClassCount]int) (string, error) {
	b, ok := seats[base]
	if !ok {
		return "", fmt.Errorf("unknown base seat %q", base)
	}
	label := evalWeightsLabel(base, &w)
	seats[label] = perturb(b, label, func(t *config.Tier) { t.EvalWeights = &w })
	return label, nil
}

func applyEvalWeightFlags(seatsSel, weightsArg, baseArg string) (string, error) {
	if (weightsArg == "") != (baseArg == "") {
		return seatsSel, fmt.Errorf("-evalweights and -evalbase must be set together")
	}
	if weightsArg == "" {
		return seatsSel, nil
	}
	w, err := parseEvalWeights(weightsArg)
	if err != nil {
		return seatsSel, fmt.Errorf("evalweights %q: %w", weightsArg, err)
	}
	label, err := registerEvalWeightsSeat(baseArg, w)
	if err != nil {
		return seatsSel, fmt.Errorf("evalbase %q: %w", baseArg, err)
	}
	if seatsSel == "" {
		return baseArg + "," + label, nil
	}
	return seatsSel, nil
}

func buildSeats() map[string]config.Tier {
	m := map[string]config.Tier{
		"easy":                 config.TierEasy,
		"medium":               config.TierMedium,
		"hard":                 config.TierHard,
		"master":               config.TierMaster,
		"grandmaster":          config.TierGrandmaster,
		"easy-vcf":             perturb(config.TierEasy, "easy-vcf", func(t *config.Tier) { t.VCF = true }),
		"easy-tt32m":           perturb(config.TierEasy, "easy-tt32m", func(t *config.Tier) { t.TTBytes = 32 << 20 }),
		"medium-vct":           perturb(config.TierMedium, "medium-vct", func(t *config.Tier) { t.VCT = true }),
		"medium-tt256m":        perturb(config.TierMedium, "medium-tt256m", func(t *config.Tier) { t.TTBytes = 256 << 20 }),
		"hard-novct":           perturb(config.TierHard, "hard-novct", func(t *config.Tier) { t.VCT = false }),
		"hard-nosolver":        perturb(config.TierHard, "hard-nosolver", func(t *config.Tier) { t.VCF = false; t.VCT = false }),
		"hard-tt256m":          perturb(config.TierHard, "hard-tt256m", func(t *config.Tier) { t.TTBytes = 256 << 20 }),
		"hard-tt512m":          perturb(config.TierHard, "hard-tt512m", func(t *config.Tier) { t.TTBytes = 512 << 20 }),
		"hard-tt1g":            perturb(config.TierHard, "hard-tt1g", func(t *config.Tier) { t.TTBytes = 1 << 30 }),
		"master-novct":         perturb(config.TierMaster, "master-novct", func(t *config.Tier) { t.VCT = false }),
		"master-noponder":      perturb(config.TierMaster, "master-noponder", func(t *config.Tier) { t.PonderThreads = 0 }),
		"master-nobook":        perturb(config.TierMaster, "master-nobook", func(t *config.Tier) { t.Book = false }),
		"grandmaster-noponder": perturb(config.TierGrandmaster, "grandmaster-noponder", func(t *config.Tier) { t.PonderThreads = 0 }),
		"grandmaster-nobook":   perturb(config.TierGrandmaster, "grandmaster-nobook", func(t *config.Tier) { t.Book = false }),
		"master-cross":         perturb(config.TierMaster, "master-cross", halveTierShape),
		"grandmaster-cross":    perturb(config.TierGrandmaster, "grandmaster-cross", halveTierShape),
	}
	return m
}

type tally struct {
	name                                   string
	wins, losses, draws, redWins, blueWins int
	games                                  int
	ponderArms, ponderAdopts               int
}

type seatState struct {
	label   string
	tier    config.Tier
	eng     server.Searcher
	clock   *clock.GameClock
	ring    server.GrantDepthRing
	tally   *tally
	armed   bool
	predict rules.Move
	base    int
}

type duel struct {
	a, b string
	pt   Ptnml
}

func main() {
	tcIdx := flag.Int("tc", 0, "time control index into config.TimeControls")
	games := flag.Int("games", 12, "games per unordered seat pair, or games per floor dir in floor mode")
	seatsArg := flag.String("seats", "", "comma-separated seat labels, default all")
	evalWeightsArg := flag.String("evalweights", "", "comma-separated pattern class weights 1..5 for a derived seat, class 0 pinned to zero")
	evalBaseArg := flag.String("evalbase", "", "base seat label the evalweights override derives from")
	floorName := flag.String("floor", "", "floor pool base name under -floorOut, drives the same-tier pool then aggregates, exclusive of the round robin")
	floorOut := flag.String("floorOut", defaultFloorOutRoot, "root dir for floor pool dirs and floor artifacts")
	floorDirs := flag.Int("floorDirs", 5, "floor dirs to drive this invocation, 0 aggregates an existing pool only")
	floorBlockDirs := flag.Int("floorBlockDirs", 5, "dirs per extension block for the aggregate verdict")
	floorMaxBlocks := flag.Int("floorMaxBlocks", 3, "max extension blocks before the aggregate refuses more")
	floorSeat := flag.String("floorSeat", "master-nobook", "same-tier seat label for floor pools")
	floorZ := flag.Float64("floorZ", 1.96, "wald z multiplier for the floor interval")
	pairOut := flag.String("pairOut", "", "append-only resumable pair log path, requires exactly 2 seats, exclusive of floor mode")
	geometryArg := flag.String("geometry", "square", "board geometry variant name")
	floorVerdict := flag.String("floorVerdict", "", "floor mode verdict prefix: drive extension blocks to a covering interval or the block cap, then print one <prefix>: DONE or FAILED line")
	passgateOpp := flag.String("passgate", "", "hex master pass gate opponent tier among easy, medium, hard: master versus it over both seats, exclusive of floor and pair modes")
	ladder := flag.Bool("ladder", false, "hex ladder inversion check: easy, medium, hard, master round-robin, exclusive of floor and pair modes")
	fixedBudgetMs := flag.Int64("fixedBudgetMs", 0, "per-move fixed budget in ms replacing the time control clock, 0 keeps the clock")
	flag.Parse()

	env, err := resolveGeometry(*geometryArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arena: %v\n", err)
		os.Exit(1)
	}
	if (*passgateOpp != "" || *ladder) && (*floorName != "" || *pairOut != "") {
		fmt.Fprintln(os.Stderr, "arena: gate modes are exclusive of floor and pair modes")
		os.Exit(1)
	}
	if *passgateOpp != "" && *ladder {
		fmt.Fprintln(os.Stderr, "arena: passgate and ladder modes are exclusive of each other")
		os.Exit(1)
	}

	seatsSel, err := applyEvalWeightFlags(*seatsArg, *evalWeightsArg, *evalBaseArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "arena: %v\n", err)
		os.Exit(1)
	}

	if *passgateOpp != "" {
		if err := runHexPassGate(os.Stdout, env, *tcIdx, *games, *passgateOpp, *fixedBudgetMs); err != nil {
			fmt.Fprintf(os.Stderr, "arena: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if *ladder {
		if err := runHexLadder(os.Stdout, env, *tcIdx, *games, *fixedBudgetMs); err != nil {
			fmt.Fprintf(os.Stderr, "arena: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *floorName != "" {
		if err := runFloorMode(env, *floorName, *floorSeat, *floorOut, *tcIdx, *games, *floorDirs, *floorBlockDirs, *floorMaxBlocks, *floorZ, *floorVerdict, *fixedBudgetMs); err != nil {
			fmt.Fprintf(os.Stderr, "arena: %v\n", err)
			os.Exit(1)
		}
		return
	}

	labels := seatLabels(seatsSel)
	var plog *pairLog
	if *pairOut != "" {
		if *floorName != "" {
			fmt.Fprintln(os.Stderr, "arena: -pairOut is exclusive of floor mode")
			os.Exit(1)
		}
		if len(labels) != 2 {
			fmt.Fprintf(os.Stderr, "arena: -pairOut needs exactly 2 seats, got %d\n", len(labels))
			os.Exit(1)
		}
		if *games < 1 {
			fmt.Fprintln(os.Stderr, "arena: -pairOut needs -games >= 1")
			os.Exit(1)
		}
		pl, err := openPairLogForAppend(*pairOut, labels[0], labels[1], *tcIdx, *games, env.variant)
		if err != nil {
			fmt.Fprintf(os.Stderr, "arena: %v\n", err)
			os.Exit(1)
		}
		plog = &pl
	}
	bookPath, hasBook := config.BookArtifactPathFor(env.variant)
	for _, label := range labels {
		if !seats[label].Book {
			continue
		}
		if !hasBook {
			break
		}
		if _, err := os.Stat(bookPath); err != nil {
			fmt.Fprintf(os.Stderr, "arena: seat %s wants the book but %s is unreadable: %v\n", label, bookPath, err)
			os.Exit(1)
		}
		break
	}
	tc := config.TimeControls[*tcIdx]
	header := fmt.Sprintf("arena tc %d+%d seats %v games %d", tc.InitialMin, tc.IncrementSec, labels, *games)
	if env.variant != "square" {
		header += " geometry " + env.variant
	}
	fmt.Println(header)

	results := map[string]*tally{}
	for _, label := range labels {
		results[label] = &tally{name: label}
	}
	gameNo := 0
	var duels []*duel
	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			a, b := labels[i], labels[j]
			d := &duel{a: a, b: b}
			duels = append(duels, d)
			startGame := 0
			if plog != nil {
				for i, w := range plog.winners {
					red, blue := a, b
					if i%2 == 1 {
						red, blue = blue, red
					}
					record(results, red, blue, w, 0)
					d.pt.AddGame(resultFor(red == a, w))
					gameNo++
				}
				startGame = plog.done
				if plog.done > 0 {
					fmt.Printf("resumed pair from log: %s %d of %d games\n", plog.path, plog.done, *games)
				}
			}
			for g := startGame; g < *games; g++ {
				gameNo++
				red, blue := a, b
				if g%2 == 1 {
					red, blue = blue, red
				}
				fmt.Printf("game %d: %s (red) vs %s (blue), tc %d+%d\n", gameNo, red, blue, tc.InitialMin, tc.IncrementSec)
				side, drawn, moves := play(env, results[red], results[blue], *tcIdx, gameNo, *fixedBudgetMs)
				winnerSeat := winnerDraw
				if !drawn {
					if side == rules.Red {
						winnerSeat = winnerRed
					} else {
						winnerSeat = winnerBlue
					}
				}
				record(results, red, blue, winnerSeat, moves)
				fmt.Println(gameLine(gameNo, red, blue, winnerSeat, moves))
				if plog != nil {
					if err := plog.appendGame(g+1, winnerSeat, moves); err != nil {
						fmt.Fprintf(os.Stderr, "arena: %v\n", err)
						os.Exit(1)
					}
				}
				if bucket, closed := d.pt.AddGame(resultFor(red == a, winnerSeat)); closed {
					emitDuelPair(os.Stdout, a, b, g/2+1, bucket)
				}
			}
			if r, had := d.pt.Finish(); had {
				emitDuelUnpaired(os.Stdout, a, b, gameNo, r)
			}
			if plog != nil {
				if !plog.complete {
					if err := plog.appendComplete(); err != nil {
						fmt.Fprintf(os.Stderr, "arena: %v\n", err)
						os.Exit(1)
					}
				}
				if err := plog.close(); err != nil {
					fmt.Fprintf(os.Stderr, "arena: %v\n", err)
					os.Exit(1)
				}
			}
			fmt.Printf("pair done: %s vs %s\n", a, b)
		}
	}

	printResults(os.Stdout, labels, results, duels)
}

func printResults(w io.Writer, labels []string, results map[string]*tally, duels []*duel) {
	fmt.Fprintln(w, "results (wins-losses-draws over games, red/blue win split, ponder adopts/arms)")
	for _, label := range labels {
		r := results[label]
		fmt.Fprintf(w, "%-22s %d-%d-%d over %d, red wins %d blue wins %d, ponder %d/%d\n",
			label, r.wins, r.losses, r.draws, r.games, r.redWins, r.blueWins, r.ponderAdopts, r.ponderArms)
	}
	if len(duels) == 0 {
		return
	}
	fmt.Fprintln(w, "ptnml (duel pairs merged from the first seat's side: 2-0 1.5-0.5 1-1 0.5-1.5 0-2)")
	for _, d := range duels {
		fmt.Fprintf(w, "%-22s %s\n", d.a+" vs "+d.b, d.pt.Summary())
	}
}

func resultFor(refRed bool, winnerSeat string) Result {
	switch winnerSeat {
	case winnerDraw:
		return Draw
	}
	if (winnerSeat == winnerRed) == refRed {
		return Win
	}
	return Loss
}

func gameLine(game int, red, blue, winnerSeat string, moves int) string {
	return fmt.Sprintf("game %d: %s (red) vs %s, winner %s over %d moves", game, red, blue, winnerSeat, moves)
}

func emitDuelPair(w io.Writer, a, b string, pair, bucket int) {
	fmt.Fprintf(w, "duel %s vs %s pair %d: %s %s\n", a, b, pair, a, pairScores[bucket])
}

func emitDuelUnpaired(w io.Writer, a, b string, game int, r Result) {
	fmt.Fprintf(w, "duel %s vs %s unpaired game %d: %s %s\n", a, b, game, a, resultWords[r])
}

func seatLabels(arg string) []string {
	if arg == "" {
		return rosterOrder
	}
	var out []string
	for _, name := range splitComma(arg) {
		if _, ok := seats[name]; !ok {
			fmt.Fprintf(os.Stderr, "arena: unknown seat %q\n", name)
			os.Exit(1)
		}
		out = append(out, name)
	}
	return out
}

var rosterOrder = []string{"easy", "easy-vcf", "medium", "medium-vct", "hard", "hard-novct", "hard-nosolver"}

func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == ',' {
			if part := s[start:i]; part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}

func newSeatState(env gameEnv, t *tally, tcIdx int) *seatState {
	tier := seats[t.name]
	return &seatState{label: t.name, tier: tier, eng: server.NewBotSearcherFor(env.variant, tier), clock: clock.NewGameClock(tcIdx), tally: t}
}

func play(env gameEnv, redT, blueT *tally, tcIdx, gameNo int, fixedMs int64) (rules.Color, bool, int) {
	st := map[rules.Color]*seatState{
		rules.Red:  newSeatState(env, redT, tcIdx),
		rules.Blue: newSeatState(env, blueT, tcIdx),
	}
	defer st[rules.Red].eng.Close()
	defer st[rules.Blue].eng.Close()

	var legalBuf [config.BoardCells]rules.Move
	b := env.board()
	var lastMove rules.Move
	turnStart := time.Now()
	for {
		side := b.Side
		s := st[side]
		budget := s.clock.Budget()
		if fixedMs > 0 {
			budget = time.Duration(fixedMs) * time.Millisecond
		}
		mv, stats, tag, outcome := collectPonder(s, b, lastMove, budget, turnStart)
		if outcome != "" {
			if outcome == "adopt" {
				s.tally.ponderAdopts++
			}
			emitPonderOutcome(os.Stdout, gameNo, b.MoveCount+1, s.label, outcome)
		}
		if outcome != "adopt" {
			mv, stats, tag = s.eng.Search(b, engine.NewFixedBudget(budget))
			if tag == "" {
				s.ring.Append(int64(budget), stats.Depth)
			}
		}
		emitMLine(os.Stdout, gameNo, b.MoveCount+1, side, mv, &stats, tag)
		if !b.IsLegal(rules.Cell(mv)) {
			n := b.LegalMoves(legalBuf[:])
			mv = legalBuf[0]
			fmt.Fprintf(os.Stderr, "arena: %s returned illegal %v, fell back (legal count %d)\n",
				s.label, cellNameOf(mv), n)
		}
		cell := rules.Cell(mv)
		b.Make(cell)
		s.clock.Commit(time.Since(turnStart))
		turnStart = time.Now()
		lastMove = mv
		if b.FastLastMoveWin(side, cell) {
			return side, false, b.MoveCount
		}
		if b.IsFull() {
			return rules.Red, true, b.MoveCount
		}
		armPonder(s, b, stats)
	}
}

func collectPonder(s *seatState, b *rules.Board, lastMove rules.Move, budget time.Duration, turnStart time.Time) (rules.Move, engine.SearchStats, string, string) {
	if !s.armed {
		return 0, engine.SearchStats{}, "", ""
	}
	s.armed = false
	pmv, pst, ptag := s.eng.StopPonder()
	hit := b.MoveCount == s.base+1 && lastMove == s.predict
	adopted := hit && server.AdoptPonder(pst, ptag, server.AssessSoftLimit(pst.ElapsedNs, int64(budget)), s.ring, int64(budget))
	if outcome := ponderOutcome(true, hit, adopted); outcome != "adopt" {
		return 0, engine.SearchStats{}, "", outcome
	}
	st, tag := server.ComposePonderStats(pst, ptag, turnStart, int64(budget))
	return pmv, st, tag, "adopt"
}

func ponderOutcome(armed, hit, adopted bool) string {
	if !armed {
		return ""
	}
	if !hit {
		return "miss"
	}
	if adopted {
		return "adopt"
	}
	return "reject"
}

func emitMLine(w io.Writer, gameNo, moveNo int, side rules.Color, mv rules.Move, st *engine.SearchStats, tag string) {
	fmt.Fprintf(w, "g%d %s\n", gameNo, server.MLine(moveNo, side, mv, st, tag))
}

func emitPonderOutcome(w io.Writer, gameNo, moveNo int, seat, outcome string) {
	fmt.Fprintf(w, "g%d ponder %s m%d %s\n", gameNo, seat, moveNo, outcome)
}

func armPonder(s *seatState, b *rules.Board, stats engine.SearchStats) {
	if s.tier.PonderThreads < 1 || stats.PVLen < 2 {
		return
	}
	pb := *b
	if !pb.IsLegal(rules.Cell(stats.PV[1])) {
		return
	}
	s.base = pb.MoveCount
	s.predict = stats.PV[1]
	pb.Make(rules.Cell(s.predict))
	s.eng.StartPonder(&pb)
	s.armed = true
	s.tally.ponderArms++
}

func record(results map[string]*tally, redLabel, blueLabel, winnerSeat string, moves int) {
	results[redLabel].games++
	results[blueLabel].games++
	switch winnerSeat {
	case winnerDraw:
		results[redLabel].draws++
		results[blueLabel].draws++
	case winnerRed:
		results[redLabel].wins++
		results[blueLabel].losses++
		results[redLabel].redWins++
	case winnerBlue:
		results[blueLabel].wins++
		results[redLabel].losses++
		results[blueLabel].blueWins++
	}
}

func cellNameOf(m rules.Move) string {
	return string(rules.AppendCellName(nil, rules.Cell(m)))
}

func appendCellName(dst []byte, m rules.Move) []byte {
	return rules.AppendCellName(dst, rules.Cell(m))
}
