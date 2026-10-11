package server

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func newPageServer(t *testing.T, s *stack) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	NewRoomPages(s.rm, s.store).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

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
	wantStone(t, body, "D4", "red")
	wantStone(t, body, "H8", "red")
	wantStone(t, body, "D5", "red")
	wantStone(t, body, "P16", "blue")
	wantStone(t, body, "P12", "blue")
	if got := strings.Count(body, `class="stone `); got != 5 {
		t.Errorf("stone count = %d, want 5", got)
	}
	for _, name := range hostWinsRed[:4] {
		if !strings.Contains(body, "<li>"+name+"</li>") {
			t.Errorf("move history misses %s", name)
		}
	}
	if !strings.Contains(body, `<li class="latest">D5</li>`) {
		t.Error("move history misses the latest highlight on D5")
	}
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

func TestRoomPagePlayableMaskForMoverOnly(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"H8", "A1"})
	ta, tb := mintSession(t, s.store, alice), mintSession(t, s.store, bob)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK {
		t.Fatalf("mover page status = %d, want 200 (body %s)", status, body)
	}
	if got := strings.Count(body, `class="cell off"`); got != 24 {
		t.Errorf("mover render dims %d cells, want the 24-cell opening ring", got)
	}
	if !strings.Contains(body, `<div class="cell off" id="c-I8"`) {
		t.Error("near cell I8 misses the off mark on the mover's board")
	}
	if !strings.Contains(body, `<div class="cell" id="c-K11"`) {
		t.Error("far cell K11 is not a plain live cell on the mover's board")
	}

	for name, token := range map[string]string{
		"opponent": tb,
		"guest":    "",
	} {
		_, other := getRoomPage(t, srv, "/rooms/"+r.ID(), token)
		if strings.Contains(other, `class="cell off"`) {
			t.Errorf("%s view carries the playable mask", name)
		}
	}
}

func TestRoomPageDeadSessionCookieRendersGuest(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed[:3])

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(),
		"deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want the 200 guest render (body %s)", status, body)
	}
	if !strings.Contains(body, `data-participant="0"`) {
		t.Error("dead session misses the guest view")
	}

	status, _ = getRoomPage(t, srv, "/rooms/history/1", "deadbeefdeadbeef")
	if status != http.StatusNotFound {
		t.Errorf("playback under a dead session = %d, want 404", status)
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
	wantBoard(t, body)

	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("host ready: %v", err)
	}
	_, body = getRoomPage(t, srv, "/rooms/"+r.ID(), mintSession(t, s.store, bob))
	if !strings.Contains(body, "alice: ready") || !strings.Contains(body, "bob: not ready") {
		t.Error("ready states did not split after the host handshake")
	}
}

func TestRoomPageJoinButtonGating(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice := seedUser(t, s.store, "alice")
	carol := seedUser(t, s.store, "carol")
	tc := mintSession(t, s.store, carol)

	open, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	joinBtn := `id="join-btn"`
	hasJoin := func(token string) bool {
		status, body := getRoomPage(t, srv, "/rooms/"+open.ID(), token)
		if status != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body %s)", status, body)
		}
		return strings.Contains(body, joinBtn)
	}
	if !hasJoin(tc) {
		t.Error("stranger on an open human room misses the join button")
	}
	if hasJoin(mintSession(t, s.store, alice)) {
		t.Error("host sees a join button on their own room")
	}
	if hasJoin("") {
		t.Error("anonymous visitor sees a join button")
	}

	bob := seedUser(t, s.store, "bob")
	if err := s.rm.Join(open.ID(), bob.ID); err != nil {
		t.Fatalf("join: %v", err)
	}
	if hasJoin(tc) {
		t.Error("stranger sees a join button on a seated room")
	}

	hard := config.Tiers[len(config.Tiers)-1]
	bot, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &hard)
	if err != nil {
		t.Fatalf("create bot room: %v", err)
	}
	status, body := getRoomPage(t, srv, "/rooms/"+bot.ID(), tc)
	if status != http.StatusOK || strings.Contains(body, joinBtn) {
		t.Errorf("bot room page: status %d, join button present", status)
	}
}

