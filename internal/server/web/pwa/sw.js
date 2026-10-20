// /api/rooms/{id}/events rides the /api/ pass-through below.
const CACHE = 'caro-static-v3';
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
		return c.addAll(ASSETS.map(function (u) {
			return new Request(u, {cache: 'reload'});
		}));
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
	if (url.pathname.indexOf(API_PREFIX) === 0) { return; }
	if (e.request.mode === 'navigate') {
		e.respondWith(fetch(e.request).catch(function () {
			return caches.match(OFFLINE).then(function (r) {
				return r || fetch(OFFLINE);
			});
		}));
		return;
	}
	if (ASSETS.indexOf(url.pathname) !== -1) {
		e.respondWith(caches.match(e.request).then(function (r) {
			return r || fetch(e.request);
		}));
	}
});
