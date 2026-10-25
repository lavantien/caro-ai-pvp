(function () {
'use strict';

function $(id) { return document.getElementById(id); }
var root = $('playback-root');
var board = $('board');
var list = $('pb-moves');
var total = Number(root.dataset.total || 0);
var step = total;
var timer = null;

function render() {
	var stones = board.querySelectorAll('.stone, .glyph');
	for (var i = 0; i < stones.length; i++) {
		var at = Number(stones[i].getAttribute('data-i'));
		stones[i].classList.toggle('hidden', at >= step);
		stones[i].classList.toggle('latest', at === step - 1);
	}
	var items = list.querySelectorAll('.m');
	for (var k = 0; k < items.length; k++) {
		var latest = items[k].getAttribute('data-i') === String(step - 1);
		items[k].classList.toggle('latest', latest);
		items[k].parentNode.classList.toggle('latest', latest);
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
	var m = e.target.closest ? e.target.closest('.m[data-i]') : null;
	if (!m) { return; }
	stopPlay();
	go(Number(m.getAttribute('data-i')) + 1);
});

$('pb-top').addEventListener('click', function () { $('pb-scroll').scrollTop = 0; });

render();
})();
