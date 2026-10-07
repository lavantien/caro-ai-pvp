package server

// The M7 tournament UI tests over a scripted TourneyService: the guest
// bounces, the setup form's config-driven defaults, the POST validation and
// the start redirect, the live run page and its polled fragment, the run
// states, the past-runs section, and the not-found shape. No engine ever
// runs here; the fake stands in for the tourney manager.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// fakeTourney scripts TourneyService: the setups StartRun saw, the id it
// hands back, the reads the pages render, the stalled closes, and the home
// banner's ongoing run.
type fakeTourney struct {
	startErr     error
	startID      int64
	started      []TourneySetup
	snapshot     TourneySnapshot
	snapErr      error
	runSummaries []TourneyRunSummary
	runsErr      error
	closeErr     error
	closed       []int64
	live         []TourneyLiveBoard
	ongoing      *TourneyBanner
	ongoingErr   error
}

func (f *fakeTourney) StartRun(_ context.Context, setup TourneySetup) (int64, error) {
	f.started = append(f.started, setup)
	if f.startErr != nil {
		return 0, f.startErr
	}
	return f.startID, nil
}

func (f *fakeTourney) CloseStalledRun(_ context.Context, runID int64) error {
	f.closed = append(f.closed, runID)
	return f.closeErr
}

func (f *fakeTourney) RunSnapshot(_ context.Context, runID int64) (TourneySnapshot, error) {
	if f.snapErr != nil {
		return TourneySnapshot{}, f.snapErr
	}
	f.snapshot.Run.ID = runID
	return f.snapshot, nil
}

func (f *fakeTourney) Runs(context.Context) ([]TourneyRunSummary, error) {
	return f.runSummaries, f.runsErr
}

func (f *fakeTourney) OngoingRun(context.Context) (TourneyBanner, bool, error) {
	if f.ongoingErr != nil {
		return TourneyBanner{}, false, f.ongoingErr
	}
	if f.ongoing == nil {
		return TourneyBanner{}, false, nil
	}
	return *f.ongoing, true, nil
}

func (f *fakeTourney) LiveBoards() []TourneyLiveBoard {
	return f.live
}

// tourneyMount mounts the pages over a scripted service and a fresh stack,
// handing back the server and the store sessions mint against.
func tourneyMount(t *testing.T, fake *fakeTourney) (*httptest.Server, *Store) {
	t.Helper()
	s := newStack(t)
	mux := http.NewServeMux()
	NewTournamentPages(s.store, fake).Mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, s.store
}

// tourneySrv mounts the pages with a plain member session (alice), the
// non-admin answer every admin gate must refuse.
func tourneySrv(t *testing.T, fake *fakeTourney) (*httptest.Server, string) {
	t.Helper()
	srv, store := tourneyMount(t, fake)
	return srv, mintSession(t, store, seedUser(t, store, "alice"))
}

// mintAdminSession mints the seeded admin account's session, the one holder
// of the tournament controls.
func mintAdminSession(t *testing.T, store *Store) string {
	t.Helper()
	admin, err := store.UserByUsername(config.AdminName)
	if err != nil {
		t.Fatalf("seeded admin row: %v", err)
	}
	return mintSession(t, store, admin)
}

func TestTourneyPagePublicAndAdminGated(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	c := noRedirectClient(srv)

	// The page is public like the rooms grid: a guest reads it, sees the
	// runs section, and never the admin's form.
	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/tourney", "", nil)
	if status != http.StatusOK {
		t.Fatalf("guest setup page: status = %d (body %s)", status, body)
	}
	wantShellBody(t, body, "no tournaments yet.", "tournaments are started by the admin account.")
	if strings.Contains(body, `action="/tourney"`) {
		t.Error("guest page carries the setup form, want the read-only page")
	}

	// A member session gets the same read-only page.
	token := mintSession(t, store, seedUser(t, store, "alice"))
	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/tourney", token, nil)
	if strings.Contains(body, `action="/tourney"`) {
		t.Error("member page carries the setup form, want the admin's alone")
	}

	// The admin session sees the form.
	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/tourney", mintAdminSession(t, store), nil)
	if !strings.Contains(body, `action="/tourney"`) {
		t.Error("admin page misses the setup form")
	}

	// The acting routes keep the login bounce for guests.
	status, h, _ := doShell(t, c, http.MethodPost, srv.URL+"/tourney", "", nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/login" {
		t.Errorf("guest start = %d %q, want 303 /login", status, h.Get("Location"))
	}
	status, h, _ = doShell(t, c, http.MethodPost, srv.URL+"/tourney/run/7/close", "", nil)
	if status != http.StatusSeeOther || h.Get("Location") != "/login" {
		t.Errorf("guest close = %d %q, want 303 /login", status, h.Get("Location"))
	}
}

