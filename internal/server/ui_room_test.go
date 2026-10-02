package server

// The M6b room page and playback board tests: a driven PvP room renders both
// players' stones server-side over the full config-sized grid, guests and
// strangers get the read-only page, the playback board replays a finished
// game's blob with its won-by tag, and the tap-to-preview selection machine
// and the template hygiene hold as plain functions and pins.

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// newPageServer mounts RoomPages alone on a fresh mux, the way the serve
// command composes the page routes over the JSON API.
func newPageServer(t *testing.T, s *stack) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	NewRoomPages(s.rm, s.store).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// getRoomPage fetches one page under an optional session token.
func getRoomPage(t *testing.T, srv *httptest.Server, path, token string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
	if err != nil {
		t.Fatalf("new request %s: %v", path, err)
	}
	if token != "" {
		req.Header.Set("Cookie", sessionCookieName+"="+token)
	}
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return resp.StatusCode, string(body)
}

// wantBoard asserts the full grid: exactly BoardSize x BoardSize cells, every
// rules coordinate name present exactly once as a cell id.
func wantBoard(t *testing.T, body string) {
	t.Helper()
	if got := strings.Count(body, `class="cell`); got != config.BoardCells {
		t.Errorf("cell count = %d, want %d", got, config.BoardCells)
	}
	for cell := range config.BoardCells {
		name, err := rules.CellName(rules.Cell(cell))
		if err != nil {
			t.Fatalf("cell name %d: %v", cell, err)
		}
		if n := strings.Count(body, `id="c-`+name+`"`); n != 1 {
			t.Errorf("cell %s appears %d times, want exactly 1", name, n)
		}
	}
}

// wantStone asserts one landed stone of a color sits inside its cell; the
// latest stone also carries the latest highlight class.
func wantStone(t *testing.T, body, name, color string) {
	t.Helper()
	i := strings.Index(body, `id="c-`+name+`"`)
	if i < 0 {
		t.Fatalf("cell %s missing", name)
	}
	if got := body[i:]; !strings.Contains(got[:min(len(got), 200)], `class="stone `+color) {
		t.Errorf("cell %s misses a %s stone nearby", name, color)
	}
}

// wantSSEWiring pins the spike's proven wiring on the room page: the hx-sse
// connection, the vendored scripts, and one literal hx-on handler per wire
// kind, so the handlers and the event vocabulary cannot drift apart.
func wantSSEWiring(t *testing.T, body, roomID string) {
	t.Helper()
	handlers := map[string]string{
		EventKindMove:    `hx-on:move="caroRoom.onMove(event.detail.data)"`,
		EventKindMLine:   `hx-on:mline="caroRoom.onMLine(event.detail.data)"`,
		EventKindGameEnd: `hx-on:gameend="caroRoom.onGameEnd(event.detail.data)"`,
		EventKindSeries:  `hx-on:series="caroRoom.onSeries(event.detail.data)"`,
	}
	for _, kind := range spikeEventKinds {
		want := handlers[kind]
		if want == "" {
			t.Fatalf("no handler pin for wire kind %q", kind)
		}
		if !strings.Contains(body, want) {
			t.Errorf("room body misses %q", want)
		}
	}
	for _, want := range []string{
		`hx-sse:connect="/api/rooms/` + roomID + `/events"`,
		`src="/static/htmx.min.js"`, `src="/static/hx-sse.min.js"`, `src="/static/room.js"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("room body misses %q", want)
		}
	}
}

func TestRoomPageRendersDrivenPvPGame(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed[:5])
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	wantBoard(t, body)
	// Five stones landed: three red (even plies) and two blue.
	wantStone(t, body, "D4", "red")
	wantStone(t, body, "H8", "red")
	wantStone(t, body, "D5", "red")
	wantStone(t, body, "P16", "blue")
	wantStone(t, body, "P12", "blue")
	if got := strings.Count(body, `class="stone `); got != 5 {
		t.Errorf("stone count = %d, want 5", got)
	}
	// Move history in play order, latest highlighted.
	for _, name := range hostWinsRed[:4] {
		if !strings.Contains(body, "<li>"+name+"</li>") {
			t.Errorf("move history misses %s", name)
		}
	}
	if !strings.Contains(body, `<li class="latest">D5</li>`) {
		t.Error("move history misses the latest highlight on D5")
	}
	// The host holds red in game 1 and blue moves after five stones.
	if !strings.Contains(body, `id="turn-line"`) || !strings.Contains(body, "blue to move") {
		t.Error("turn line misses the blue-to-move state")
	}
	for _, id := range []string{"clock-red", "clock-blue", "clock-who-red", "clock-who-blue",
		"score-line", "move-history", "bot-log", "room-status"} {
		if !strings.Contains(body, `id="`+id+`"`) {
			t.Errorf("room body misses #%s", id)
		}
	}
	if !strings.Contains(body, `data-participant="1"`) {
		t.Error("host view misses the participant flag")
	}
	wantSSEWiring(t, body, r.ID())
	if strings.Contains(body, "ZgotmplZ") {
		t.Error("room body contains a html/template escape marker")
	}
}

