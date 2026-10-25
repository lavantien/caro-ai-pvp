package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/geom"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func newHexPvPRoom(t *testing.T, s *stack) (host, guest User, r *Room) {
	t.Helper()
	host, guest = seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob")
	r, err := s.rm.Create(host.ID, 0, config.SeriesBO3, nil, "hex")
	if err != nil {
		t.Fatalf("create hex room: %v", err)
	}
	if err := s.rm.Join(r.ID(), guest.ID); err != nil {
		t.Fatalf("join hex room: %v", err)
	}
	return host, guest, r
}

func playHexScript(t *testing.T, r *Room, redUser, blueUser int64, names []string) {
	t.Helper()
	for i, name := range names {
		mover := redUser
		if i%2 == 1 {
			mover = blueUser
		}
		cell, err := r.ParseCell(name)
		if err != nil {
			t.Fatalf("parse hex cell %s: %v", name, err)
		}
		if err := r.PlayMove(mover, cell); err != nil {
			t.Fatalf("hex move %d %s by %d: %v", i+1, name, mover, err)
		}
	}
}

var hexCellIDPattern = regexp.MustCompile(`id="c-([A-W][0-9]{1,2})"`)

func wantHexBoard(t *testing.T, body string) {
	t.Helper()
	g := mustHexGeometry(t)
	if !strings.Contains(body, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="`) {
		t.Fatal("hex board page misses the svg board")
	}
	if strings.Contains(body, `style="grid-template-columns:`) {
		t.Error("hex board page carries the square grid style")
	}
	if !strings.Contains(body, `style="aspect-ratio:`) {
		t.Error("hex board misses its aspect-ratio style")
	}
	ids := hexCellIDPattern.FindAllStringSubmatch(body, -1)
	if len(ids) != 397 {
		t.Fatalf("hex cell count = %d, want the 397 playable cells", len(ids))
	}
	seen := make(map[string]bool, len(ids))
	for _, m := range ids {
		name := m[1]
		if seen[name] {
			t.Fatalf("cell %s rendered twice", name)
		}
		seen[name] = true
		if _, err := g.ParseCell(name); err != nil {
			t.Errorf("cell id %s is not a region cell: %v", name, err)
		}
	}
}

func TestHexBoardViewGeometryAndStones(t *testing.T) {
	g := mustHexGeometry(t)
	view := hexBoardOf(g, []string{"L12", "M13"}, nil)

	if len(view.Cells) != 397 {
		t.Fatalf("hex board cells = %d, want 397", len(view.Cells))
	}
	for _, c := range view.Cells {
		if _, err := g.ParseCell(c.Name); err != nil {
			t.Errorf("cell %s is not playable: %v", c.Name, err)
		}
		if !strings.HasPrefix(c.Transform, "translate(") {
			t.Errorf("cell %s misses its translate transform", c.Name)
		}
	}
	if got := len(strings.Fields(view.Points)); got != 12 {
		t.Errorf("hex polygon points numbers = %d, want 12", got)
	}
	vb := strings.Fields(view.ViewBox)
	if len(vb) != 4 {
		t.Fatalf("viewbox fields = %d, want 4", len(vb))
	}
	w, errW := strconv.ParseFloat(vb[2], 64)
	h, errH := strconv.ParseFloat(vb[3], 64)
	if errW != nil || errH != nil || w <= 0 || h <= 0 {
		t.Fatalf("viewbox size %q is not two positive floats", view.ViewBox)
	}
	if w <= h {
		t.Errorf("hex board viewBox %s is taller than wide, want wider", view.ViewBox)
	}
	ratio, errR := strconv.ParseFloat(view.Aspect, 64)
	if errR != nil || ratio <= 1 || ratio > 2 {
		t.Errorf("hex board aspect %q is not a sane width-over-height number", view.Aspect)
	}

	stones := 0
	for _, c := range view.Cells {
		switch c.Name {
		case "L12":
			if c.Stone != "red" || c.Glyph != "O" || c.Idx != 0 || c.Latest {
				t.Errorf("L12 stone = %q glyph %q idx %d latest %v, want red O idx 0 not latest", c.Stone, c.Glyph, c.Idx, c.Latest)
			}
		case "M13":
			if c.Stone != "blue" || c.Glyph != "X" || c.Idx != 1 || !c.Latest {
				t.Errorf("M13 stone = %q glyph %q idx %d latest %v, want blue X idx 1 latest", c.Stone, c.Glyph, c.Idx, c.Latest)
			}
		}
		if c.Stone != "" {
			stones++
		}
	}
	if stones != 2 {
		t.Errorf("hex board stone cells = %d, want 2", stones)
	}
}

