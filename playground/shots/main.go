// Command shots serves the production web surface over staged, engine-free
// state for screenshot capture: a real store and room manager seeded at boot
// (users, a live PvP room, a finished PvP series, a finished human-vs-bot
// game with stat lines, an un-readied bot room) plus a scripted
// TourneyService standing in for the conductor. No searcher is ever
// constructed. Capture at 390x844 against the printed address; POST
// /shots/banner toggles the home live-run banner for the banner capture.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// Staging fixtures, the whole harness's tunable surface.
const (
	stageTCIdx = 1 // 2+1
	stageBO    = config.SeriesBO3
	aliceName  = "alice"
	alicePass  = "alicepw"
	bobName    = "bob"
	bobPass    = "bobpw"
	botNameTag = "hard-b3d17c92" // the seeded bot game's seat, room-shaped
)

// movesOf parses coordinate names into the move list the room and store eat.
func movesOf(names []string) []rules.Move {
	ms := make([]rules.Move, 0, len(names))
	for _, n := range names {
		c, err := rules.ParseCell(n)
		if err != nil {
			log.Fatalf("shots: stage move %q: %v", n, err)
		}
		ms = append(ms, rules.Move(c))
	}
	return ms
}

// playLine applies one scripted game's moves to a room, alternating by the
// mover list, then re-readies both seats for the next game of the series.
func playLine(r *server.Room, redID, blueID int64, names []string) {
	for i, m := range movesOf(names) {
		who := blueID
		if i%2 == 0 {
			who = redID
		}
		if err := r.PlayMove(who, rules.Cell(m)); err != nil {
			log.Fatalf("shots: stage move %d %s: %v", i+1, names[i], err)
		}
	}
}

func main() {
	addr := flag.String("addr", "127.0.0.1:39778", "listen address")
	dbPath := flag.String("db", "", "sqlite path (default: fresh temp dir, removed on exit)")
	keep := flag.Bool("keep", false, "keep the staging database on exit")
	flag.Parse()

	dir := ""
	if *dbPath == "" {
		d, err := os.MkdirTemp("", "caro-shots-")
		if err != nil {
			log.Fatalf("shots: temp dir: %v", err)
		}
		dir, *dbPath = d, filepath.Join(d, "shots.db")
	}
	remove := *dbPath
	if *keep || dir == "" {
		remove = ""
	}

	store, err := server.Open(*dbPath)
	if err != nil {
		log.Fatalf("shots: open store: %v", err)
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rooms := server.NewRoomManager(hub, store, wq)
	tour := newScriptTourney()

	ctx := context.Background()
	alice, _, err := server.LoginOrCreate(store, aliceName, alicePass, time.Now().Unix())
	if err != nil {
		log.Fatalf("shots: seed alice: %v", err)
	}
	bob, _, err := server.LoginOrCreate(store, bobName, bobPass, time.Now().Unix())
	if err != nil {
		log.Fatalf("shots: seed bob: %v", err)
	}

	// Live room: alice hosts red, 12 stones in, alice to move.
	live, err := rooms.Create(alice.ID, stageTCIdx, stageBO, nil)
	if err != nil {
		log.Fatalf("shots: live room: %v", err)
	}
	if err := rooms.Join(live.ID(), bob.ID); err != nil {
		log.Fatalf("shots: join live: %v", err)
	}
	if err := live.Ready(alice.ID); err != nil {
		log.Fatalf("shots: ready alice: %v", err)
	}
	if err := live.Ready(bob.ID); err != nil {
		log.Fatalf("shots: ready bob: %v", err)
	}
	playLine(live, alice.ID, bob.ID, []string{
		"H8", "I9", "D5", "J10", "E6", "H11", "F7", "G10", "I7", "J9", "K7", "H10",
	})

	// Terminal room: bob takes game 1 on the board; game 2 auto-starts with
	// alice on red, and POST /shots/finish later plays it out so an open
	// page holds the series-verdict render after the room retires.
	done, err := rooms.Create(bob.ID, stageTCIdx, stageBO, nil)
	if err != nil {
		log.Fatalf("shots: done room: %v", err)
	}
	if err := rooms.Join(done.ID(), alice.ID); err != nil {
		log.Fatalf("shots: join done: %v", err)
	}
	if err := done.Ready(bob.ID); err != nil {
		log.Fatalf("shots: ready bob (done): %v", err)
	}
	if err := done.Ready(alice.ID); err != nil {
		log.Fatalf("shots: ready alice (done): %v", err)
	}
	playLine(done, bob.ID, alice.ID, []string{
		"G8", "B2", "D4", "B3", "H8", "C2", "I8", "C3", "J8", "C4", "K8",
	})

	// Grid bot room, created and never readied: card on the grid, zero
	// engines, one ledger core held.
	if _, err := rooms.Create(alice.ID, stageTCIdx, stageBO, &config.TierEasy); err != nil {
		log.Fatalf("shots: bot room: %v", err)
	}

	seedHistory(ctx, store, alice.ID, bob.ID)

	root := http.NewServeMux()
	api := server.NewHTTPAPI(store, rooms)
	root.Handle("/api/", api)
	root.Handle("/static/", api)
	root.Handle("/spike", api)
	root.Handle("/shots/banner", shotsBanner(tour))
	root.Handle("/shots/finish", shotsFinish(done, alice.ID, bob.ID))
	server.NewRoomPages(rooms, store).Mount(root)
	server.NewTournamentPages(store, tour).Mount(root)
	root.Handle("/", server.NewShellPages(store, rooms, tour))

	srv := &http.Server{Addr: *addr, Handler: root}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		ctxTO, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctxTO)
	}()
	fmt.Printf("shots: listening on http://%s\n", *addr)
	fmt.Printf("shots: logins %s/%s %s/%s admin/%s\n",
		aliceName, alicePass, bobName, bobPass, config.AdminPassword)
	fmt.Printf("shots: live room %s (alice to move), terminal room %s\n", live.ID(), done.ID())
	err = srv.ListenAndServe()
	_ = srv.Close()
	rooms.Shutdown()
	wq.Close()
	hub.Close()
	_ = store.Close()
	if remove != "" {
		_ = os.RemoveAll(remove)
	}
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("shots: serve: %v", err)
	}
}

