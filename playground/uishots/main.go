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
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func mustUser(store *server.Store, name, pass string) server.User {
	u, _, err := server.LoginOrCreate(store, name, pass, time.Now().Unix())
	if err != nil {
		log.Fatalf("uishots: seed %s: %v", name, err)
	}
	return u
}

func mustBotID(store *server.Store, tier config.Tier) int64 {
	id, err := store.BotAccountID(tier)
	if err != nil {
		log.Fatalf("uishots: bot seat %s: %v", tier.Name, err)
	}
	return id
}

var (
	seedSquareWinNames = []string{"H8", "I8", "M5", "J8", "H9", "K8", "H10", "L8", "H11", "B2", "H12"}
	seedHexWinNames    = []string{"L12", "M13", "P12", "N13", "N12", "O13", "O12", "L13", "M12"}
)

func movesIn(variant string, names []string) []rules.Move {
	g, err := geom.ByName(variant)
	if err != nil {
		log.Fatalf("uishots: variant %q: %v", variant, err)
	}
	ms := make([]rules.Move, 0, len(names))
	for _, n := range names {
		c, err := g.ParseCell(n)
		if err != nil {
			log.Fatalf("uishots: %s move %q: %v", variant, n, err)
		}
		ms = append(ms, rules.Move(c))
	}
	return ms
}

func replayOutcome(variant string, names []string) (rules.Color, string) {
	g, err := geom.ByName(variant)
	if err != nil {
		log.Fatalf("uishots: variant %q: %v", variant, err)
	}
	b := rules.NewBoardFor(g)
	for i, n := range names {
		parsed, err := g.ParseCell(n)
		if err != nil {
			log.Fatalf("uishots: %s replay %q: %v", variant, n, err)
		}
		c := rules.Cell(parsed)
		if !b.IsLegal(c) {
			log.Fatalf("uishots: %s replay move %d %q illegal", variant, i, n)
		}
		side := b.Side
		b.Make(c)
		if !b.FastLastMoveWin(side, c) {
			if i != len(names)-1 {
				continue
			}
			log.Fatalf("uishots: %s replay ends without a win", variant)
		}
		if i != len(names)-1 {
			log.Fatalf("uishots: %s replay move %d %q wins before the end", variant, i, n)
		}
		b.Unmake()
		tag := server.WonByTag(b, side)
		b.Make(c)
		return side, tag
	}
	log.Fatalf("uishots: %s replay never ended", variant)
	return rules.Empty, ""
}

func mustSeries(ctx context.Context, store *server.Store, red, blue int64) int64 {
	sr, err := store.CreateSeries(ctx, 1, config.SeriesBO3, red, blue)
	if err != nil {
		log.Fatalf("uishots: series: %v", err)
	}
	return sr.ID
}

func seedWin(ctx context.Context, store *server.Store, red, blue int64, botName string) {
	winner, tag := replayOutcome(config.DefaultVariant, seedSquareWinNames)
	if winner != rules.Red {
		log.Fatalf("uishots: square seed winner = %v, want red", winner)
	}
	wonBy := tag
	series := mustSeries(ctx, store, red, blue)
	finish := &server.SeriesFinish{SeriesID: series, Winner: &red, FinishedAt: time.Now().Unix()}
	if err := store.ApplyCompletion(ctx, server.Completion{
		Games: []server.Game{{
			SeriesID: series, IdxInSeries: 0, RedUser: red, BlueUser: blue,
			Outcome:   server.OutcomeRed,
			Moves:     server.EncodeMoves(nil, movesIn(config.DefaultVariant, seedSquareWinNames)),
			FullTurns: len(seedSquareWinNames) / 2, WonBy: &wonBy, BotName: botName,
		}},
		Finish: finish,
	}); err != nil {
		log.Fatalf("uishots: seed win %d over %d: %v", red, blue, err)
	}
}

