package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
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

// TestHTTPLoginSummaryFailureSetsNoCookie pins the response ordering of
// handleLogin: when the summary read fails after the login itself
// succeeded, the response must carry no session cookie. Dropping the
// rating-events table from inside the package is the injection seam: only
// the summary reads it, the login leg touches users and sessions alone.
func TestHTTPLoginSummaryFailureSetsNoCookie(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()

	// A real registration: the stored argon2 hash must verify for the
	// login leg to pass (seedUser rows cannot).
	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "",
		map[string]string{"username": "alice", "password": "hunter2"})
	wantStatus(t, got, http.StatusOK, nil)

	if _, err := s.store.db.Exec(`DROP TABLE rating_events`); err != nil {
		t.Fatalf("drop rating_events: %v", err)
	}

	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/login", "",
		map[string]string{"username": "alice", "password": "hunter2"})
	wantAPIError(t, got, http.StatusInternalServerError, "internal")
	for _, ck := range got.cookies {
		if ck.Name == sessionCookieName {
			t.Fatalf("cookie %+v set on the failed summary, want none", ck)
		}
	}
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

	// The grid lists both, guest-readable. Order between same-instant rooms
	// is the manager's tie-break on the random id, so assert membership;
	// the creation order itself is the landed domain test's concern.
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms", "", nil)
	wantStatus(t, got, http.StatusOK, &rooms)
	ids := map[string]bool{}
	for _, rs := range rooms {
		ids[rs.ID] = true
	}
	if len(rooms) != 2 || !ids[created.ID] || !ids[botRoom.ID] {
		t.Fatalf("grid = %+v, want exactly %s and %s", rooms, created.ID, botRoom.ID)
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

// playHTTPScript alternates move posts, names[0] by the mover the redFirst
// flag names, then alternating: every post must answer 204.
func playHTTPScript(t *testing.T, srv *httptest.Server, roomID, redTok, blueTok string, names []string, redFirst bool) {
	t.Helper()
	tok := blueTok
	if redFirst {
		tok = redTok
	}
	for i, name := range names {
		got := doJSON(t, srv.Client(), http.MethodPost,
			srv.URL+"/api/rooms/"+roomID+"/move", tok, map[string]string{"cell": name})
		if got.status != http.StatusNoContent {
			t.Fatalf("move %d %s: status %d body %s, want 204", i+1, name, got.status, got.body)
		}
		if tok == redTok {
			tok = blueTok
		} else {
			tok = redTok
		}
	}
}

func TestHTTPRoomActionsPlaySeries(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	carol := seedUser(t, s.store, "carol")
	ta, tb, tcTok := mintSession(t, s.store, alice), mintSession(t, s.store, bob), mintSession(t, s.store, carol)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)
	base := srv.URL + "/api/rooms/" + room.ID

	// Acting on a room needs a session.
	got = doJSON(t, c, http.MethodPost, base+"/join", "", nil)
	wantAPIError(t, got, http.StatusUnauthorized, "unauthorized")

	// The host cannot join its own seat.
	got = doJSON(t, c, http.MethodPost, base+"/join", ta, nil)
	wantAPIError(t, got, http.StatusBadRequest, "bad_request")

	// Bob joins, the third player bounces off the taken seat.
	got = doJSON(t, c, http.MethodPost, base+"/join", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/join", tcTok, nil)
	wantAPIError(t, got, http.StatusConflict, "room_full")

	// Handshake surface: stranger refused, moves before both ready refused.
	got = doJSON(t, c, http.MethodPost, base+"/ready", tcTok, nil)
	wantAPIError(t, got, http.StatusForbidden, "not_participant")
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "D4"})
	wantAPIError(t, got, http.StatusConflict, "not_ready")

	// Ready handshake, idempotent on retry.
	got = doJSON(t, c, http.MethodPost, base+"/ready", ta, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", ta, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)

	// Game 1 is live with the host on red per the spec.
	got = doJSON(t, c, http.MethodGet, base, "", nil)
	var detail roomDetail
	wantStatus(t, got, http.StatusOK, &detail)
	if detail.Game == nil || detail.Game.Turn != "red" || detail.Game.TurnUserID != alice.ID ||
		detail.Game.RedUserID != alice.ID || len(detail.Game.Moves) != 0 {
		t.Fatalf("game 1 snapshot = %+v, want the host on red with an empty board", detail.Game)
	}
	max := int64(config.TimeControls[0].InitialSec * 1000)
	for i, ms := range detail.Game.ClockMs {
		if ms <= 0 || ms > max {
			t.Errorf("game 1 clock[%d] = %dms, want inside (0, %d]", i, ms, max)
		}
	}

	// A guest cannot act on the live room.
	got = doJSON(t, c, http.MethodPost, base+"/move", "", map[string]string{"cell": "D4"})
	wantAPIError(t, got, http.StatusUnauthorized, "unauthorized")

	// Turn and rules rejections over the wire.
	got = doJSON(t, c, http.MethodPost, base+"/move", tb, map[string]string{"cell": "D4"})
	wantAPIError(t, got, http.StatusConflict, "not_your_turn")
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "D4"})
	wantStatus(t, got, http.StatusNoContent, nil)
	// After red's opening stone the turn reads blue with bob to move.
	got = doJSON(t, c, http.MethodGet, base, "", nil)
	wantStatus(t, got, http.StatusOK, &detail)
	if detail.Game == nil || detail.Game.Turn != "blue" || detail.Game.TurnUserID != bob.ID {
		t.Fatalf("post-D4 snapshot = %+v, want bob on blue to move", detail.Game)
	}
	got = doJSON(t, c, http.MethodPost, base+"/move", tb, map[string]string{"cell": "D4"})
	wantAPIError(t, got, http.StatusConflict, "illegal_move")
	got = doJSON(t, c, http.MethodPost, base+"/move", tb, map[string]string{"cell": "Z9"})
	wantAPIError(t, got, http.StatusBadRequest, "bad_request")
	got = doJSON(t, c, http.MethodPost, base+"/move", tb, json.RawMessage(`{"cell": 7}`))
	wantAPIError(t, got, http.StatusBadRequest, "bad_request")
	got = doJSON(t, c, http.MethodPost, base+"/move", tb, map[string]string{"cell": "P16"})
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "D5"})
	wantAPIError(t, got, http.StatusConflict, "illegal_move")

	// Unknown rooms answer the 404 envelope on every action route.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms/deadbeef/join", ta, nil)
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms/deadbeef/ready", ta, nil)
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")

	// Mid-game board over the wire: after blue's stone the turn is red's.
	got = doJSON(t, c, http.MethodGet, base, "", nil)
	wantStatus(t, got, http.StatusOK, &detail)
	if detail.Game == nil || detail.Game.Turn != "red" || detail.Game.TurnUserID != alice.ID {
		t.Fatalf("mid-game snapshot = %+v, want alice on red to move", detail.Game)
	}
	if want := []string{"D4", "P16"}; len(detail.Game.Moves) != 2 ||
		detail.Game.Moves[0] != want[0] || detail.Game.Moves[1] != want[1] {
		t.Fatalf("mid-game moves = %v, want %v", detail.Game.Moves, want)
	}

	// Game 1 lands on the scripted host win (opening legality holds from H8).
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "H8"})
	wantStatus(t, got, http.StatusNoContent, nil)
	playHTTPScript(t, srv, room.ID, ta, tb, hostWinsRed[3:], false)

	// Game 2 is live with the loser rotation: bob holds red, alice swept the
	// first point.
	got = doJSON(t, c, http.MethodGet, base, "", nil)
	wantStatus(t, got, http.StatusOK, &detail)
	if detail.State != "in-game" || detail.HostWins != 1 || detail.GuestWins != 0 {
		t.Fatalf("mid-series line = %s %d-%d, want in-game 1-0", detail.State, detail.HostWins, detail.GuestWins)
	}
	if detail.Game == nil || detail.Game.RedUserID != bob.ID || detail.Game.TurnUserID != bob.ID ||
		len(detail.Game.Moves) != 0 {
		t.Fatalf("game 2 snapshot = %+v, want bob on red fresh", detail.Game)
	}

	// Game 2 lands on the scripted blue win: the host sweeps, the room
	// retires, and every further access answers 404.
	playHTTPScript(t, srv, room.ID, ta, tb, guestRedLosesToBlue, false)
	got = doJSON(t, c, http.MethodGet, base, "", nil)
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "D4"})
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms", "", nil)
	var rooms []roomSummary
	wantStatus(t, got, http.StatusOK, &rooms)
	if len(rooms) != 0 {
		t.Fatalf("grid after series = %+v, want empty", rooms)
	}

	// The played series lands on the profile and the history tab: alice
	// swept 2-0, the rating chain priced both games at 0 vs 0 then 30 vs
	// -30, and the history lists the two games newest first.
	_, _, _, afterOne := RatingDeltas(0, 0, RedWins)
	_, _, _, afterTwo := RatingDeltas(afterOne, -afterOne, BlueWins)
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/me", ta, nil)
	var me userSummary
	wantStatus(t, got, http.StatusOK, &me)
	if me.Username != "alice" || me.Rating != afterTwo || me.Wins != 2 ||
		me.Losses != 0 || me.Draws != 0 || me.Level != 1 {
		t.Errorf("post-series me = %+v, want alice at rating %d with 2-0-0 and level 1", me, afterTwo)
	}
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/history", ta, nil)
	var rows []historyEntry
	wantStatus(t, got, http.StatusOK, &rows)
	if len(rows) != 2 {
		t.Fatalf("history rows = %d, want the two played games", len(rows))
	}
	if rows[0].FullTurns != len(guestRedLosesToBlue)/2 || rows[1].FullTurns != len(hostWinsRed)/2 {
		t.Errorf("history turns = %d and %d, want %d and %d",
			rows[0].FullTurns, rows[1].FullTurns, len(guestRedLosesToBlue)/2, len(hostWinsRed)/2)
	}
	if rows[0].Truncated || rows[1].Truncated || rows[0].WonBy == nil || rows[1].WonBy == nil {
		t.Errorf("history rows truncated %t/%t wonBy %v/%v, want full previews with win tags",
			rows[0].Truncated, rows[1].Truncated, rows[0].WonBy, rows[1].WonBy)
	}
}

