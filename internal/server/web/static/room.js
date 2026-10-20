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
var SideHost = 'host', SideGuest = 'guest';
var root = $('room-root');
var detailURL = '/api/rooms/' + root.dataset.room;
var participant = root.dataset.participant === '1';
var myID = Number(root.dataset.myId || 0);
var hostID = Number(root.dataset.hostId || 0);
var guestID = Number(root.dataset.guestId || 0);
var hostName = root.dataset.hostName;
var guestName = root.dataset.guestName;
var seatedAtLoad = guestID !== 0;
var coarse = window.matchMedia && window.matchMedia('(pointer: coarse)').matches;
var selected = '';
var terminal = false;
var seriesSide = '';
var scoreFinal = false;
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

function status(msg, err) {
	var el = $('room-status');
	el.textContent = msg;
	el.classList.toggle('err', Boolean(err));
}

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

function moveRow(n) {
	var li = document.createElement('li');
	li.className = 'pair';
	var num = document.createElement('span');
	num.className = 'n';
	num.textContent = String(n);
	var red = document.createElement('span');
	red.className = 'half red';
	var blue = document.createElement('span');
	blue.className = 'half blue';
	li.appendChild(num);
	li.appendChild(red);
	li.appendChild(blue);
	return li;
}

function markLatestPly(i) {
	var ol = $('move-history');
	var marks = ol.querySelectorAll('.latest');
	for (var k = 0; k < marks.length; k++) { marks[k].classList.remove('latest'); }
	if (i < 0) { return; }
	var row = ol.children[Math.floor(i / 2)];
	if (!row) { return; }
	row.classList.add('latest');
	var half = row.children[1 + (i % 2)];
	if (half) {
		half.classList.add('latest');
		if (ol.scrollHeight > ol.clientHeight) { ol.scrollTop = ol.scrollHeight; }
	}
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
	for (var m = 0; m < names.length; m += 2) {
		var row = moveRow(m / 2 + 1);
		row.children[1].textContent = names[m];
		if (m + 1 < names.length) { row.children[2].textContent = names[m + 1]; }
		ol.appendChild(row);
	}
	markLatestPly(names.length - 1);
	root.setAttribute('data-move-count', String(names.length));
}

function paintActiveSeat() {
	var live = !terminal && !!turn;
	var red = $('card-red'), blue = $('card-blue');
	if (red) { red.classList.toggle('active', live && turn === 'red'); }
	if (blue) { blue.classList.toggle('active', live && turn === 'blue'); }
}

function setTurnLine() {
	var line = $('turn-line');
	var mine = participant && detail && detail.game && Number(detail.game.turnUserId) === myID;
	if (terminal) {
		line.textContent = '';
		$('board').classList.remove('live');
		paintActiveSeat();
		return;
	}
	if (!turn || !detail || !detail.game) {
		line.textContent = 'waiting for both to ready';
		$('board').classList.remove('live');
		paintActiveSeat();
		return;
	}
	line.textContent = mine ? 'your move (' + turn + ')' : turn + ' to move (' + seatName(detail.game.turnUserId) + ')';
	$('board').classList.toggle('live', mine && !terminal);
	paintActiveSeat();
}

function paintClocks() {
	$('clock-red').textContent = fmtClock(remMs.red);
	$('clock-blue').textContent = fmtClock(remMs.blue);
}

function applyPlayable(list) {
	var ok = null;
	if ($('board').classList.contains('live') && list) {
		ok = {};
		for (var i = 0; i < list.length; i++) { ok[list[i]] = 1; }
	}
	var cells = $('board').querySelectorAll('.cell');
	for (var j = 0; j < cells.length; j++) {
		cells[j].classList.toggle('off', !!ok && !cells[j].classList.contains('occ') &&
			!ok[cells[j].dataset.cell]);
	}
}