func seedDraw(ctx context.Context, store *server.Store, red, blue int64) {
	series := mustSeries(ctx, store, red, blue)
	if err := store.ApplyCompletion(ctx, server.Completion{
		Games: []server.Game{{
			SeriesID: series, IdxInSeries: 0, RedUser: red, BlueUser: blue,
			Outcome: server.OutcomeDraw,
			Moves: server.EncodeMoves(nil, movesIn(config.DefaultVariant, []string{
				"H8", "B2", "D4", "B3", "H9", "C2", "I8", "C3", "J8", "C4",
			})),
			FullTurns: 5,
		}},
	}); err != nil {
		log.Fatalf("uishots: seed draw %d vs %d: %v", red, blue, err)
	}
}

func seedHexWin(ctx context.Context, store *server.Store, red, blue int64, botName string) {
	hexName := config.Variants[1].Name
	winner, tag := replayOutcome(hexName, seedHexWinNames)
	if winner != rules.Red {
		log.Fatalf("uishots: hex seed winner = %v, want red", winner)
	}
	wonBy := tag
	series, err := store.CreateSeriesFor(ctx, 1, config.SeriesBO3, red, blue, hexName)
	if err != nil {
		log.Fatalf("uishots: hex series: %v", err)
	}
	finish := &server.SeriesFinish{SeriesID: series.ID, Winner: &red, FinishedAt: time.Now().Unix()}
	if err := store.ApplyCompletion(ctx, server.Completion{
		Games: []server.Game{{
			SeriesID: series.ID, IdxInSeries: 0, RedUser: red, BlueUser: blue,
			Outcome:   server.OutcomeRed,
			Moves:     server.EncodeMoves(nil, movesIn(hexName, seedHexWinNames)),
			FullTurns: len(seedHexWinNames) / 2, WonBy: &wonBy, BotName: botName,
		}},
		Finish: finish,
	}); err != nil {
		log.Fatalf("uishots: seed hex win %d over %d: %v", red, blue, err)
	}
}

func seedProfiles(store *server.Store) {
	ctx := context.Background()
	alice := mustUser(store, "alice", "alicepw")
	bob := mustUser(store, "bob", "bobpw")
	easy := mustBotID(store, config.TierEasy)
	medium := mustBotID(store, config.TierMedium)
	seedWin(ctx, store, alice.ID, bob.ID, "")
	seedWin(ctx, store, bob.ID, alice.ID, "")
	seedWin(ctx, store, alice.ID, easy, "easy-b7f3a1c9")
	seedWin(ctx, store, medium, alice.ID, "medium-52dd8e40")
	seedDraw(ctx, store, alice.ID, bob.ID)
	seedWin(ctx, store, alice.ID, easy, "easy-31c9ab77")
	seedHexWin(ctx, store, alice.ID, easy, "easy-b7f3a1c9")
	fmt.Printf("uishots: seeded /players/alice, /players/bob, /players/%s, /players/%s, /players/%s\n",
		config.BotAccountName(0), config.BotAccountName(1), config.AdminName)
}

func main() {
	addr := flag.String("addr", "127.0.0.1:39779", "listen address")
	dbPath := flag.String("db", "", "sqlite path (default: fresh temp dir, removed on exit)")
	flag.Parse()
	dir := ""
	if *dbPath == "" {
		d, err := os.MkdirTemp("", "caro-uishots-")
		if err != nil {
			log.Fatalf("uishots: temp dir: %v", err)
		}
		dir, *dbPath = d, filepath.Join(d, "uishots.db")
	}
	remove := *dbPath
	if dir == "" {
		remove = ""
	}
	store, err := server.Open(*dbPath)
	if err != nil {
		log.Fatalf("uishots: open store: %v", err)
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rooms := server.NewRoomManager(hub, store, wq)
	seedProfiles(store)
	root := http.NewServeMux()
	api := server.NewHTTPAPI(store, rooms)
	root.Handle("/static/", api)
	server.NewLobbyPages(store, rooms).Mount(root)
	server.NewProfilePages(store).Mount(root)
	root.Handle("/", server.NewShellPages(store, rooms, nil))
	srv := &http.Server{Addr: *addr, Handler: root}
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
		<-sig
		ctxTO, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctxTO)
	}()
	fmt.Printf("uishots: listening on http://%s\n", *addr)
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
		log.Fatalf("uishots: serve: %v", err)
	}
}