func TestHTTPForfeitRetiresRoom(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	carol := seedUser(t, s.store, "carol")
	ta, tb, tcTok := mintSession(t, s.store, alice), mintSession(t, s.store, bob), mintSession(t, s.store, carol)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)
	base := srv.URL + "/api/rooms/" + room.ID

	// Abandoning an open room costs nothing: the host forfeit retires it.
	got = doJSON(t, c, http.MethodPost, base+"/forfeit", ta, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodGet, base, "", nil)
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")

	// A live series: the stranger is refused, the guest forfeit sweeps and
	// retires.
	got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	wantStatus(t, got, http.StatusCreated, &room)
	base = srv.URL + "/api/rooms/" + room.ID
	got = doJSON(t, c, http.MethodPost, base+"/join", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", ta, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	// One stone in: the quit is mid-game. (A forfeit before any stone now
	// persists a zero-length blob too; the room-level regression is pinned
	// in TestForfeitBeforeFirstStonePersistsZeroLengthBlob.)
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "D4"})
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/forfeit", tcTok, nil)
	wantAPIError(t, got, http.StatusForbidden, "not_participant")
	got = doJSON(t, c, http.MethodPost, base+"/forfeit", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/forfeit", tb, nil)
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")
}

// sseFrame is one parsed SSE frame off the wire.
type sseFrame struct {
	event string
	data  string
}