func TestTourneySetupFormRendersConfigDefaults(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	token := mintAdminSession(t, store)

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney", token, nil)
	if status != http.StatusOK {
		t.Fatalf("setup: status = %d (body %s)", status, body)
	}

	// The selects and inputs render from the config hub: every time control,
	// every series length, the start rating default, the parallel options
	// with the spec's TournamentParallelMatches selected, the participant
	// counts with the full default roster selected, and the two-per-tier
	// prefill with each row's tier selected.
	wants := []string{
		`action="/tourney"`,
		"1&#43;0", "2&#43;1", "3&#43;2", "bo3", "bo5", "bo7", "bo11",
		`name="rating" type="number" value="` + strconv.Itoa(config.TournamentStartRating) + `"`,
		`<option value="` + strconv.Itoa(config.TournamentParallelMatches) + `" selected>`,
		`<option value="` + strconv.Itoa(config.TournamentMaxParticipants) + `" selected>`,
		"no tournaments yet.",
	}
	for i, row := range config.DefaultRoster() {
		wants = append(wants,
			`name="name`+strconv.Itoa(i)+`" maxlength="`+strconv.Itoa(config.TournamentNameMaxBytes)+
				`" value="`+row.Name+`"`,
			`<option value="`+row.Tier+`" selected>AI `+row.Tier+`</option>`,
		)
	}
	wantShellBody(t, body, wants...)
	for _, count := range []int{2, 3, 4, 5} {
		if !strings.Contains(body, `<option value="`+strconv.Itoa(count)+`">`) {
			t.Errorf("participant count %d missing from the select", count)
		}
	}
}

