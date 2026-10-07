# PWA install shell

The v0.27 install surface: the web app manifest, the service worker, the offline page, and the committed icons. Scope is installability and an offline shell, never game data.

## Surface

| route | file | served as |
| --- | --- | --- |
| `/manifest.webmanifest` | `internal/server/web/pwa/manifest.webmanifest` | `application/manifest+json`, no-cache |
| `/sw.js` | `internal/server/web/pwa/sw.js` | `text/javascript`, no-cache |
| `/offline.html` | `internal/server/web/pwa/offline.html` | `text/html`, no-cache |
| `/static/pwa.js` | `internal/server/web/static/pwa.js` | static mount, day cache |
| `/static/icons/*.png` | `internal/server/web/static/icons/` | static mount, day cache |

The worker sits at the root, so its scope covers the whole site with no `Service-Worker-Allowed` header. `theme_color` and `background_color` are `#101318`, the `--background` anchor of docs/design-system.md; they change together. Every page head (base.tmpl, room.html, playback.html) carries the manifest link, the theme-color meta, the apple touch icon and meta set, and `/static/pwa.js` deferred.

## The boundary law

The worker holds an explicit `const ASSETS` allowlist and answers exactly two kinds of requests:

1. Exact pathname matches on the allowlist, cache-first: shell.css, the vendored htmx and hx-sse scripts, room.js, playback.js, favicon.svg, the icons, offline.html.
2. Document navigations, network-first, falling back to the cached `/offline.html` only when the fetch itself fails.

Everything else passes through with no `respondWith` at all. In particular the worker never answers anything under `/api/`, which carries the JSON endpoints and the room SSE event streams (`/api/rooms/{id}/events`), and it never answers a room or tournament page: those are live state, the network is their only source. `ui_pwa_test.go` pins this textually: the `/api/` pass-through guard must precede every `e.respondWith(` in the served source, and the worker must carry exactly the two intended calls.

Never cached, by design: any `/api/` response, any SSE stream, every room (`/rooms/{id}`), playback, history, tournament, login, or home document (navigations are network-first, and the offline page is the only offline answer), `/static/pwa.js` itself, and the manifest.

## Cache bump law

The cache name is `caro-static-v1`. Bump the version in `sw.js` whenever any allowlisted asset changes content under the same URL (a shell.css token edit, a re-vendored htmx, a regenerated icon). The activate step drops every cache whose name is not the current one, so the bump is the whole migration.

## Icons

`make icons` regenerates the four committed PNG files via `playground/icongen` (stdlib only), rasterizing the favicon.svg geometry with 4x supersampling:

| file | size | law |
| --- | --- | --- |
| `icon-192.png`, `icon-512.png` | 192, 512 | favicon geometry verbatim (rounded square, red disc, blue disc at 0.9) |
| `apple-touch-icon.png` | 180 | full-bleed square: iOS applies its own corner mask, pre-rounded corners would show through it |
| `icon-512-maskable.png` | 512 | full-bleed square, stones shrunk onto the centered 80% maskable safe zone |

The background fill is opaque in every icon, so launcher masks crop onto the page color.

## Registration and install action

`/static/pwa.js` registers `/sw.js`, captures `beforeinstallprompt`, and surfaces the hidden `#install-btn` (a `.btn-quiet` in the base header nav) only when the browser offers the prompt; inside an installed display mode the button stays hidden. The standalone room and playback pages register the worker too but carry no button.
