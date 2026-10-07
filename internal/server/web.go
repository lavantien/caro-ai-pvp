package server

// The M6b SSE spike: the vendored htmx 4 assets behind /static and the
// /spike scratch page running one room stream through both push wirings
// side by side, the htmx 4 hx-sse extension and a plain EventSource, with
// per-list counters so a browser check reads at a glance. Nothing here
// changes the landed JSON/SSE handlers; the mux only gains the two mounts.

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// The vendored exact-version htmx 4 distribution (see web/static/README.md):
// core plus the separate hx-sse extension file htmx 4 ships in dist/ext.
//
//go:embed web/static
var webStatic embed.FS

// staticFS is the asset subtree the /static handler serves.
var staticFS = mustSubFS(webStatic, "web/static")

// mustSubFS narrows the embedded tree; it can only fail on a packaging bug,
// so it fails loudly at init instead of serving 404s at runtime.
func mustSubFS(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic("server: static asset dir: " + err.Error())
	}
	return sub
}

// Static asset knobs: exact content types (pinned here, not guessed from the
// OS mime table) and the cache window. The artifacts are version-pinned
// vendor files that change only on a re-vendor, so a day of shared caching
// is safe without versioned URLs.
const (
	staticPrefix       = "/static/"
	staticCacheControl = "public, max-age=86400"
)

// staticContentTypes maps every extension the vendor dir carries. Unknown
// extensions still serve, as octet-stream.
var staticContentTypes = map[string]string{
	".js":   "text/javascript; charset=utf-8",
	".htmx": "text/plain; charset=utf-8",
	".md":   "text/plain; charset=utf-8",
	".png":  "image/png",
	".svg":  "image/svg+xml",
}

// Spike page knobs: the watched room is the plan's hardcoded id unless the
// query names a real room-id shape, and the one-click scratch room plays the
// lowest tier over the shortest series so the browser pass produces events
// fast.
const (
	spikeRoomParam     = "room"
	spikeDefaultRoomID = "spike"
	spikeScratchTCIdx  = 0
	spikeScratchBOLen  = config.SeriesBO3
)

// spikeEventKinds is the wire's event vocabulary. It feeds the page's
// EventSource wiring; the htmx wiring names the same kinds in literal hx-on
// attributes, and the spike page test pins the two together.
var spikeEventKinds = []string{EventKindMove, EventKindMLine, EventKindGameEnd, EventKindSeries}

// spikeScratchBot is the tier of the one-click scratch room.
var spikeScratchBot = config.TierEasy.Name

// spikeView carries the rendered page: the watched room id, the live rooms
// grid, the event kinds, and the one-click scratch room settings.
type spikeView struct {
	RoomID       string
	Rooms        []RoomInfo
	KindsCSV     string
	ScratchBot   string
	ScratchTCIdx int
	ScratchBOLen int
}

var spikeTmpl = template.Must(template.New("spike").Parse(spikeHTML))