func TestTourneyStartValidatesAndRedirects(t *testing.T) {
	fake := &fakeTourney{startID: 7}
	srv, store := tourneyMount(t, fake)
	token := mintAdminSession(t, store)
	c := noRedirectClient(srv)

	// The happy path: a 3-bot roster over the first three default rows, the
	// spec's default rating, 1 parallel. The service must see exactly the
	// form's seats in slot order, the rows past the count ignored.
	def := config.DefaultRoster()
	form := url.Values{
		"tc": {"1"}, "bo": {strconv.Itoa(config.SeriesBO3)},
		"rating":   {strconv.Itoa(config.TournamentStartRating)},
		"parallel": {"1"}, "count": {"3"},
		"name0": {def[0].Name}, "tier0": {def[0].Tier},
		"name1": {"custom bot"}, "tier1": {def[1].Tier},
		"name2": {def[2].Name}, "tier2": {def[2].Tier},
		"name3": {"ignored"}, "tier3": {def[3].Tier},
	}
	status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/tourney", token, form)
	if status != http.StatusSeeOther {
		t.Fatalf("start: status = %d, want 303 (body %s)", status, body)
	}
	if loc := h.Get("Location"); loc != "/tourney/run/7" {
		t.Errorf("start redirect = %q, want /tourney/run/7", loc)
	}
	if len(fake.started) != 1 {
		t.Fatalf("StartRun called %d times, want 1", len(fake.started))
	}
	got := fake.started[0]
	if got.TCIdx != 1 || got.BOLen != config.SeriesBO3 ||
		got.StartRating != config.TournamentStartRating || got.Parallel != 1 {
		t.Errorf("setup scalars = %+v", got)
	}
	if len(got.Seats) != 3 {
		t.Fatalf("seats = %+v, want the 3 counted rows", got.Seats)
	}
	for i, seat := range got.Seats {
		if seat.Slot != i {
			t.Errorf("seat %d carries slot %d", i, seat.Slot)
		}
	}
	if got.Seats[0].Name != def[0].Name || got.Seats[0].Tier != def[0].Tier ||
		got.Seats[1].Name != "custom bot" || got.Seats[2].Tier != def[2].Tier {
		t.Errorf("seats = %+v, want the submitted rows in order", got.Seats)
	}

	// Every config law the form must enforce answers inline at 400 with the
	// submitted names preserved.
	long := strings.Repeat("x", config.TournamentNameMaxBytes+1)
	for name, form := range map[string]url.Values{
		"bad tc":           {"tc": {"9"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"}},
		"bad bo":           {"tc": {"0"}, "bo": {"4"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"}},
		"bad rating":       {"tc": {"0"}, "bo": {"3"}, "rating": {"fast"}, "parallel": {"1"}, "count": {"2"}},
		"rating unbounded": {"tc": {"0"}, "bo": {"3"}, "rating": {strconv.Itoa(config.TournamentStartRatingAbsMax + 1)}, "parallel": {"1"}, "count": {"2"}},
		"bad parallel":     {"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"0"}, "count": {"2"}},
		"bad count":        {"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"1"}},
		"empty name":       {"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"}, "name0": {""}, "tier0": {"easy"}},
		"long name":        {"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"}, "name0": {long}, "tier0": {"easy"}},
		"unknown tier":     {"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"}, "name0": {"a"}, "tier0": {"mythic"}},
		"duplicate name":   {"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"}, "name0": {"a"}, "tier0": {"easy"}, "name1": {"a"}, "tier1": {"easy"}},
	} {
		if _, ok := form["name1"]; !ok {
			form["name1"] = []string{"kept name"}
		}
		form["tier1"] = []string{config.TierMedium.Name}
		status, _, body := doShell(t, c, http.MethodPost, srv.URL+"/tourney", token, form)
		if status != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", name, status, body)
			continue
		}
		if !strings.Contains(body, `class="error"`) {
			t.Errorf("%s: body misses the inline error (body %s)", name, body)
		}
		if want := form["name1"][0]; !strings.Contains(body, `value="`+want+`"`) {
			t.Errorf("%s: body loses the submitted names (body %s)", name, body)
		}
	}

	// A body that cannot decode as a form fails the same way.
	status, body = doRawPost(t, c, srv.URL+"/tourney", token,
		"application/x-www-form-urlencoded", "%zz=1")
	if status != http.StatusBadRequest || !strings.Contains(body, "malformed form body") {
		t.Errorf("undecodable start body = %d %s, want 400 with the inline error", status, body)
	}

	// A member's post never reaches the service: the admin gate answers the
	// read-only setup page with the refusal inline, through the error slot
	// outside the form branch.
	member := mintSession(t, store, seedUser(t, store, "bob"))
	status, _, body = doShell(t, c, http.MethodPost, srv.URL+"/tourney", member, url.Values{
		"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"},
		"name0": {"a"}, "tier0": {"easy"}, "name1": {"b"}, "tier1": {"easy"},
	})
	if status != http.StatusForbidden {
		t.Errorf("member start = %d, want 403", status)
	}
	wantShellBody(t, body, `class="error"`, "only the admin account starts tournaments")
	if len(fake.started) != 1 {
		t.Errorf("service saw %d starts after the member post, want the admin's 1 alone", len(fake.started))
	}

	// A service failure after clean validation is an outage, not a form
	// error.
	fake.startErr = errors.New("boom")
	status, _, _ = doShell(t, c, http.MethodPost, srv.URL+"/tourney", token, url.Values{
		"tc": {"0"}, "bo": {"3"}, "rating": {"1000"}, "parallel": {"1"}, "count": {"2"},
		"name0": {"a"}, "tier0": {"easy"}, "name1": {"b"}, "tier1": {"easy"},
	})
	if status != http.StatusInternalServerError {
		t.Errorf("service failure = %d, want 500", status)
	}
}