func TestRoomPageBadOrRetiredRoomIs404(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)

	for _, bad := range []string{
		"/rooms/nope",
		"/rooms/" + strings.Repeat("z", 2*roomIDBytes),
		"/rooms/" + strings.Repeat("a", 2*roomIDBytes),
	} {
		status, body := getRoomPage(t, srv, bad, "")
		if status != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404 (body %s)", bad, status, body)
		}
	}

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

	if status, _ := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[1]), tb); status != http.StatusOK {
		t.Errorf("participant playback status = %d, want 200", status)
	}
	for name, tok := range map[string]string{"stranger": tc, "anonymous": ""} {
		if status, _ := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[0]), tok); status != http.StatusNotFound {
			t.Errorf("%s: status = %d, want 404", name, status)
		}
	}

	for _, bad := range []string{"/rooms/history/abc", "/rooms/history/0", "/rooms/history/9999"} {
		if status, _ := getRoomPage(t, srv, bad, ta); status != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", bad, status)
		}
	}
}

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
		{-1, "00:00.0"},
	} {
		if got := formatClockMs(tc.ms); got != tc.want {
			t.Errorf("formatClockMs(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}

func TestTCLabelPinnedToConfig(t *testing.T) {
	for i := range config.TimeControls {
		tc := config.TimeControls[i]
		want := fmt.Sprintf("%d+%d", tc.InitialMin, tc.IncrementSec)
		if got := tcLabel(i); got != want {
			t.Errorf("tcLabel(%d) = %q, want %q", i, got, want)
		}
	}
}

func TestRoomViewOfRetiredRoomIsNotFound(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	r.Close()
	p := NewRoomPages(s.rm, s.store)
	req := httptest.NewRequest(http.MethodGet, "/rooms/"+r.ID(), nil)
	if _, err := p.roomViewOf(req, r); !errors.Is(err, ErrRoomNotFound) {
		t.Errorf("retired room view err = %v, want ErrRoomNotFound", err)
	}
}

func TestPlaybackPageForfeitSweptGames(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed[:3])
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}

	ids := gameIDs(t, s)
	if len(ids) != 3 {
		t.Fatalf("persisted games = %d, want the bo3 sweep's 3", len(ids))
	}
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[0]), ta)
	if status != http.StatusOK {
		t.Fatalf("live-game playback status = %d, want 200", status)
	}
	wantBoard(t, body)
	if got := strings.Count(body, `class="stone `); got != 3 {
		t.Errorf("live game stone count = %d, want the 3 played", got)
	}
	if strings.Contains(body, `id="won-by"`) {
		t.Error("forfeit sweep game carries a won-by tag")
	}

	status, body = getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[1]), ta)
	if status != http.StatusOK {
		t.Fatalf("booked game playback status = %d, want 200", status)
	}
	wantBoard(t, body)
	if strings.Contains(body, `class="stone `) {
		t.Error("booked game renders stones from an empty blob")
	}
	if strings.Contains(body, `id="won-by"`) {
		t.Error("booked game carries a won-by tag")
	}
	for _, want := range []string{`>0 / 0<`, `id="pb-first"`, `id="pb-play"`, `src="/static/playback.js"`} {
		if !strings.Contains(body, want) {
			t.Errorf("booked game body misses %q", want)
		}
	}
}

