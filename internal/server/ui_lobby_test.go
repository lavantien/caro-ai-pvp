package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestLobbySnapshotCarriesLiveBoard(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	snap := s.rm.LobbySnapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot rooms = %d, want 1", len(snap))
	}
	if created := snap[0]; created.Live || created.MoveCount != 0 || len(created.Moves) != 0 {
		t.Errorf("created room board = live %t moves %v, want no live game", created.Live, created.Moves)
	}
	if err := s.rm.Join(r.ID(), bob.ID); err != nil {
		t.Fatalf("join: %v", err)
	}
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"D4", "P16"})
	snap = s.rm.LobbySnapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot rooms = %d, want 1", len(snap))
	}
	got := snap[0]
	if !got.Live || got.MoveCount != 2 || len(got.Moves) != 2 {
		t.Fatalf("live board = live %t count %d moves %v, want the two played stones", got.Live, got.MoveCount, got.Moves)
	}
	if got.Moves[0] != "D4" || got.Moves[1] != "P16" {
		t.Errorf("moves = %v, want D4 then P16", got.Moves)
	}
	if !got.RedIsHost {
		t.Error("red is host = false, want the host holding red in a fresh series")
	}
}

func TestLobbyLivecardsMarkup(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	carol := seedUser(t, s.store, "carol")
	open, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create open: %v", err)
	}
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.rm.Join(r.ID(), bob.ID); err != nil {
		t.Fatalf("join: %v", err)
	}
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"D4", "P16"})
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	for _, path := range []string{"/lobby", "/partials/lobby"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		body := rec.Body.String()
		for _, want := range []string{
			`class="liveboards"`,
			`class="livecard"`,
			`href="/rooms/` + r.ID() + `"`,
			`class="board mini"`,
			`aria-hidden="true"`,
			`class="stone red"`,
			`class="stone blue latest"`,
			"2 stones",
			`<span class="mono">` + r.ID()[:8] + `</span>`,
			`href="/players/alice"`,
			lobbyModeCasual,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s missing %q", path, want)
			}
		}
	}
	req := httptest.NewRequest(http.MethodGet, "/lobby", nil)
	req.AddCookie(lobbyCookieFor(t, s.store, carol))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("member lobby = %d", rec.Code)
	}
	body := rec.Body.String()
	anchor := `href="/rooms/` + open.ID() + `"`
	ai := strings.Index(body, anchor)
	if ai < 0 {
		t.Fatalf("member lobby missing the spectate anchor")
	}
	ae := ai + strings.Index(body[ai:], "</a>")
	ji := strings.Index(body, `action="/lobby/`+open.ID()+`/join"`)
	if ae < 0 || ji < 0 {
		t.Fatalf("member card incomplete: anchor close %d join form %d", ae, ji)
	}
	if ji < ae {
		t.Error("join form sits inside the spectate anchor, want it outside")
	}
	if !strings.Contains(body[ai:ae], `class="board mini"`) {
		t.Error("spectate anchor missing the mini board")
	}
}

func TestLobbyQuickStartForm(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	get := func(path string, cookie *http.Cookie) string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s = %d", path, rec.Code)
		}
		return rec.Body.String()
	}
	member := get("/lobby", lobbyCookieFor(t, s.store, alice))
	if !strings.Contains(member, `method="post" action="/rooms"`) {
		t.Error("member lobby missing the quick-start create form")
	}
	if got := strings.Count(member, `class="tcchip"`); got != len(config.TimeControls) {
		t.Errorf("tc chips = %d, want one per time control (%d)", got, len(config.TimeControls))
	}
	for i := range config.TimeControls {
		if !strings.Contains(member, `name="tc" value="`+strconv.Itoa(i)+`"`) {
			t.Errorf("member lobby missing the tc chip for index %d", i)
		}
	}
	if !strings.Contains(member, `type="radio"`) || !strings.Contains(member, "checked") {
		t.Error("tc chips must be radios with a default pick")
	}
	for _, bo := range config.SeriesLengths {
		if !strings.Contains(member, `value="`+strconv.Itoa(bo)+`"`) {
			t.Errorf("member lobby missing the bo %d option", bo)
		}
	}
	if !strings.Contains(member, "human, open seat") {
		t.Error("member lobby missing the human opponent option")
	}
	for i := range config.Tiers {
		if !strings.Contains(member, `value="`+config.Tiers[i].Name+`"`) {
			t.Errorf("member lobby missing the %s bot option", config.Tiers[i].Name)
		}
	}
	guest := get("/lobby", nil)
	if strings.Contains(guest, `action="/rooms"`) {
		t.Error("guest lobby offers the create form, want the login line")
	}
	if !strings.Contains(guest, "login / create") || !strings.Contains(guest, `href="/login"`) {
		t.Error("guest lobby missing the login / create line")
	}
}