// sseReader reads frames, skipping keepalive comment frames and counting
// them so tests can prove the cadence.
type sseReader struct {
	br         *bufio.Reader
	keepalives int
}

// openSSE opens one room's event stream on a context bounded by timeout;
// the body and the context close through the test cleanup, and the cancel
// func is returned for explicit client-disconnect tests.
func openSSE(t *testing.T, c *http.Client, url string, timeout time.Duration) (*sseReader, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		cancel()
		t.Fatalf("new sse request: %v", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		cancel()
		t.Fatalf("open sse %s: %v", url, err)
	}
	t.Cleanup(func() {
		cancel()
		_ = resp.Body.Close()
	})
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("sse content type = %q, want text/event-stream", ct)
	}
	return &sseReader{br: bufio.NewReader(resp.Body)}, cancel
}

// next reads the next event frame. ok is false on the stream's clean end;
// a frame cut in half by the end fails the test.
func (sr *sseReader) next(t *testing.T) (sseFrame, bool) {
	t.Helper()
	var f sseFrame
	for {
		line, err := sr.br.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				if f.event != "" || f.data != "" {
					t.Fatalf("stream ended inside a frame: %+v", f)
				}
				return f, false
			}
			t.Fatalf("read sse line: %v", err)
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		switch {
		case line == "":
			if f.event != "" || f.data != "" {
				return f, true
			}
		case strings.HasPrefix(line, ":"):
			sr.keepalives++
		case strings.HasPrefix(line, "event: "):
			f.event = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			f.data += strings.TrimPrefix(line, "data: ")
		default:
			t.Fatalf("stray sse line %q", line)
		}
	}
}

