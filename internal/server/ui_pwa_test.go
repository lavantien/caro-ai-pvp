package server

// The v0.27 PWA install shell tests: the manifest parses with the required
// install fields over the committed design tokens, the service worker serves
// with update semantics and its source contract pins the cache boundary (no
// respondWith is reachable for /api/ or the room SSE event stream), the
// offline fallback renders on the shared stylesheet, the icons serve through
// the static mount as decodable PNG files, and every page head carries the
// install wiring. Everything here rides the real shell mux and template
// set; nothing constructs searchers or plays games.

import (
	"bytes"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pwaThemeColor is the manifest's theme and background fill: the committed
// --background anchor of docs/design-system.md. Change them together.
const pwaThemeColor = "#101318"

func TestPWAManifestServesInstallFields(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()

	status, h, body := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/manifest.webmanifest", "", nil)
	if status != http.StatusOK {
		t.Fatalf("manifest: status = %d, want 200 (body %s)", status, body)
	}
	if got := h.Get("Content-Type"); got != "application/manifest+json" {
		t.Errorf("manifest Content-Type = %q", got)
	}
	if got := h.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("manifest Cache-Control = %q, want no-cache", got)
	}
	var m struct {
		Name            string `json:"name"`
		StartURL        string `json:"start_url"`
		Display         string `json:"display"`
		ThemeColor      string `json:"theme_color"`
		BackgroundColor string `json:"background_color"`
		Icons           []struct {
			Src     string `json:"src"`
			Sizes   string `json:"sizes"`
			Type    string `json:"type"`
			Purpose string `json:"purpose"`
		} `json:"icons"`
	}
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("manifest does not parse as JSON: %v (body %s)", err, body)
	}
	if m.Name != "caro" {
		t.Errorf("manifest name = %q, want caro", m.Name)
	}
	if m.StartURL != "/" {
		t.Errorf("manifest start_url = %q, want /", m.StartURL)
	}
	if m.Display != "standalone" {
		t.Errorf("manifest display = %q, want standalone", m.Display)
	}
	if m.ThemeColor != pwaThemeColor || m.BackgroundColor != pwaThemeColor {
		t.Errorf("manifest colors = %q / %q, want both %s",
			m.ThemeColor, m.BackgroundColor, pwaThemeColor)
	}
	var has192, has512, hasMaskable bool
	for _, icon := range m.Icons {
		if !strings.HasPrefix(icon.Src, "/static/icons/") {
			t.Errorf("icon src %q leaves the static icons dir", icon.Src)
		}
		if icon.Type != "image/png" {
			t.Errorf("icon %s type = %q, want image/png", icon.Src, icon.Type)
		}
		switch {
		case icon.Sizes == "192x192":
			has192 = true
		case icon.Sizes == "512x512" && icon.Purpose == "":
			has512 = true
		case icon.Sizes == "512x512" && strings.Contains(icon.Purpose, "maskable"):
			hasMaskable = true
		}
	}
	if !has192 || !has512 || !hasMaskable {
		t.Errorf("manifest icons miss a required size (192 %t, 512 %t, maskable %t)",
			has192, has512, hasMaskable)
	}
}

func TestPWAServiceWorkerServesWithUpdateSemantics(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()

	status, h, body := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/sw.js", "", nil)
	if status != http.StatusOK {
		t.Fatalf("sw.js: status = %d, want 200 (body %s)", status, body)
	}
	if got := h.Get("Content-Type"); got != "text/javascript; charset=utf-8" {
		t.Errorf("sw.js Content-Type = %q", got)
	}
	// The browser checks the worker script itself for updates, so it must
	// revalidate every fetch, never ride a shared cache window.
	if got := h.Get("Cache-Control"); got != "no-cache" {
		t.Errorf("sw.js Cache-Control = %q, want no-cache", got)
	}
	for _, want := range []string{
		"const CACHE = 'caro-static-v2'",
		"'/shell.css'",
		"'/static/htmx.min.js'",
		"'/static/hx-sse.min.js'",
		"'/static/room.js'",
		"'/static/playback.js'",
		"'/static/favicon.svg'",
		"'/offline.html'",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sw.js misses allowlist entry %q", want)
		}
	}
}

// TestPWAServiceWorkerBoundary pins the worker's boundary law textually, the
// only enforcement a source contract can carry: the /api/ pass-through guard
// (naming the SSE event stream) must precede every respondWith in the file,
// the worker must hold exactly the two intended respondWith calls, the
// offline fallback and the exact-match allowlist check must stay named, both
// branches must fall back to the network on a cache miss (r || fetch), and
// the precache must wrap every asset in a cache-reload Request so a bumped
// cache never bakes stale bytes from the browser HTTP cache.
func TestPWAServiceWorkerBoundary(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()

	_, _, body := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/sw.js", "", nil)
	const guard = "url.pathname.indexOf(API_PREFIX) === 0"
	guardAt := strings.Index(body, guard)
	if guardAt < 0 {
		t.Fatalf("sw.js loses the /api/ pass-through guard %q", guard)
	}
	if i := strings.Index(body, "const API_PREFIX = '/api/'"); i < 0 || i > guardAt {
		t.Errorf("sw.js does not define API_PREFIX as '/api/' before the guard")
	}
	if !strings.Contains(body, "/api/rooms/{id}/events") {
		t.Error("sw.js does not name the SSE event stream route in its boundary")
	}
	responds := 0
	for pos := 0; ; {
		i := strings.Index(body[pos:], "e.respondWith(")
		if i < 0 {
			break
		}
		responds++
		if at := pos + i; at < guardAt {
			t.Errorf("respondWith #%d at byte %d precedes the /api/ guard at %d",
				responds, at, guardAt)
		}
		pos += i + len("e.respondWith(")
	}
	if responds != 2 {
		t.Errorf("sw.js carries %d respondWith calls, want exactly 2 (navigation fallback, allowlist)", responds)
	}
	for _, want := range []string{
		"caches.match(OFFLINE)",
		"ASSETS.indexOf(url.pathname) !== -1",
		"caches.match(e.request)",
		"r || fetch(e.request)",
		"r || fetch(OFFLINE)",
		"c.addAll(ASSETS.map(",
		"new Request(u, {cache: 'reload'})",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("sw.js misses the pinned boundary string %q", want)
		}
	}
}

