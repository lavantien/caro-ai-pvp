package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestProfileShareGuardsZeroGames(t *testing.T) {
	for _, row := range []struct {
		n, games int
		want     string
	}{
		{1, 1, "100.0%"},
		{1, 3, "33.3%"},
		{2, 3, "66.7%"},
		{0, 0, "0.0%"},
		{0, 7, "0.0%"},
	} {
		if got := profileShare(row.n, row.games); got != row.want {
			t.Errorf("profileShare(%d, %d) = %q, want %q", row.n, row.games, got, row.want)
		}
	}
}

func TestProfileSingleGameRendersShareBar(t *testing.T) {
	s := newStack(t)
	alice, bob := seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob")
	seedProfileGame(t, s.store, alice.ID, bob.ID, OutcomeRed, "")
	mux := http.NewServeMux()
	NewProfilePages(s.store).Mount(mux)
	req := httptest.NewRequest(http.MethodGet, "/players/alice", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("profile page = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"1 games", "1W-0L-0D", `width:100.0%`} {
		if !strings.Contains(body, want) {
			t.Errorf("single-game profile misses %q (body %s)", want, body)
		}
	}
	if strings.Contains(body, "no results yet.") {
		t.Error("a one-game profile renders the empty state, want the share bar")
	}
}

func TestLobbySeatFallbackMarkerUsesBaseTen(t *testing.T) {
	s := newStack(t)
	var ghost User
	for i := 0; ; i++ {
		u := seedUser(t, s.store, "ghost"+strconv.Itoa(i))
		if u.ID >= 10 {
			ghost = u
			break
		}
	}
	if _, err := s.rm.Create(ghost.ID, 0, config.SeriesBO3, nil, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.store.db.Exec(`DELETE FROM users WHERE id = ?`, ghost.ID); err != nil {
		t.Fatalf("delete ghost row: %v", err)
	}
	snap := s.rm.LobbySnapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot rooms = %d, want 1", len(snap))
	}
	if want := "#" + strconv.FormatInt(ghost.ID, 10); snap[0].Host.Name != want {
		t.Errorf("missing host seat = %q, want the base-ten marker %q", snap[0].Host.Name, want)
	}
}

func TestLobbySeatedCreatedRoomStaysUnjoinable(t *testing.T) {
	s := newStack(t)
	newPvPRoom(t, s)
	snap := s.rm.LobbySnapshot()
	if len(snap) != 1 {
		t.Fatalf("snapshot rooms = %d, want 1", len(snap))
	}
	if snap[0].Joinable {
		t.Errorf("seated created room = %+v, want joinable false while an opponent holds the seat", snap[0])
	}
	if snap[0].Status != "created" {
		t.Errorf("seated room state = %q, want created", snap[0].Status)
	}
}

func TestLobbyViewsKeepHostOnRedBeforeHandshake(t *testing.T) {
	s := newStack(t)
	_, _, r := newPvPRoom(t, s)
	views := lobbyViews(s.rm, nil)
	if len(views) != 1 {
		t.Fatalf("lobby views = %d, want 1", len(views))
	}
	if views[0].Red.Name != "alice" || views[0].Blue.Name != "bob" {
		t.Errorf("pre-handshake card = red %q blue %q, want the host alice on red", views[0].Red.Name, views[0].Blue.Name)
	}
	if r.gameSnapshot() != nil {
		t.Fatal("fixture room carries a live board, want the pre-handshake state")
	}
}

func TestPlaybackHeaderPinsTwoDigitGameIDInBaseTen(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob := seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob")
	sr := seedSeries(t, s.store, alice, bob)
	blob := EncodeMoves(nil, seqMoves(2))
	var last int64
	for i := 1; i <= 12; i++ {
		g, err := s.store.AppendGame(Game{SeriesID: sr.ID, IdxInSeries: i,
			RedUser: alice.ID, BlueUser: bob.ID, Outcome: OutcomeRed,
			Moves: blob, FullTurns: 1})
		if err != nil {
			t.Fatalf("append game %d: %v", i, err)
		}
		last = g.ID
	}
	if last < 10 {
		t.Fatalf("last seeded game id = %d, want two digits off the fresh rowid sequence", last)
	}
	id := strconv.FormatInt(last, 10)
	status, body := getRoomPage(t, srv, "/rooms/history/"+id, mintSession(t, s.store, alice))
	if status != http.StatusOK {
		t.Fatalf("playback of game %s: status = %d (body %s)", id, status, body)
	}
	if !strings.Contains(body, `title="game `+id+`">`+id+`</span>`) {
		t.Errorf("playback header misses the base-ten id %s (body %s)", id, body)
	}
}

func TestSignedDeltaSignsZeroAndNegatives(t *testing.T) {
	for _, row := range []struct {
		v    int
		want string
	}{
		{0, "+0"},
		{30, "+30"},
		{-1, "-1"},
		{-30, "-30"},
	} {
		if got := signedDelta(row.v); got != row.want {
			t.Errorf("signedDelta(%d) = %q, want %q", row.v, got, row.want)
		}
	}
}

func TestPlaybackBlueWinBillsTheBlueViewerWin(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, bob := seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob")
	sr := seedSeries(t, s.store, alice, bob)
	blob := EncodeMoves(nil, movesOf(t, hostWinsRed[:2]))
	if _, err := s.store.AppendGame(Game{SeriesID: sr.ID, IdxInSeries: 0,
		RedUser: alice.ID, BlueUser: bob.ID, Outcome: OutcomeBlue,
		Moves: blob, FullTurns: 1}); err != nil {
		t.Fatalf("append game: %v", err)
	}
	status, body := getRoomPage(t, srv, "/rooms/history/1", mintSession(t, s.store, bob))
	if status != http.StatusOK {
		t.Fatalf("playback: status = %d (body %s)", status, body)
	}
	if !strings.Contains(body, `<p class="pstat">record &#43;1W</p>`) {
		t.Errorf("blue winner's own playback misses the win record (body %s)", body)
	}
	if strings.Contains(body, `record &#43;1L`) {
		t.Error("blue winner's own playback carries a loss record")
	}
}

func TestFormatClockMsTenthDigitCrossesTheTenthsBoundary(t *testing.T) {
	for _, row := range []struct {
		ms   int64
		want string
	}{
		{1200, "00:01.2"},
		{6100, "00:06.1"},
		{20_100, "00:20.1"},
		{99, "00:00.0"},
	} {
		if got := formatClockMs(row.ms); got != row.want {
			t.Errorf("formatClockMs(%d) = %q, want %q", row.ms, got, row.want)
		}
	}
}

func TestShortIDReturnsShortIdsWhole(t *testing.T) {
	for _, id := range []string{"abc", "abcd", "abcde", "abcdef", "abcdefg", "abcdefgh"} {
		if got := shortID(id); got != id {
			t.Errorf("shortID(%q) = %q, want the id whole", id, got)
		}
	}
	long := "0123456789abcdef"
	if got := shortID(long); got != "0123"+"cdef" {
		t.Errorf("shortID(%q) = %q, want first and last four", long, got)
	}
}

func TestBotNameDisplayTrimsARoomIDAfterALeadingDash(t *testing.T) {
	room := "0123456789abcdef0123456789abcdef"
	if got := botNameDisplay("-" + room); got != "-"+shortID(room) {
		t.Errorf("botNameDisplay(-room) = %q, want %q", got, "-"+shortID(room))
	}
	if got := botNameDisplay("plain"); got != "plain" {
		t.Errorf("botNameDisplay(plain) = %q, want plain", got)
	}
}

func TestSegViewDefaultFirstOnEmptyAndSingleOptions(t *testing.T) {
	empty := segView{Name: "tc"}.defaultFirst()
	if len(empty.Options) != 0 {
		t.Errorf("empty seg grew options = %d", len(empty.Options))
	}
	single := segView{Name: "tc", Options: []segOption{{Value: "a", Label: "a"}}}.defaultFirst()
	if !single.Options[0].Selected {
		t.Error("a lone unselected option must become the default")
	}
}

func TestTourneyTierSegmentsRenderTight(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	_, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney", mintAdminSession(t, store), nil)
	if !strings.Contains(body, `class="seg tight"`) {
		t.Errorf("tier rows miss the tight seg class (body %s)", body)
	}
}

func TestTourneyRunIDsRenderInBaseTen(t *testing.T) {
	runs := &fakeTourney{runSummaries: []TourneyRunSummary{{
		TourneyRunInfo: TourneyRunInfo{ID: 10, CreatedAt: 1790970000, TCIdx: 1,
			BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating, Finished: true},
		Seats: []TourneySeat{{Slot: 0, Name: "easy-1"}, {Slot: 1, Name: "hard-1"}},
	}}}
	srv, _ := tourneySrv(t, runs)
	_, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney", "", nil)
	if !strings.Contains(body, `title="run 10">10</span>`) {
		t.Errorf("runs list misses the base-ten run id (body %s)", body)
	}

	page := &fakeTourney{snapshot: liveSnapshot()}
	pageSrv, _ := tourneySrv(t, page)
	_, _, body = doShell(t, noRedirectClient(pageSrv), http.MethodGet, pageSrv.URL+"/tourney/run/10", "", nil)
	if !strings.Contains(body, `title="run 10">10</span>`) {
		t.Errorf("run header misses the base-ten run id (body %s)", body)
	}
}

func TestWorstRoomCoresEmptyRosterNeedsZeroCores(t *testing.T) {
	if got := worstRoomCores(nil); got != 0 {
		t.Errorf("worstRoomCores(nil) = %d, want 0", got)
	}
	easy := worstRoomCores([]*config.Tier{&config.TierEasy})
	if easy != config.RoomCores(config.TierEasy, config.TierEasy) {
		t.Errorf("worstRoomCores(easy) = %d, want the self-pair peak %d", easy, config.RoomCores(config.TierEasy, config.TierEasy))
	}
}

func TestTourneyParallelKeepsTheBoundaryCountFeasible(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	_, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney", mintAdminSession(t, store), nil)
	if !strings.Contains(body, `name="parallel" value="1" checked>`) {
		t.Errorf("the boundary parallel count 1 at count*worst == MachineCores must stay feasible and checked (body %s)", body)
	}
	if !strings.Contains(body, `name="parallel" value="2" disabled`) {
		t.Errorf("count 2 over the pair-peak budget must render disabled (body %s)", body)
	}
}

func TestTourneyParallelDefaultStaysTwoOnAnAllEasyRoster(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	form := url.Values{"tc": {"9"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"}}
	for i := range config.Tiers {
		form["name"+strconv.Itoa(i)] = []string{"bot" + strconv.Itoa(i)}
	}
	for i := 0; i < config.TournamentMaxParticipants; i++ {
		form["tier"+strconv.Itoa(i)] = []string{config.TierEasy.Name}
	}
	status, _, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney", mintAdminSession(t, store), form)
	if status != http.StatusBadRequest {
		t.Fatalf("bad tc re-render = %d, want 400 (body %s)", status, body)
	}
	if !strings.Contains(body, `name="parallel" value="2" checked>`) {
		t.Errorf("all-easy roster must check the default 2 parallel rooms (body %s)", body)
	}
	if strings.Contains(body, `name="parallel" value="1" checked`) {
		t.Errorf("all-easy roster demoted the checked default to 1, want the config default 2 (body %s)", body)
	}
}

func TestTourneyStartAcceptsParallelAtTheChoiceCeiling(t *testing.T) {
	fake := &fakeTourney{startID: 7}
	srv, store := tourneyMount(t, fake)
	form := url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
		"rating":   {strconv.Itoa(config.TournamentStartRating)},
		"parallel": {strconv.Itoa(config.TournamentParallelChoices)}, "count": {"2"},
		"name0": {"a"}, "tier0": {config.TierEasy.Name},
		"name1": {"b"}, "tier1": {config.TierEasy.Name},
	}
	status, _, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney", mintAdminSession(t, store), form)
	if status != http.StatusSeeOther {
		t.Fatalf("parallel at the ceiling: status = %d, want 303 (body %s)", status, body)
	}
	if got := fake.started[0].Parallel; got != config.TournamentParallelChoices {
		t.Errorf("parallel reached the service as %d, want %d", got, config.TournamentParallelChoices)
	}
}

func TestTourneyStartRefusesCoresDemandBeyondTheMachine(t *testing.T) {
	fake := &fakeTourney{startID: 7}
	srv, store := tourneyMount(t, fake)
	gm := config.Tiers[len(config.Tiers)-1].Name
	form := url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
		"rating":   {strconv.Itoa(config.TournamentStartRating)},
		"parallel": {"2"}, "count": {"2"},
		"name0": {"a"}, "tier0": {gm},
		"name1": {"b"}, "tier1": {gm},
	}
	status, _, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney", mintAdminSession(t, store), form)
	if status != http.StatusBadRequest {
		t.Fatalf("core demand beyond the machine: status = %d, want 400 (body %s)", status, body)
	}
	if !strings.Contains(body, "live-search cores each need") {
		t.Errorf("refused start misses the core budget message (body %s)", body)
	}
	if len(fake.started) != 0 {
		t.Errorf("service saw %d starts, want 0", len(fake.started))
	}
}
