(function () {
'use strict';

function $(id) { return document.getElementById(id); }
var root = $('playback-root');
var board = $('board');
var list = $('pb-moves');
var total = Number(root.dataset.total || 0);
var step = total;
var timer = null;

function indexHalves() {
	var rows = list.children;
	for (var r = 0; r < rows.length; r++) {
		var halves = rows[r].getElementsByClassName('half');
		for (var h = 0; h < halves.length; h++) {
			if (halves[h].textContent) { halves[h].setAttribute('data-i', String(2 * r + h)); }
		}
	}
}

function render() {
	var stones = board.querySelectorAll('.stone');
	for (var i = 0; i < stones.length; i++) {
		var at = Number(stones[i].getAttribute('data-i'));
		stones[i].classList.toggle('hidden', at >= step);
		stones[i].classList.toggle('latest', at === step - 1);
	}
	var rows = list.children;
	for (var r = 0; r < rows.length; r++) {
		var halves = rows[r].getElementsByClassName('half');
		var rowLatest = false;
		for (var h = 0; h < halves.length; h++) {
			var latest = halves[h].getAttribute('data-i') === String(step - 1);
			halves[h].classList.toggle('latest', latest);
			if (latest) { rowLatest = true; }
		}
		rows[r].classList.toggle('latest', rowLatest);
	}
	$('pb-counter').textContent = step + ' / ' + total;
	$('pb-first').disabled = step === 0;
	$('pb-prev').disabled = step === 0;
	$('pb-next').disabled = step === total;
	$('pb-last').disabled = step === total;
}

function go(n) {
	step = Math.max(0, Math.min(total, n));
	render();
}

function stopPlay() {
	if (timer) { clearInterval(timer); timer = null; }
	$('pb-play').textContent = 'play';
}

$('pb-first').addEventListener('click', function () { stopPlay(); go(0); });
$('pb-prev').addEventListener('click', function () { stopPlay(); go(step - 1); });
$('pb-next').addEventListener('click', function () { stopPlay(); go(step + 1); });
$('pb-last').addEventListener('click', function () { stopPlay(); go(total); });
$('pb-play').addEventListener('click', function () {
	if (timer) { stopPlay(); return; }
	if (step >= total) { go(0); }
	$('pb-play').textContent = 'pause';
	timer = setInterval(function () {
		if (step >= total) { stopPlay(); return; }
		go(step + 1);
	}, 700);
});

list.addEventListener('click', function (e) {
	var half = e.target.closest ? e.target.closest('.half[data-i]') : null;
	if (!half) { return; }
	stopPlay();
	go(Number(half.getAttribute('data-i')) + 1);
});

indexHalves();
render();
})();
