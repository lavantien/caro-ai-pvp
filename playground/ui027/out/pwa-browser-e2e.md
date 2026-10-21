# pwa browser e2e, dead-server offline proof

Date: 2026-10-22. Worktree branch audit of the v0.27 groundwork directive against the shipped state at HEAD, plus the one missing browser-level proof.

## Audit result

The directive's items are already shipped and gated at HEAD, so this batch adds evidence only, no product changes:

- Design system: `docs/design-system.md` is the shipped re-skin contract (ten fixed anchors, amber law, flat borders, no gradients), tokens live in `internal/server/web/shell.css`, and `make darkcontrast` machine-checks the contrast contract inside `ci`. A light theme is explicitly out of contract there, so none was added.
- Re-skin: templates carry the component classes; the overhaul evidence was recaptured in f6908e9.
- PWA shell: `internal/server/ui_pwa.go` mounts `/manifest.webmanifest`, `/sw.js`, `/offline.html`; `ui_pwa_test.go` pins the manifest fields, icon decodability, head wiring on base, room, and playback, and the worker boundary strings statically.
- Screenshot state: 25 captures in `docs/screenshots` already reflect the shipped skin (f6908e9), no recapture needed.

## Browser proofs

Server: `caro-shots.exe serve -addr 127.0.0.1:8431 -db .scratch/pwa-e2e.db`, built from this HEAD, killed for the offline phase. Browser: playwright, viewport 390x844.

1. Manifest and worker registration: `GET /manifest.webmanifest` 200 over HTTP. `navigator.serviceWorker.ready` resolves `activated`, scope `http://127.0.0.1:8431/`, page `controlled: true`.
2. Cache inventory: one cache `caro-static-v3` holding exactly the 11 allowlisted static URLs (shell.css, htmx, hx-sse, room.js, playback.js, favicon, 4 icons, offline.html). No `/api/` URL, no navigation URL.
3. SSE never worker-answered: with the page controlled, a live room's `/api/rooms/<id>/events` request returned 200 from the network and `caches.match` for that URL is a miss. The worker passed the stream through untouched.
4. Offline fallback with the server process dead: after `Stop-Process` on the server pid, navigating to `/` renders the cached fallback, title "caro offline", heading "caro is offline", copy "needs a connection", still controlled. Screenshot: `pwa-offline-fallback-390.png` beside this file.

## Reading log

- read internal/server/ui_pwa.go full - pwa routes and content types
- read internal/server/web.go full - static serving and cache headers
- read internal/server/ui_shell.go full - shell routes, css handler
- read internal/server/web/shell.css full - token layer state
- read internal/server/web/templates/base.tmpl, board.html, room.html - markup and classes
- read internal/server/web/static/pwa.js full - install surface
- read internal/server/web/pwa/sw.js head - boundary implementation
- read internal/server/ui_pwa_test.go full - shipped pwa test coverage
- read docs/design-system.md full - shipped design contract
- grep internal/server ui tests - markup assertions
- grep Makefile shots/darkcontrast/icons/ci - target wiring