func TestRoomPageGuestAndStrangerSeeNoControls(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed[:3])
	carol := seedUser(t, s.store, "carol")

	for name, token := range map[string]string{
		"anonymous": "",
		"stranger":  mintSession(t, s.store, carol),
	} {
		status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), token)
		if status != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", name, status)
		}
		if !strings.Contains(body, `data-participant="0"`) {
			t.Errorf("%s: read-only view misses the non-participant flag", name)
		}
		for _, banned := range []string{`id="ready-btn"`, `id="forfeit-btn"`, `board live`} {
			if strings.Contains(body, banned) {
				t.Errorf("%s: guest body contains control %q", name, banned)
			}
		}
		wantBoard(t, body)
		wantSSEWiring(t, body, r.ID())
	}
}

func TestRoomPagePreMatchReadyHandshake(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)

	ta := mintSession(t, s.store, alice)
	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	for _, want := range []string{
		`id="ready-btn"`, `id="forfeit-btn"`, `id="ready-host"`, `id="ready-guest"`,
		"alice: not ready", "bob: not ready", "waiting for both to ready",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("pre-match body misses %q", want)
		}
	}
	if strings.Contains(body, `class="board live"`) {
		t.Error("pre-match board is interactive before the handshake completes")
	}
	// The full grid renders before any game: the spectator's first paint is
	// the board, never a blank square.
	wantBoard(t, body)

	// One side readied: the per-player state splits, the game stays pending.
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("host ready: %v", err)
	}
	_, body = getRoomPage(t, srv, "/rooms/"+r.ID(), mintSession(t, s.store, bob))
	if !strings.Contains(body, "alice: ready") || !strings.Contains(body, "bob: not ready") {
		t.Error("ready states did not split after the host handshake")
	}
}

func TestRoomPageBadOrRetiredRoomIs404(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)

	for _, bad := range []string{
		"/rooms/nope",
		"/rooms/" + strings.Repeat("z", 2*roomIDBytes),
		"/rooms/" + strings.Repeat("a", 2*roomIDBytes), // id shape, no such room
	} {
		status, body := getRoomPage(t, srv, bad, "")
		if status != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404 (body %s)", bad, status, body)
		}
	}

	// A finished series retires the room; the page must not render it.
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)
	playScript(t, r, bob.ID, alice.ID, guestRedLosesToBlue)
	if status, _ := getRoomPage(t, srv, "/rooms/"+r.ID(), ""); status != http.StatusNotFound {
		t.Errorf("retired room page status = %d, want 404", status)
	}
}

func TestPlaybackPageReplaysSeededFinishedGame(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)
	playScript(t, r, bob.ID, alice.ID, guestRedLosesToBlue)

	ids := gameIDs(t, s)
	if len(ids) != 2 {
		t.Fatalf("persisted games = %d, want 2", len(ids))
	}
	ta, tb := mintSession(t, s.store, alice), mintSession(t, s.store, bob)
	carol := seedUser(t, s.store, "carol")
	tc := mintSession(t, s.store, carol)

	status, body := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[0]), ta)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	wantBoard(t, body)
	if got := strings.Count(body, `class="stone `); got != len(hostWinsRed) {
		t.Errorf("stone count = %d, want %d", got, len(hostWinsRed))
	}
	for i, name := range hostWinsRed {
		wantStone(t, body, name, map[bool]string{true: "red", false: "blue"}[i%2 == 0])
	}
	if !strings.Contains(body, `id="won-by">won by open 4<`) {
		t.Error("playback body misses the won-by tag")
	}
	if !strings.Contains(body, fmt.Sprintf(`id="pb-counter">%d / %d<`, len(hostWinsRed), len(hostWinsRed))) {
		t.Error("playback body misses the move counter at the final step")
	}
	for _, id := range []string{"pb-first", "pb-prev", "pb-next", "pb-last", "pb-play", "playback-root"} {
		if !strings.Contains(body, `id="`+id+`"`) {
			t.Errorf("playback body misses #%s", id)
		}
	}
	if !strings.Contains(body, `src="/static/playback.js"`) {
		t.Error("playback body misses its script")
	}

	// Both participants may replay their own games; a stranger and an
	// anonymous visitor get the 404, the visibility /api/history grants.
	if status, _ := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[1]), tb); status != http.StatusOK {
		t.Errorf("participant playback status = %d, want 200", status)
	}
	for name, tok := range map[string]string{"stranger": tc, "anonymous": ""} {
		if status, _ := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[0]), tok); status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, status)
		}
	}

	// Garbage ids never reach the store.
	for _, bad := range []string{"/rooms/history/abc", "/rooms/history/0", "/rooms/history/9999"} {
		if status, _ := getRoomPage(t, srv, bad, ta); status != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", bad, status)
		}
	}
}

