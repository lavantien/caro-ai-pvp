<script lang="ts">
	import type { MoveRecord } from '$lib/stores/gameStore.svelte';

	interface Props {
		moves: MoveRecord[];
		currentMoveNumber?: number;
	}

	let { moves, currentMoveNumber }: Props = $props();

	function formatMove(move: MoveRecord): string {
		return `${move.moveNumber}. ${move.player.charAt(0).toUpperCase() + move.player.slice(1)}: (${move.x}, ${move.y})`;
	}

	function isLatestMove(move: MoveRecord): boolean {
		return currentMoveNumber !== undefined && move.moveNumber === currentMoveNumber;
	}
</script>

<div class="w-full max-w-md mx-auto">
	<h3 class="text-lg font-semibold mb-2 text-gray-800">Move History</h3>

	{#if moves.length === 0}
		<p class="text-gray-500 text-center py-4">No moves yet</p>
	{:else}
		<div class="border rounded-lg p-4 bg-white shadow-sm overflow-y-auto max-h-64">
			{#each moves as move (move.moveNumber)}
				<div
					class="flex justify-between items-center py-1 px-2 rounded {isLatestMove(move)
						? move.player === 'red'
							? 'bg-red-100'
							: 'bg-blue-100'
						: ''}">
					<span class="text-sm font-medium text-gray-700">{formatMove(move)}</span>
				</div>
			{/each}
		</div>
	{/if}
</div>
