package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

const genFormatVersion = 1

type genGame struct {
	idx     int
	outcome gameOutcome
	cells   []int
	illegal int
}

type gamesFile struct {
	fp       string
	tier     string
	budgetMs int64
	jitter   float64
	games    int
	seed     int64
	clean    int64
	torn     bool
	blocks   []genGame
}

type genOpts struct {
	Path   string
	Tier   string
	Budget time.Duration
	Jitter float64
	Games  int
	Seed   int64
	Seats  func() ([2]server.Searcher, error)
	MLines io.Writer
}

func genFingerprint(tier string, budgetMs int64, jitter float64, games int, seed int64) string {
	return fmt.Sprintf("F games v%d tier=%s budget-ms=%d jitter=%g games=%d seed=%d",
		genFormatVersion, tier, budgetMs, jitter, games, seed)
}

func genSeriesKey(path string, idx int) string {
	return fmt.Sprintf("%s#g%d", path, idx)
}

func outcomeName(o gameOutcome) string {
	switch o {
	case outcomeRedWin:
		return "red"
	case outcomeBlueWin:
		return "blue"
	}
	return "draw"
}

func parseOutcomeName(s string, idx int) (gameOutcome, error) {
	switch s {
	case "red":
		return outcomeRedWin, nil
	case "blue":
		return outcomeBlueWin, nil
	case "draw":
		return outcomeDraw, nil
	}
	return 0, fmt.Errorf("evalfit: game %d outcome %q is neither red, blue, nor draw", idx, s)
}

func writeGenHeader(path, fp string) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, fmt.Errorf("evalfit: write header %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	n, err := fmt.Fprintf(f, "%s\n", fp)
	if err != nil {
		return 0, fmt.Errorf("evalfit: write header %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		return 0, fmt.Errorf("evalfit: sync header %s: %w", path, err)
	}
	return int64(n), nil
}

func headerValue(fields []string, key string) (string, bool) {
	for _, f := range fields[2:] {
		if v, ok := strings.CutPrefix(f, key+"="); ok {
			return v, true
		}
	}
	return "", false
}

func parseGenFingerprint(line, path string) (string, error) {
	fields := strings.Fields(line)
	if len(fields) < 7 || fields[0] != "F" || fields[1] != "games" || fields[2] != fmt.Sprintf("v%d", genFormatVersion) {
		return "", fmt.Errorf("evalfit: games %s line 1: missing or malformed games header", path)
	}
	tier, ok := headerValue(fields, "tier")
	bms, okB := headerValue(fields, "budget-ms")
	jit, okJ := headerValue(fields, "jitter")
	games, okG := headerValue(fields, "games")
	seed, okS := headerValue(fields, "seed")
	if !ok || !okB || !okJ || !okG || !okS {
		return "", fmt.Errorf("evalfit: games %s line 1: games header lacks a field", path)
	}
	budget, err := strconv.ParseInt(bms, 10, 64)
	if err != nil {
		return "", fmt.Errorf("evalfit: games %s line 1: budget-ms unparsable", path)
	}
	jitter, err := strconv.ParseFloat(jit, 64)
	if err != nil {
		return "", fmt.Errorf("evalfit: games %s line 1: jitter unparsable", path)
	}
	n, err := strconv.Atoi(games)
	if err != nil {
		return "", fmt.Errorf("evalfit: games %s line 1: games unparsable", path)
	}
	sd, err := strconv.ParseInt(seed, 10, 64)
	if err != nil {
		return "", fmt.Errorf("evalfit: games %s line 1: seed unparsable", path)
	}
	return genFingerprint(tier, budget, jitter, n, sd), nil
}

func parseGamesFile(path string) (*gamesFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("evalfit: read games %s: %w", path, err)
	}
	gf := &gamesFile{}
	end := len(data)
	if end > 0 && data[end-1] != '\n' {
		gf.torn = true
		if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
			end = i + 1
		} else {
			end = 0
		}
	}
	lineno := 0
	off := 0
	if end > 0 {
		nl := bytes.IndexByte(data[:end], '\n')
		if nl < 0 {
			return nil, fmt.Errorf("evalfit: games %s line 1: missing or malformed games header", path)
		}
		fp, err := parseGenFingerprint(string(data[:nl]), path)
		if err != nil {
			return nil, err
		}
		gf.fp = fp
		gf.clean = int64(nl + 1)
		off = nl + 1
		fields := strings.Fields(string(data[:nl]))
		gf.tier, _ = headerValue(fields, "tier")
		gf.budgetMs, _ = strconv.ParseInt(mustField(fields, "budget-ms"), 10, 64)
		gf.jitter, _ = strconv.ParseFloat(mustField(fields, "jitter"), 64)
		gf.games, _ = strconv.Atoi(mustField(fields, "games"))
		gf.seed, _ = strconv.ParseInt(mustField(fields, "seed"), 10, 64)
	} else {
		return nil, fmt.Errorf("evalfit: games %s: empty file", path)
	}
	fail := func(reason string) error {
		return fmt.Errorf("evalfit: games %s line %d: %s", path, lineno, reason)
	}
	inBlock := false
	var cells []int
	var blk genGame
	for off < end {
		nl := off + bytes.IndexByte(data[off:end], '\n')
		line := data[off:nl]
		off = nl + 1
		lineno++
		parts := strings.Fields(string(line))
		if len(parts) == 0 {
			return nil, fail("blank line")
		}
		switch parts[0] {
		case "G":
			if inBlock {
				return nil, fail("game header inside a game block")
			}
			if len(parts) != 3 {
				return nil, fail("game header wants index and outcome")
			}
			idx, err := strconv.Atoi(parts[1])
			if err != nil || idx < 0 {
				return nil, fail("game header index")
			}
			if idx != len(gf.blocks) {
				return nil, fail(fmt.Sprintf("game index %d breaks the recorded sequence at %d", idx, len(gf.blocks)))
			}
			outcome, err := parseOutcomeName(parts[2], idx)
			if err != nil {
				return nil, fail(fmt.Sprintf("outcome %q is neither red, blue, nor draw", parts[2]))
			}
			blk = genGame{idx: idx, outcome: outcome}
			cells = nil
			inBlock = true
		case "M":
			if !inBlock {
				return nil, fail("move record outside a game block")
			}
			if len(parts) != 2 {
				return nil, fail("move record wants one cell")
			}
			cell, err := strconv.Atoi(parts[1])
			if err != nil || cell < 0 || cell >= config.BoardCells {
				return nil, fail("move cell out of the board")
			}
			cells = append(cells, cell)
		case "E":
			if !inBlock {
				return nil, fail("block end outside a game block")
			}
			if len(parts) != 3 {
				return nil, fail("block end wants the move and illegal counts")
			}
			n, err := strconv.Atoi(parts[1])
			if err != nil || n != len(cells) {
				return nil, fail(fmt.Sprintf("block end count %d over %d moves", n, len(cells)))
			}
			illegal, err := strconv.Atoi(parts[2])
			if err != nil || illegal < 0 {
				return nil, fail("block end illegal count")
			}
			blk.cells = append([]int(nil), cells...)
			blk.illegal = illegal
			gf.blocks = append(gf.blocks, blk)
			gf.clean = int64(off)
			inBlock = false
		default:
			return nil, fail(fmt.Sprintf("unknown line kind %q", parts[0]))
		}
	}
	if inBlock {
		gf.torn = true
	}
	return gf, nil
}

