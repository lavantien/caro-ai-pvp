// The v0.27 PWA wiring: register the shell service worker, and surface the
// browser's install prompt in the header nav. The action stays hidden until
// the browser offers a prompt, and disappears inside an installed display
// mode (launching from the home screen has nothing left to install). The
// standalone room and playback pages carry this script too; there it finds
// no button and only registers the worker.
(function () {
'use strict';
if (!('serviceWorker' in navigator)) { return; }
navigator.serviceWorker.register('/sw.js');

var btn = document.getElementById('install-btn');
if (!btn) { return; }
var deferred = null;
function standalone() {
	return window.matchMedia('(display-mode: standalone)').matches
		|| navigator.standalone === true;
}
window.addEventListener('beforeinstallprompt', function (e) {
	e.preventDefault();
	deferred = e;
	if (!standalone()) { btn.hidden = false; }
});
btn.addEventListener('click', function () {
	if (!deferred) { return; }
	btn.hidden = true;
	deferred.prompt();
	deferred = null;
});
if (standalone()) { btn.hidden = true; }
})();