func TestHTTPSSEStreamsSeriesToCleanClose(t *testing.T) {
	s := newStack(t)
	api := &apiServer{store: s.store, rooms: s.rm, keepalive: 20 * time.Millisecond}
	srv := httptest.NewServer(api.routes())
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	ta, tb := mintSession(t, s.store, alice), mintSession(t, s.store, bob)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)
	base := srv.URL + "/api/rooms/" + room.ID

	// The guest opens the stream before the handshake, no session involved.
	sr, _ := openSSE(t, c, base+"/events", 15*time.Second)
	got = doJSON(t, c, http.MethodPost, base+"/join", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", ta, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)

	playHTTPScript(t, srv, room.ID, ta, tb, hostWinsRed, true)
	time.Sleep(60 * time.Millisecond) // keepalive frames must flow while idle
	playHTTPScript(t, srv, room.ID, ta, tb, guestRedLosesToBlue, false)

	// Read to the clean close: every stone in order, both game ends, the
	// series frame, then EOF; keepalives interleaved as comments.
	var moves []string
	var ends []string
	series := ""
	for {
		f, ok := sr.next(t)
		if !ok {
			break
		}
		switch f.event {
		case EventKindMove:
			moves = append(moves, f.data)
		case EventKindGameEnd:
			ends = append(ends, f.data)
		case EventKindSeries:
			series = f.data
		default:
			t.Fatalf("unexpected frame event %q data %q", f.event, f.data)
		}
	}
	want := append(append([]string{}, hostWinsRed...), guestRedLosesToBlue...)
	if !slices.Equal(moves, want) {
		t.Errorf("move frames = %v, want %v", moves, want)
	}
	if !slices.Equal(ends, []string{"red", "blue"}) {
		t.Errorf("game end frames = %v, want red then blue", ends)
	}
	if series != "host" {
		t.Errorf("series frame = %q, want host", series)
	}
	if sr.keepalives == 0 {
		t.Error("no keepalive frame seen during the idle window")
	}
}

func TestHTTPSSECleanCloseWhenHubEndsSubscription(t *testing.T) {
	s := newStack(t)
	api := &apiServer{store: s.store, rooms: s.rm, keepalive: time.Hour}
	srv := httptest.NewServer(api.routes())
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	ta, tb := mintSession(t, s.store, alice), mintSession(t, s.store, bob)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)
	base := srv.URL + "/api/rooms/" + room.ID
	got = doJSON(t, c, http.MethodPost, base+"/join", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", ta, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)

	sr, _ := openSSE(t, c, base+"/events", 15*time.Second)
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "D4"})
	wantStatus(t, got, http.StatusNoContent, nil)
	f, ok := sr.next(t)
	if !ok || f.event != EventKindMove || f.data != "D4" {
		t.Fatalf("first frame = %+v ok %t, want the D4 move", f, ok)
	}

	// The hub closing every subscription ends the stream cleanly: the next
	// read sees the stream end, not an error.
	s.hub.Close()
	if _, ok = sr.next(t); ok {
		t.Fatal("frame after hub close, want the clean stream end")
	}
}

func TestHTTPSSESubscribeFailureMapsToInternal(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	ta := mintSession(t, s.store, alice)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)

	// A closed hub refuses new subscriptions: the stream open answers the
	// internal envelope instead of a wedged connection.
	s.hub.Close()
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms/"+room.ID+"/events", "", nil)
	wantAPIError(t, got, http.StatusInternalServerError, "internal")
}