// handleStatic serves one vendored asset: exact-name lookup in the embedded
// directory, so directories, missing files, and nested paths all miss. The
// mux pattern already cleaned the request path.
func (a *apiServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, staticPrefix)
	data, err := fs.ReadFile(staticFS, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	contentType, ok := staticContentTypes[path.Ext(name)]
	if !ok {
		contentType = "application/octet-stream"
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", staticCacheControl)
	_, _ = w.Write(data)
}

// handleSpike renders the scratch page: the login-free rooms grid plus both
// stream wirings over the watched room. Guests get the page and both
// streams; only the acting buttons need a session.
func (a *apiServer) handleSpike(w http.ResponseWriter, r *http.Request) {
	roomID := spikeDefaultRoomID
	if q := r.URL.Query().Get(spikeRoomParam); isRoomID(q) {
		roomID = q
	}
	view := spikeView{
		RoomID:       roomID,
		Rooms:        a.rooms.List(),
		KindsCSV:     strings.Join(spikeEventKinds, ","),
		ScratchBot:   spikeScratchBot,
		ScratchTCIdx: spikeScratchTCIdx,
		ScratchBOLen: spikeScratchBOLen,
	}
	var buf bytes.Buffer
	if err := spikeTmpl.Execute(&buf, view); err != nil {
		http.Error(w, "spike render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(buf.Bytes())
}

// isRoomID reports whether s carries the live room id shape, the exact
// lowercase-hex encoding of newRoomID. The spike room param accepts nothing
// else, so a query string cannot inject markup into the page.
func isRoomID(s string) bool {
	if len(s) != 2*roomIDBytes {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// spikeHTML is the scratch page. The htmx wiring stays declarative (the
// hx-sse:connect attribute plus one hx-on handler per event kind, the
// htmx 4 consumption pattern since named SSE events dispatch as DOM
// events); the EventSource wiring and the setup/move calls ride one small
// script. Counters per list make the browser pass a glance check.
const spikeHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>caro SSE spike (M6b)</title>
<script src="/static/htmx.min.js"></script>
<script src="/static/hx-sse.min.js"></script>
<style>
:root { color-scheme: light dark }
body { font-family: system-ui, sans-serif; margin: 0 auto; max-width: 46rem; padding: .75rem }
section { margin: 1rem 0 }
h2 { font-size: 1rem; margin: .25rem 0 }
#feeds { display: grid; gap: 1rem }
@media (min-width: 44rem) { #feeds { grid-template-columns: 1fr 1fr } }
ul { list-style: none; margin: .25rem 0; padding: 0; max-height: 40vh; overflow-y: auto;
	border: 1px solid #8884; border-radius: .25rem }
li { font-family: ui-monospace, monospace; font-size: .85rem; padding: .1rem .4rem;
	border-bottom: 1px solid #8883 }
.status { font-family: ui-monospace, monospace; font-size: .85rem }
input { font: inherit; padding: .25rem }
button { font: inherit; padding: .25rem .75rem }
code { font-family: ui-monospace, monospace; word-break: break-all }
form { margin: .35rem 0 }
</style>
</head>
<body>
<h1>caro SSE spike (M6b)</h1>
<p>room <code id="room-id">{{.RoomID}}</code><br>
<span class="status" id="htmx-status">htmx: connecting</span><br>
<span class="status" id="es-status">es: connecting</span></p>

<section id="setup">
<h2>scratch setup (watching needs no login)</h2>
<form id="spike-login">
<input id="spike-user" placeholder="username" autocomplete="username">
<input id="spike-pass" type="password" placeholder="password" autocomplete="current-password">
<button>login / register</button>
</form>
<button id="spike-open" data-bot="{{.ScratchBot}}" data-tc="{{.ScratchTCIdx}}" data-bo="{{.ScratchBOLen}}">open scratch room vs bot</button>
<form id="spike-move">
<input id="spike-cell" placeholder="cell, e.g. H8" autocomplete="off">
<button>post move</button>
<span class="status" id="spike-move-status"></span>
</form>
<p class="status" id="spike-setup-status">login, open a room, the page reloads onto its stream</p>
</section>

<section id="rooms">
<h2>rooms (guest view)</h2>
{{if .Rooms}}<ul id="room-list">
{{range .Rooms}}<li><a href="/spike?room={{.ID}}">{{.ID}}</a> {{.State}} tc{{.TCIdx}} bo{{.BOLen}}{{if .VsBotTier}} vs {{.VsBotTier}}{{end}}</li>{{end}}
</ul>{{else}}<p>no live rooms</p>{{end}}
</section>

<div id="feeds">
<section>
<h2>a) htmx 4 hx-sse</h2>
<p>events: <span id="htmx-count">0</span></p>
<div id="htmx-feed"
hx-sse:connect="/api/rooms/{{.RoomID}}/events"
hx-config="sse.reconnectMaxAttempts:5"
hx-on:move="spikeEvent('htmx', 'move', event.detail.data)"
hx-on:mline="spikeEvent('htmx', 'mline', event.detail.data)"
hx-on:gameend="spikeEvent('htmx', 'gameend', event.detail.data)"
hx-on:series="spikeEvent('htmx', 'series', event.detail.data)"
data-kinds="{{.KindsCSV}}">
<ul id="htmx-list"></ul>
</div>
</section>
<section>
<h2>b) plain EventSource</h2>
<p>events: <span id="es-count">0</span></p>
<ul id="es-list"></ul>
</section>
</div>

<script>
(function () {
	'use strict';
	function $(id) { return document.getElementById(id); }
	var room = $('room-id').textContent;
	var counters = { htmx: 0, es: 0 };

	function spikeEvent(list, kind, data) {
		var ul = $(list + '-list');
		var li = document.createElement('li');
		li.textContent = kind + ' ' + data;
		ul.appendChild(li);
		counters[list] += 1;
		$(list + '-count').textContent = counters[list];
		while (ul.children.length > 50) ul.removeChild(ul.firstChild);
	}
	window.spikeEvent = spikeEvent;

	var es = new EventSource('/api/rooms/' + room + '/events');
	es.onopen = function () { $('es-status').textContent = 'es: open'; };
	es.onerror = function () { $('es-status').textContent = 'es: error, retrying'; };
	$('htmx-feed').dataset.kinds.split(',').forEach(function (kind) {
		es.addEventListener(kind, function (e) { spikeEvent('es', kind, e.data); });
	});

	document.addEventListener('htmx:sse:after:connection', function (e) {
		$('htmx-status').textContent = 'htmx: open ' + e.detail.connection.status;
	});
	document.addEventListener('htmx:sse:error', function (e) {
		$('htmx-status').textContent = 'htmx: error ' + (e.detail.status || e.detail.error || '');
	});
	document.addEventListener('htmx:sse:close', function (e) {
		$('htmx-status').textContent = 'htmx: closed ' + e.detail.reason;
	});

	$('spike-login').addEventListener('submit', function (e) {
		e.preventDefault();
		fetch('/api/login', { method: 'POST', headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ username: $('spike-user').value, password: $('spike-pass').value })
		}).then(function (r) {
			$('spike-setup-status').textContent = 'login ' + r.status + (r.ok ? ', cookie set' : '');
		});
	});
	$('spike-open').addEventListener('click', function (e) {
		var btn = e.currentTarget;
		fetch('/api/rooms', { method: 'POST', headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ tcIdx: Number(btn.dataset.tc), boLen: Number(btn.dataset.bo), bot: btn.dataset.bot })
		}).then(function (r) {
			if (!r.ok) { $('spike-setup-status').textContent = 'open room ' + r.status; return; }
			return r.json().then(function (created) { location.href = '/spike?room=' + created.id; });
		});
	});
	$('spike-move').addEventListener('submit', function (e) {
		e.preventDefault();
		fetch('/api/rooms/' + room + '/move', { method: 'POST', headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ cell: $('spike-cell').value })
		}).then(function (r) {
			if (r.ok) { $('spike-move-status').textContent = '204 ok'; return; }
			r.text().then(function (t) { $('spike-move-status').textContent = r.status + ' ' + t; });
		});
	});
})();
</script>
</body>
</html>
`
