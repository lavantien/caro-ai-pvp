// The M6b room page driver: SSE stones per the spike's proven wiring (the
// hx-on handlers below call into window.caroRoom; a plain EventSource takes
// over once htmx stops reconnecting), clock polling and ticking, the ready
// handshake, forfeit with a confirm step, and the tap-to-preview selection
// machine for coarse pointers. Guests run the same page with the acting
// handlers simply absent.
(function () {
'use strict';

/* caro-tap-select:begin */
function tapSelect(sel, cell) {
	if (sel === cell) { return { select: '', confirm: true }; }
	return { select: cell, confirm: false };
}
/* caro-tap-select:end */

/* caro-ui-mline:begin */
function uiMLine(line) {
	var fields = line.split(', ');
	var kept = [];
	for (var i = 0; i < fields.length; i++) {
		var f = fields[i];
		if (f.indexOf('ebf=') === 0 || f.indexOf('hf=') === 0 || f.indexOf('fh1=') === 0) { continue; }
		kept.push(f);
	}
	return kept.join(', ');
}
/* caro-ui-mline:end */

function $(id) { return document.getElementById(id); }
var root = $('room-root');
var detailURL = '/api/rooms/' + root.dataset.room;
var participant = root.dataset.participant === '1';
var myID = Number(root.dataset.myId || 0);
var hostID = Number(root.dataset.hostId || 0);
var guestID = Number(root.dataset.guestId || 0);
var hostName = root.dataset.hostName;
var guestName = root.dataset.guestName;
// Seat names freeze at render: a page opened while the guest seat was open
// (host or spectator) must reload once the seat fills, the only render that
// carries the seated name and the participant handshake.
var seatedAtLoad = guestID !== 0;
var coarse = window.matchMedia && window.matchMedia('(pointer: coarse)').matches;
var selected = '';
var terminal = false;
var awaitingNewGame = false;
var es = null;
var pollTimer = null;
var tickTimer = null;
var detail = null;
var turn = '';
var remMs = { red: 0, blue: 0 };
var lastSync = Date.now();

function seatName(id) { return Number(id) === hostID ? hostName : guestName; }
function otherSeat(id) { return Number(id) === hostID ? guestID : hostID; }
function glyphOf(color) { return color === 'red' ? 'O' : 'X'; }
function stoneCount() { return $('board').querySelectorAll('.stone').length; }
function status(msg) { $('room-status').textContent = msg; }

function fmtClock(ms) {
	if (ms < 0) { ms = 0; }
	var s = Math.floor(ms / 1000);
	var mm = String(Math.floor(s / 60));
	var ss = String(s % 60);
	var t = String(Math.floor((ms % 1000) / 100));
	return (mm.length < 2 ? '0' + mm : mm) + ':' + (ss.length < 2 ? '0' + ss : ss) + '.' + t;
}

function applyStone(name, color, idx) {
	var cell = $('c-' + name);
	if (!cell || cell.classList.contains('occ')) { return null; }
	var s = document.createElement('span');
	s.className = 'stone ' + color;
	if (typeof idx === 'number') { s.setAttribute('data-i', String(idx)); }
	s.textContent = glyphOf(color);
	cell.appendChild(s);
	cell.classList.add('occ');
	return s;
}

function rebuildBoard(names) {
	var board = $('board');
	var stones = board.querySelectorAll('.stone');
	for (var i = 0; i < stones.length; i++) { stones[i].remove(); }
	var occs = board.querySelectorAll('.cell.occ');
	for (var j = 0; j < occs.length; j++) { occs[j].classList.remove('occ'); }
	for (var k = 0; k < names.length; k++) { applyStone(names[k], k % 2 === 0 ? 'red' : 'blue', k); }
	var last = board.querySelector('.stone[data-i="' + (names.length - 1) + '"]');
	if (last) { last.classList.add('latest'); }
	var ghost = board.querySelector('.cell.ghost');
	if (ghost && ghost.classList.contains('occ')) { ghost.classList.remove('ghost'); selected = ''; }
	var ol = $('move-history');
	ol.textContent = '';
	for (var m = 0; m < names.length; m++) {
		var li = document.createElement('li');
		li.textContent = names[m];
		ol.appendChild(li);
	}
	if (ol.lastElementChild) { ol.lastElementChild.classList.add('latest'); }
	root.setAttribute('data-move-count', String(names.length));
}

function setTurnLine() {
	var line = $('turn-line');
	var mine = participant && detail && detail.game && Number(detail.game.turnUserId) === myID;
	if (!turn || !detail || !detail.game) {
		line.textContent = 'waiting for both to ready';
		$('board').classList.remove('live');
		return;
	}
	line.textContent = mine ? 'your move (' + turn + ')' : turn + ' to move (' + seatName(detail.game.turnUserId) + ')';
	$('board').classList.toggle('live', mine && !terminal);
}

function paintClocks() {
	$('clock-red').textContent = fmtClock(remMs.red);
	$('clock-blue').textContent = fmtClock(remMs.blue);
}

// renderDetail reconciles the whole view with one room detail: the board
// after a reload-safe event gap, both banks, the turn, and the score line.
function renderDetail(d) {
	detail = d;
	// The handshake section and the seat names render server-side at load:
	// a page opened while the guest seat was open trades one reload for
	// the seated render, whatever the viewer's role.
	if (!d.game && Number(d.guestUserId) !== 0 && !seatedAtLoad) {
		location.reload();
		return;
	}
	if (!d.game) {
		turn = '';
		remMs.red = 0;
		remMs.blue = 0;
		paintClocks();
		setTurnLine();
		return;
	}
	turn = d.game.turn;
	remMs.red = d.game.clockMs[0];
	remMs.blue = d.game.clockMs[1];
	lastSync = Date.now();
	rebuildBoard(d.game.moves);
	paintClocks();
	setTurnLine();
	// The red seat rotates every game (the decisive loser takes red), so
	// the clock labels and the mover's ghost color follow redUserId, not
	// the load-time render.
	$('clock-who-red').textContent = 'red / ' + seatName(d.game.redUserId);
	$('clock-who-blue').textContent = 'blue / ' + seatName(otherSeat(d.game.redUserId));
	if (participant) {
		$('board').setAttribute('data-my-color',
			Number(d.game.redUserId) === myID ? 'red' : 'blue');
	}
	$('score-line').textContent = hostName + ' ' + d.hostWins + ' - ' + d.guestWins + ' ' + guestName;
}

function sync() {
	fetch(detailURL).then(function (r) {
		// A retired room answers 409 room_closed while still mapped and
		// 404 room_not_found once evicted; both end the page, so a missed
		// series frame never parks it on an endless poll.
		if (r.status === 404 || r.status === 409) { onTerminal('room closed'); return null; }
		if (!r.ok) { return null; }
		return r.json();
	}).then(function (d) {
		if (d) { renderDetail(d); }
	}).catch(function () {});
}

function tick() {
	if (terminal || !turn) { return; }
	remMs[turn] -= Date.now() - lastSync;
	lastSync = Date.now();
	if (remMs[turn] < 0) { remMs[turn] = 0; }
	paintClocks();
}

function onMove(name) {
	if (terminal || !name) { return; }
	// A move after a gameend opens the next game: the board resets in the
	// same server critical section as the completion, so the SSE stream
	// never carries an explicit reset frame.
	if (awaitingNewGame) {
		awaitingNewGame = false;
		rebuildBoard([]);
		turn = '';
	}
	var cell = $('c-' + name);
	if (!cell || cell.classList.contains('occ')) { return; }
	var i = stoneCount();
	var stone = applyStone(name, i % 2 === 0 ? 'red' : 'blue', i);
	// The latest ring tracks the stream, not just the one-second poll, so
	// the winning stone stays highlighted on the terminal page.
	if (stone) {
		var prevStone = $('board').querySelector('.stone.latest');
		if (prevStone) { prevStone.classList.remove('latest'); }
		stone.classList.add('latest');
	}
	var ol = $('move-history');
	var prev = ol.querySelector('li.latest');
	if (prev) { prev.classList.remove('latest'); }
	var li = document.createElement('li');
	li.textContent = name;
	li.classList.add('latest');
	ol.appendChild(li);
	root.setAttribute('data-move-count', String(i + 1));
	if (turn) { turn = turn === 'red' ? 'blue' : 'red'; setTurnLine(); }
}

function onMLine(line) {
	var ul = $('bot-log');
	if (ul.firstElementChild && ul.children.length === 1 &&
		ul.firstElementChild.textContent === 'no bot lines yet') {
		ul.textContent = '';
	}
	var li = document.createElement('li');
	li.textContent = uiMLine(line);
	ul.appendChild(li);
	ul.scrollTop = ul.scrollHeight;
}

function onGameEnd(outcome) {
	awaitingNewGame = true;
	status('game over: ' + outcome);
	// The room either retires (a series event follows) or resets for the
	// next game with the loser on red; one detail sync paints either.
	setTimeout(sync, 700);
}

function onSeries(side) { onTerminal('series finished: ' + side); }

function onTerminal(msg) {
	if (terminal) { return; }
	terminal = true;
	status(msg);
	turn = '';
	setTurnLine();
	stopTimers();
	if (es) { es.close(); es = null; }
}

var rejectWords = {
	not_your_turn: 'not your turn',
	illegal_move: 'illegal move',
	not_ready: 'both players must ready first',
	room_closed: 'room closed',
	room_not_found: 'room closed',
	series_finished: 'series finished',
	unauthorized: 'login required'
};

// reject renders one refused acting call through the wire's error codes;
// every handler failure a player can provoke lands as words, not a status.
function reject(r, what) {
	if (r.status === 401 || r.status === 404 || r.status === 409) {
		return r.json().then(function (e) {
			status(rejectWords[e.error] || ('rejected: ' + e.error));
		}, function () { status(what + ' failed (' + r.status + ')'); });
	}
	status(what + ' failed (' + r.status + ')');
}

function clearGhost() {
	selected = '';
	var g = $('board').querySelector('.cell.ghost');
	if (g) { g.classList.remove('ghost'); }
}

function confirmMove(name) {
	clearGhost();
	fetch(detailURL + '/move', {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ cell: name })
	}).then(function (r) {
		if (r.ok) { return; }
		return reject(r, 'move');
	}).catch(function () { status('move failed'); });
}

