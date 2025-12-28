<script lang="ts">
	import type { Cell } from '$lib/types/game';
	import CellComponent from './Cell.svelte';
	import { calculateGhostStonePosition, isValidCell } from '$lib/utils/boardUtils';
	import { vibrateOnValidMove, vibrateOnInvalidMove } from '$lib/utils/haptics';

	interface Props {
		board: Cell[];
		onMove: (x: number, y: number) => void;
	}

	let { board, onMove }: Props = $props();

	let ghostPosition = $state<{ x: number; y: number } | null>(null);

	function handleCellClick(x: number, y: number) {
		const cell = board.find((c) => c.x === x && c.y === y);
		if (!cell || cell.player !== 'none') {
			vibrateOnInvalidMove();
			return;
		}

		vibrateOnValidMove();
		onMove(x, y);
	}

	function handleTouchMove(event: TouchEvent) {
		const touch = event.touches[0];
		const element = document.elementFromPoint(touch.clientX, touch.clientY);

		if (element instanceof HTMLElement) {
			const x = parseInt(element.dataset.x ?? '-1');
			const y = parseInt(element.dataset.y ?? '-1');

			if (isValidCell(x, y)) {
				const rect = element.getBoundingClientRect();
				ghostPosition = calculateGhostStonePosition(
					rect.left + rect.width / 2,
					rect.top + rect.height / 2
				);
			}
		}
	}
</script>

<div class="relative">
	<div
		class="grid gap-0 bg-amber-100 p-4 rounded-lg shadow-lg touch-none select-none"
		style="display: grid; grid-template-columns: repeat(15, 40px); grid-template-rows: repeat(15, 40px); width: 632px; height: 632px;"
		ontouchmove={handleTouchMove}
		ontouchend={() => (ghostPosition = null)}
	>
		{#each board as cell}
			<CellComponent
				x={cell.x}
				y={cell.y}
				player={cell.player}
				onclick={() => handleCellClick(cell.x, cell.y)}
				onkeydown={(e) => e.key === 'Enter' && handleCellClick(cell.x, cell.y)} />
		{/each}
	</div>

	{#if ghostPosition}
		<div
			class="fixed pointer-events-none w-10 h-10 rounded-full border-4 border-dashed border-gray-400 opacity-60"
			style="left: {ghostPosition.x - 20}px; top: {ghostPosition.y - 20}px;"
		>
			<span class="flex items-center justify-center h-full text-2xl text-gray-400">?</span>
		</div>
	{/if}
</div>