func TestHTTPSSEClientDisconnectEndsStream(t *testing.T) {
	s := newStack(t)
	api := &apiServer{store: s.store, rooms: s.rm, keepalive: time.Hour}
	srv := httptest.NewServer(api.routes())
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	ta, tb := mintSession(t, s.store, alice), mintSession(t, s.store, bob)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)
	base := srv.URL + "/api/rooms/" + room.ID
	got = doJSON(t, c, http.MethodPost, base+"/join", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", ta, nil)
	wantStatus(t, got, http.StatusNoContent, nil)
	got = doJSON(t, c, http.MethodPost, base+"/ready", tb, nil)
	wantStatus(t, got, http.StatusNoContent, nil)

	// The spectator leaves mid-stream: the request context done arm must
	// release the handler, not park it on the keepalive ticker.
	sr, cancel := openSSE(t, c, base+"/events", 15*time.Second)
	got = doJSON(t, c, http.MethodPost, base+"/move", ta, map[string]string{"cell": "D4"})
	wantStatus(t, got, http.StatusNoContent, nil)
	f, ok := sr.next(t)
	if !ok || f.event != EventKindMove || f.data != "D4" {
		t.Fatalf("first frame = %+v ok %t, want the D4 move", f, ok)
	}
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestHTTPSSEUnknownRoom(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	got := doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/rooms/deadbeef/events", "", nil)
	wantAPIError(t, got, http.StatusNotFound, "room_not_found")
}

func TestHTTPSSEMidRetirementRoomRefused(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	ta := mintSession(t, s.store, alice)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)

	// Retire without dropping the manager entry: the exact state a stream
	// open races. The stream must refuse instead of parking on keepalives.
	r, err := s.rm.Get(room.ID)
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	r.Close()
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms/"+room.ID+"/events", "", nil)
	wantAPIError(t, got, http.StatusConflict, "room_closed")
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms/"+room.ID, "", nil)
	wantAPIError(t, got, http.StatusConflict, "room_closed")
}

// TestHTTPSSEStreamEndsWhenRoomRetiresMidStream pins the subscribe-before-
// liveness ordering: a room retired after the stream is established (here
// through Room.Close, which publishes no event, the exact state an
// eventless retirement leaves) must end the stream, not park it on
// keepalive frames forever.
func TestHTTPSSEStreamEndsWhenRoomRetiresMidStream(t *testing.T) {
	s := newStack(t)
	api := &apiServer{store: s.store, rooms: s.rm, keepalive: 20 * time.Millisecond}
	srv := httptest.NewServer(api.routes())
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	ta := mintSession(t, s.store, alice)

	got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", ta,
		map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
	var room roomSummary
	wantStatus(t, got, http.StatusCreated, &room)

	// Headers back means the handler is inside the stream loop already.
	sr, _ := openSSE(t, c, srv.URL+"/api/rooms/"+room.ID+"/events", 5*time.Second)

	// Retire the room out from under the live stream without any hub event.
	r, err := s.rm.Get(room.ID)
	if err != nil {
		t.Fatalf("get room: %v", err)
	}
	r.Close()

	// The stream must end on its own (plain EOF) well inside the request
	// deadline; a handler parked on keepalives only ends when the context
	// cuts it, which the next read reports as an error instead.
	f, ok := sr.next(t)
	if ok {
		t.Fatalf("frame %+v after the retirement, want the plain stream end", f)
	}
}

// historyMoves builds n distinct in-board cells for a seeded moves blob.
func historyMoves(t *testing.T, n int) []rules.Move {
	t.Helper()
	if n*5+3 > config.BoardCells {
		t.Fatalf("historyMoves: %d moves run off the board", n)
	}
	out := make([]rules.Move, 0, n)
	for i := range n {
		out = append(out, rules.Move(i*5+3))
	}
	return out
}