func TestLobbyQuickStartCreateErrorRendersLobby(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	mux.Handle("/", NewShellPages(s.store, s.rm, nil))
	post := func(form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/rooms", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(lobbyCookieFor(t, s.store, alice))
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec
	}
	rec := post(url.Values{"tc": {"99"}, "bo": {"3"}, "origin": {"lobby"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("lobby-origin create = %d, want 400", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "invalid room settings, pick from the lists") {
		t.Error("lobby-origin create misses the inline error line")
	}
	if !strings.Contains(body, `<legend>quick start</legend>`) || !strings.Contains(body, `aria-label="lobby"`) {
		t.Error("lobby-origin create re-rendered a page other than the lobby")
	}
	if strings.Contains(body, `<legend>create room</legend>`) {
		t.Error("lobby-origin create fell back to the home form legend")
	}
	rec = post(url.Values{"tc": {"99"}, "bo": {"3"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("originless create = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `<legend>create room</legend>`) {
		t.Error("originless create lost the home render")
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

func TestLobbySnapshotOrdersOldestRoomFirst(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	first, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	second.mu.Lock()
	second.createdAt = second.createdAt.Add(-time.Minute)
	second.mu.Unlock()
	snap := s.rm.LobbySnapshot()
	if len(snap) != 2 {
		t.Fatalf("snapshot rooms = %d, want 2", len(snap))
	}
	if snap[0].ID != second.ID() || snap[1].ID != first.ID() {
		t.Errorf("snapshot order = %s then %s, want the backdated room listed first", snap[0].ID, snap[1].ID)
	}
}

func TestLobbySnapshotSkipsClosedRoom(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	live, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create live: %v", err)
	}
	closed, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create closed: %v", err)
	}
	closed.Close()
	snap := s.rm.LobbySnapshot()
	if len(snap) != 1 || snap[0].ID != live.ID() {
		t.Errorf("snapshot after close = %d rooms, want only the live %s", len(snap), live.ID())
	}
}

func TestLobbySnapshotSeatFallsBackToIDMarkerWhenUserDeleted(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.store.db.Exec(`DELETE FROM users WHERE username = 'alice'`); err != nil {
		t.Fatalf("delete host: %v", err)
	}
	snap := s.rm.LobbySnapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot rooms = %d, want 1", len(snap))
	}
	if want := "#" + strconv.FormatInt(alice.ID, 10); snap[0].Host.Name != want {
		t.Errorf("host seat = %q, want the id marker %q", snap[0].Host.Name, want)
	}
	if snap[0].ID != r.ID() {
		t.Errorf("snapshot id = %s, want %s", snap[0].ID, r.ID())
	}
}

func TestLobbySeatViewOfProfiles(t *testing.T) {
	easyIdx := 0
	for i := range config.Tiers {
		if config.Tiers[i].Name == config.TierEasy.Name {
			easyIdx = i
		}
	}
	rows := []struct {
		seat        LobbySeat
		wantProfile string
	}{
		{LobbySeat{Name: "easy-room-1", Tier: config.TierEasy.Name}, "/players/" + url.PathEscape(config.BotAccountName(easyIdx))},
		{LobbySeat{Name: "ghost-room-1", Tier: "ghost"}, ""},
		{LobbySeat{Name: "alice"}, "/players/alice"},
		{LobbySeat{Name: openSeatLabel}, ""},
	}
	for _, row := range rows {
		got := lobbySeatViewOf(row.seat)
		if got.Profile != row.wantProfile || got.Name != row.seat.Name || got.Tier != row.seat.Tier {
			t.Errorf("lobbySeatViewOf(%+v) = %+v, want profile %q", row.seat, got, row.wantProfile)
		}
	}
}

func TestLobbyBotRoomLinksTierSeatToBotAccount(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &config.TierEasy); err != nil {
		t.Fatalf("create vs bot: %v", err)
	}
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	req := httptest.NewRequest(http.MethodGet, "/lobby", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /lobby = %d", rec.Code)
	}
	want := `href="/players/` + url.PathEscape(config.BotAccountName(0)) + `"`
	if !strings.Contains(rec.Body.String(), want) {
		t.Errorf("lobby body misses the tier seat link %q", want)
	}
}

func TestLobbyViewsSwapSeatsWhenGuestHoldsRed(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)
	playScript(t, r, bob.ID, alice.ID, []string{"D4"})
	snap := s.rm.LobbySnapshot()
	if len(snap) != 1 || !snap[0].Live || snap[0].RedIsHost {
		t.Fatalf("snapshot = %+v, want one live room with the guest holding red", snap)
	}
	views := lobbyViews(s.rm, nil)
	if len(views) != 1 {
		t.Fatalf("lobby views = %d, want 1", len(views))
	}
	if views[0].Red.Name != "bob" || views[0].Blue.Name != "alice" {
		t.Errorf("card seats = red %q blue %q, want the red guest bob against alice", views[0].Red.Name, views[0].Blue.Name)
	}
}

func TestLobbyPagesBrokenStoreAnswers500(t *testing.T) {
	dead := mustOpen(t, dbPath(t))
	if err := dead.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	mux := http.NewServeMux()
	NewLobbyPages(dead, s.rm).Mount(mux)
	for name, tc := range map[string]struct {
		method, path, want string
	}{
		"page":    {http.MethodGet, "/lobby", "lobby read failed"},
		"partial": {http.MethodGet, "/partials/lobby", "lobby read failed"},
		"join":    {http.MethodPost, "/lobby/" + r.ID() + "/join", "join read failed"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: strings.Repeat("0", 2*config.SessionTokenBytes)})
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s over a broken store = %d %q, want 500 with %q", name, rec.Code, rec.Body.String(), tc.want)
		}
	}
}

func TestLobbyJoinClosedRoomRendersClosedMessage(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	r.Close()
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	req := httptest.NewRequest(http.MethodPost, "/lobby/"+r.ID()+"/join", nil)
	req.AddCookie(lobbyCookieFor(t, s.store, bob))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("join closed room = %d, want 409", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "room closed") {
		t.Errorf("join closed room body misses the closed-room line: %s", rec.Body.String())
	}
}

func TestLobbyJoinWriteFailureAnswers500(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.store.db.Exec(`CREATE TRIGGER block_series_insert BEFORE INSERT ON series BEGIN SELECT RAISE(ABORT, 'series insert blocked'); END;`); err != nil {
		t.Fatalf("block series writes: %v", err)
	}
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	req := httptest.NewRequest(http.MethodPost, "/lobby/"+r.ID()+"/join", nil)
	req.AddCookie(lobbyCookieFor(t, s.store, bob))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("join with blocked series writes = %d, want 500 (body %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "join failed") {
		t.Errorf("join write failure body misses the plain failure line: %s", rec.Body.String())
	}
}

func TestJoinFailMessageCodes(t *testing.T) {
	rows := []struct {
		code, want string
	}{
		{codeRoomNotFound, "room not found or already closed"},
		{codeRoomFull, "room already has an opponent"},
		{codeRoomClosed, "room closed"},
		{codeBadRequest, "this room cannot be joined"},
		{codeInternal, "join failed, try again"},
	}
	for _, row := range rows {
		if got := joinFailMessage(row.code); got != row.want {
			t.Errorf("joinFailMessage(%q) = %q, want %q", row.code, got, row.want)
		}
	}
}