// gameIDs lists the persisted game ids in insert order straight off the
// store's connection, the read the playback route owns.
func gameIDs(t *testing.T, s *stack) []int64 {
	t.Helper()
	rows, err := s.store.db.Query(`SELECT id FROM games ORDER BY id`)
	if err != nil {
		t.Fatalf("list games: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan game id: %v", err)
		}
		out = append(out, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list games: %v", err)
	}
	return out
}

func TestTapSelectStateMachine(t *testing.T) {
	for _, tc := range []struct {
		selected    string
		tapped      string
		wantSel     string
		wantConfirm bool
	}{
		{"", "H8", "H8", false},
		{"H8", "H8", "", true},
		{"H8", "J9", "J9", false},
	} {
		sel, confirm := tapSelect(tc.selected, tc.tapped)
		if sel != tc.wantSel || confirm != tc.wantConfirm {
			t.Errorf("tapSelect(%q, %q) = (%q, %v), want (%q, %v)",
				tc.selected, tc.tapped, sel, confirm, tc.wantSel, tc.wantConfirm)
		}
	}
}

// TestRoomJSTapSelectTwinIsPinned pins the browser twin of tapSelect: the
// marked block in the served room.js must stay byte-identical to the
// reviewed canonical source, so the Go semantics and the shipped script
// cannot drift silently.
func TestRoomJSTapSelectTwinIsPinned(t *testing.T) {
	src, err := fs.ReadFile(staticFS, "room.js")
	if err != nil {
		t.Fatalf("read room.js: %v", err)
	}
	begin, end := tapSelectJSBegin+"\n", "\n"+tapSelectJSEnd
	i := strings.Index(string(src), begin)
	j := strings.Index(string(src), end)
	if i < 0 || j < 0 || j < i {
		t.Fatal("room.js misses the pinned tapSelect block markers")
	}
	if got := string(src)[i : j+len(end)]; got != tapSelectJS {
		t.Errorf("pinned tapSelect block drifted:\n got: %q\nwant: %q", got, tapSelectJS)
	}
}

