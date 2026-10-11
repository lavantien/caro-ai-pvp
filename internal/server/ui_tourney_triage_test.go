package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func tourneyTriageSelectRegion(t *testing.T, body, name string) string {
	t.Helper()
	open := `<select name="` + name + `">`
	start := strings.Index(body, open)
	if start < 0 {
		t.Fatalf("body misses the %s select (body %s)", name, body)
	}
	start += len(open)
	end := strings.Index(body[start:], "</select>")
	if end < 0 {
		t.Fatalf("the %s select never closes (body %s)", name, body)
	}
	return body[start : start+end]
}

func tourneyTriageOption(v int, selected bool) string {
	mark := ""
	if selected {
		mark = " selected"
	}
	s := strconv.Itoa(v)
	return `<option value="` + s + `"` + mark + `>` + s + `</option>`
}

func tourneyTriageBaseForm() url.Values {
	return url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
		"rating":   {strconv.Itoa(config.TournamentStartRating)},
		"parallel": {"1"}, "count": {"2"},
		"name0": {"a"}, "tier0": {config.TierEasy.Name},
		"name1": {"b"}, "tier1": {config.TierEasy.Name},
	}
}

func TestTourneyBlockedErrorNamesRunIDInDecimal(t *testing.T) {
	for _, row := range []struct {
		id   int64
		want string
	}{
		{9, "run 9 is still in progress, close it before starting another"},
		{19, "run 19 is still in progress, close it before starting another"},
	} {
		if got := (&TourneyBlockedError{RunID: row.id}).Error(); got != row.want {
			t.Errorf("run %d blocked error = %q, want %q", row.id, got, row.want)
		}
	}
}

func TestTourneySetupSelectsRenderExactOptionLists(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	_, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney",
		mintAdminSession(t, store), nil)

	parallel := tourneyTriageSelectRegion(t, body, "parallel")
	if n := strings.Count(parallel, "<option"); n != config.TournamentParallelMatches {
		t.Errorf("parallel options = %d, want %d (region %s)", n, config.TournamentParallelMatches, parallel)
	}
	if strings.Contains(parallel, `value="0"`) {
		t.Errorf("parallel select carries a zero option (region %s)", parallel)
	}
	for v := 1; v <= config.TournamentParallelMatches; v++ {
		if want := tourneyTriageOption(v, v == config.TournamentParallelMatches); !strings.Contains(parallel, want) {
			t.Errorf("parallel select misses %q (region %s)", want, parallel)
		}
	}

	count := tourneyTriageSelectRegion(t, body, "count")
	if n := strings.Count(count, "<option"); n != config.TournamentMaxParticipants-1 {
		t.Errorf("count options = %d, want %d (region %s)", n, config.TournamentMaxParticipants-1, count)
	}
	if strings.Contains(count, `value="0"`) || strings.Contains(count, `value="1"`) {
		t.Errorf("count select carries a phantom or sub-2 option (region %s)", count)
	}
	for v := 2; v <= config.TournamentMaxParticipants; v++ {
		if want := tourneyTriageOption(v, v == config.TournamentMaxParticipants); !strings.Contains(count, want) {
			t.Errorf("count select misses %q (region %s)", want, count)
		}
	}
}

func TestTourneyStartReRenderKeepsPostedTier(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	form := tourneyTriageBaseForm()
	form.Set("tc", "abc")
	form.Set("tier0", config.TierMedium.Name)

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney",
		mintAdminSession(t, store), form)
	if status != http.StatusBadRequest {
		t.Fatalf("re-render: status = %d, want 400 (body %s)", status, body)
	}
	posted := `<option value="` + config.TierMedium.Name + `" selected>AI ` + config.TierMedium.Name + `</option>`
	if n := strings.Count(body, posted); n != config.InstancesPerTier+1 {
		t.Errorf("posted tier selected on %d rows, want %d (body %s)", n, config.InstancesPerTier+1, body)
	}
	defaulted := `<option value="` + config.TierEasy.Name + `" selected>AI ` + config.TierEasy.Name + `</option>`
	if n := strings.Count(body, defaulted); n != config.InstancesPerTier-1 {
		t.Errorf("row 0 default tier still selected on %d rows, want %d (body %s)", n, config.InstancesPerTier-1, body)
	}
}

func TestTourneyStartReRenderKeepsParsedRating(t *testing.T) {
	srv, store := tourneyMount(t, &fakeTourney{})
	form := tourneyTriageBaseForm()
	form.Set("tc", "abc")
	rating := config.TournamentStartRating + 5
	form.Set("rating", strconv.Itoa(rating))

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney",
		mintAdminSession(t, store), form)
	if status != http.StatusBadRequest {
		t.Fatalf("re-render: status = %d, want 400 (body %s)", status, body)
	}
	want := `<input name="rating" type="number" value="` + strconv.Itoa(rating) + `">`
	if !strings.Contains(body, want) {
		t.Errorf("re-rendered rating input misses the posted value %q (body %s)", want, body)
	}
}

