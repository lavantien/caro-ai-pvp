package server

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// The HTTP transport tests drive the landed stack (temp-file store, write
// queue, hub, room manager) through an httptest.Server, so every exchange
// exercises the persistence and event paths production serves. Real argon2
// derivations only happen in the login test; every other session is minted
// straight into the store.

// apiResp is one finished exchange: status, drained body, set cookies.
type apiResp struct {
	status  int
	body    []byte
	cookies []*http.Cookie
}

// doJSON runs one request with an optional JSON body under an optional
// session cookie token, and returns the exchange with the body drained.
func doJSON(t *testing.T, c *http.Client, method, url, token string, in any) apiResp {
	t.Helper()
	var rd io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshal %s %s: %v", method, url, err)
		}
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		t.Fatalf("new request %s %s: %v", method, url, err)
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Cookie", sessionCookieName+"="+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read %s %s: %v", method, url, err)
	}
	return apiResp{status: resp.StatusCode, body: body, cookies: resp.Cookies()}
}

// wantStatus asserts the status and decodes the JSON body into want.
func wantStatus(t *testing.T, got apiResp, status int, want any) {
	t.Helper()
	if got.status != status {
		t.Fatalf("status = %d, want %d (body %s)", got.status, status, got.body)
	}
	if want != nil {
		if err := json.Unmarshal(got.body, want); err != nil {
			t.Fatalf("decode body %q: %v", got.body, err)
		}
	}
}

// wantAPIError asserts the stable error envelope: status plus code.
func wantAPIError(t *testing.T, got apiResp, status int, code string) {
	t.Helper()
	if got.status != status {
		t.Fatalf("status = %d, want %d (body %s)", got.status, status, got.body)
	}
	var e apiError
	if err := json.Unmarshal(got.body, &e); err != nil {
		t.Fatalf("decode error body %q: %v", got.body, err)
	}
	if e.Error != code {
		t.Fatalf("error code = %q, want %q (body %s)", e.Error, code, got.body)
	}
}

// mintSession seeds a session row directly for a seeded user, bypassing
// argon2: the suite's real-derivation budget stays inside the login test.
func mintSession(t *testing.T, st *Store, u User) string {
	t.Helper()
	sess := NewSession(u.ID, time.Now().Unix())
	if err := st.InsertSession(sess); err != nil {
		t.Fatalf("insert session for %s: %v", u.Username, err)
	}
	return hex.EncodeToString(sess.Token)
}

func TestHTTPLoginCreateAndSessionCookie(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()

	// Unknown username registers and logs in: the zero-stat summary line.
	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "",
		map[string]string{"username": "alice", "password": "hunter2"})
	var sum userSummary
	wantStatus(t, got, http.StatusOK, &sum)
	if sum != (userSummary{Username: "alice"}) {
		t.Errorf("fresh summary = %+v, want the zero-stat alice line", sum)
	}
	var cookie *http.Cookie
	for _, ck := range got.cookies {
		if ck.Name == sessionCookieName {
			cookie = ck
		}
	}
	if cookie == nil || cookie.Value == "" {
		t.Fatalf("login cookies = %+v, want a %s value", got.cookies, sessionCookieName)
	}
	if !cookie.HttpOnly || cookie.Path != "/" {
		t.Errorf("cookie = HttpOnly %t Path %q, want HttpOnly on /", cookie.HttpOnly, cookie.Path)
	}
	token := cookie.Value

	// Cookie reuse: /api/me reflects the same line.
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/me", token, nil)
	var me userSummary
	wantStatus(t, got, http.StatusOK, &me)
	if me != sum {
		t.Errorf("me = %+v, want the login summary %+v", me, sum)
	}

	// Re-login with the right password succeeds.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "",
		map[string]string{"username": "alice", "password": "hunter2"})
	wantStatus(t, got, http.StatusOK, nil)

	// Wrong password: the opaque 401.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "",
		map[string]string{"username": "alice", "password": "wrong"})
	wantAPIError(t, got, http.StatusUnauthorized, "bad_credentials")

	// Syntactically unusable usernames never touch the store.
	for _, name := range []string{"", strings.Repeat("x", config.UsernameMaxBytes+1), "bad\x00name"} {
		got = doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "",
			map[string]string{"username": name, "password": "p"})
		wantAPIError(t, got, http.StatusBadRequest, "invalid_username")
		if len(got.cookies) != 0 {
			t.Errorf("invalid username %q set cookies, want none", name)
		}
	}

	// Malformed body.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "", json.RawMessage(`{"username": 3}`))
	wantAPIError(t, got, http.StatusBadRequest, "bad_request")
}

