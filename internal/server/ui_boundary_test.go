package server

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestTourneyStartRatingBoundaryAccepted(t *testing.T) {
	for _, rating := range []int{config.TournamentStartRatingAbsMax, -config.TournamentStartRatingAbsMax} {
		fake := &fakeTourney{startID: 7}
		srv, store := tourneyMount(t, fake)
		token := mintAdminSession(t, store)
		form := url.Values{
			"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
			"rating":   {strconv.Itoa(rating)},
			"parallel": {"1"}, "count": {"2"},
			"name0": {"a"}, "tier0": {config.TierEasy.Name},
			"name1": {"b"}, "tier1": {config.TierEasy.Name},
		}
		status, h, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney", token, form)
		if status != http.StatusSeeOther {
			t.Fatalf("rating %d: status = %d, want 303 (body %s)", rating, status, body)
		}
		if loc := h.Get("Location"); loc != "/tourney/run/7" {
			t.Errorf("rating %d: redirect = %q, want /tourney/run/7", rating, loc)
		}
		if got := fake.started[0].StartRating; got != rating {
			t.Errorf("rating %d reached the service as %d", rating, got)
		}
	}
}

func TestTourneyStartRedirectFormatsDecimalRunID(t *testing.T) {
	fake := &fakeTourney{startID: 12}
	srv, store := tourneyMount(t, fake)
	token := mintAdminSession(t, store)
	form := url.Values{
		"tc": {"0"}, "bo": {strconv.Itoa(config.SeriesBO3)},
		"rating":   {strconv.Itoa(config.TournamentStartRating)},
		"parallel": {"1"}, "count": {"2"},
		"name0": {"a"}, "tier0": {config.TierEasy.Name},
		"name1": {"b"}, "tier1": {config.TierEasy.Name},
	}
	status, h, body := doShell(t, noRedirectClient(srv), http.MethodPost, srv.URL+"/tourney", token, form)
	if status != http.StatusSeeOther {
		t.Fatalf("start: status = %d, want 303 (body %s)", status, body)
	}
	if loc := h.Get("Location"); loc != "/tourney/run/12" {
		t.Errorf("start redirect = %q, want /tourney/run/12 in decimal", loc)
	}
}

func TestTourneyRunIDOneRenders(t *testing.T) {
	srv, _ := tourneySrv(t, &fakeTourney{snapshot: liveSnapshot()})
	status, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney/run/1", "", nil)
	if status != http.StatusOK {
		t.Fatalf("run 1: status = %d, want 200 (body %s)", status, body)
	}
}

func TestTourneyRunHeaderRendersMinuteBoundaryCreatedAt(t *testing.T) {
	snap := liveSnapshot()
	snap.Run.CreatedAt = 600
	srv, _ := tourneySrv(t, &fakeTourney{snapshot: snap})
	_, _, body := doShell(t, noRedirectClient(srv), http.MethodGet, srv.URL+"/tourney/run/1", "", nil)
	if want := time.Unix(600, 0).UTC().Format(historyTimeFormat); !strings.Contains(body, want) {
		t.Errorf("run header time missing %q (body %s)", want, body)
	}
}

func TestWriteQueueDefaultApplyTimeoutFromConfig(t *testing.T) {
	q := NewWriteQueue(nil)
	defer q.Close()
	want := time.Duration(config.WriteQueueApplyTimeoutMs) * time.Millisecond
	if q.ApplyTimeout != want {
		t.Errorf("ApplyTimeout = %v, want %v from the config hub", q.ApplyTimeout, want)
	}
}
