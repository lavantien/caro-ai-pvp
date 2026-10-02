package server

// The M6b shell UI tests: the login/create form round trip against the
// real store, the guest home, the create-room form over every config hub
// combination, the match history lines with the preview boundary, and
// template hygiene against hostile usernames. Real argon2 derivations only
// happen where a test registers or verifies through the form; every other
// session is minted straight into the store.

import (
	"context"
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

// noRedirectClient returns a client over the test server's transport that
// hands back the redirect response itself, so the tests pin Location and
// status instead of following the bounce.
func noRedirectClient(srv *httptest.Server) *http.Client {
	return &http.Client{
		Transport: srv.Client().Transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// doShell runs one shell exchange: an optional session cookie and an
// optional urlencoded form body, status, headers, and the drained body
// back. A nil form is a plain GET.
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

// doRawPost posts one literal body under the content type, for payloads
// no url.Values can produce (a body that does not decode as a form). The
// optional token rides the session cookie so a guarded route still reaches
// its body parse.
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

// shellCookieValue extracts the session cookie value from the Set-Cookie
// headers; it fails the test when none is present.
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

// shellRegister drives the login form once for a fresh account and returns
// the session cookie value.
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

// wantShellBody asserts every needle lands in the rendered body.
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
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
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
	// Cookie semantics identical to /api/login: same name, path, lifetime,
	// and hardening flags.
	setCookie := h.Values("Set-Cookie")[0]
	for _, want := range []string{
		"Path=/", "HttpOnly", "SameSite=Lax", "Max-Age=" + strconv.Itoa(int(sessionTTLSec)),
	} {
		if !strings.Contains(setCookie, want) {
			t.Errorf("login cookie misses %q: %s", want, setCookie)
		}
	}

	// One real decisive game through the completion unit: the home line
	// must show the store's own numbers, not placeholders.
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
	// The rating law prices a 30-0 upset at 0-0 as a flat +30.
	wantShellBody(t, body,
		">alice</span>", "rating 30", ">1W-0L-0D<", "level 0",
		`href="/history"`, `action="/logout"`, `action="/rooms"`,
		`id="rooms"`, `hx-get="/partials/rooms"`, `hx-trigger="every `+
			strconv.FormatInt(roomsPollInterval.Milliseconds(), 10)+`ms"`,
		"bot tournament", "M7",
	)
	if strings.Contains(body, `class="guestnote"`) {
		t.Error("logged-in home carries the guest login prompt")
	}

	// Already logged in: the login form bounces home.
	status, h, _ = doShell(t, c, http.MethodGet, srv.URL+"/login", token, nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/" {
		t.Errorf("GET /login logged in = %d %q, want 303 /", status, h.Get("Location"))
	}
}

func TestShellLoginFailuresRenderInline(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
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
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	room, err := s.rm.Create(alice.ID, 1, config.SeriesBO5, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	// A host whose user row vanished (deleted account, store hiccup) must
	// not break the listing: the seat falls back to the numeric id.
	ghostRoom, err := s.rm.Create(777, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create ghost room: %v", err)
	}

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/", "", nil)
	if status != http.StatusOK {
		t.Fatalf("guest home: status = %d", status)
	}
	wantShellBody(t, body,
		`id="rooms"`, `href="/rooms/`+room.ID()+`"`,
		`href="/rooms/`+ghostRoom.ID()+`"`, "#777 vs open seat",
		// html/template renders "+" as &#43; in text nodes (UTF-7 era
		// hardening), so the clock notation needles use the served bytes.
		"2&#43;1", "bo5", "created", "alice vs open seat",
		`class="guestnote"`, `href="/login"`,
	)
	for _, banned := range []string{`class="me"`, `action="/rooms"`, `action="/logout"`, "W-0L-0D"} {
		if strings.Contains(body, banned) {
			t.Errorf("guest home contains %q", banned)
		}
	}

	// The history tab is session territory: guests bounce to the form.
	status, h, _ := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/history", "", nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/login" {
		t.Errorf("guest /history = %d %q, want 303 /login", status, h.Get("Location"))
	}

	// The rooms collection URL is the home grid, never a 405 dead end.
	status, h, _ = doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/rooms", "", nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/" {
		t.Errorf("GET /rooms = %d %q, want 303 /", status, h.Get("Location"))
	}
}

func TestShellCreateRoomAcceptsEveryConfigCombination(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
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
	for i := range config.Tiers {
		status, _, body := doShell(t, c, http.MethodPost, srv.URL+"/rooms", token, url.Values{
			"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)}, "bot": {config.Tiers[i].Name},
		})
		if status != http.StatusSeeOther {
			t.Fatalf("create bot %s: status = %d, want 303 (body %s)", config.Tiers[i].Name, status, body)
		}
		created++
	}

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/", token, nil)
	if status != http.StatusOK {
		t.Fatalf("home: status = %d", status)
	}
	// The select options are rendered from the config hub, so the whole
	// vocabulary must appear and nothing else ("+" arrives escaped, the
	// UTF-7 hardening of html/template text nodes).
	wantShellBody(t, body, append([]string{
		"1&#43;0", "2&#43;1", "3&#43;2", "bo3", "bo5", "bo7", "bo11",
		"human, open seat", "AI easy", "AI medium", "AI hard",
	}, roomGridLabels(config.Tiers)...)...)
	if got := strings.Count(body, `class="room"`); got != created {
		t.Errorf("grid shows %d rooms, want %d", got, created)
	}
}

// roomGridLabels names the tier rooms the way the grid renders them, so
// the options test follows the config table instead of restating it.
func roomGridLabels(tiers [len(config.Tiers)]config.Tier) []string {
	labels := make([]string, 0, len(tiers))
	for i := range tiers {
		labels = append(labels, "alice vs AI "+tiers[i].Name)
	}
	return labels
}

func TestShellCreateRoomRejectsGarbageSettings(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
	defer srv.Close()
	c := noRedirectClient(srv)

	// A guest bounces to the form before any settings parse.
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

	// A body that cannot decode as a form fails the same way, and the login
	// form answers its own undecodable body inline too.
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
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	sr := seedSeries(t, s.store, alice, bob)

	// Short game: exactly HistoryPreviewTurns-2 full turns, all shown. Long
	// game: past the boundary, first 2*HistoryPreviewTurns stones then the
	// ellipsis. Stones are any decodable cells; the history read never
	// judges legality.
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
	// A row with an undecodable moves blob (this server never writes one)
	// is skipped, not fatal: the good rows still render.
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
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
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

	// The empty grid is its own fragment shape.
	empty := newStack(t)
	emptySrv := httptest.NewServer(NewShellPages(empty.store, empty.rm))
	defer emptySrv.Close()
	_, _, body = doShell(t, emptySrv.Client(), http.MethodGet, emptySrv.URL+"/partials/rooms", "", nil)
	if !strings.Contains(body, "no live rooms") {
		t.Errorf("empty grid fragment = %q", body)
	}
}

// TestShellBaseAssetsAndStylesheet pins the shared base wiring: every page
// loads both vendored scripts (the room pages ride hx-sse off the same
// base) and the one shell-owned stylesheet, which the shell serves itself.
func TestShellBaseAssetsAndStylesheet(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
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
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
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
	if sc := h.Values("Set-Cookie"); len(sc) == 0 || !strings.Contains(sc[0], "Max-Age=0") {
		t.Errorf("logout cookie = %v, want an expired session cookie", sc)
	}

	// The token is gone server-side: the old cookie reads as a guest.
	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/", token, nil)
	if !strings.Contains(body, `class="guestnote"`) {
		t.Error("home after logout still shows the logged-in view")
	}

	// A logout with no cookie at all still bounces home, natural expiry
	// raced.
	status, _, _ = doShell(t, c, http.MethodPost, srv.URL+"/logout", "", nil)
	if status != http.StatusSeeOther {
		t.Errorf("cookieless logout = %d, want 303", status)
	}
}

func TestShellRendersHostileUsernameInert(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm))
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
	srv := httptest.NewServer(NewShellPages(dead, s.rm))
	defer srv.Close()
	c := noRedirectClient(srv)

	// A present but unresolvable cookie must surface the store failure on
	// every session-reading page, not a silent guest view.
	token := strings.Repeat("0", 2*config.SessionTokenBytes)
	for _, path := range []string{"/", "/login", "/history"} {
		status, _, _ := doShell(t, c, http.MethodGet, srv.URL+path, token, nil)
		if status != http.StatusInternalServerError {
			t.Errorf("GET %s over a broken store = %d, want 500", path, status)
		}
	}
	// Login and logout surface the same failure: the generic retry message
	// for the form, a plain 500 for the token drop.
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

// seqMoves yields n stones at distinct cells 0..n-1.
func seqMoves(n int) []rules.Move {
	moves := make([]rules.Move, n)
	for i := range moves {
		moves[i] = rules.Move(i)
	}
	return moves
}