func TestHTTPLogoutInvalidatesSession(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	token := mintSession(t, s.store, alice)

	got := doJSON(t, c, http.MethodGet, srv.URL+"/api/me", token, nil)
	wantStatus(t, got, http.StatusOK, nil)

	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/logout", token, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	var cleared *http.Cookie
	for _, ck := range got.cookies {
		if ck.Name == sessionCookieName {
			cleared = ck
		}
	}
	if cleared == nil || cleared.MaxAge >= 0 {
		t.Fatalf("logout cookie = %+v, want the expired %s cookie", got.cookies, sessionCookieName)
	}

	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/me", token, nil)
	wantAPIError(t, got, http.StatusUnauthorized, "unauthorized")

	// Missing, garbage, and logout-without-cookie stay on the same surface.
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/me", "", nil)
	wantAPIError(t, got, http.StatusUnauthorized, "unauthorized")
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/me", "not-hex", nil)
	wantAPIError(t, got, http.StatusUnauthorized, "unauthorized")
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/logout", "", nil)
	wantStatus(t, got, http.StatusNoContent, nil)
}

func TestHTTPRoomsGridAndDetail(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	ta := mintSession(t, s.store, alice)

	// The grid is public: a guest sees it empty.
	got := doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms", "", nil)
	var rooms []roomSummary
	wantStatus(t, got, http.StatusOK, &rooms)
	if len(rooms) != 0 {
		t.Fatalf("fresh grid = %+v, want empty", rooms)
	}

	// Acting on the grid needs a session.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", "",
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	wantAPIError(t, got, http.StatusUnauthorized, "unauthorized")

	// Settings and body shapes the transport rejects up front.
	for _, body := range []any{
		map[string]any{"tcIdx": len(config.TimeControls), "boLen": config.SeriesBO3},
		map[string]any{"tcIdx": -1, "boLen": config.SeriesBO3},
		map[string]any{"tcIdx": 0, "boLen": 4},
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3, "bot": "nosuch"},
		json.RawMessage(`{"tcIdx": "x"}`),
	} {
		got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta, body)
		wantAPIError(t, got, http.StatusBadRequest, "bad_request")
	}

	// Create a bo5 room under the second time control.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 1, "boLen": config.SeriesBO5})
	var created roomSummary
	wantStatus(t, got, http.StatusCreated, &created)
	if created.ID == "" || created.HostUserID != alice.ID || created.GuestUserID != 0 ||
		created.TCIdx != 1 || created.BOLen != config.SeriesBO5 || created.State != "created" ||
		created.VsBotTier != "" || created.HostWins != 0 || created.GuestWins != 0 {
		t.Errorf("created = %+v, want the fresh alice bo5 room", created)
	}

	// A bot room by config tier name.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3, "bot": config.TierEasy.Name})
	var botRoom roomSummary
	wantStatus(t, got, http.StatusCreated, &botRoom)
	if botRoom.VsBotTier != config.TierEasy.Name || botRoom.GuestUserID == 0 {
		t.Errorf("bot room = %+v, want the easy bot seated", botRoom)
	}

	// The grid lists both in creation order, guest-readable.
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms", "", nil)
	wantStatus(t, got, http.StatusOK, &rooms)
	if len(rooms) != 2 || rooms[0].ID != created.ID || rooms[1].ID != botRoom.ID {
		t.Fatalf("grid = %+v, want %s then %s in creation order", rooms, created.ID, botRoom.ID)
	}

	// The open pvp room carries no live game.
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms/"+created.ID, "", nil)
	var detail roomDetail
	wantStatus(t, got, http.StatusOK, &detail)
	if detail.ID != created.ID || detail.Game != nil {
		t.Errorf("open room detail = %+v, want the summary and no live game", detail)
	}

	// Unknown rooms answer the 404 envelope.
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms/deadbeef", "", nil)
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")
}

func TestHTTPStoreFailureMapsToInternal(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	token := mintSession(t, s.store, alice)

	if err := s.store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	got := doJSON(t, c, http.MethodGet, srv.URL+"/api/me", token, nil)
	wantAPIError(t, got, http.StatusInternalServerError, "internal")
}