function renderDetail(d) {
	detail = d;
	if (d.state === 'finished') { scoreFinal = true; }
	if (!terminal && !d.game && Number(d.guestUserId) !== 0 && !seatedAtLoad) {
		location.reload();
		return;
	}
	if (!d.game) {
		turn = '';
		remMs.red = 0;
		remMs.blue = 0;
		paintClocks();
		$('score-line').textContent = hostName + ' ' + d.hostWins + ' - ' + d.guestWins + ' ' + guestName;
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
	$('clock-who-red').textContent = 'red / ' + seatName(d.game.redUserId);
	$('clock-who-blue').textContent = 'blue / ' + seatName(otherSeat(d.game.redUserId));
	if (participant) {
		var me = Number(d.game.redUserId) === myID ? 'red' : 'blue';
		root.setAttribute('data-me-seat', me);
		$('board').setAttribute('data-my-color', me);
	}
	$('score-line').textContent = hostName + ' ' + d.hostWins + ' - ' + d.guestWins + ' ' + guestName;
	applyPlayable(d.game.playable);
}

function sync() {
	fetch(detailURL).then(function (r) {
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
	if (awaitingNewGame) {
		awaitingNewGame = false;
		rebuildBoard([]);
		turn = '';
	}
	var cell = $('c-' + name);
	if (!cell || cell.classList.contains('occ')) { return; }
	var i = stoneCount();
	var stone = applyStone(name, i % 2 === 0 ? 'red' : 'blue', i);
	if (stone) {
		var prevStone = $('board').querySelector('.stone.latest');
		if (prevStone) { prevStone.classList.remove('latest'); }
		stone.classList.add('latest');
	}
	var ol = $('move-history');
	var row = ol.children[Math.floor(i / 2)];
	if (!row) {
		row = moveRow(Math.floor(i / 2) + 1);
		ol.appendChild(row);
	}
	row.children[1 + (i % 2)].textContent = name;
	markLatestPly(i);
	root.setAttribute('data-move-count', String(i + 1));
	if (turn) { turn = turn === 'red' ? 'blue' : 'red'; setTurnLine(); }
	scheduleSync();
}

var syncTimer = null;
function scheduleSync() {
	if (syncTimer) { return; }
	syncTimer = setTimeout(function () {
		syncTimer = null;
		sync();
	}, 150);
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
	setTimeout(sync, 700);
}

function onSeries(side) {
	seriesSide = side;
	onTerminal('series finished: ' + side);
}

function onTerminal(msg) {
	if (terminal) { return; }
	terminal = true;
	status(msg);
	turn = '';
	setTurnLine();
	stopTimers();
	if (es) { es.close(); es = null; }
	var feed = $('sse-feed');
	if (feed) { feed.remove(); }
	if (readyBtn) { readyBtn.disabled = true; }
	if (joinBtn) { joinBtn.disabled = true; }
	if (forfeitBtn) {
		forfeitBtn.disabled = true;
		clearTimeout(forfeitTimer);
		forfeitBtn.classList.remove('armed');
	}
	fetch(detailURL).then(function (r) {
		if (r.ok) { return r.json(); }
		paintSeriesVerdict();
		return null;
	}).then(function (d) { if (d) { renderDetail(d); } }).catch(function () {});
}

function paintSeriesVerdict() {
	if (!seriesSide || scoreFinal) { return; }
	$('score-line').textContent = 'series won by ' +
		(seriesSide === SideHost ? hostName : seriesSide === SideGuest ? guestName : 'no one');
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

function reject(r, what) {
	if (r.status === 401 || r.status === 404 || r.status === 409) {
		return r.json().then(function (e) {
			status(rejectWords[e.error] || ('rejected: ' + e.error), true);
		}, function () { status(what + ' failed (' + r.status + ')', true); });
	}
	status(what + ' failed (' + r.status + ')', true);
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

$('board').addEventListener('click', function (e) {
	if (!participant || terminal) { return; }
	var cell = e.target.closest ? e.target.closest('.cell') : null;
	if (!cell || cell.classList.contains('occ')) { return; }
	if (cell.classList.contains('off')) { status('not a legal space', true); return; }
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
var forfeitTimer = null;
if (forfeitBtn) {
	var armed = false;
	var label = forfeitBtn.textContent;
	forfeitBtn.addEventListener('click', function () {
		if (!armed) {
			armed = true;
			forfeitBtn.textContent = 'confirm forfeit?';
			forfeitBtn.classList.add('armed');
			forfeitTimer = setTimeout(function () {
				armed = false;
				forfeitBtn.textContent = label;
				forfeitBtn.classList.remove('armed');
			}, 3000);
			return;
		}
		clearTimeout(forfeitTimer);
		fetch(detailURL + '/forfeit', { method: 'POST' }).then(function (r) {
			if (r.ok) { return; }
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