func mustField(fields []string, key string) string {
	v, _ := headerValue(fields, key)
	return v
}

func appendGenGame(path string, clean int64, blk genGame) (int64, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return 0, fmt.Errorf("evalfit: open games %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := f.Truncate(clean); err != nil {
		return 0, fmt.Errorf("evalfit: truncate games %s to %d: %w", path, clean, err)
	}
	if _, err := f.Seek(clean, io.SeekStart); err != nil {
		return 0, fmt.Errorf("evalfit: seek games %s: %w", path, err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "G %d %s\n", blk.idx, outcomeName(blk.outcome))
	for _, c := range blk.cells {
		fmt.Fprintf(&b, "M %d\n", c)
	}
	fmt.Fprintf(&b, "E %d %d\n", len(blk.cells), blk.illegal)
	if _, err := f.WriteString(b.String()); err != nil {
		return 0, fmt.Errorf("evalfit: append game %d: %w", blk.idx, err)
	}
	if err := f.Sync(); err != nil {
		return 0, fmt.Errorf("evalfit: sync games %s: %w", path, err)
	}
	return clean + int64(b.Len()), nil
}

func drawGenBudget(rng *rand.Rand, base time.Duration, jitter float64) time.Duration {
	if jitter <= 0 {
		return base
	}
	u := rng.Float64()
	f := 1 + jitter*(2*u-1)
	return time.Duration(float64(base) * f)
}

func genBudgets(base time.Duration, jitter float64, seed int64, idx, n int) []time.Duration {
	rng := rand.New(rand.NewSource(seed*1000003 + int64(idx)))
	out := make([]time.Duration, 0, n)
	for range n {
		out = append(out, drawGenBudget(rng, base, jitter))
	}
	return out
}

func playGenGame(engines [2]server.Searcher, redSeat, gameIdx int, base time.Duration, jitter float64, seed int64, mlines io.Writer) genGame {
	rng := rand.New(rand.NewSource(seed*1000003 + int64(gameIdx)))
	var legalBuf [config.BoardCells]rules.Move
	b := rules.NewBoard()
	res := genGame{idx: gameIdx}
	for {
		side := b.Side
		seat := redSeat
		if side == rules.Blue {
			seat = 1 - redSeat
		}
		var mv rules.Move
		legal := true
		if b.MoveCount == 0 {
			mv = rules.Move(config.BookOpeningCells[gameIdx%len(config.BookOpeningCells)])
			name, nerr := rules.CellName(rules.Cell(mv))
			if nerr != nil {
				name = strconv.Itoa(int(mv))
			}
			fmt.Fprintf(mlines, "g%d opening %s\n", gameIdx+1, name)
			if !b.IsLegal(rules.Cell(mv)) {
				n := b.LegalMoves(legalBuf[:])
				fmt.Fprintf(mlines, "g%d illegal opening %d, fell back (legal count %d)\n", gameIdx+1, mv, n)
				mv = legalBuf[0]
				legal = false
			}
		} else {
			budget := drawGenBudget(rng, base, jitter)
			var st engine.SearchStats
			var tag string
			mv, st, tag = engines[seat].Search(b, engine.NewFixedBudget(budget))
			fmt.Fprintf(mlines, "g%d %s\n", gameIdx+1, server.MLine(b.MoveCount+1, side, mv, &st, tag))
			if !b.IsLegal(rules.Cell(mv)) {
				n := b.LegalMoves(legalBuf[:])
				fmt.Fprintf(mlines, "g%d illegal %d from seat %d, fell back (legal count %d)\n", gameIdx+1, mv, seat, n)
				mv = legalBuf[0]
				legal = false
			}
		}
		if !legal {
			res.illegal++
		}
		res.cells = append(res.cells, int(mv))
		cell := rules.Cell(mv)
		b.Make(cell)
		if b.FastLastMoveWin(side, cell) {
			res.outcome = outcomeRedWin
			if side == rules.Blue {
				res.outcome = outcomeBlueWin
			}
			return res
		}
		if b.IsFull() {
			res.outcome = outcomeDraw
			return res
		}
	}
}

func genTier(name string) (config.Tier, error) {
	for i := range config.Tiers {
		if config.Tiers[i].Name == name {
			t := config.Tiers[i]
			t.Book = false
			return t, nil
		}
	}
	return config.Tier{}, fmt.Errorf("evalfit: unknown tier %q", name)
}

func runGenerate(o genOpts, progress func(idx int, res genGame)) error {
	tier, err := genTier(o.Tier)
	if err != nil {
		return err
	}
	if o.Budget <= 0 {
		return fmt.Errorf("evalfit: gen budget must be positive, got %v", o.Budget)
	}
	if o.Games <= 0 {
		return fmt.Errorf("evalfit: gen games must be positive, got %d", o.Games)
	}
	if o.Jitter < 0 || o.Jitter > 1 {
		return fmt.Errorf("evalfit: gen jitter must sit in [0, 1], got %v", o.Jitter)
	}
	want := genFingerprint(o.Tier, o.Budget.Milliseconds(), o.Jitter, o.Games, o.Seed)
	gf, err := parseGamesFile(o.Path)
	var clean int64
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		clean, err = writeGenHeader(o.Path, want)
		if err != nil {
			return err
		}
		gf = &gamesFile{fp: want, clean: clean}
	} else if gf.fp != want {
		return fmt.Errorf("evalfit: games %s holds fingerprint %q, run config wants %q", o.Path, gf.fp, want)
	} else {
		clean = gf.clean
	}
	mlines := o.MLines
	if mlines == nil {
		mlines = io.Discard
	}
	for i := len(gf.blocks); i < o.Games; i++ {
		engines, err := o.seatPair(tier)
		if err != nil {
			return err
		}
		res := playGenGame(engines, i%2, i, o.Budget, o.Jitter, o.Seed, mlines)
		engines[0].Close()
		engines[1].Close()
		clean, err = appendGenGame(o.Path, clean, res)
		if err != nil {
			return err
		}
		progress(i, res)
	}
	return nil
}

func (o genOpts) seatPair(tier config.Tier) ([2]server.Searcher, error) {
	if o.Seats != nil {
		return o.Seats()
	}
	return [2]server.Searcher{server.NewBotSearcher(tier), server.NewBotSearcher(tier)}, nil
}

func loadGamesFile(path string, tiers map[string]bool) (*gamesFile, bool, error) {
	gf, err := parseGamesFile(path)
	if err != nil {
		return nil, false, err
	}
	if len(tiers) > 0 && !tiers[gf.tier] {
		return nil, false, nil
	}
	return gf, true, nil
}

func gamesToParsed(gf *gamesFile) []parsedGame {
	out := make([]parsedGame, 0, len(gf.blocks))
	for _, blk := range gf.blocks {
		moves := make([]gameMove, 0, len(blk.cells))
		color := rules.Red
		for _, c := range blk.cells {
			moves = append(moves, gameMove{Color: color, Cell: rules.Cell(c)})
			color = color.Opponent()
		}
		out = append(out, parsedGame{
			Red:     seat{Name: "gen-red", Tier: gf.tier},
			Blue:    seat{Name: "gen-blue", Tier: gf.tier},
			Outcome: blk.outcome,
			Moves:   moves,
		})
	}
	return out
}
