<script lang="ts">
	import type { Player } from '$lib/types/game';

	interface Props {
		winner: Player | undefined;
		onNewGame: () => void;
	}

	let { winner, onNewGame }: Props = $props();
</script>

<div
	class="fixed top-0 left-0 right-0 z-40 flex items-center justify-between px-4 py-2 shadow-lg animate-slide-down {winner === 'red'
		? 'bg-red-600'
		: winner === 'blue'
			? 'bg-blue-600'
			: 'bg-slate-600'}"
>
	<h2 class="text-lg font-bold uppercase tracking-wide text-white">
		{#if winner === 'red' || winner === 'blue'}
			{winner} Wins!
		{:else}
			Draw
		{/if}
	</h2>
	<button
		onclick={onNewGame}
		class="px-4 py-1.5 text-sm font-bold text-white rounded-lg bg-white/20 hover:bg-white/30 transition-colors"
	>
		New Game
	</button>
</div>

<style>
	@keyframes slide-down {
		from {
			transform: translateY(-100%);
		}
		to {
			transform: translateY(0);
		}
	}

	.animate-slide-down {
		animation: slide-down 0.3s ease-out forwards;
	}
</style>