func TestHTTPHistoryPreviewAndPlaybackBlob(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")

	// Private surface: a guest gets the 401 envelope.
	got := doJSON(t, c, http.MethodGet, srv.URL+"/api/history", "", nil)
	wantAPIError(t, got, http.StatusUnauthorized, "unauthorized")

	// Two rows straight into the store: the transport only renders. The
	// long game runs past the preview window, the short one inside it.
	sr := seedSeries(t, s.store, alice, bob)
	longBlob := EncodeMoves(nil, historyMoves(t, 40))
	wonBy := WonByFour
	if _, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: longBlob, FullTurns: 20, WonBy: &wonBy,
	}); err != nil {
		t.Fatalf("append long game: %v", err)
	}
	shortBlob := EncodeMoves(nil, historyMoves(t, 6))
	if _, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 1, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeBlue, Moves: shortBlob, FullTurns: 3,
	}); err != nil {
		t.Fatalf("append short game: %v", err)
	}

	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/history", mintSession(t, s.store, alice), nil)
	var rows []historyEntry
	wantStatus(t, got, http.StatusOK, &rows)
	if len(rows) != 2 {
		t.Fatalf("history rows = %d, want 2 (body %s)", len(rows), got.body)
	}
	// Newest first: the second append lands on top.
	long, short := rows[1], rows[0]
	if len(short.Preview) != 6 || short.Truncated || !bytes.Equal(short.Moves, shortBlob) {
		t.Errorf("short row = preview %v truncated %t, want the full 6 names", short.Preview, short.Truncated)
	}
	if len(long.Preview) != 2*config.HistoryPreviewTurns || !long.Truncated {
		t.Errorf("long row preview = %d names truncated %t, want %d and the ellipsis flag",
			len(long.Preview), long.Truncated, 2*config.HistoryPreviewTurns)
	}
	for i, name := range long.Preview {
		if want := cellName(rules.Cell(historyMoves(t, 40)[i])); name != want {
			t.Errorf("long preview[%d] = %q, want %q", i, name, want)
			break
		}
	}
	if long.FullTurns != 20 || long.Red != "alice" || long.Blue != "bob" ||
		long.RedWins != 1 || long.BlueWins != 0 || long.WonBy == nil || *long.WonBy != WonByFour {
		t.Errorf("long row = %+v, want the alice red win line", long)
	}
	if !bytes.Equal(long.Moves, longBlob) {
		t.Error("long row moves blob altered, want the full blob for the playback board")
	}
	if short.WonBy != nil {
		t.Errorf("short row wonBy = %q, want nil", *short.WonBy)
	}

	// A row this server never writes (odd-length blob) is skipped, not
	// fatal: the two valid rows still answer the listing.
	if _, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 2, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeDraw, Moves: []byte{1, 2, 3}, FullTurns: 1,
	}); err != nil {
		t.Fatalf("append corrupt game: %v", err)
	}
	got = doJSON(t, c, http.MethodGet, srv.URL+"/api/history", mintSession(t, s.store, alice), nil)
	var after []historyEntry
	wantStatus(t, got, http.StatusOK, &after)
	if len(after) != len(rows) {
		t.Fatalf("history rows after the corrupt append = %d, want the %d valid ones",
			len(after), len(rows))
	}
}

// TestHTTPHistoryCorruptBlobSkippedNotListingFatal pins the isolation of
// one hostile history row: an undecodable moves blob (odd length, never
// written by this server) must not 500 the whole listing, the valid rows
// still answer.
func TestHTTPHistoryCorruptBlobSkippedNotListingFatal(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")

	sr := seedSeries(t, s.store, alice, bob)
	goodBlob := EncodeMoves(nil, historyMoves(t, 6))
	if _, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeRed, Moves: goodBlob, FullTurns: 3,
	}); err != nil {
		t.Fatalf("append good game: %v", err)
	}
	if _, err := s.store.AppendGame(Game{
		SeriesID: sr.ID, IdxInSeries: 1, RedUser: alice.ID, BlueUser: bob.ID,
		Outcome: OutcomeDraw, Moves: []byte{1, 2, 3}, FullTurns: 1,
	}); err != nil {
		t.Fatalf("append corrupt game: %v", err)
	}

	got := doJSON(t, c, http.MethodGet, srv.URL+"/api/history", mintSession(t, s.store, alice), nil)
	var rows []historyEntry
	wantStatus(t, got, http.StatusOK, &rows)
	if len(rows) != 1 || !bytes.Equal(rows[0].Moves, goodBlob) {
		t.Errorf("rows = %+v, want only the valid game with its blob untouched", rows)
	}
}