// Coarse pointers run the tap-to-preview machine: select, confirm on the
// same cell, reselect elsewhere. Fine pointers confirm on click and preview
// through the hover ghost the board css paints.
$('board').addEventListener('click', function (e) {
	if (!participant || terminal) { return; }
	var cell = e.target.closest ? e.target.closest('.cell') : null;
	if (!cell || cell.classList.contains('occ')) { return; }
	var name = cell.dataset.cell;
	if (!coarse) { confirmMove(name); return; }
	var r = tapSelect(selected, name);
	clearGhost();
	if (r.confirm) { confirmMove(name); return; }
	selected = r.select;
	cell.classList.add('ghost');
});

var readyBtn = $('ready-btn');
if (readyBtn) {
	readyBtn.addEventListener('click', function () {
		fetch(detailURL + '/ready', { method: 'POST' }).then(function (r) {
			if (r.ok) { location.reload(); return; }
			return reject(r, 'ready');
		}).catch(function () { status('ready failed'); });
	});
}

// Taking the open seat flips the viewer into a participant; the reload
// re-renders the handshake section server-side.
var joinBtn = $('join-btn');
if (joinBtn) {
	joinBtn.addEventListener('click', function () {
		fetch(detailURL + '/join', { method: 'POST' }).then(function (r) {
			if (r.ok) { location.reload(); return; }
			return reject(r, 'join');
		}).catch(function () { status('join failed'); });
	});
}

