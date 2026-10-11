package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func noRedirectClient(srv *httptest.Server) *http.Client {
	return &http.Client{
		Transport: srv.Client().Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func doShell(t *testing.T, c *http.Client, method, rawurl, token string, form url.Values) (int, http.Header, string) {
	t.Helper()
	var rd io.Reader
	if form != nil {
		rd = strings.NewReader(form.Encode())
	}
	req, err := http.NewRequest(method, rawurl, rd)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, rawurl, err)
	}
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if token != "" {
		req.Header.Set("Cookie", sessionCookieName+"="+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, rawurl, err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read %s %s: %v", method, rawurl, err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

func doRawPost(t *testing.T, c *http.Client, rawurl, token, contentType, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, rawurl, strings.NewReader(body))
	if err != nil {
		t.Fatalf("new post %s: %v", rawurl, err)
	}
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("Cookie", sessionCookieName+"="+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("post %s: %v", rawurl, err)
	}
	b, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read post %s: %v", rawurl, err)
	}
	return resp.StatusCode, string(b)
}

func shellCookieValue(t *testing.T, h http.Header) string {
	t.Helper()
	for _, c := range h.Values("Set-Cookie") {
		if !strings.HasPrefix(c, sessionCookieName+"=") {
			continue
		}
		v := strings.TrimPrefix(c, sessionCookieName+"=")
		if i := strings.IndexByte(v, ';'); i >= 0 {
			v = v[:i]
		}
		return v
	}
	t.Fatal("no session cookie in the response headers")
	return ""
}

func shellRegister(t *testing.T, c *http.Client, base, name, pass string) string {
	t.Helper()
	status, h, body := doShell(t, c, http.MethodPost, base+"/login", "", url.Values{
		"username": {name}, "password": {pass},
	})
	if status != http.StatusSeeOther {
		t.Fatalf("register %s: status = %d, want 303 (body %s)", name, status, body)
	}
	return shellCookieValue(t, h)
}

func wantShellBody(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(body, want) {
			t.Errorf("body misses %q", want)
		}
	}
}

func TestShellLoginCreateRoundTripRendersRealStats(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)

	status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/login", "", url.Values{
		"username": {"alice"}, "password": {"hunter2"},
	})
	if status != http.StatusSeeOther {
		t.Fatalf("login: status = %d, want 303 (body %s)", status, body)
	}
	if loc := h.Get("Location"); loc != "/" {
		t.Errorf("login redirect = %q, want /", loc)
	}
	token := shellCookieValue(t, h)
	setCookie := h.Values("Set-Cookie")[0]
	for _, want := range []string{
		"Path=/", "HttpOnly", "SameSite=Lax", "Max-Age=" + strconv.Itoa(int(sessionTTLSec)),
	} {
		if !strings.Contains(setCookie, want) {
			t.Errorf("login cookie misses %q: %s", want, setCookie)
		}
	}

	alice, err := s.store.UserByUsername("alice")
	if err != nil {
		t.Fatalf("fetch alice: %v", err)
	}
	bob := seedUser(t, s.store, "bob")
	sr := seedSeries(t, s.store, alice, bob)
	won := WonByFour
	if err := s.store.ApplyCompletion(context.Background(), Completion{Games: []Game{{
		SeriesID: sr.ID, IdxInSeries: 1, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: []byte{}, WonBy: &won,
	}}}); err != nil {
		t.Fatalf("seed completion: %v", err)
	}

	status, h, body = doShell(t, c, http.MethodGet, srv.URL+"/", token, nil)
	if status != http.StatusOK {
		t.Fatalf("home: status = %d, want 200 (body %s)", status, body)
	}
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Errorf("home Cache-Control = %q, want no-store", got)
	}
	if got := h.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("home Content-Type = %q, want text/html", got)
	}
	wantShellBody(t, body,
		">alice</span>", "rating 30", ">1W-0L-0D<", "level 0",
		`href="/history"`, `action="/logout"`, `action="/rooms"`,
		`id="rooms"`, `hx-get="/partials/rooms"`, `hx-trigger="every `+
			strconv.Itoa(config.PagePollMs)+`ms"`,
		"bot tournament",
		`href="/tourney">bot tournament<`,
	)
	if strings.Contains(body, `class="guestnote"`) {
		t.Error("logged-in home carries the guest login prompt")
	}

	status, h, _ = doShell(t, c, http.MethodGet, srv.URL+"/login", token, nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/" {
		t.Errorf("GET /login logged in = %d %q, want 303 /", status, h.Get("Location"))
	}
}