func TestTourneyStartReRenderKeepsExistingRuns(t *testing.T) {
	fake := &fakeTourney{runSummaries: []TourneyRunSummary{{
		TourneyRunInfo: TourneyRunInfo{ID: 5, CreatedAt: 1790970000, TCIdx: 1,
			BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating, Finished: true},
		Seats: []TourneySeat{
			{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "hard-1", Tier: config.TierHard.Name},
		},
		Leader: "hard-1",
	}}}
	srv, store := tourneyMount(t, fake)
	form := tourneyTriageBaseForm()
	form.Set("tc", "abc")

	status, _, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney",
		mintAdminSession(t, store), form)
	if status != http.StatusBadRequest {
		t.Fatalf("re-render: status = %d, want 400 (body %s)", status, body)
	}
	if !strings.Contains(body, `href="/tourney/run/5"`) {
		t.Errorf("error re-render drops the existing run line (body %s)", body)
	}
	if strings.Contains(body, "no tournaments yet.") {
		t.Errorf("error re-render claims there are no runs (body %s)", body)
	}
}

func TestTourneyStartBoundaryTable(t *testing.T) {
	fake := &fakeTourney{startID: 9}
	srv, store := tourneyMount(t, fake)
	token := mintAdminSession(t, store)
	c := noRedirectClient(srv)

	badBO := 1
	for slices.Contains(config.SeriesLengths[:], badBO) {
		badBO++
	}
	rejects := []struct {
		name string
		set  func(url.Values)
		want string
	}{
		{"tc not numeric", func(f url.Values) { f.Set("tc", "abc") }, "pick a time control from the list"},
		{"tc negative", func(f url.Values) { f.Set("tc", "-1") }, "pick a time control from the list"},
		{"tc past the list", func(f url.Values) {
			f.Set("tc", strconv.Itoa(len(config.TimeControls)))
		}, "pick a time control from the list"},
		{"bo not numeric", func(f url.Values) { f.Set("bo", "abc") }, "pick a best of from the list"},
		{"bo off the list", func(f url.Values) { f.Set("bo", strconv.Itoa(badBO)) }, "pick a best of from the list"},
		{"rating not numeric", func(f url.Values) { f.Set("rating", "abc") }, "start rating must sit within"},
		{"rating above the cap", func(f url.Values) {
			f.Set("rating", strconv.Itoa(config.TournamentStartRatingAbsMax+1))
		}, "start rating must sit within"},
		{"rating below the cap", func(f url.Values) {
			f.Set("rating", strconv.Itoa(-config.TournamentStartRatingAbsMax-1))
		}, "start rating must sit within"},
		{"parallel not numeric", func(f url.Values) { f.Set("parallel", "abc") }, "parallel must sit between 1 and"},
		{"parallel zero", func(f url.Values) { f.Set("parallel", "0") }, "parallel must sit between 1 and"},
		{"parallel past the cap", func(f url.Values) {
			f.Set("parallel", strconv.Itoa(config.TournamentParallelMatches+1))
		}, "parallel must sit between 1 and"},
		{"count not numeric", func(f url.Values) { f.Set("count", "abc") }, "participants must sit between 2 and"},
		{"count one", func(f url.Values) { f.Set("count", "1") }, "participants must sit between 2 and"},
		{"count past the cap", func(f url.Values) {
			f.Set("count", strconv.Itoa(config.TournamentMaxParticipants+1))
		}, "participants must sit between 2 and"},
		{"seat 1 name empty", func(f url.Values) { f.Set("name0", "") }, "bot 1 needs a name"},
		{"seat 1 tier missing", func(f url.Values) { f.Set("tier0", "") }, "bot 1 needs a tier from the list"},
	}
	for _, row := range rejects {
		form := tourneyTriageBaseForm()
		row.set(form)
		status, _, body := doShell(t, c, http.MethodPost, srv.URL+"/tourney", token, form)
		if status != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400 (body %s)", row.name, status, body)
			continue
		}
		if !strings.Contains(body, row.want) {
			t.Errorf("%s: body misses %q (body %s)", row.name, row.want, body)
		}
		if len(fake.started) != 0 {
			t.Fatalf("%s: StartRun reached the service with garbage (%+v)", row.name, fake.started)
		}
	}

	parallelCapForm := tourneyTriageBaseForm()
	parallelCapForm.Set("parallel", strconv.Itoa(config.TournamentParallelMatches))
	nameCapForm := tourneyTriageBaseForm()
	nameCapForm.Set("name0", strings.Repeat("a", config.TournamentNameMaxBytes))
	budgetEdgeForm := tourneyTriageBaseForm()
	budgetEdgeForm.Set("tier0", config.TierGrandmaster.Name)
	budgetEdgeForm.Set("tier1", config.TierGrandmaster.Name)
	maxCountForm := tourneyTriageBaseForm()
	maxCountForm.Set("count", strconv.Itoa(config.TournamentMaxParticipants))
	for i := range config.DefaultRoster() {
		maxCountForm.Set("name"+strconv.Itoa(i), "bot"+strconv.Itoa(i))
		maxCountForm.Set("tier"+strconv.Itoa(i), config.TierEasy.Name)
	}
	accepts := []struct {
		name string
		form url.Values
	}{
		{"parallel at the cap", parallelCapForm},
		{"count at the cap", maxCountForm},
		{"name at the byte cap", nameCapForm},
		{"cores exactly at the machine budget", budgetEdgeForm},
	}
	for i, row := range accepts {
		status, h, body := doShell(t, c, http.MethodPost, srv.URL+"/tourney", token, row.form)
		if status != http.StatusSeeOther {
			t.Errorf("%s: status = %d, want 303 (body %s)", row.name, status, body)
			continue
		}
		if loc := h.Get("Location"); loc != "/tourney/run/9" {
			t.Errorf("%s: redirect = %q, want /tourney/run/9", row.name, loc)
		}
		if len(fake.started) != i+1 {
			t.Errorf("%s: StartRun calls = %d, want %d", row.name, len(fake.started), i+1)
		}
	}
}

