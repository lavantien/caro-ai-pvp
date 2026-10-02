// The M6b playback board driver: step controls over the server-rendered
// stones, each tagged with its move index, so stepping only toggles
// visibility. The board opens on the final position (the finished game is
// the product's visual priority) and the controls walk it back and forth.
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
	var stones = board.querySelectorAll('.stone');
	for (var i = 0; i < stones.length; i++) {
		var at = Number(stones[i].getAttribute('data-i'));
		stones[i].classList.toggle('hidden', at >= step);
		stones[i].classList.toggle('latest', at === step - 1);
	}
	for (var j = 0; j < list.children.length; j++) {
		list.children[j].classList.toggle('latest', j === step - 1);
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

render();
})();