var forfeitBtn = $('forfeit-btn');
if (forfeitBtn) {
	var armed = false;
	var armTimer = null;
	var label = forfeitBtn.textContent;
	forfeitBtn.addEventListener('click', function () {
		if (!armed) {
			armed = true;
			forfeitBtn.textContent = 'confirm forfeit?';
			forfeitBtn.classList.add('armed');
			armTimer = setTimeout(function () {
				armed = false;
				forfeitBtn.textContent = label;
				forfeitBtn.classList.remove('armed');
			}, 3000);
			return;
		}
		clearTimeout(armTimer);
		fetch(detailURL + '/forfeit', { method: 'POST' }).then(function (r) {
			if (r.ok) { return; } // the series event ends the page cleanly
			return reject(r, 'forfeit');
		}).catch(function () { status('forfeit failed'); });
	});
}

function dispatch(kind, data) {
	if (kind === 'move') { onMove(data); }
	else if (kind === 'mline') { onMLine(data); }
	else if (kind === 'gameend') { onGameEnd(data); }
	else if (kind === 'series') { onSeries(data); }
}

// The EventSource fallback of the spike's dual wiring: it starts only after
// htmx reports its stream closed, so one wiring is live at a time, and its
// own retries stop through onTerminal once the detail poll reads the room
// closed.
function startES() {
	if (es || terminal) { return; }
	es = new EventSource(detailURL + '/events');
	root.dataset.kinds.split(',').forEach(function (kind) {
		es.addEventListener(kind, function (e) { dispatch(kind, e.data); });
	});
	es.onerror = function () { sync(); };
}

document.addEventListener('htmx:sse:close', function () { startES(); });

window.caroRoom = { onMove: onMove, onMLine: onMLine, onGameEnd: onGameEnd, onSeries: onSeries };

function stopTimers() {
	if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
	if (tickTimer) { clearInterval(tickTimer); tickTimer = null; }
}

pollTimer = setInterval(sync, 1000);
tickTimer = setInterval(tick, 100);
})();