// liveSnapshot scripts a two-bot run mid-flight: pairing 0 settled for the
// red-first seat, pairing 1 still playing, the board pre-ordered the way the
// store's leaderboard orders it.
func liveSnapshot() TourneySnapshot {
	winner := 0
	return TourneySnapshot{
		Run: TourneyRunInfo{ID: 7, CreatedAt: 1790970000, TCIdx: 1, BOLen: config.SeriesBO3,
			StartRating: config.TournamentStartRating, Running: true},
		Seats: []TourneySeat{
			{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "hard-1", Tier: config.TierHard.Name},
		},
		Series: []TourneySeriesLine{
			{PairingSlot: 0, RedFirstSlot: 0, BlueFirstSlot: 1, RedFirstWins: 2, BlueFirstWins: 0, WinnerSlot: &winner, Finished: true},
			{PairingSlot: 1, RedFirstSlot: 1, BlueFirstSlot: 0, RedFirstWins: 0, BlueFirstWins: 0},
		},
		Board: []TourneyStanding{
			{Slot: 0, Rating: config.TournamentStartRating + 40, Wins: 2, GamesPlayed: 2, SeriesWon: 1},
			{Slot: 1, Rating: config.TournamentStartRating - 40, Losses: 2, GamesPlayed: 2},
		},
	}
}

func TestTourneyRunPageRendersLiveBoard(t *testing.T) {
	fake := &fakeTourney{snapshot: liveSnapshot()}
	srv, token := tourneySrv(t, fake)
	c := noRedirectClient(srv)

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/7", token, nil)
	if status != http.StatusOK {
		t.Fatalf("run page: status = %d (body %s)", status, body)
	}
	wantShellBody(t, body,
		">running<",
		"series 1/2 settled",
		// Red-first order: the pairing-0 line seats easy-1 first with its
		// settled score and winner, pairing 1 renders still playing.
		"easy-1 (red first) 2-0 hard-1",
		"easy-1 wins",
		"hard-1 (red first) 0-0 easy-1",
		">playing<",
		// The leaderboard renders the given order with ranks and both seats'
		// names and tiers.
		"<td>1</td><td>easy-1</td><td>"+config.TierEasy.Name+"</td><td>"+
			strconv.Itoa(config.TournamentStartRating+40)+"</td>",
		"<td>2</td><td>hard-1</td><td>"+config.TierHard.Name+"</td><td>"+
			strconv.Itoa(config.TournamentStartRating-40)+"</td>",
		// The rooms-grid poll pattern: the board partial refreshes in place.
		`id="tourboard"`, `hx-get="/tourney/run/7/board"`,
		`hx-trigger="every `+strconv.Itoa(config.PagePollMs)+`ms"`, `hx-swap="innerHTML"`,
		`href="/tourney"`,
	)

	// The fragment endpoint serves the polled region only, public like the
	// run page.
	status, h, frag := doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/7/board", "", nil)
	if status != http.StatusOK {
		t.Fatalf("board fragment: status = %d", status)
	}
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Errorf("fragment Cache-Control = %q, want no-store", got)
	}
	wantShellBody(t, frag, "series 1/2 settled", "<td>1</td><td>easy-1</td>")
	for _, page := range []string{"<!doctype", "<html", "<header"} {
		if strings.Contains(frag, page) {
			t.Errorf("board fragment carries the whole page (%q)", page)
		}
	}
}