func TestShellLoginFailuresRenderInline(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)
	shellRegister(t, c, srv.URL, "alice", "hunter2")

	status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/login", "", url.Values{
		"username": {"alice"}, "password": {"wrong"},
	})
	if status != http.StatusUnauthorized {
		t.Fatalf("wrong password: status = %d, want 401 (body %s)", status, body)
	}
	wantShellBody(t, body, `class="error"`, "wrong password", `action="/login"`,
		`name="username"`, `name="password"`)
	for _, sc := range h.Values("Set-Cookie") {
		if strings.HasPrefix(sc, sessionCookieName+"=") {
			t.Errorf("failed login set a session cookie: %s", sc)
		}
	}

	status, _, body = doShell(t, c, http.MethodPost, srv.URL+"/login", "", url.Values{
		"username": {""}, "password": {"x"},
	})
	if status != http.StatusBadRequest {
		t.Fatalf("empty username: status = %d, want 400 (body %s)", status, body)
	}
	wantShellBody(t, body, `class="error"`, "invalid username")
}

func TestShellGuestHomeShowsGridWithoutStats(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	room, err := s.rm.Create(alice.ID, 1, config.SeriesBO5, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	ghostRoom, err := s.rm.Create(777, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create ghost room: %v", err)
	}
	release := make(chan struct{})
	s.rm.makeSearcher = func(config.Tier) searcher {
		return &gatedBot{seen: make(chan struct{}), release: release}
	}
	t.Cleanup(func() { close(release) })
	botRoom, err := s.rm.CreateBotVsBot(&config.TierEasy, "easy-a", &config.TierEasy, "easy-b", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create same-tier pairing: %v", err)
	}

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/", "", nil)
	if status != http.StatusOK {
		t.Fatalf("guest home: status = %d", status)
	}
	wantShellBody(t, body,
		`id="rooms"`, `href="/rooms/`+room.ID()+`"`,
		`href="/rooms/`+ghostRoom.ID()+`"`, "#777 vs open seat",
		"easy-a-"+botRoom.ID()+" vs easy-b-"+botRoom.ID(),
		"2&#43;1", "bo5", "created", "alice vs open seat",
		`class="guestnote"`, `href="/login"`,
	)
	for _, banned := range []string{`class="me"`, `action="/rooms"`, `action="/logout"`, "W-0L-0D"} {
		if strings.Contains(body, banned) {
			t.Errorf("guest home contains %q", banned)
		}
	}

	status, h, _ := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/history", "", nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/login" {
		t.Errorf("guest /history = %d %q, want 303 /login", status, h.Get("Location"))
	}

	status, h, _ = doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/rooms", "", nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/" {
		t.Errorf("GET /rooms = %d %q, want 303 /", status, h.Get("Location"))
	}
}

func TestShellCreateRoomAcceptsEveryConfigCombination(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)
	token := shellRegister(t, c, srv.URL, "alice", "hunter2")

	created := 0
	for tcIdx := range config.TimeControls {
		for _, bo := range config.SeriesLengths {
			status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/rooms", token, url.Values{
				"tc": {strconv.Itoa(tcIdx)}, "bo": {strconv.Itoa(bo)}, "bot": {""},
			})
			if status != http.StatusSeeOther {
				t.Fatalf("create tc=%d bo=%d: status = %d, want 303 (body %s)", tcIdx, bo, status, body)
			}
			if loc := h.Get("Location"); !strings.HasPrefix(loc, "/rooms/") {
				t.Errorf("create tc=%d bo=%d redirects to %q, want a /rooms/ path", tcIdx, bo, loc)
			}
			created++
		}
	}
	var botIDs [len(config.Tiers)]string
	for i := range config.Tiers {
		status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/rooms", token, url.Values{
			"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)}, "bot": {config.Tiers[i].Name},
		})
		if status != http.StatusSeeOther {
			t.Fatalf("create bot %s: status = %d, want 303 (body %s)", config.Tiers[i].Name, status, body)
		}
		botIDs[i] = strings.TrimPrefix(h.Get("Location"), "/rooms/")
		status, _, gridBody := doShell(t, c, http.MethodGet, srv.URL+"/", token, nil)
		if status != http.StatusOK ||
			!strings.Contains(gridBody, "alice vs "+config.Tiers[i].Name+"-"+botIDs[i]) {
			t.Fatalf("grid after bot %s: status = %d, want the room card (body %s)",
				config.Tiers[i].Name, status, gridBody)
		}
		room, err := s.rm.Get(botIDs[i])
		if err != nil {
			t.Fatalf("get the %s room: %v", config.Tiers[i].Name, err)
		}
		room.Close()
	}

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/", token, nil)
	if status != http.StatusOK {
		t.Fatalf("home: status = %d", status)
	}
	wantShellBody(t, body,
		"1&#43;0", "2&#43;1", "3&#43;2", "bo3", "bo5", "bo7", "bo11",
		"human, open seat", "AI easy", "AI medium", "AI hard")
	if got := strings.Count(body, `class="room"`); got != created {
		t.Errorf("grid shows %d rooms, want %d", got, created)
	}
}

