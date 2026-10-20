package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func lobbyCookieFor(t *testing.T, st *Store, u User) *http.Cookie {
	t.Helper()
	tok := make([]byte, config.SessionTokenBytes)
	if _, err := rand.Read(tok); err != nil {
		t.Fatalf("session token: %v", err)
	}
	if err := st.InsertSession(Session{Token: tok, UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour).Unix()}); err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return &http.Cookie{Name: sessionCookieName, Value: hex.EncodeToString(tok)}
}

func TestLobbyModeMatrix(t *testing.T) {
	rows := []struct {
		host, guest, want string
	}{
		{"", "", lobbyModeCasual},
		{"", config.TierEasy.Name, lobbyModeVsBot},
		{config.TierEasy.Name, config.TierMedium.Name, lobbyModeBotVsBot},
	}
	for _, row := range rows {
		if got := lobbyMode(row.host, row.guest); got != row.want {
			t.Errorf("lobbyMode(%q, %q) = %q, want %q", row.host, row.guest, got, row.want)
		}
	}
}

func TestLobbySnapshotRoomsAndSpectators(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 1, config.SeriesBO5, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	snap := s.rm.LobbySnapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot rooms = %d, want 1", len(snap))
	}
	got := snap[0]
	if got.ID != r.ID() || got.Mode != lobbyModeCasual || got.Status != "created" || !got.Joinable {
		t.Errorf("casual snapshot = %+v, want an open casual room in created state", got)
	}
	if got.Host.Name != "alice" || got.Host.Tier != "" || got.Guest.Name != openSeatLabel || got.Guest.Tier != "" {
		t.Errorf("casual seats = host %+v guest %+v, want alice against the open seat", got.Host, got.Guest)
	}
	if got.TCIdx != 1 || got.BOLen != config.SeriesBO5 || got.Spectators != 0 {
		t.Errorf("casual settings = %+v, want tc 1 bo %d and 0 spectators", got, config.SeriesBO5)
	}
	subs := make([]*Subscription, 0, 2)
	for range 2 {
		sub, err := r.Subscribe()
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		subs = append(subs, sub)
	}
	if snap = s.rm.LobbySnapshot(); snap[0].Spectators != 2 {
		t.Errorf("spectators = %d, want 2", snap[0].Spectators)
	}
	for _, sub := range subs {
		sub.Unsubscribe()
	}
	if snap = s.rm.LobbySnapshot(); snap[0].Spectators != 0 {
		t.Errorf("spectators after unsubscribe = %d, want 0", snap[0].Spectators)
	}
	botRoom, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy)
	if err != nil {
		t.Fatalf("create vs bot: %v", err)
	}
	if snap = s.rm.LobbySnapshot(); len(snap) != 2 {
		t.Fatalf("snapshot rooms = %d, want 2", len(snap))
	}
	var bot LobbyRoom
	for _, lr := range snap {
		if lr.ID == botRoom.ID() {
			bot = lr
		}
	}
	if bot.Mode != lobbyModeVsBot || bot.Joinable || bot.Guest.Tier != config.TierEasy.Name || bot.Host.Name != "alice" {
		t.Errorf("bot snapshot = %+v, want a not-joinable vs-bot room with the easy tier seated", bot)
	}
	if bot.Guest.Name == "" || bot.Guest.Name == openSeatLabel {
		t.Errorf("bot guest name = %q, want the room bot display name", bot.Guest.Name)
	}
}

func TestLobbyPagesRoutes(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	carol := seedUser(t, s.store, "carol")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	call := func(method, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	rec := call(http.MethodGet, "/lobby", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /lobby = %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`hx-get="/partials/lobby"`,
		"every " + strconv.FormatInt(config.PagePollMs, 10) + "ms",
		r.ID()[:8],
		lobbyModeCasual,
		"alice",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("lobby body missing %q", want)
		}
	}
	if strings.Contains(body, "/join") {
		t.Errorf("guest lobby body offers a join action")
	}
	bobCookie := lobbyCookieFor(t, s.store, bob)
	rec = call(http.MethodGet, "/lobby", bobCookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `action="/lobby/`+r.ID()+`/join"`) {
		t.Errorf("member lobby = %d, want the join form on the open room", rec.Code)
	}
	rec = call(http.MethodGet, "/partials/lobby", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), lobbyModeCasual) {
		t.Errorf("partial = %d, want the polled fragment with the room", rec.Code)
	}
	rec = call(http.MethodPost, "/lobby/"+r.ID()+"/join", nil)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/login" {
		t.Errorf("guest join = %d %s, want the login redirect", rec.Code, rec.Header().Get("Location"))
	}
	rec = call(http.MethodPost, "/lobby/"+r.ID()+"/join", lobbyCookieFor(t, s.store, alice))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("host self join = %d, want 400", rec.Code)
	}
	rec = call(http.MethodPost, "/lobby/not-a-room/join", bobCookie)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "room not found") {
		t.Errorf("join with a malformed id = %d, want 400 with the not-found line", rec.Code)
	}
	rec = call(http.MethodPost, "/lobby/"+r.ID()+"/join", bobCookie)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/rooms/"+r.ID() {
		t.Errorf("bob join = %d %s, want the redirect into the room", rec.Code, rec.Header().Get("Location"))
	}
	info, ok := r.Info()
	if !ok || info.GuestUserID != bob.ID {
		t.Errorf("room after join = %+v ok %t, want bob seated", info, ok)
	}
	rec = call(http.MethodPost, "/lobby/"+r.ID()+"/join", lobbyCookieFor(t, s.store, carol))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "opponent") {
		t.Errorf("third join = %d, want 409 with the full-room line", rec.Code)
	}
}