func TestUIMLineHidesSpecHiddenFields(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{
			"M24, Red, J9, d=14, n=6.25m, nps=2.5m, ebf=2.1, tt=38%, hf=45%, fh1=93%, s=+150, thr=4, t=2.50, alloc=2.50, pv=J9 K10 K9 L9 M8 L8",
			"M24, Red, J9, d=14, n=6.25m, nps=2.5m, tt=38%, s=+150, thr=4, t=2.50, alloc=2.50, pv=J9 K10 K9 L9 M8 L8",
		},
		{
			"M31, Blue, G7, d=9, n=45k, nps=1.2m, ebf=1.4, tt=18%, hf=60%, fh1=88%, s=M9, thr=4, t=0.03, alloc=3.00, [VCT], pv=G7 H7 G8 G6 G9 G10 F8 E9 I8",
			"M31, Blue, G7, d=9, n=45k, nps=1.2m, tt=18%, s=M9, thr=4, t=0.03, alloc=3.00, [VCT], pv=G7 H7 G8 G6 G9 G10 F8 E9 I8",
		},
		{"M12, Red, H10, pv=H10", "M12, Red, H10, pv=H10"},
	} {
		if got := uiMLine(tc.in); got != tc.want {
			t.Errorf("uiMLine(%q)\n got: %q\nwant: %q", tc.in, got, tc.want)
		}
	}
}

func TestRoomJSUIMLineTwinIsPinned(t *testing.T) {
	src, err := fs.ReadFile(staticFS, "room.js")
	if err != nil {
		t.Fatalf("read room.js: %v", err)
	}
	begin, end := uiMLineJSBegin+"\n", "\n"+uiMLineJSEnd
	i := strings.Index(string(src), begin)
	j := strings.Index(string(src), end)
	if i < 0 || j < 0 || j < i {
		t.Fatal("room.js misses the pinned uiMLine block markers")
	}
	if got := string(src)[i : j+len(end)]; got != uiMLineJS {
		t.Errorf("pinned uiMLine block drifted:\n got: %q\nwant: %q", got, uiMLineJS)
	}
	if !strings.Contains(string(src), "r.status === 404 || r.status === 409") {
		t.Error("room.js detail poll no longer treats eviction's 404 as terminal")
	}
}

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

func TestRoomJSCarriesPlayableMask(t *testing.T) {
	src, err := fs.ReadFile(staticFS, "room.js")
	if err != nil {
		t.Fatalf("read room.js: %v", err)
	}
	for _, want := range []string{
		"applyPlayable(d.game.playable)",
		"classList.contains('off')",
		"not a legal space",
		"!cells[j].classList.contains('occ')",
	} {
		if !strings.Contains(string(src), want) {
			t.Errorf("room.js misses playable-mask wiring %q", want)
		}
	}
}

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

func TestGameByIDReadsAndMisses(t *testing.T) {
	s := newStack(t)
	red, blue := seedUser(t, s.store, "red"), seedUser(t, s.store, "blue")
	sr := seedSeries(t, s.store, red, blue)
	tag := "open 4"
	blob := EncodeMoves(nil, movesOf(t, hostWinsRed))
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

func TestRoomAndPlaybackBrokenStoreAnswers500(t *testing.T) {
	dead := mustOpen(t, dbPath(t))
	if err := dead.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	live, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	mux := http.NewServeMux()
	NewRoomPages(s.rm, dead).Mount(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	token := strings.Repeat("0", 2*config.SessionTokenBytes)
	for _, path := range []string{"/rooms/" + live.ID(), "/rooms/history/1"} {
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatalf("new request %s: %v", path, err)
		}
		req.Header.Set("Cookie", sessionCookieName+"="+token)
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("GET %s with a session over a broken store = %d, want 500", path, resp.StatusCode)
		}
	}
	status, _ := getRoomPage(t, srv, "/rooms/history/1", "")
	if status != http.StatusNotFound {
		t.Errorf("anonymous playback over a broken store = %d, want 404", status)
	}
}

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

