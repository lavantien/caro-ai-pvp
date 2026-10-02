package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
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
	// One stone in: the quit is mid-game. (A forfeit before any stone hits a
	// landed-domain defect: the first synthetic game binds a nil moves blob
	// into the NOT NULL column; reported to the lead with this milestone.)
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

// openSSE opens one room's event stream on a bounded context; the body is
// closed by the test cleanup.
func openSSE(t *testing.T, c *http.Client, url string) *sseReader {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	return &sseReader{br: bufio.NewReader(resp.Body)}
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
	sr := openSSE(t, c, base+"/events")
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

	sr := openSSE(t, c, base+"/events")
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

func TestHTTPSSEUnknownRoom(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	got := doJSON(t, srv.Client(), http.MethodGet, srv.URL+"/api/rooms/deadbeef/events", "", nil)
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