func wantHexMiniBoard(t *testing.T, body string) {
	t.Helper()
	if !strings.Contains(body, `class="board hex mini"`) {
		t.Fatal("mini hex board class missing")
	}
	if got := strings.Count(body, `<g class="cell`); got != 397 {
		t.Errorf("mini hex cell count = %d, want 397", got)
	}
	if hexCellIDPattern.MatchString(body) {
		t.Error("mini hex board carries cell ids, want the inert mini")
	}
}

func TestMiniHexBoardStaysLegibleAtCaptureScale(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newHexPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playHexScript(t, r, alice.ID, bob.ID, []string{"L12", "M13"})
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := srv.Client().Get(srv.URL + "/lobby")
	if err != nil {
		t.Fatalf("lobby get: %v", err)
	}
	body := readAllBody(t, resp)
	for _, want := range []string{
		`.board.mini.hex .cell polygon { fill: none; stroke: var(--border); stroke-width: 1; vector-effect: non-scaling-stroke }`,
		`.board.mini.hex .cell circle.stone.latest { stroke-width: 1.5; vector-effect: non-scaling-stroke }`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("mini hex legibility rule missing: %s", want)
		}
	}
	if strings.Contains(body, `text.glyph { display: none }`) {
		t.Error("mini hex css hides the stone glyphs, want every hex render path to carry them")
	}
	if n := strings.Count(body, `<text class="glyph"`); n == 0 {
		t.Error("mini hex board markup loses the glyph text nodes")
	}
}

func TestHexRoomPageRendersOnlyPlayableCells(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newHexPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playHexScript(t, r, alice.ID, bob.ID, []string{"L12", "M13", "P12"})
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK {
		t.Fatalf("hex room page status = %d (body %s)", status, body)
	}
	wantHexBoard(t, body)
	if got := strings.Count(body, `class="cell`); got != 397 {
		t.Errorf("hex page cell markers = %d, want 397", got)
	}
	if !strings.Contains(body, `data-stone-r="6.30"`) {
		t.Error("hex board div misses its data-stone-r radius for JS stones")
	}
	i := strings.Index(body, `id="c-L12"`)
	if i < 0 || !strings.Contains(body[i:i+250], `class="stone red"`) {
		t.Error("L12 misses its red stone")
	}
	if i < 0 || !strings.Contains(body[i:i+250], `<circle class="stone red" data-i="0" r="6.30"/><text class="glyph" dy=".35em" data-i="0">O</text>`) {
		t.Error("L12 misses its circle stone with the O glyph text")
	}
	i = strings.Index(body, `id="c-P12"`)
	if i < 0 || !strings.Contains(body[i:i+250], `class="stone red latest"`) {
		t.Error("P12 misses its red latest stone")
	}
	if i < 0 || !strings.Contains(body[i:i+400], `<text class="glyph" dy=".35em" data-i="2">O</text>`) {
		t.Error("P12 misses its O glyph text")
	}
	if !strings.Contains(body, "playing / hex /") && !strings.Contains(body, "/ hex /") {
		t.Error("room status line misses the hex variant")
	}
	if got := strings.Count(body, `class="stone `); got != 3 {
		t.Errorf("hex page stones = %d, want 3", got)
	}
}

func TestSquareRoomPageKeepsTheGridMarkup(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"H8"})
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK {
		t.Fatalf("square room page status = %d", status)
	}
	wantBoard(t, body)
	if !strings.Contains(body, `style="grid-template-columns:repeat(`+
		strconv.Itoa(config.BoardSize)+`,1fr)"`) {
		t.Error("square board lost its grid-template-columns style")
	}
	if strings.Contains(body, `<svg xmlns="http://www.w3.org/2000/svg"`) {
		t.Error("square board page renders the hex svg branch")
	}
}

