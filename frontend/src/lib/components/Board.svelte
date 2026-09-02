<script lang="ts">
	import { onMount } from 'svelte';
	import type { Cell } from '$lib/types/game';
	import CellComponent from './Cell.svelte';
	import WinningLine from './WinningLine.svelte';
	import { calculateGhostStonePosition, isValidCell, computeCellSize } from '$lib/utils/boardUtils';
	import { vibrateOnValidMove, vibrateOnInvalidMove } from '$lib/utils/haptics';
	import { GameConfig } from '$lib/config/gameConfig';
	import { UCIConfig } from '$lib/config/uciConfig';
	import { UIConfig } from '$lib/config/uiConfig';

	interface Props {
		board: Cell[];
		onMove: (x: number, y: number) => void;
		winningLine?: Array<{ x: number; y: number }>;
		lastMove?: { x: number; y: number } | null;
		openRuleInvalid?: Set<string>;
	}

	let { board, onMove, winningLine = [], lastMove = null, openRuleInvalid = new Set<string>() }: Props = $props();

	let ghostPosition = $state<{ x: number; y: number } | null>(null);
	let cellSize = $state(
		computeCellSize(typeof window !== 'undefined' ? window.innerWidth : UIConfig.ssrViewportFallbackWidthPx)
	);
	let boardEl: HTMLDivElement | undefined = $state();

	const labelSize = $derived(
		Math.max(cellSize * UIConfig.labelSizeFraction, UIConfig.labelMinSizePx)
	);
	const labelFont = $derived(`${labelSize * 0.75}px`);
	const cols = $derived(
		Array.from({ length: GameConfig.boardSize }, (_, i) => String.fromCharCode(UCIConfig.asciiLowerA + i))
	);
	const rows = $derived(Array.from({ length: GameConfig.boardSize }, (_, i) => i + 1));

	function handleCellClick(x: number, y: number) {
		const cell = board[y * GameConfig.boardSize + x];
		if (!cell || cell.player !== 'none') {
			vibrateOnInvalidMove();
			return;
		}

		vibrateOnValidMove();
		onMove(x, y);
	}

	function clearGhost() {
		ghostPosition = null;
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
					cellSize * UIConfig.ghostStoneScale
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

<div class="w-full mx-auto" style="max-width: {UIConfig.maxContentWidthPx}px;" bind:this={boardEl}>
	<div
		class="relative inline-block"
		ontouchmove={handleTouchMove}
		ontouchend={clearGhost}
		ontouchcancel={clearGhost}
	>
		<!-- Outer wrapper: labels + grid -->
		<div
			class="grid gap-0 touch-none select-none"
			style="display: grid; grid-template-columns: {labelSize}px repeat({GameConfig.boardSize}, {cellSize}px) {labelSize}px; grid-template-rows: {labelSize}px repeat({GameConfig.boardSize}, {cellSize}px) {labelSize}px;"
		>
			<!-- Top-left corner -->
			<div></div>
			<!-- Top column labels -->
			{#each cols as col, i}
				<div class="flex items-center justify-center text-gray-400 font-mono" style="font-size: {labelFont};">{col}</div>
			{/each}
			<!-- Top-right corner -->
			<div></div>

			<!-- Board rows with row labels -->
			{#each rows as row, y}
				<!-- Left row label -->
				<div class="flex items-center justify-center text-gray-400 font-mono" style="font-size: {labelFont};">{row}</div>
				<!-- Board cells for this row -->
				{#each cols as _, x}
					{@const cell = board[y * GameConfig.boardSize + x]}
					{@const key = `${x},${y}`}
					<CellComponent
						x={x}
						y={y}
						player={cell.player}
						isLastMove={lastMove !== null && x === lastMove.x && y === lastMove.y}
						isOpenRuleInvalid={openRuleInvalid.has(key)}
						{cellSize}
						onclick={() => handleCellClick(x, y)} />
				{/each}
				<!-- Right row label -->
				<div class="flex items-center justify-center text-gray-400 font-mono" style="font-size: {labelFont};">{row}</div>
			{/each}

			<!-- Bottom-left corner -->
			<div></div>
			<!-- Bottom column labels -->
			{#each cols as col, i}
				<div class="flex items-center justify-center text-gray-400 font-mono" style="font-size: {labelFont};">{col}</div>
			{/each}
			<!-- Bottom-right corner -->
			<div></div>
		</div>

		<!-- Board background overlay for rounded corners and shadow -->
		<div
			class="absolute bg-amber-100 rounded-lg shadow-lg pointer-events-none -z-10"
			style="left: {labelSize}px; top: {labelSize}px; width: {GameConfig.boardSize * cellSize}px; height: {GameConfig.boardSize * cellSize}px;"
		></div>

		<WinningLine winningLine={winningLine} boardSize={GameConfig.boardSize} {cellSize} {labelSize} />

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