// driveSeriesHTTP plays one full bo3 over the wire and reports the first
// failure as an error, so it can run on its own goroutine.
func driveSeriesHTTP(srv *httptest.Server, roomID, hostTok, guestTok string) error {
	post := func(path, tok string, body map[string]string) error {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		req, err := http.NewRequest(http.MethodPost, srv.URL+path, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Cookie", sessionCookieName+"="+tok)
		resp, err := srv.Client().Do(req)
		if err != nil {
			return err
		}
		out, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusNoContent {
			return fmt.Errorf("%s %s: status %d body %s", http.MethodPost, path, resp.StatusCode, out)
		}
		return nil
	}
	for _, tok := range []string{hostTok, guestTok} {
		if err := post("/api/rooms/"+roomID+"/ready", tok, nil); err != nil {
			return err
		}
	}
	for _, script := range []struct {
		names    []string
		redFirst bool
	}{{hostWinsRed, true}, {guestRedLosesToBlue, false}} {
		tok := guestTok
		if script.redFirst {
			tok = hostTok
		}
		for i, name := range script.names {
			if err := post("/api/rooms/"+roomID+"/move", tok, map[string]string{"cell": name}); err != nil {
				return fmt.Errorf("move %d %s: %w", i+1, name, err)
			}
			if tok == hostTok {
				tok = guestTok
			} else {
				tok = hostTok
			}
		}
	}
	return nil
}

func TestHTTPParallelRoomsSmoke(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	c := srv.Client()

	pairs := [2][2]string{{"alice", "bob"}, {"carol", "dan"}}
	tokens := [2][2]string{}
	for i, pair := range pairs {
		for j, name := range pair {
			tokens[i][j] = mintSession(t, s.store, seedUser(t, s.store, name))
		}
	}
	rooms := [2]string{}
	for i := range pairs {
		got := doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms", tokens[i][0],
			map[string]any{"tcIdx": 0, "boLen": config.SeriesBO3})
		var rs roomSummary
		wantStatus(t, got, http.StatusCreated, &rs)
		rooms[i] = rs.ID
		got = doJSON(t, c, http.MethodPost, srv.URL+"/api/rooms/"+rs.ID+"/join", tokens[i][1], nil)
		wantStatus(t, got, http.StatusNoContent, nil)
	}

	// Two full series through the same server, store, and write queue at
	// once; the race detector rides every exchange.
	done := make(chan error, len(pairs))
	for i := range pairs {
		go func(i int) {
			done <- driveSeriesHTTP(srv, rooms[i], tokens[i][0], tokens[i][1])
		}(i)
	}
	for range pairs {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	got := doJSON(t, c, http.MethodGet, srv.URL+"/api/rooms", "", nil)
	var grid []roomSummary
	wantStatus(t, got, http.StatusOK, &grid)
	if len(grid) != 0 {
		t.Fatalf("grid after both series = %d rooms, want empty", len(grid))
	}
	for i, pair := range pairs {
		got = doJSON(t, c, http.MethodGet, srv.URL+"/api/history", tokens[i][0], nil)
		var rows []historyEntry
		wantStatus(t, got, http.StatusOK, &rows)
		if len(rows) != 2 {
			t.Fatalf("%s history rows = %d, want both swept games (body %s)", pair[0], len(rows), got.body)
		}
	}
}

func TestHTTPErrorResponseMapping(t *testing.T) {
	cases := []struct {
		err    error
		code   string
		status int
	}{
		{ErrBadCredentials, codeBadCredentials, http.StatusUnauthorized},
		{ErrInvalidUsername, codeInvalidUsername, http.StatusBadRequest},
		{ErrRoomNotFound, codeRoomNotFound, http.StatusNotFound},
		{ErrRoomFull, codeRoomFull, http.StatusConflict},
		{ErrRoomClosed, codeRoomClosed, http.StatusConflict},
		{ErrNotYourTurn, codeNotYourTurn, http.StatusConflict},
		{ErrIllegalMove, codeIllegalMove, http.StatusConflict},
		{ErrNotParticipant, codeNotParticipant, http.StatusForbidden},
		{ErrNotReady, codeNotReady, http.StatusConflict},
		{ErrSeriesFinished, codeSeriesFinished, http.StatusConflict},
		{ErrBadTimeControl, codeBadRequest, http.StatusBadRequest},
		{ErrBadSeriesLength, codeBadRequest, http.StatusBadRequest},
		{ErrSamePlayer, codeBadRequest, http.StatusBadRequest},
		{ErrBadOwner, codeBadRequest, http.StatusBadRequest},
		{errors.New("store blew up"), codeInternal, http.StatusInternalServerError},
	}
	for _, tc := range cases {
		code, status := errorResponse(tc.err)
		if code != tc.code || status != tc.status {
			t.Errorf("errorResponse(%v) = %q %d, want %q %d", tc.err, code, status, tc.code, tc.status)
		}
	}
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
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/me", nil},
		{http.MethodPost, "/api/login", map[string]string{"username": "alice", "password": "p"}},
		{http.MethodPost, "/api/logout", nil},
		{http.MethodGet, "/api/history", nil},
	} {
		got := doJSON(t, c, tc.method, srv.URL+tc.path, token, tc.body)
		wantAPIError(t, got, http.StatusInternalServerError, "internal")
	}
}