// TestTourneyRunPageRendersLiveBoardSection pins the live spectating cards:
// one per ongoing room, linked, the score told from red's side, and the mini
// board an inert grid of stones in play order. The section rides the polled
// fragment and stays off every page whose run is not live.
func TestTourneyRunPageRendersLiveBoardSection(t *testing.T) {
	sn := liveSnapshot()
	fake := &fakeTourney{snapshot: sn, live: []TourneyLiveBoard{{
		RoomID: "roomabc", RedName: "easy-1", BlueName: "hard-1",
		RedWins: 1, BlueWins: 2, Turn: "blue", Moves: []string{"H8", "H9", "I9"},
	}}}
	srv, token := tourneySrv(t, fake)
	c := noRedirectClient(srv)

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/7", token, nil)
	if status != http.StatusOK {
		t.Fatalf("run page: status = %d (body %s)", status, body)
	}
	wantShellBody(t, body,
		`class="liveboards"`,
		`href="/rooms/roomabc"`,
		"easy-1 vs hard-1",
		// Red's perspective: 1-2 beside red-named-first seats.
		">1-2<",
		"blue to move, 3 stones",
		`class="miniboard" style="grid-template-columns:repeat(`+
			strconv.Itoa(config.BoardSize)+`,1fr)"`,
	)
	// Stones by play parity: H8 and I9 red, H9 blue, the latest marked once.
	if n := strings.Count(body, `m-red`); n != 2 {
		t.Errorf("red mini stones = %d, want 2 (body %s)", n, body)
	}
	if n := strings.Count(body, `m-blue`); n != 1 {
		t.Errorf("blue mini stones = %d, want 1 (body %s)", n, body)
	}
	if n := strings.Count(body, `m-latest`); n != 1 {
		t.Errorf("latest mini stone marks = %d, want 1 (body %s)", n, body)
	}
	if strings.Contains(body, `class="mcell" data-cell`) {
		t.Error("mini cells carry tap targets, want an inert spectating grid")
	}

	// The polled fragment serves the same cards.
	_, _, frag := doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/7/board", "", nil)
	if !strings.Contains(frag, "easy-1 vs hard-1") || !strings.Contains(frag, `href="/rooms/roomabc"`) {
		t.Errorf("board fragment misses the live cards (body %s)", frag)
	}

	// A run whose drive is not live never shows boards, even with live rooms
	// behind the service: only the ongoing run's page owns them.
	stalled := stalledSnapshot()
	stalledSrv, _ := tourneySrv(t, &fakeTourney{snapshot: stalled, live: fake.live})
	_, _, body = doShell(t, c, http.MethodGet, stalledSrv.URL+"/tourney/run/7", "", nil)
	if strings.Contains(body, `class="liveboards"`) || strings.Contains(body, "miniboard") {
		t.Errorf("non-running run page carries the live section (body %s)", body)
	}
}

func TestTourneyRunStatesAndNotFound(t *testing.T) {
	srv, _ := tourneySrv(t, &fakeTourney{snapshot: liveSnapshot()})
	c := noRedirectClient(srv)

	// A garbage id and an unknown run are the room pages' 404 shape.
	for _, path := range []string{"/tourney/run/abc", "/tourney/run/0", "/tourney/run/7/board/x"} {
		status, _, _ := doShell(t, c, http.MethodGet, srv.URL+path, "", nil)
		if status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, status)
		}
	}
	unknown, _ := tourneySrv(t, &fakeTourney{snapErr: fmt.Errorf("tourney: run 99: %w", ErrNotFound)})
	status, _, body := doShell(t, c, http.MethodGet, unknown.URL+"/tourney/run/99", "", nil)
	if status != http.StatusNotFound || !strings.Contains(body, "not found") {
		t.Errorf("unknown run = %d %s, want the 404 page", status, body)
	}

	// A failed drive renders its state and the terminal error; a finished
	// run renders finished.
	failed := liveSnapshot()
	failed.Run.Running, failed.Run.Failure = false, "pairing 0: too slow"
	failedSrv, failedStore := tourneyMount(t, &fakeTourney{snapshot: failed})
	status, _, body = doShell(t, c, http.MethodGet, failedSrv.URL+"/tourney/run/7", "", nil)
	if status != http.StatusOK {
		t.Fatalf("failed run: status = %d", status)
	}
	wantShellBody(t, body, ">failed<", "run failed: pairing 0: too slow")
	// The failed drive's ongoing row holds the run gate with no live drive:
	// the admin's close form is the escape, the member's page stays
	// form-free.
	_, _, body = doShell(t, c, http.MethodGet, failedSrv.URL+"/tourney/run/7", mintAdminSession(t, failedStore), nil)
	wantShellBody(t, body, `action="/tourney/run/7/close"`, "undriven run")
	memberTok := mintSession(t, failedStore, seedUser(t, failedStore, "carol"))
	_, _, body = doShell(t, c, http.MethodGet, failedSrv.URL+"/tourney/run/7", memberTok, nil)
	if strings.Contains(body, "/close") {
		t.Error("member page carries the close form on a failed run, want the admin's alone")
	}

	done := liveSnapshot()
	done.Run.Running, done.Run.Finished = false, true
	doneSrv, _ := tourneySrv(t, &fakeTourney{snapshot: done})
	_, _, body = doShell(t, c, http.MethodGet, doneSrv.URL+"/tourney/run/7", "", nil)
	if !strings.Contains(body, ">finished<") {
		t.Errorf("finished run body misses its state (body %s)", body)
	}

	// A read failure behind the service is an outage.
	errSrv, _ := tourneySrv(t, &fakeTourney{snapErr: errors.New("boom")})
	status, _, _ = doShell(t, c, http.MethodGet, errSrv.URL+"/tourney/run/7", "", nil)
	if status != http.StatusInternalServerError {
		t.Errorf("snapshot failure = %d, want 500", status)
	}
}