// shotsBanner toggles the scripted service's ongoing-run flag so the home
// page's banner appears only for the banner capture.
func shotsBanner(tour *scriptTourney) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		tour.bannerOn.Store(!tour.bannerOn.Load())
		w.WriteHeader(http.StatusNoContent)
	})
}

// shotsFinish plays the terminal room's second game out so an open browser
// page receives the series end over its stream and holds the honest terminal
// render: the client keeps the verdict after the room retires server-side.
func shotsFinish(r *server.Room, redID, blueID int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "post only", http.StatusMethodNotAllowed)
			return
		}
		playLine(r, redID, blueID, []string{
			"H8", "I6", "P1", "J6", "A1", "K6", "P16", "L6", "A16", "M6",
		})
		w.WriteHeader(http.StatusNoContent)
	})
}

// seedHistory writes the finished games the history and playback pages read:
// alice beating the hard bot (with stat lines) and bob beating alice.
func seedHistory(ctx context.Context, store *server.Store, aliceID, bobID int64) {
	botID, err := store.BotAccountID(config.TierHard)
	if err != nil {
		log.Fatalf("shots: bot seat: %v", err)
	}
	botSeries := mustSeries(ctx, store, aliceID, botID)
	pvpSeries := mustSeries(ctx, store, bobID, aliceID)
	botMoves := movesOf([]string{"H8", "I8", "M5", "J8", "H9", "K8", "H10", "L8", "H11", "B2", "H12"})
	pvpMoves := movesOf([]string{"G8", "B2", "D4", "B3", "H8", "C2", "I8", "C3", "J8", "C4", "K8"})
	wonBy := "4"
	if err := store.ApplyCompletion(ctx, server.Completion{
		Games: []server.Game{{
			SeriesID:    botSeries,
			IdxInSeries: 0, RedUser: aliceID, BlueUser: botID,
			Outcome:   server.OutcomeRed,
			Moves:     server.EncodeMoves(nil, botMoves),
			FullTurns: len(botMoves) / 2, WonBy: &wonBy, BotName: botNameTag,
			StatLines: []server.GameStat{
				{MoveNo: 2, Line: "M2, Blue, I8, d=10, n=22.4m, nps=3.31m, ebf=4.9, tt=11%, hf=99%, fh1=91%, s=-35, thr=4, t=20.87, alloc=20.87, pv=I8 J8"},
				{MoveNo: 4, Line: "M4, Blue, J8, d=11, n=24.1m, nps=3.28m, ebf=4.6, tt=12%, hf=99%, fh1=90%, s=-40, thr=4, t=21.03, alloc=21.03, pv=J8 K8"},
				{MoveNo: 6, Line: "M6, Blue, K8, d=10, n=18.9m, nps=3.05m, ebf=4.4, tt=10%, hf=99%, fh1=89%, s=-120, thr=4, t=19.44, alloc=19.44, pv=K8 L8"},
				{MoveNo: 8, Line: "M8, Blue, L8, d=8, n=9.8m, nps=3.12m, ebf=4.0, tt=8%, hf=99%, fh1=87%, s=M5, thr=4, t=17.02, alloc=17.02, pv=L8 M8"},
			},
		}},
		Finish: &server.SeriesFinish{SeriesID: botSeries, Winner: &aliceID, FinishedAt: time.Now().Unix()},
	}); err != nil {
		log.Fatalf("shots: seed bot game: %v", err)
	}
	if err := store.ApplyCompletion(ctx, server.Completion{
		Games: []server.Game{{
			SeriesID:    pvpSeries,
			IdxInSeries: 0, RedUser: bobID, BlueUser: aliceID,
			Outcome:   server.OutcomeRed,
			Moves:     server.EncodeMoves(nil, pvpMoves),
			FullTurns: len(pvpMoves) / 2, WonBy: &wonBy,
		}},
		Finish: &server.SeriesFinish{SeriesID: pvpSeries, Winner: &bobID, FinishedAt: time.Now().Unix()},
	}); err != nil {
		log.Fatalf("shots: seed pvp game: %v", err)
	}
}

// mustSeries opens an ongoing series row and returns its id.
func mustSeries(ctx context.Context, store *server.Store, red, blue int64) int64 {
	sr, err := store.CreateSeries(ctx, stageTCIdx, stageBO, red, blue)
	if err != nil {
		log.Fatalf("shots: seed series: %v", err)
	}
	return sr.ID
}
