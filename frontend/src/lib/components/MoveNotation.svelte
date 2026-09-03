<script lang="ts">
	import type { MoveRecord } from '$lib/stores/gameStore.svelte';
	import { toUCI } from '$lib/uciEngine';
	import { UIConfig } from '$lib/config';

	interface Props {
		moves: MoveRecord[];
		currentMoveNumber?: number;
	}

	let { moves, currentMoveNumber }: Props = $props();

	let scrollContainer: HTMLDivElement | undefined = $state();

	function formatMove(move: MoveRecord): string {
		return `${move.moveNumber}.${toUCI(move.x, move.y)}`;
	}

	function isLatestMove(move: MoveRecord): boolean {
		return currentMoveNumber !== undefined && move.moveNumber === currentMoveNumber;
	}

	$effect(() => {
		if (scrollContainer) {
			scrollContainer.scrollLeft = scrollContainer.scrollWidth;
		}
	});
</script>

<div class="w-full mx-auto px-1" style="max-width: {UIConfig.maxContentWidthPx}px;" data-testid="move-notation">
	{#if moves.length > 0}
		<div
			bind:this={scrollContainer}
			class="flex items-center gap-1.5 overflow-x-auto py-2 px-2 scrollbar-none"
		>
			{#each moves as move (move.moveNumber)}
				<span
					class="shrink-0 px-1.5 py-0.5 rounded text-xs font-mono {isLatestMove(move)
						? move.player === 'red'
							? 'bg-red-100 text-red-700 font-bold'
							: 'bg-blue-100 text-blue-700 font-bold'
						: move.player === 'red'
							? 'text-red-600'
							: 'text-blue-600'}"
				>
					{formatMove(move)}
				</span>
			{/each}
		</div>
	{:else}
		<p class="text-xs text-gray-400 text-center py-2">No moves yet</p>
	{/if}
</div>

<style>
	.scrollbar-none {
		-ms-overflow-style: none;
		scrollbar-width: none;
	}
	.scrollbar-none::-webkit-scrollbar {
		display: none;
	}
</style>