// TestTourneyStartBlockedByOngoingRun pins the run gate's page shape: the
// start handler renders the blocking run's id inline instead of a bare
// outage.
func TestTourneyStartBlockedByOngoingRun(t *testing.T) {
	fake := &fakeTourney{startErr: &TourneyBlockedError{RunID: 4}}
	srv, store := tourneyMount(t, fake)
	token := mintAdminSession(t, store)

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney", token, url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
		"rating":   {strconv.Itoa(config.TournamentStartRating)},
		"parallel": {"1"}, "count": {"2"},
		"name0": {"a"}, "tier0": {config.TierEasy.Name},
		"name1": {"b"}, "tier1": {config.TierEasy.Name},
	})
	if status != http.StatusConflict {
		t.Fatalf("blocked start = %d, want 409 (body %s)", status, body)
	}
	if !strings.Contains(body, `class="error"`) || !strings.Contains(body, "run 4") {
		t.Errorf("blocked start body misses the blocking run id inline (body %s)", body)
	}
	if len(fake.started) != 1 {
		t.Errorf("start calls = %d, want 1", len(fake.started))
	}
}

// stalledSnapshot is a run no process drives: ongoing row, no drive, no
// failure, one settled pairing.
func stalledSnapshot() TourneySnapshot {
	snap := liveSnapshot()
	winner := 0
	snap.Run.Running, snap.Run.Failure = false, ""
	snap.Series = []TourneySeriesLine{
		{PairingSlot: 0, RedFirstSlot: 0, BlueFirstSlot: 1, RedFirstWins: 2, BlueFirstWins: 0, WinnerSlot: &winner, Finished: true},
		{PairingSlot: 1, RedFirstSlot: 1, BlueFirstSlot: 0},
	}
	return snap
}