func TestPWAOfflinePageServesOnSharedTokens(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()

	status, h, body := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/offline.html", "", nil)
	if status != http.StatusOK {
		t.Fatalf("offline.html: status = %d, want 200 (body %s)", status, body)
	}
	if got := h.Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Errorf("offline.html Content-Type = %q", got)
	}
	for _, want := range []string{
		`href="/shell.css"`,
		`href="/"`,
		"unreachable",
		"needs a connection",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("offline.html misses %q", want)
		}
	}
}

// TestPWAIconsServeDecodableAtPinnedSizes drives the static mount: every
// icon the manifest and the worker allowlist name must serve at its URL as
// a PNG of the exact rasterized size.
func TestPWAIconsServeDecodableAtPinnedSizes(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewHTTPAPI(s.store, s.rm))
	defer srv.Close()

	for name, size := range map[string]int{
		"icon-192.png":          192,
		"icon-512.png":          512,
		"icon-512-maskable.png": 512,
		"apple-touch-icon.png":  180,
	} {
		status, h, body := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/static/icons/"+name, "", nil)
		if status != http.StatusOK {
			t.Errorf("icon %s: status = %d, want 200", name, status)
			continue
		}
		if got := h.Get("Content-Type"); got != "image/png" {
			t.Errorf("icon %s Content-Type = %q, want image/png", name, got)
		}
		img, err := png.Decode(strings.NewReader(body))
		if err != nil {
			t.Errorf("icon %s does not decode as PNG: %v", name, err)
			continue
		}
		b := img.Bounds()
		if b.Dx() != size || b.Dy() != size {
			t.Errorf("icon %s bounds = %dx%d, want %dx%d", name, b.Dx(), b.Dy(), size, size)
		}
	}
}

// TestPWABaseHeadWiring pins the install wiring every shell page carries:
// the manifest link, the theme-color meta over the committed token, the
// apple touch icon and meta set, the registration script, and the hidden
// header install action the script surfaces.
func TestPWABaseHeadWiring(t *testing.T) {
	s := newStack(t)
	srv := httptest.NewServer(NewShellPages(s.store, s.rm, nil))
	defer srv.Close()

	status, _, body := doShell(t, srv.Client(), http.MethodGet, srv.URL+"/login", "", nil)
	if status != http.StatusOK {
		t.Fatalf("login page: status = %d", status)
	}
	for _, want := range []string{
		`<link rel="manifest" href="/manifest.webmanifest">`,
		`<meta name="theme-color" content="` + pwaThemeColor + `">`,
		`<link rel="apple-touch-icon" href="/static/icons/apple-touch-icon.png">`,
		`<meta name="mobile-web-app-capable" content="yes">`,
		`<meta name="apple-mobile-web-app-capable" content="yes">`,
		`<meta name="apple-mobile-web-app-title" content="caro">`,
		`<script src="/static/pwa.js" defer></script>`,
		`<button type="button" id="install-btn" class="btn-quiet" hidden>install</button>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("base head wiring misses %q", want)
		}
	}
}

// TestPWARoomAndPlaybackHeadWiring renders the two standalone documents
// directly and pins the same install wiring on them.
func TestPWARoomAndPlaybackHeadWiring(t *testing.T) {
	wiring := []string{
		`<link rel="manifest" href="/manifest.webmanifest">`,
		`<meta name="theme-color" content="` + pwaThemeColor + `">`,
		`<link rel="apple-touch-icon" href="/static/icons/apple-touch-icon.png">`,
		`<meta name="mobile-web-app-capable" content="yes">`,
		`<script src="/static/pwa.js" defer></script>`,
	}
	var out bytes.Buffer
	if err := executeRoom(&out, roomView{RoomID: strings.Repeat("a", 2*roomIDBytes),
		TCLabel: "1+0", BOLen: 3, HostName: "h", GuestName: "g", State: "created",
		KindsCSV: "move"}); err != nil {
		t.Fatalf("render room: %v", err)
	}
	for _, want := range wiring {
		if !strings.Contains(out.String(), want) {
			t.Errorf("room.html head wiring misses %q", want)
		}
	}
	out.Reset()
	if err := executePlayback(&out, playbackView{GameID: 1, RedName: "r", BlueName: "b"}); err != nil {
		t.Fatalf("render playback: %v", err)
	}
	for _, want := range wiring {
		if !strings.Contains(out.String(), want) {
			t.Errorf("playback.html head wiring misses %q", want)
		}
	}
}
