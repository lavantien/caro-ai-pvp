package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
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
// the per-series txt logs and the summary land under config.TournamentLogRoot in the run's own timestamped folder. The driver
// may sit ahead of or behind the flags (Go's flag package stops at the
// first positional, so a leading driver is peeled off before parsing).
// SIGINT or SIGTERM aborts the run through the conductor's cancel path.
// Any conductor error exits 1.
func runTourney(args []string) int {
	driver := ""
	idArg := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		driver, args = args[0], args[1:]
		// close and resume carry one more positional, the run id, peeled
		// with the driver so the id may also lead the flags.
		if (driver == "close" || driver == "resume") && len(args) > 0 && !strings.HasPrefix(args[0], "-") {
			idArg, args = args[0], args[1:]
		}
	}
	fs := flag.NewFlagSet("tourney", flag.ContinueOnError)
	dbPath := fs.String("db", defaultDBPath, "SQLite database path")
	parallel := fs.Int("parallel", config.TournamentParallelMatches, "rooms live at once")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	rest := fs.Args()
	if driver == "" && len(rest) > 0 {
		driver, rest = rest[0], rest[1:]
	}
	if driver == "close" || driver == "resume" {
		// The id rides either position: peeled ahead of the flags or as the
		// one positional left behind them.
		if idArg == "" && len(rest) == 1 {
			idArg, rest = rest[0], rest[1:]
		}
		if idArg == "" || len(rest) != 0 {
			fmt.Fprintf(os.Stderr, "caro: tourney %s needs exactly one run id\n", driver)
			return 2
		}
		if driver == "close" {
			return runTourneyClose(idArg, *dbPath)
		}
		return runTourneyResume(idArg, *dbPath, *parallel)
	}
	if len(rest) != 0 || driver == "" {
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
	rooms, wq, hub := bootRooms(store)
	conductor := tourney.NewConductor(tourney.RoomSource{RM: rooms})

	ctx, stop := signal.NotifyContext(context.Background(), serveSignals...)
	defer stop()
	res, err := conductor.Run(ctx, tourney.NewStore(store), spec.Roster,
		spec.TCIdx, spec.BOLen, spec.StartRating, *parallel, driver)

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

// runTourneyResume is the recovery arm's drive half: it boots the room stack
// exactly like a driver and hands the run to the conductor's resume, so an
// interrupted run (a cancelled drive, a killed process, a machine cut)
// finishes from its own persisted state: settled series stand as evidence,
// interrupted ones scrub and replay, the summary lands in the run's own
// folder. Any conductor error exits 1.
func runTourneyResume(idArg string, dbPath string, parallel int) int {
	id, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil || id <= 0 {
		fmt.Fprintf(os.Stderr, "caro: tourney resume needs a positive run id, got %q\n", idArg)
		return 2
	}
	store, err := server.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
		return 1
	}
	rooms, wq, hub := bootRooms(store)
	conductor := tourney.NewConductor(tourney.RoomSource{RM: rooms})

	ctx, stop := signal.NotifyContext(context.Background(), serveSignals...)
	defer stop()
	res, rerr := conductor.Resume(ctx, tourney.NewStore(store), id, parallel)
	if rerr == nil {
		roster, gerr := tourney.NewStore(store).Roster(ctx, id)
		if gerr != nil {
			rerr = gerr
		} else if perr := printRunResult(os.Stdout, roster, res); perr != nil {
			fmt.Fprintln(os.Stderr, "caro:", perr)
		}
	}
	rooms.Shutdown()
	wq.Close()
	hub.Close()
	if cerr := store.Close(); cerr != nil {
		fmt.Fprintln(os.Stderr, "caro:", cerr)
	}
	if rerr != nil {
		fmt.Fprintln(os.Stderr, "caro:", rerr)
		return 1
	}
	return 0
}

// bootRooms wires the room stack the driving arms share: the write queue,
// the hub, and the room manager over the opened store, closed by the caller
// in reverse boot order.
func bootRooms(store *server.Store) (*server.RoomManager, *server.WriteQueue, *server.Hub) {
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	return server.NewRoomManager(hub, store, wq), wq, hub
}

// runTourneyClose is the run gate's recovery arm: a run whose owning process
// died leaves an ongoing row that refuses every new start, and this flips it
// to finished through the manager's stalled close, the same path the run
// page's close form rides. No rooms boot: the close drives no matches, so the
// manager's match source stays nil.
func runTourneyClose(idArg string, dbPath string) int {
	id, err := strconv.ParseInt(idArg, 10, 64)
	if err != nil || id <= 0 {
		fmt.Fprintf(os.Stderr, "caro: tourney close needs a positive run id, got %q\n", idArg)
		return 2
	}
	store, err := server.Open(dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "caro:", err)
		return 1
	}
	closeErr := tourney.NewManager(tourney.NewStore(store), nil).
		CloseStalled(context.Background(), id)
	cerr := store.Close()
	if closeErr != nil {
		fmt.Fprintln(os.Stderr, "caro:", closeErr)
		return 1
	}
	if cerr != nil {
		fmt.Fprintln(os.Stderr, "caro:", cerr)
	}
	fmt.Printf("run %d closed\n", id)
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
