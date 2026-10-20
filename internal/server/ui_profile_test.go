package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func seedProfileGame(t *testing.T, s *Store, red, blue int64, outcome, botName string) {
	t.Helper()
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, red, blue)
	if err != nil {
		t.Fatalf("seed series: %v", err)
	}
	if _, err := s.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: red, BlueUser: blue,
		Outcome: outcome, Moves: []byte{1, 0}, FullTurns: 1, BotName: botName,
	}); err != nil {
		t.Fatalf("seed game: %v", err)
	}
}

func TestPlayerProfileSplitsTiersAndRecent(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	easy, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("easy seat: %v", err)
	}
	medium, err := s.BotAccountID(config.TierMedium)
	if err != nil {
		t.Fatalf("medium seat: %v", err)
	}
	seedProfileGame(t, s, alice.ID, bob.ID, OutcomeRed, "")
	seedProfileGame(t, s, alice.ID, bob.ID, OutcomeDraw, "")
	seedProfileGame(t, s, alice.ID, easy, OutcomeRed, "easy-abc123")
	seedProfileGame(t, s, alice.ID, medium, OutcomeBlue, "medium-def456")
	pp, err := s.PlayerProfile(alice)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if pp.Games != 4 || pp.All.Wins != 2 || pp.All.Losses != 1 || pp.All.Draws != 1 {
		t.Errorf("totals = %d %+v, want 4 games at 2-1-1", pp.Games, pp.All)
	}
	if pp.PvP != (wldRecord{Wins: 1, Draws: 1}) {
		t.Errorf("pvp split = %+v, want 1-0-1", pp.PvP)
	}
	if pp.Bot != (wldRecord{Wins: 1, Losses: 1}) {
		t.Errorf("bot split = %+v, want 1-1-0", pp.Bot)
	}
	wantTiers := []TierRecord{{Tier: config.TierEasy.Name, wldRecord: wldRecord{Wins: 1}}, {Tier: config.TierMedium.Name, wldRecord: wldRecord{Losses: 1}}}
	if len(pp.Tiers) != 2 || pp.Tiers[0] != wantTiers[0] || pp.Tiers[1] != wantTiers[1] {
		t.Errorf("tiers = %+v, want %+v in config order", pp.Tiers, wantTiers)
	}
	if len(pp.Recent) != 4 {
		t.Fatalf("recent rows = %d, want 4", len(pp.Recent))
	}
	newest := pp.Recent[0]
	if newest.Opponent != "medium-def456" || newest.Result != "loss" || newest.Turns != 1 {
		t.Errorf("newest recent row = %+v, want the medium loss with its bot display name", newest)
	}
	for _, g := range pp.Recent {
		if g.OpponentProfile == "" {
			t.Errorf("recent row %+v carries no profile link", g)
		}
	}
	botUser, err := s.UserByUsername(config.BotAccountName(0))
	if err != nil {
		t.Fatalf("bot seat user: %v", err)
	}
	bp, err := s.PlayerProfile(botUser)
	if err != nil {
		t.Fatalf("bot profile: %v", err)
	}
	if bp.Games != 1 || bp.All.Losses != 1 || bp.PvP.Losses != 1 {
		t.Errorf("bot seat profile = %d %+v, want its single loss in the human column", bp.Games, bp.All)
	}
	if len(bp.Recent) != 1 || bp.Recent[0].Opponent != "alice" || bp.Recent[0].Result != "loss" {
		t.Errorf("bot seat recent = %+v, want the loss against alice", bp.Recent)
	}
}

