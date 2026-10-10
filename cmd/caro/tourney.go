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

var tourneyDrivers = map[string]func() tourney.RunSpec{
	"smoke32":     tourney.SmokeRoster32,
	"smoke10":     tourney.SmokeRoster10,
	"smoke105":    tourney.SmokeRoster105,
	"ponderprobe": tourney.PonderProbe,
	"full":        tourney.FullRoster24,
}

func runTourney(args []string) int {
	driver := ""
	idArg := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		driver, args = args[0], args[1:]
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
		fmt.Fprintln(os.Stderr, "caro: tourney needs exactly one driver (smoke32, smoke10, smoke105, ponderprobe, or full) plus flags")
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

func bootRooms(store *server.Store) (*server.RoomManager, *server.WriteQueue, *server.Hub) {
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	return server.NewRoomManager(hub, store, wq), wq, hub
}

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
	if closeErr != nil {
		fmt.Fprintln(os.Stderr, "caro:", closeErr)
		_ = store.Close()
		return 1
	}
	if cerr := store.Close(); cerr != nil {
		fmt.Fprintln(os.Stderr, "caro:", cerr)
		return 1
	}
	fmt.Printf("run %d closed\n", id)
	return 0
}

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