func TestPlaybackGameIDDigitNineInBaseTen(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob := seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob")
	sr := seedSeries(t, s.store, alice, bob)
	blob := EncodeMoves(nil, seqMoves(2))
	var last int64
	for i := 1; i <= 19; i++ {
		g, err := s.store.AppendGame(Game{SeriesID: sr.ID, IdxInSeries: i,
			RedUser: alice.ID, BlueUser: bob.ID, Outcome: OutcomeRed,
			Moves: blob, FullTurns: 1})
		if err != nil {
			t.Fatalf("append game %d: %v", i, err)
		}
		last = g.ID
	}
	if last != 19 {
		t.Fatalf("last seeded game id = %d, want 19 off the fresh rowid sequence", last)
	}
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/history/19", ta)
	if status != http.StatusOK {
		t.Fatalf("playback of game 19: status = %d, want 200 (body %s)", status, body)
	}
	wantBoard(t, body)
	if !strings.Contains(body, `id="pb-counter">2 / 2<`) {
		t.Error("playback body misses the final-step move counter")
	}
}

func TestRoomPageJoinedCreatedRoomHidesReadyControlsFromGuest(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	_, bob, r := newPvPRoom(t, s)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), "")
	if status != http.StatusOK {
		t.Fatalf("guest page: status = %d, want 200 (body %s)", status, body)
	}
	if !strings.Contains(body, `data-participant="0"`) {
		t.Error("guest view misses the non-participant flag")
	}
	for _, banned := range []string{`id="controls"`, `id="ready-btn"`, `id="ready-host"`,
		`id="ready-guest"`, `id="forfeit-btn"`, `id="join-btn"`} {
		if strings.Contains(body, banned) {
			t.Errorf("guest body contains the participant control %q", banned)
		}
	}

	_, body = getRoomPage(t, srv, "/rooms/"+r.ID(), mintSession(t, s.store, bob))
	for _, want := range []string{`id="ready-btn"`, `id="ready-host"`, `id="ready-guest"`} {
		if !strings.Contains(body, want) {
			t.Errorf("participant body misses %q", want)
		}
	}
}

func TestRoomPageRedClockLineReadsRedBank(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed[:1])

	r.mu.Lock()
	r.clock[rules.Red].Commit(time.Hour)
	redLeft := r.clock[rules.Red].Remaining().Milliseconds()
	r.mu.Unlock()
	if got := formatClockMs(redLeft); got != "00:00.0" {
		t.Fatalf("red bank after the burn = %s, want the floored 00:00.0", got)
	}

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	if !strings.Contains(body, `id="clock-red">00:00.0</div>`) {
		t.Error("red clock line does not read the red bank")
	}
	if strings.Contains(body, `id="clock-blue">00:00.0</div>`) {
		t.Error("blue bank collapsed onto the red line's value")
	}
}

func TestRoomPageViewerColorPinsRedSeat(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed[:1])
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	if !strings.Contains(body, `data-my-color="red"`) {
		t.Error("red participant's board misses data-my-color=red")
	}
	if strings.Contains(body, `data-my-color="blue"`) {
		t.Error("red participant's board carries the inverted color")
	}
}

func TestRoomPageUnusableSessionStaysGuestOnOpenRoom(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(),
		strings.Repeat("ab", config.SessionTokenBytes))
	if status != http.StatusOK {
		t.Fatalf("status = %d, want the 200 guest render (body %s)", status, body)
	}
	if !strings.Contains(body, `data-participant="0"`) {
		t.Error("dead session reads as a participant over the open guest seat")
	}
	for _, banned := range []string{`id="controls"`, `id="ready-btn"`, `id="forfeit-btn"`, `id="join-btn"`} {
		if strings.Contains(body, banned) {
			t.Errorf("dead-session body contains control %q", banned)
		}
	}
}

func TestRoomPageOpenGuestSeatWaitingLine(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), "")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	if !strings.Contains(body, "waiting for opponent") {
		t.Error("open guest seat misses the waiting line")
	}
}

func TestBoardCellsLatestFlagOnLastMove(t *testing.T) {
	cells := boardCells(hostWinsRed[:3], nil)
	var latest []string
	for _, c := range cells {
		if c.Latest {
			latest = append(latest, c.Name)
		}
	}
	if len(latest) != 1 || latest[0] != "H8" {
		t.Errorf("latest cells = %v, want exactly [H8]", latest)
	}
}