func TestHexRoomPagePlayableMaskUnderHexDistance(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob, r := newHexPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playHexScript(t, r, alice.ID, bob.ID, []string{"L12", "W12"})
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/"+r.ID(), ta)
	if status != http.StatusOK {
		t.Fatalf("mover page status = %d (body %s)", status, body)
	}
	wantHexBoard(t, body)
	if got := strings.Count(body, `class="cell off"`); got != 18 {
		t.Errorf("hex mover render dims %d cells, want the 19-cell opening ball minus the stone", got)
	}
	for _, want := range []string{
		`class="cell off" id="c-M12"`,
		`class="cell off" id="c-K12"`,
		`class="cell off" id="c-M13"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("near cell %s misses the off mark", want)
		}
	}
	if !strings.Contains(body, `class="cell" id="c-I12"`) {
		t.Error("distance-3 cell I12 is dimmed, want playable")
	}
	if !strings.Contains(body, `class="cell" id="c-V12"`) {
		t.Error("far cell V12 is dimmed, want playable")
	}
}

func TestHexLobbyCardRendersMiniHexBoard(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newHexPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playHexScript(t, r, alice.ID, bob.ID, []string{"L12", "M13"})
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	resp, err := srv.Client().Get(srv.URL + "/lobby")
	if err != nil {
		t.Fatalf("lobby get: %v", err)
	}
	body := readAllBody(t, resp)
	wantHexMiniBoard(t, body)
	if !strings.Contains(body, `<span class="chip"><svg class="vglyph vglyph-hex"`) {
		t.Error("lobby hex card misses the variant chip")
	}
}

func TestHexPlaybackPageRendersHexBoard(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob := seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob")
	g := mustHexGeometry(t)
	names := []string{"L12", "M13", "P12", "N13", "N12", "O13", "O12", "L13", "M12"}
	moves := make([]rules.Move, 0, len(names))
	for _, n := range names {
		cell, err := g.ParseCell(n)
		if err != nil {
			t.Fatalf("parse %s: %v", n, err)
		}
		moves = append(moves, rules.Move(cell))
	}
	sr, err := s.store.CreateSeriesFor(context.Background(), 1, config.SeriesBO3, alice.ID, bob.ID, "hex")
	if err != nil {
		t.Fatalf("hex series row: %v", err)
	}
	wonBy := "5"
	if err := s.store.ApplyCompletion(context.Background(), Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: bob.ID,
			Outcome: OutcomeRed, Moves: EncodeMoves(nil, moves),
			FullTurns: len(moves) / 2, WonBy: &wonBy,
		}},
	}); err != nil {
		t.Fatalf("hex completion: %v", err)
	}
	rows, err := s.store.matchHistoryWithIDs(alice.ID, "")
	if err != nil || len(rows) != 1 {
		t.Fatalf("history rows = %d err %v, want the hex game", len(rows), err)
	}
	ta := mintSession(t, s.store, alice)

	status, body := getRoomPage(t, srv, "/rooms/history/"+strconv.FormatInt(rows[0].GameID, 10), ta)
	if status != http.StatusOK {
		t.Fatalf("hex playback status = %d (body %s)", status, body)
	}
	wantHexBoard(t, body)
}

func readAllBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	return buf.String()
}

func TestCreateFormsOfferTheBoardSelect(t *testing.T) {
	s := newStack(t)
	mux := http.NewServeMux()
	NewLobbyPages(s.store, s.rm).Mount(mux)
	root := http.NewServeMux()
	root.Handle("/", NewShellPages(s.store, s.rm, &fakeTourney{}))
	root.Handle("/lobby", mux)
	NewTournamentPages(s.store, &fakeTourney{}).Mount(root)
	srv := httptest.NewServer(root)
	t.Cleanup(srv.Close)
	alice := seedUser(t, s.store, "alice")
	ta := mintSession(t, s.store, alice)
	c := noRedirectClient(srv)

	for _, path := range []string{"/", "/lobby", "/tourney"} {
		token := ta
		if path == "/tourney" {
			token = mintAdminSession(t, s.store)
		}
		req, err := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if err != nil {
			t.Fatalf("new request %s: %v", path, err)
		}
		req.Header.Set("Cookie", sessionCookieName+"="+token)
		resp, err := c.Do(req)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		body := readAllBody(t, resp)
		if !strings.Contains(body, `<input type="radio" name="variant"`) {
			t.Errorf("%s misses the board seg", path)
			continue
		}
		if !strings.Contains(body, `name="variant" value="square" checked><span>square 16x16</span>`) {
			t.Errorf("%s misses the checked square default", path)
		}
		if !strings.Contains(body, `name="variant" value="hex"><span>hexagon 12</span>`) {
			t.Errorf("%s misses the hexagon option", path)
		}
	}

	form := url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)}, "bot": {""}, "variant": {"hex"},
	}
	status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/rooms", ta, form)
	if status != http.StatusSeeOther {
		t.Fatalf("hex create status = %d (body %s)", status, body)
	}
	loc := h.Get("Location")
	roomID := strings.TrimPrefix(loc, "/rooms/")
	room, err := s.rm.Get(roomID)
	if err != nil {
		t.Fatalf("created room fetch: %v", err)
	}
	if v := room.Variant(); v != "hex" {
		t.Errorf("created room variant = %q, want hex", v)
	}
}

func TestTourneyStartPassesHexVariant(t *testing.T) {
	fake := &fakeTourney{startID: 9}
	srv, store := tourneyMount(t, fake)
	token := mintAdminSession(t, store)
	form := url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
		"rating":   {strconv.Itoa(config.TournamentStartRating)},
		"parallel": {"1"}, "count": {"2"}, "variant": {"hex"},
		"name0": {"a"}, "tier0": {config.TierEasy.Name},
		"name1": {"b"}, "tier1": {config.TierEasy.Name},
	}
	status, h, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney", token, form)
	if status != http.StatusSeeOther || h.Get("Location") != "/tourney/run/9" {
		t.Fatalf("hex tourney start status = %d loc %q (body %s)", status, h.Get("Location"), body)
	}
	if got := fake.started[0].Variant; got != "hex" {
		t.Errorf("tourney setup variant = %q, want hex", got)
	}
}

func TestHexTourneyLiveBoardRendersHexMini(t *testing.T) {
	fake := &fakeTourney{snapshot: liveSnapshot(), live: []TourneyLiveBoard{{
		RoomID: "5e3f8a01c2d4e6f7", RedName: "hard-3", BlueName: "hard-4",
		RedWins: 1, BlueWins: 0, Turn: "red", Variant: "hex",
		Moves: []string{"L12", "M13", "P12"},
	}}}
	srv, _ := tourneyMount(t, fake)
	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney/run/1", "", nil)
	if status != http.StatusOK {
		t.Fatalf("hex live board run status = %d (body %s)", status, body)
	}
	wantHexMiniBoard(t, body)
}

type gatedHexBot struct {
	g     *geom.Geometry
	calls int
	gate  chan struct{}
}

func (b *gatedHexBot) Search(*rules.Board, engine.Deadline) (rules.Move, engine.SearchStats, string) {
	if b.calls >= 1 {
		<-b.gate
	}
	b.calls++
	return rules.Move(b.g.Center), engine.SearchStats{}, ""
}
func (b *gatedHexBot) StartPonder(*rules.Board) {}
func (b *gatedHexBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return rules.MoveNone, engine.SearchStats{}, ""
}
func (b *gatedHexBot) Close() {}

func TestHexBotVsBotRoomFeedsTheLiveBotBoard(t *testing.T) {
	s := newStack(t)
	gate := make(chan struct{})
	t.Cleanup(func() { close(gate) })
	bot := &gatedHexBot{g: mustHexGeometry(t), gate: gate}
	s.rm.makeSearcher = func(_ string, _ config.Tier) searcher { return bot }

	if _, err := s.rm.CreateBotVsBot(&config.TierEasy, "easy-1", &config.TierEasy, "easy-2",
		0, config.SeriesBO3, "hex"); err != nil {
		t.Fatalf("hex bot-vs-bot room: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	var boards []LiveBotBoard
	for {
		boards = s.rm.BotBoards()
		if len(boards) == 1 && len(boards[0].Moves) == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("first bot move never landed, boards %+v", boards)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if boards[0].Variant != "hex" {
		t.Errorf("live bot board variant = %q, want hex", boards[0].Variant)
	}
	if boards[0].Moves[0] != "L12" {
		t.Errorf("bot move name = %q, want the hex center L12 through the hex codec", boards[0].Moves[0])
	}
}