func TestShellCreateRoomRejectsGarbageSettings(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)

	status, h, _ := doShell(t, c, http.MethodPost, srv.URL+"/rooms", "", url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
	})
	if status != http.StatusSeeOther || h.Get("Location") != "/login" {
		t.Errorf("guest create = %d %q, want 303 /login", status, h.Get("Location"))
	}

	token := shellRegister(t, c, srv.URL, "alice", "hunter2")
	for name, form := range map[string]url.Values{
		"non-numeric tc":  {"tc": {"fast"}, "bo": {"3"}},
		"tc out of range": {"tc": {"99"}, "bo": {"3"}},
		"bo out of range": {"tc": {"0"}, "bo": {"2"}},
		"unknown tier":    {"tc": {"0"}, "bo": {"3"}, "bot": {"ultra"}},
	} {
		status, _, body := doShell(t, c, http.MethodPost, srv.URL+"/rooms", token, form)
		if status != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", name, status, body)
			continue
		}
		if !strings.Contains(body, "invalid room settings") {
			t.Errorf("%s: body misses the inline error (body %s)", name, body)
		}
	}

	status, body := doRawPost(t, c, srv.URL+"/rooms", token,
		"application/x-www-form-urlencoded", "%zz=1")
	if status != http.StatusBadRequest || !strings.Contains(body, "invalid room settings") {
		t.Errorf("undecodable create body = %d %s, want 400 with the inline error", status, body)
	}
	status, body = doRawPost(t, c, srv.URL+"/login", "",
		"application/x-www-form-urlencoded", "%zz=1")
	if status != http.StatusBadRequest || !strings.Contains(body, "malformed form body") {
		t.Errorf("undecodable login body = %d %s, want 400 with the inline error", status, body)
	}
}

func TestShellHistoryPreviewBoundaryAndPlaybackLink(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	sr := seedSeries(t, s.store, alice, bob)

	shortMoves := seqMoves(2 * (config.HistoryPreviewTurns - 2))
	longMoves := seqMoves(2 * (config.HistoryPreviewTurns + 12))
	won := WonByOpenFour
	short, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 1, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: EncodeMoves(nil, shortMoves),
		FullTurns: len(shortMoves) / 2, WonBy: &won,
	})
	if err != nil {
		t.Fatalf("append short game: %v", err)
	}
	long, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 2, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: EncodeMoves(nil, longMoves),
		FullTurns: len(longMoves) / 2,
	})
	if err != nil {
		t.Fatalf("append long game: %v", err)
	}
	if _, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 3, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: []byte{0x01}, FullTurns: 0,
	}); err != nil {
		t.Fatalf("append corrupt game: %v", err)
	}

	token := mintSession(t, s.store, alice)
	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/history", token, nil)
	if status != http.StatusOK {
		t.Fatalf("history: status = %d", status)
	}

	preview := func(moves []rules.Move) string {
		names := make([]string, len(moves))
		for i, m := range moves {
			names[i] = cellName(rules.Cell(m))
		}
		return strings.Join(names, " ")
	}
	limit := 2 * config.HistoryPreviewTurns
	wantShellBody(t, body,
		time.Unix(short.PlayedAt, 0).UTC().Format(historyTimeFormat),
		"alice vs bob", "1-0", "2-0",
		strconv.Itoa(short.FullTurns)+" turns", strconv.Itoa(long.FullTurns)+" turns",
		"won by "+won,
		`href="/rooms/history/`+strconv.FormatInt(short.ID, 10)+`"`,
		`href="/rooms/history/`+strconv.FormatInt(long.ID, 10)+`"`,
		">"+preview(shortMoves)+"<",
		">"+preview(longMoves[:limit])+" …<",
	)
	if strings.Count(body, "…") != 1 {
		t.Errorf("ellipsis appears %d times, want 1 (only the truncated row)", strings.Count(body, "…"))
	}
	if over := cellName(rules.Cell(longMoves[limit])); strings.Contains(body, over) {
		t.Errorf("history shows stone %s past the preview boundary", over)
	}
	if strings.Contains(body, "ZgotmplZ") {
		t.Error("history body contains ZgotmplZ, an html/template escape marker")
	}
}