func TestTourneyCloseRunRedirectDecimal(t *testing.T) {
	fake := &fakeTourney{snapshot: stalledSnapshot()}
	srv, store := tourneyMount(t, fake)

	status, h, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney/run/10/close",
		mintAdminSession(t, store), url.Values{})
	if status != http.StatusSeeOther {
		t.Fatalf("close: status = %d, want 303 (body %s)", status, body)
	}
	if loc := h.Get("Location"); loc != "/tourney/run/10" {
		t.Errorf("close redirect = %q, want /tourney/run/10", loc)
	}
	if len(fake.closed) != 1 || fake.closed[0] != 10 {
		t.Errorf("closed = %v, want [10]", fake.closed)
	}
}

func TestTourneyRunIDParseTable(t *testing.T) {
	srv, _ := tourneySrv(t, &fakeTourney{snapshot: liveSnapshot()})
	c := noRedirectClient(srv)

	status, _, body := doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/9", "", nil)
	if status != http.StatusOK {
		t.Errorf("GET run 9 = %d, want 200 (body %s)", status, body)
	}

	status, _, body = doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/a", "", nil)
	if status != http.StatusNotFound {
		t.Errorf("GET run a = %d, want 404 (body %s)", status, body)
	}
	if strings.Contains(body, `id="tourboard"`) || strings.Contains(body, "easy-1") {
		t.Errorf("base-11 id renders the run page under a decimal parse refusal (body %s)", body)
	}

	status, _, body = doShell(t, c, http.MethodGet, srv.URL+"/tourney/run/abc", "", nil)
	if status != http.StatusNotFound {
		t.Errorf("GET run abc = %d, want 404 (body %s)", status, body)
	}
	if !strings.Contains(body, "not found") {
		t.Errorf("garbage id misses the 404 page (body %s)", body)
	}
	if strings.Contains(body, `id="tourboard"`) || strings.Contains(body, "easy-1") {
		t.Errorf("garbage id renders the run page after the 404 (body %s)", body)
	}
}

func TestTourneySnapshotSingleErrorResponse(t *testing.T) {
	errSrv, _ := tourneySrv(t, &fakeTourney{snapErr: errors.New("boom")})
	status, _, body := doShell(t, noRedirectClient(errSrv), http.MethodGet, errSrv.URL+"/tourney/run/7", "", nil)
	if status != http.StatusInternalServerError {
		t.Errorf("snapshot failure = %d, want 500", status)
	}
	if body != "tournament read failed\n" {
		t.Errorf("snapshot failure body = %q, want the bare outage line", body)
	}

	missing, _ := tourneySrv(t, &fakeTourney{snapErr: fmt.Errorf("tourney: run 7: %w", ErrNotFound)})
	status, _, body = doShell(t, noRedirectClient(missing), http.MethodGet, missing.URL+"/tourney/run/7", "", nil)
	if status != http.StatusNotFound {
		t.Errorf("unknown run = %d, want 404", status)
	}
	if !strings.Contains(body, "not found") {
		t.Errorf("unknown run body misses the 404 page (body %s)", body)
	}
	if strings.Contains(body, `id="tourboard"`) {
		t.Errorf("unknown run body renders the run page after the 404 (body %s)", body)
	}
}