func TestProfileHeroRender(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	easy, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("easy seat: %v", err)
	}
	medium, err := s.BotAccountID(config.TierMedium)
	if err != nil {
		t.Fatalf("medium seat: %v", err)
	}
	seedProfileGame(t, s, alice.ID, bob.ID, OutcomeRed, "")
	seedProfileGame(t, s, bob.ID, alice.ID, OutcomeRed, "")
	seedProfileGame(t, s, alice.ID, bob.ID, OutcomeDraw, "")
	seedProfileGame(t, s, alice.ID, easy, OutcomeRed, "easy-abc123")
	seedProfileGame(t, s, medium, alice.ID, OutcomeRed, "medium-def456")
	seedProfileGame(t, s, alice.ID, easy, OutcomeRed, "easy-abc123")
	mux := http.NewServeMux()
	NewProfilePages(s).Mount(mux)
	get := func(path string) string {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		return rec.Body.String()
	}
	body := get("/players/alice")
	for _, want := range []string{
		`class="p-name"`, "alice", "6 games", "3W-2L-1D",
		strconv.Itoa(config.RatingStart),
		"seg-win", "width:50.0%", "seg-loss", "width:33.3%", "seg-draw", "width:16.7%",
		`class="chip win"`, `class="chip loss"`, `class="chip draw"`,
		`4</span> red`, `2</span> blue`, "/players/bob", "/rooms/history/",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("hero body missing %q", want)
		}
	}
	botBody := get("/players/" + url.PathEscape(config.BotAccountName(0)))
	for _, want := range []string{
		"AI seat", "0W-2L-0D", "width:100.0%", `class="chip loss"`,
	} {
		if !strings.Contains(botBody, want) {
			t.Errorf("bot seat body missing %q", want)
		}
	}
	if strings.Contains(botBody, "NaN") {
		t.Errorf("bot seat body carries a NaN width")
	}
}

func TestProfileZeroGamesRender(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	mux := http.NewServeMux()
	NewProfilePages(s).Mount(mux)
	req := httptest.NewRequest(http.MethodGet, "/players/"+config.AdminName, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /players/%s = %d", config.AdminName, rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"0 games", "no results yet.", "no games yet."} {
		if !strings.Contains(body, want) {
			t.Errorf("zero-game body missing %q", want)
		}
	}
	for _, banned := range []string{"NaN", `class="seg`, `style="width:`} {
		if strings.Contains(body, banned) {
			t.Errorf("zero-game body carries %q", banned)
		}
	}
}

func TestPlayerProfileRecentCap(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	for range config.ProfileRecentGames + 2 {
		seedProfileGame(t, s, bob.ID, alice.ID, OutcomeDraw, "")
	}
	pp, err := s.PlayerProfile(alice)
	if err != nil {
		t.Fatalf("profile: %v", err)
	}
	if pp.Games != config.ProfileRecentGames+2 {
		t.Errorf("games = %d, want %d", pp.Games, config.ProfileRecentGames+2)
	}
	if len(pp.Recent) != config.ProfileRecentGames {
		t.Errorf("recent rows = %d, want the cap %d", len(pp.Recent), config.ProfileRecentGames)
	}
}

func TestProfilePageRoute(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	bob := seedUser(t, s, "bob")
	easy, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("easy seat: %v", err)
	}
	seedProfileGame(t, s, alice.ID, bob.ID, OutcomeRed, "")
	seedProfileGame(t, s, alice.ID, easy, OutcomeBlue, "easy-abc123")
	mux := http.NewServeMux()
	NewProfilePages(s).Mount(mux)
	get := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	rec := get("/players/alice")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /players/alice = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"alice", "2 games", "1W-1L-0D", "humans", "bots",
		`AI ` + config.TierEasy.Name, "easy-abc123", "/rooms/history/",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("profile body missing %q", want)
		}
	}
	rec = get("/players/nobody")
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET /players/nobody = %d, want 404", rec.Code)
	}
	rec = get("/players/" + url.PathEscape(config.BotAccountName(0)))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /players/%s = %d, want the seeded seat page", config.BotAccountName(0), rec.Code)
	}
	botBody := rec.Body.String()
	if !strings.Contains(botBody, config.BotAccountName(0)) || !strings.Contains(botBody, "AI seat") {
		t.Errorf("bot seat page misses its name or seat chip")
	}
	if !strings.Contains(botBody, "win") || !strings.Contains(botBody, "alice") {
		t.Errorf("bot seat page misses its win row against alice")
	}
	rec = get("/players/" + config.AdminName)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "no games yet.") {
		t.Errorf("GET /players/%s = %d, want the empty profile", config.AdminName, rec.Code)
	}
}