func TestShellRoomsPartialServesFragmentOnly(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	room, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	status, h, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/partials/rooms", "", nil)
	if status != http.StatusOK {
		t.Fatalf("partial: status = %d", status)
	}
	if got := h.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("partial Content-Type = %q", got)
	}
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Errorf("partial Cache-Control = %q, want no-store", got)
	}
	wantShellBody(t, body, `<ul class="rooms">`, `href="/rooms/`+room.ID()+`"`)
	for _, page := range []string{"<!doctype", "<html", "<header"} {
		if strings.Contains(body, page) {
			t.Errorf("rooms partial carries the whole page (%q)", page)
		}
	}

	empty := newStack(t)
	emptySrv := httptest.NewServer(NewShellPages(empty.store, empty.rm, nil))
	defer emptySrv.Close()
	_, _, body = doShell(t, emptySrv.Client(), http.MethodGet, emptySrv.URL+"/partials/rooms", "", nil)
	if !strings.Contains(body, "no live rooms") {
		t.Errorf("empty grid fragment = %q", body)
	}
}

func TestShellBaseAssetsAndStylesheet(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()

	status, _, body := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/login", "", nil)
	if status != http.StatusOK {
		t.Fatalf("login page: status = %d", status)
	}
	wantShellBody(t, body,
		`<script src="/static/htmx.min.js"></script>`,
		`<script src="/static/hx-sse.min.js"></script>`,
		`<link rel="stylesheet" href="/shell.css">`,
	)

	status, h, css := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/shell.css", "", nil)
	if status != http.StatusOK {
		t.Fatalf("shell.css: status = %d", status)
	}
	if got := h.Get("Content-Type"); got != "text/css; charset=utf-8" {
		t.Errorf("shell.css Content-Type = %q", got)
	}
	if got := h.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("shell.css Cache-Control = %q, want no-cache", got)
	}
	if !strings.Contains(css, ".me .name") || !strings.Contains(css, "rem") {
		t.Errorf("shell.css does not carry the shell rules: %q", css)
	}
}

func TestShellLogoutDropsSession(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)
	token := shellRegister(t, c, srv.URL, "alice", "hunter2")

	status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/logout", token, nil)
	if status != http.StatusSeeOther {
		t.Fatalf("logout: status = %d, want 303 (body %s)", status, body)
	}
	if loc := h.Get("Location"); loc != "/" {
		t.Errorf("logout redirect = %q, want /", loc)
	}
	sc := h.Values("Set-Cookie")
	if len(sc) == 0 {
		t.Fatal("logout set no session cookie")
	}
	for _, want := range []string{"Path=/", "Max-Age=0", "HttpOnly"} {
		if !strings.Contains(sc[0], want) {
			t.Errorf("logout cookie misses %q: %s", want, sc[0])
		}
	}

	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/", token, nil)
	if !strings.Contains(body, `class="guestnote"`) {
		t.Error("home after logout still shows the logged-in view")
	}

	status, _, _ = doShell(t, c, http.MethodPost, srv.URL+"/logout", "", nil)
	if status != http.StatusSeeOther {
		t.Errorf("cookieless logout = %d, want 303", status)
	}
}

func TestShellRendersHostileUsernameInert(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)
	token := shellRegister(t, c, srv.URL, `<b>&"x`, "hunter2")

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/", token, nil)
	if status != http.StatusOK {
		t.Fatalf("home: status = %d", status)
	}
	if !strings.Contains(body, `&lt;b&gt;&amp;&#34;x`) {
		t.Errorf("hostile username not rendered inert: %s", body)
	}
	if strings.Contains(body, "<b>") {
		t.Error("live markup injected through the username")
	}
	if strings.Contains(body, "ZgotmplZ") {
		t.Error("body contains ZgotmplZ, an html/template escape marker")
	}
}

func TestShellBrokenStoreAnswers500(t *testing.T) {
	dead := mustOpen(t, dbPath(t))
	if err := dead.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(dead, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)

	token := strings.Repeat("0", 2*config.SessionTokenBytes)
	for _, path := range []string{"/", "/login", "/history"} {
		status, _, _ := doShell(t, c, http.MethodGet, srv.URL+path, token, nil)
		if status != http.StatusInternalServerError {
			t.Errorf("GET %s over a broken store = %d, want 500", path, status)
		}
	}
	status, _, body := doShell(t, c, http.MethodPost, srv.URL+"/login", "", url.Values{
		"username": {"alice"}, "password": {"hunter2"},
	})
	if status != http.StatusInternalServerError || !strings.Contains(body, "login failed, try again") {
		t.Errorf("login over a broken store = %d %s, want 500 with the retry message", status, body)
	}
	status, _, _ = doShell(t, c, http.MethodPost, srv.URL+"/logout", token, nil)
	if status != http.StatusInternalServerError {
		t.Errorf("logout over a broken store = %d, want 500", status)
	}
}

