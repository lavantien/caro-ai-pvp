package server

// The M6b SSE spike surface tests: /static serves the vendored htmx 4
// artifacts byte-exact with pinned content and cache headers and nothing
// else (no listing, no missing file), and /spike renders the scratch room
// view with both push wirings without any session.

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// doGet runs one plain GET (no cookie, no body) and returns status, header,
// and the drained body.
func doGet(t *testing.T, c *http.Client, url string) (int, http.Header, string) {
	t.Helper()
	resp, err := c.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatalf("read GET %s: %v", url, err)
	}
	return resp.StatusCode, resp.Header, string(body)
}

func TestStaticServesVendoredArtifacts(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()

	for name, contentType := range map[string]string{
		"htmx.min.js":   "text/javascript; charset=utf-8",
		"hx-sse.min.js": "text/javascript; charset=utf-8",
		"LICENSE.htmx":  "text/plain; charset=utf-8",
	} {
		want, err := fs.ReadFile(staticFS, name)
		if err != nil {
			t.Fatalf("read embedded %s: %v", name, err)
		}
		status, h, body := doGet(t, srv.Client(), srv.URL+"/static/"+name)
		if status != http.StatusOK {
			t.Errorf("%s: status = %d, want 200", name, status)
		}
		if got := h.Get("Content-Type"); got != contentType {
			t.Errorf("%s: Content-Type = %q, want %q", name, got, contentType)
		}
		if got := h.Get("Cache-Control"); got != staticCacheControl {
			t.Errorf("%s: Cache-Control = %q, want %q", name, got, staticCacheControl)
		}
		if body != string(want) {
			t.Errorf("%s: served body differs from the embedded artifact (%d vs %d bytes)",
				name, len(body), len(want))
		}
	}
}

func TestStaticRefusesNonFiles(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()

	for _, path := range []string{"/static/", "/static/nope.js", "/static/../go.mod", "/static/web"} {
		status, _, _ := doGet(t, srv.Client(), srv.URL+path)
		if status != http.StatusNotFound {
			t.Errorf("GET %s: status = %d, want 404", path, status)
		}
	}
}

func TestSpikePageServesBothWiringsWithoutSession(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()

	status, h, body := doGet(t, srv.Client(), srv.URL+"/spike")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", status, body)
	}
	if got := h.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html", got)
	}
	if got := h.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	// Both wirings present, aimed at the hardcoded default room.
	for _, want := range []string{
		`id="htmx-list"`, `id="es-list"`,
		`id="htmx-count"`, `id="es-count"`,
		`hx-sse:connect="/api/rooms/` + spikeDefaultRoomID + `/events"`,
		"new EventSource",
		`src="/static/htmx.min.js"`, `src="/static/hx-sse.min.js"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("spike body misses %q", want)
		}
	}
	// Every wire event kind has its htmx handler: the literal hx-on
	// attributes and the kinds slice must not drift apart.
	for _, kind := range spikeEventKinds {
		if attr := `hx-on:` + kind + `="spikeEvent('htmx', '` + kind + `', event.detail.data)"`; !strings.Contains(body, attr) {
			t.Errorf("spike body misses the hx-on handler %q", attr)
		}
	}
	// html/template renders ZgotmplZ where an unsafe interpolation landed;
	// any occurrence means the wiring markup was silently mangled.
	if strings.Contains(body, "ZgotmplZ") {
		t.Error("spike body contains ZgotmplZ, an html/template escape marker")
	}
}

func TestSpikeRoomParamTakesValidRoomIDOnly(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	room, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	// A real room id steers both streams.
	_, _, body := doGet(t, srv.Client(), srv.URL+"/spike?room="+room.ID())
	for _, want := range []string{
		`hx-sse:connect="/api/rooms/` + room.ID() + `/events"`,
		"/api/rooms/" + room.ID() + "/events",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body misses %q", want)
		}
	}

	// Anything that is not a 32-char lowercase hex room id falls back to the
	// hardcoded default, so the query string cannot inject markup.
	for _, bad := range []string{
		"<script>alert(1)</script>",
		"short",
		strings.Repeat("A", 2*roomIDBytes),
		strings.Repeat("g", 2*roomIDBytes),
	} {
		_, _, body := doGet(t, srv.Client(), srv.URL+"/spike?room="+bad)
		if strings.Contains(body, bad) {
			t.Errorf("room=%q leaked into the page", bad)
		}
		if !strings.Contains(body, `hx-sse:connect="/api/rooms/`+spikeDefaultRoomID+`/events"`) {
			t.Errorf("room=%q did not fall back to %q", bad, spikeDefaultRoomID)
		}
	}
}

func TestSpikeListsRoomsWithoutSession(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()
	alice := seedUser(t, s.store, "alice")
	room, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	status, _, body := doGet(t, srv.Client(), srv.URL+"/spike")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want 200", status)
	}
	if !strings.Contains(body, `href="/spike?room=`+room.ID()+`"`) {
		t.Errorf("spike body misses the room listing link for %s", room.ID())
	}
}

// TestSpikeScratchRoomSettingsArePlayable pins the one-click room's settings
// against the config hub: the page would render fine and then 400 on click
// if the tier name or series length drifted from what a room accepts.
func TestSpikeScratchRoomSettingsArePlayable(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	if _, err := s.rm.Create(alice.ID, spikeScratchTCIdx, spikeScratchBOLen, tierByName(spikeScratchBot)); err != nil {
		t.Fatalf("scratch room settings rejected: %v", err)
	}
}
