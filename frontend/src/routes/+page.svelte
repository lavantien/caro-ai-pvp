<script lang="ts">
	import { GameConfig } from '$lib/config';
</script>

<div class="max-w-lg mx-auto px-4 py-6 sm:p-8 text-center">
	<h1 class="text-3xl sm:text-4xl font-bold mb-3 text-gray-800">Caro</h1>
	<p class="text-base sm:text-lg mb-4 text-gray-600">
		{GameConfig.boardSize}&times;{GameConfig.boardSize} &middot; Open Rule &middot; Exact {GameConfig.winLength}
	</p>

	<div class="my-6 p-4 sm:p-6 bg-amber-50 rounded-lg border border-amber-200 text-left">
		<h2 class="text-xl font-semibold mb-2 text-amber-800">Rules</h2>
		<ul class="space-y-1.5 text-gray-700 text-sm sm:text-base">
			<li>Red O vs Blue X</li>
			<li>Red's second move must be at least {GameConfig.openRuleMinDistance} intersections away</li>
			<li>Exactly 5 in a row to win (no overlines)</li>
			<li>Both ends blocked = no win</li>
		</ul>
	</div>

	<a href="/game">
		<button
			class="w-full sm:w-auto px-8 py-4 bg-amber-500 hover:bg-amber-600 text-white text-lg font-bold rounded-lg shadow-lg transition-colors">
			Start Game
		</button>
	</a>
</div>
