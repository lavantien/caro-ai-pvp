// The caro install shell's service worker. The boundary law lives in
// docs/pwa-shell.md and is pinned by test: this worker answers only the
// ASSETS allowlist below, on exact pathname matches, and document
// navigations (network first, cached offline page when the fetch itself
// fails). The /api/ tree, the /api/rooms/{id}/events SSE streams, and every
// other request pass through with no respondWith at all.
const CACHE = 'caro-static-v1';
// Bump CACHE whenever any allowlisted asset changes; activate drops every
// cache whose name is not the current one.
const ASSETS = [
	'/shell.css',
	'/static/htmx.min.js',
	'/static/hx-sse.min.js',
	'/static/room.js',
	'/static/playback.js',
	'/static/favicon.svg',
	'/static/icons/icon-192.png',
	'/static/icons/icon-512.png',
	'/static/icons/icon-512-maskable.png',
	'/static/icons/apple-touch-icon.png',
	'/offline.html',
];
const OFFLINE = '/offline.html';
const API_PREFIX = '/api/';

self.addEventListener('install', function (e) {
	e.waitUntil(caches.open(CACHE).then(function (c) {
		return c.addAll(ASSETS);
	}).then(function () {
		return self.skipWaiting();
	}));
});

self.addEventListener('activate', function (e) {
	e.waitUntil(caches.keys().then(function (names) {
		return Promise.all(names.filter(function (n) {
			return n !== CACHE;
		}).map(function (n) {
			return caches.delete(n);
		}));
	}).then(function () {
		return self.clients.claim();
	}));
});

self.addEventListener('fetch', function (e) {
	var url = new URL(e.request.url);
	if (url.origin !== self.location.origin) { return; }
	// The pass-through law: the JSON endpoints, the SSE event streams, and
	// every stateful URL ride the network alone.
	if (url.pathname.indexOf(API_PREFIX) === 0) { return; }
	if (e.request.mode === 'navigate') {
		e.respondWith(fetch(e.request).catch(function () {
			return caches.match(OFFLINE);
		}));
		return;
	}
	if (ASSETS.indexOf(url.pathname) !== -1) {
		e.respondWith(caches.match(e.request));
	}
});