func TestFormatClockMs(t *testing.T) {
	for _, tc := range []struct {
		ms   int64
		want string
	}{
		{0, "00:00.0"},
		{600_000, "10:00.0"},
		{65_430, "01:05.4"},
		{59_999, "00:59.9"},
		{-5, "00:00.0"},
	} {
		if got := formatClockMs(tc.ms); got != tc.want {
			t.Errorf("formatClockMs(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

func TestTCLabelPinnedToConfig(t *testing.T) {
	for i := range config.TimeControls {
		tc := config.TimeControls[i]
		want := fmt.Sprintf("%d+%d", tc.InitialSec, tc.IncrementSec)
		if got := tcLabel(i); got != want {
			t.Errorf("tcLabel(%d) = %q, want %q", i, got, want)
		}
	}
}

// TestRoomJSCarriesSeatRotationSync pins the page driver's seat-rotation
// sync: the clock labels and the mover's ghost color follow every new
// game's redUserId (the decisive loser takes red), which the load-time
// server render can only freeze. The literals tie room.js to the template
// ids the room page test already pins.
func TestRoomJSCarriesSeatRotationSync(t *testing.T) {
	src, err := fs.ReadFile(staticFS, "room.js")
	if err != nil {
		t.Fatalf("read room.js: %v", err)
	}
	for _, want := range []string{
		"otherSeat",
		"$('clock-who-red').textContent = 'red / ' + seatName(d.game.redUserId)",
		"$('clock-who-blue').textContent = 'blue / ' + seatName(otherSeat(d.game.redUserId))",
		"setAttribute('data-my-color'",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("room.js misses seat-rotation sync %q", want)
		}
	}
}

// TestRoomTemplatesEscapePoisonedPayloads pins the hygiene: every
// interpolation the room and playback pages carry renders HTML specials
// inert even when a view is poisoned past what the domain can produce.
func TestRoomTemplatesEscapePoisonedPayloads(t *testing.T) {
	poison := `<script>alert(1)</script>`
	cells := []cellView{{Name: poison, Stone: "red", Glyph: "O"}}

	var out strings.Builder
	rv := roomView{RoomID: strings.Repeat("a", 2*roomIDBytes), TCLabel: "1+0", BOLen: config.SeriesBO3,
		HostName: poison, GuestName: poison, ViewerName: poison, State: poison,
		Turn: poison, TurnName: poison, RedName: poison, BlueName: poison,
		Moves: []moveLine{{Name: poison}}, Cells: cells, KindsCSV: strings.Join(spikeEventKinds, ",")}
	if err := executeRoom(&out, rv); err != nil {
		t.Fatalf("render room: %v", err)
	}
	if strings.Contains(out.String(), "<script>alert") {
		t.Error("room output carries a raw script payload")
	}
	if !strings.Contains(out.String(), "&lt;script&gt;") {
		t.Error("room output lost the escaped payload entirely")
	}

	pv := playbackView{RedName: poison, BlueName: poison, Outcome: poison, WonBy: poison,
		Moves:  []string{poison},
		Cells:  []cellView{{Name: poison, Stone: "blue", Glyph: "X", Idx: 0}},
		GameID: 1}
	out.Reset()
	if err := executePlayback(&out, pv); err != nil {
		t.Fatalf("render playback: %v", err)
	}
	if strings.Contains(out.String(), "<script>alert") {
		t.Error("playback output carries a raw script payload")
	}
	if !strings.Contains(out.String(), "&lt;script&gt;") {
		t.Error("playback output lost the escaped payload entirely")
	}
}

// TestGameByIDReadsAndMisses covers the playback data path accessor: a
// seeded game round-trips and a missing id maps to ErrNotFound.
func TestGameByIDReadsAndMisses(t *testing.T) {
	s := newStack(t)
	red, blue := seedUser(t, s.store, "red"), seedUser(t, s.store, "blue")
	sr := seedSeries(t, s.store, red, blue)
	tag := "open 4"
	blob := encodeMoves(nil, movesOf(t, hostWinsRed))
	if _, err := s.store.AppendGame(Game{SeriesID: sr.ID, IdxInSeries: 0,
		RedUser: red.ID, BlueUser: blue.ID, Outcome: OutcomeRed,
		Moves: blob, FullTurns: len(hostWinsRed) / 2, WonBy: &tag}); err != nil {
		t.Fatalf("append game: %v", err)
	}
	g, err := s.store.gameByID(1)
	if err != nil {
		t.Fatalf("game by id: %v", err)
	}
	if g.SeriesID != sr.ID || g.Outcome != OutcomeRed || g.WonBy == nil || *g.WonBy != tag {
		t.Errorf("game row = %+v, want the seeded red win with its tag", g)
	}
	if _, err := s.store.gameByID(42); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing game err = %v, want ErrNotFound", err)
	}
}

// TestRoomPagesComposeWithJSONAPI mounts the pages beside the JSON mux the
// way the serve command does: page routes win on /rooms, everything else
// falls through to the API untouched.
func TestRoomPagesComposeWithJSONAPI(t *testing.T) {
	s := newStack(t)
	mux := http.NewServeMux()
	mux.Handle("/", NewHTTPAPI(s.store, s.rm))
	NewRoomPages(s.rm, s.store).Mount(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed[:1])
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK || !strings.Contains(body, `id="c-D4"`) {
		t.Fatalf("composed mux room page status = %d, want the rendered board", status)
	}
	if status, _, _ := doGet(t, srv.Client(), srv.URL+"/api/rooms/"+r.ID()); status != http.StatusOK {
		t.Errorf("composed mux api detail status = %d, want 200", status)
	}
}