// TestTourneyRunCloseForm pins the stalled state's close surface: the form
// renders on the stalled page for the admin only, guests and members are
// refused without touching the service, success bounces back onto the run
// page, and a refusal renders inline.
func TestTourneyRunCloseForm(t *testing.T) {
	fake := &fakeTourney{snapshot: stalledSnapshot()}
	srv, store := tourneyMount(t, fake)
	token := mintAdminSession(t, store)
	c := noRedirectClient(srv)
	form := url.Values{}

	// The form rides the stalled page for the admin, not for a member or a
	// guest, and not on the other run states.
	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/7", token, nil)
	if status != http.StatusOK {
		t.Fatalf("stalled run page: status = %d", status)
	}
	wantShellBody(t, body, `action="/tourney/run/7/close"`, ">stalled<")
	member := mintSession(t, store, seedUser(t, store, "alice"))
	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/7", member, nil)
	if strings.Contains(body, "/close") {
		t.Error("member page carries the close form, want the admin's alone")
	}
	_, _, body = doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/7", "", nil)
	if strings.Contains(body, "/close") {
		t.Error("guest page carries the close form, want the admin's alone")
	}
	live := &fakeTourney{snapshot: liveSnapshot()}
	liveSrv, _ := tourneySrv(t, live)
	status, _, body = doShell(t, c, http.MethodGet, liveSrv.URL+"/tourney/run/7", token, nil)
	if status != http.StatusOK || strings.Contains(body, "/close") {
		t.Errorf("running run page carries the close form (status %d)", status)
	}

	// The POST bounces guests like every acting route.
	status, h, _ := doShell(t, c, http.MethodPost, srv.URL+"/tourney/run/7/close", "", form)
	if status != http.StatusSeeOther || h.Get("Location") != "/login" {
		t.Errorf("guest close = %d %q, want 303 /login", status, h.Get("Location"))
	}
	if len(fake.closed) != 0 {
		t.Fatalf("guest close reached the service %d times, want 0", len(fake.closed))
	}

	// A member's close renders the refusal inline and never reaches the
	// service.
	status, _, body = doShell(t, c, http.MethodPost, srv.URL+"/tourney/run/7/close", member, form)
	if status != http.StatusForbidden {
		t.Fatalf("member close = %d, want 403 (body %s)", status, body)
	}
	wantShellBody(t, body, "only the admin account closes runs")
	if len(fake.closed) != 0 {
		t.Errorf("member close reached the service %d times, want 0", len(fake.closed))
	}

	// The admin's close lands and bounces back onto the run page.
	status, h, _ = doShell(t, c, http.MethodPost, srv.URL+"/tourney/run/7/close", token, form)
	if status != http.StatusSeeOther || h.Get("Location") != "/tourney/run/7" {
		t.Errorf("admin close = %d %q, want 303 back onto the run page", status, h.Get("Location"))
	}
	if len(fake.closed) != 1 || fake.closed[0] != 7 {
		t.Errorf("closed = %v, want [7]", fake.closed)
	}

	// A refusal renders the reason inline on the run page.
	fake.closeErr = errors.New("tourney: run 7: the run's drive is still live")
	status, _, body = doShell(t, c, http.MethodPost, srv.URL+"/tourney/run/7/close", token, form)
	if status != http.StatusConflict {
		t.Fatalf("refused close = %d, want 409 (body %s)", status, body)
	}
	wantShellBody(t, body, `class="error"`, "still live")

	// An unknown run stays the 404 page.
	fake.closeErr = fmt.Errorf("tourney: run 7: %w", ErrNotFound)
	status, _, _ = doShell(t, c, http.MethodPost, srv.URL+"/tourney/run/7/close", token, form)
	if status != http.StatusNotFound {
		t.Errorf("close of an unknown run = %d, want 404", status)
	}
	// A garbage id never reaches the service.
	if code := func() int {
		status, _, _ = doShell(t, c, http.MethodPost, srv.URL+"/tourney/run/abc/close", token, form)
		return status
	}(); code != http.StatusNotFound {
		t.Errorf("close of a garbage id = %d, want 404", code)
	}
}

func TestTourneySetupListsRuns(t *testing.T) {
	const created = 1790970000
	fake := &fakeTourney{runSummaries: []TourneyRunSummary{{
		TourneyRunInfo: TourneyRunInfo{ID: 5, CreatedAt: created, TCIdx: 1,
			BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating, Finished: true},
		Seats: []TourneySeat{
			{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "hard-1", Tier: config.TierHard.Name},
		},
		Leader: "hard-1",
	}}}
	srv, token := tourneySrv(t, fake)

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney", token, nil)
	if status != http.StatusOK {
		t.Fatalf("setup: status = %d", status)
	}
	wantShellBody(t, body,
		`href="/tourney/run/5"`, `>run <span class="mono">5</span><`,
		time.Unix(created, 0).UTC().Format(historyTimeFormat),
		"2&#43;1", "bo3", ">finished<",
		"easy-1, hard-1", ">hard-1<",
	)

	// A guest reads the same runs list, form-free.
	_, _, body = doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney", "", nil)
	if !strings.Contains(body, `href="/tourney/run/5"`) {
		t.Errorf("guest page misses the runs list (body %s)", body)
	}
	if strings.Contains(body, `action="/tourney"`) {
		t.Error("guest page carries the setup form, want the runs list alone")
	}

	// A listing failure fails the setup page.
	bad, badTok := tourneySrv(t, &fakeTourney{runsErr: errors.New("boom")})
	status, _, _ = doShell(t, noRedirectClient(bad), http.MethodGet, bad.URL+"/tourney", badTok, nil)
	if status != http.StatusInternalServerError {
		t.Errorf("runs failure = %d, want 500", status)
	}
}
