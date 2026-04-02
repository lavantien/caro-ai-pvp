<script lang="ts">
	import type { Player } from '$lib/types/game';

	interface Props {
		winner: Player | undefined;
		onNewGame: () => void;
	}

	let { winner, onNewGame }: Props = $props();
</script>

<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
	<div class="bg-white rounded-2xl shadow-2xl p-6 sm:p-8 mx-4 max-w-sm w-full text-center">
		<h2
			class="text-2xl sm:text-3xl font-bold uppercase tracking-wide {winner === 'red'
				? 'text-red-600'
				: 'text-blue-600'}"
		>
			{winner} Wins!
		</h2>
		<button
			onclick={onNewGame}
			class="mt-5 w-full px-6 py-4 text-lg font-bold text-white rounded-xl shadow-lg transition-colors min-h-[48px] {winner === 'red'
				? 'bg-red-600 hover:bg-red-700'
				: 'bg-blue-600 hover:bg-blue-700'}"
		>
			New Game
		</button>
	</div>
</div>
