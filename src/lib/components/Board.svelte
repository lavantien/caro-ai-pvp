<script lang="ts">
	import { onMount } from 'svelte';
	import type { Cell } from '$lib/types/game';
	import CellComponent from './Cell.svelte';
	import WinningLine from './WinningLine.svelte';
	import { calculateGhostStonePosition, isValidCell, computeCellSize } from '$lib/utils/boardUtils';
	import { vibrateOnValidMove, vibrateOnInvalidMove } from '$lib/utils/haptics';
	import { GameConfig } from '$lib/config/gameConfig';

	interface Props {
		board: Cell[];
		onMove: (x: number, y: number) => void;
		winningLine?: Array<{ x: number; y: number }>;
		lastMove?: { x: number; y: number } | null;
	}

	let { board, onMove, winningLine = [], lastMove = null }: Props = $props();

	let ghostPosition = $state<{ x: number; y: number } | null>(null);
	let cellSize = $state(computeCellSize(typeof window !== 'undefined' ? window.innerWidth : 1024));
	let boardEl: HTMLDivElement | undefined = $state();

	function handleCellClick(x: number, y: number) {
		const cell = board[x * GameConfig.boardSize + y];
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
					rect.top + rect.height / 2,
					cellSize * 0.78
				);
			}
		}
	}

	onMount(() => {
		const observer = new ResizeObserver((entries) => {
			for (const entry of entries) {
				const width = entry.contentRect.width;
				if (width > 0) {
					cellSize = computeCellSize(width);
				}
			}
		});

		if (boardEl) {
			observer.observe(boardEl);
		}

		return () => observer.disconnect();
	});
</script>

<div class="w-full max-w-[1024px] mx-auto" bind:this={boardEl}>
	<div class="relative">
		<div
			class="grid gap-0 bg-amber-100 rounded-lg shadow-lg touch-none select-none"
			style="display: grid; grid-template-columns: repeat({GameConfig.boardSize}, {cellSize}px); grid-template-rows: repeat({GameConfig.boardSize}, {cellSize}px); width: {GameConfig.boardSize * cellSize}px; height: {GameConfig.boardSize * cellSize}px;"
			ontouchmove={handleTouchMove}
			ontouchend={() => (ghostPosition = null)}
		>
			{#each board as cell}
				<CellComponent
					x={cell.x}
					y={cell.y}
					player={cell.player}
					isLastMove={lastMove !== null && cell.x === lastMove.x && cell.y === lastMove.y}
					{cellSize}
					onclick={() => handleCellClick(cell.x, cell.y)}
					onkeydown={(e) => e.key === 'Enter' && handleCellClick(cell.x, cell.y)} />
			{/each}
		</div>

		<WinningLine winningLine={winningLine} boardSize={GameConfig.boardSize} {cellSize} />

		{#if ghostPosition}
			<div
				class="fixed pointer-events-none rounded-full border-2 border-dashed border-gray-400 opacity-60"
				style="width: {cellSize}px; height: {cellSize}px; left: {ghostPosition.x - cellSize / 2}px; top: {ghostPosition.y - cellSize / 2}px;"
			>
				<span class="flex items-center justify-center h-full text-gray-400" style="font-size: {cellSize * 0.4}px;">?</span>
			</div>
		{/if}
	</div>
</div>