func TestHomeShowsLiveTournamentBanner(t *testing.T) {
	s := newStack(t)
	fake := &fakeTourney{ongoing: &TourneyBanner{RunID: 9, Done: 4, Total: 30}}
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, fake))
	defer srv.Close()
	c := noRedirectClient(srv)

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/", "", nil)
	if status != http.StatusOK {
		t.Fatalf("guest home: status = %d (body %s)", status, body)
	}
	wantShellBody(t, body,
		"a bot tournament is live",
		`href="/tourney/run/9"`,
		"series 4/30 settled",
	)
	status, _, body = doShell(t, c, http.MethodGet, srv.URL+"/partials/rooms", "", nil)
	if status != http.StatusOK {
		t.Fatalf("rooms partial: status = %d", status)
	}
	wantShellBody(t, body, "a bot tournament is live", "series 4/30 settled")

	fake.ongoing = nil
	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/", "", nil)
	if strings.Contains(body, "tournament is live") {
		t.Errorf("idle home carries the banner (body %s)", body)
	}
	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/partials/rooms", "", nil)
	if strings.Contains(body, "tournament is live") {
		t.Errorf("idle fragment carries the banner (body %s)", body)
	}

	fake.ongoingErr = errors.New("boom")
	status, _, body = doShell(t, c, http.MethodGet, srv.URL+"/", "", nil)
	if status != http.StatusOK || strings.Contains(body, "tournament is live") {
		t.Errorf("failed banner read = %d with banner %t, want 200 without it", status,
			strings.Contains(body, "tournament is live"))
	}
}

func TestShellHomeGridCardShortIDAndStateScore(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	c := noRedirectClient(srv)
	alice, bob, r := newPvPRoom(t, s)

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/", "", nil)
	if status != http.StatusOK {
		t.Fatalf("home: status = %d (body %s)", status, body)
	}
	wantShellBody(t, body,
		`href="/rooms/`+r.ID()+`"`,
		`<span class="mono">`+r.ID()[:8]+`</span>`,
		`"chip">created<`,
	)
	if strings.Contains(body, `class="score"`) {
		t.Error("a created room's card carries a series score line")
	}

	readyBoth(t, r, alice, bob)
	status, _, body = doShell(t, c, http.MethodGet, srv.URL+"/", "", nil)
	if status != http.StatusOK {
		t.Fatalf("home after handshake: status = %d (body %s)", status, body)
	}
	if got := strings.Count(body, `class="score"`); got != 1 {
		t.Errorf("in-game card shows %d score lines, want 1 (body %s)", got, body)
	}
	if !strings.Contains(body, `<span class="score">0-0</span>`) {
		t.Error("in-game card misses the 0-0 series score")
	}
}

func TestShellHistoryStampMinuteBoundary(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	sr := seedSeries(t, s.store, alice, bob)
	won := WonByOpenFour
	g, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 1, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: EncodeMoves(nil, seqMoves(2)),
		FullTurns: 1, WonBy: &won,
	})
	if err != nil {
		t.Fatalf("append game: %v", err)
	}
	if _, err := s.store.db.Exec(`UPDATE games SET played_at = 600 WHERE id = ?`, g.ID); err != nil {
		t.Fatalf("stamp the minute-aligned played_at: %v", err)
	}

	token := mintSession(t, s.store, alice)
	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/history", token, nil)
	if status != http.StatusOK {
		t.Fatalf("history: status = %d (body %s)", status, body)
	}
	want := time.Unix(600, 0).UTC().Format(historyTimeFormat)
	if !strings.Contains(body, want) {
		t.Errorf("history stamp misses the minute-aligned %q (body %s)", want, body)
	}
	if prev := time.Unix(540, 0).UTC().Format(historyTimeFormat); strings.Contains(body, prev) {
		t.Errorf("history stamp renders the previous minute %q", prev)
	}
}

func seqMoves(n int) []rules.Move {
	moves := make([]rules.Move, n)
	for i := range moves {
		moves[i] = rules.Move(i)
	}
	return moves
}
