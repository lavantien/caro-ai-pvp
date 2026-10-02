package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"text/tabwriter"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

// tourneyDrivers maps the subcommand's driver names onto the headless specs
// of first-cause.md's Scenario 2 implications.
var tourneyDrivers = map[string]func() tourney.RunSpec{
	"smoke32": tourney.SmokeRoster32,
	"smoke10": tourney.SmokeRoster10,
	"full":    tourney.FullRoster24,
}

// runTourney drives one headless tournament against a real store: the
// conductor runs every pairing as a bot-vs-bot bo series on the room
// surface, per-series lines and the final leaderboard print to stdout, and
// the per-series txt logs land under config.TournamentLogDir. The driver
// may sit ahead of or behind the flags (Go's flag package stops at the
// first positional, so a leading driver is peeled off before parsing).
// SIGINT or SIGTERM aborts the run through the conductor's cancel path.
// Any conductor error exits 1.
func runTourney(args []string) int {
	driver := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		driver, args = args[0], args[1:]
	}
	fs := flag.NewFlagSet("tourney", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDBPath, "SQLite database path")
	parallel := fs.Int("parallel", config.TournamentParallelMatches, "rooms live at once")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if rest := fs.Args(); len(rest) == 1 && driver == "" {
		driver = rest[0]
	} else if len(rest) != 0 || driver == "" {
		fmt.Fprintln(os.Stderr, "caro: tourney needs exactly one driver (smoke32, smoke10, or full) plus flags")
		return 2
	}
	specFn, ok := tourneyDrivers[driver]
	if !ok {
		fmt.Fprintf(os.Stderr, "caro: unknown tourney driver %q\n", driver)
		return 2
	}
	spec := specFn()

	store, err := server.Open(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
		return 1
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rooms := server.NewRoomManager(hub, store, wq)
	conductor := tourney.NewConductor(tourney.RoomSource{RM: rooms})

	ctx, stop := signal.NotifyContext(context.Background(), serveSignals...)
	defer stop()
	res, err := conductor.Run(ctx, tourney.NewStore(store), spec.Roster,
		spec.TCIdx, spec.BOLen, spec.StartRating, *parallel)

	if err == nil {
		if perr := printRunResult(os.Stdout, spec.Roster, res); perr != nil {
			fmt.Fprintln(os.Stderr, "caro:", perr)
		}
	}
	// The stack closes in reverse boot order; the run already retired every
	// room, so Shutdown is the sweep that guarantees it.
	rooms.Shutdown()
	wq.Close()
	hub.Close()
	if cerr := store.Close(); cerr != nil {
		fmt.Fprintln(os.Stderr, "caro:", cerr)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
		return 1
	}
	return 0
}

// printRunResult writes the run's per-series lines and the final leaderboard.
func printRunResult(w io.Writer, roster []tourney.Participant, res tourney.RunResult) error {
	names := make(map[int]string, len(roster))
	for _, p := range roster {
		names[p.Slot] = p.Name
	}
	winner := func(slot *int) string {
		if slot == nil {
			return "drawn"
		}
		return names[*slot]
	}
	for _, line := range res.Series {
		if _, err := fmt.Fprintf(w, "series %2d: %s (red-first) %d - %d %s, winner %s\n",
			line.PairingSlot, names[line.RedFirst.Slot], line.RedFirstWins,
			line.BlueFirstWins, names[line.BlueFirst.Slot], winner(line.WinnerSlot)); err != nil {
			return fmt.Errorf("caro: print series line: %w", err)
		}
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "rank\tparticipant\trating\twins\tlosses\tdraws\tseries\tgames"); err != nil {
		return fmt.Errorf("caro: print leaderboard head: %w", err)
	}
	for i, st := range res.Board {
		if _, err := fmt.Fprintf(tw, "%d\t%s\t%d\t%d\t%d\t%d\t%d\t%d\n",
			i+1, names[st.Slot], st.Rating, st.Wins, st.Losses, st.Draws, st.SeriesWon, st.GamesPlayed); err != nil {
			return fmt.Errorf("caro: print leaderboard row: %w", err)
		}
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("caro: flush leaderboard: %w", err)
	}
	return nil
}
