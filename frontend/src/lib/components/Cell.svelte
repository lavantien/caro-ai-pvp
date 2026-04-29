<script lang="ts">
	import type { Player } from '$lib/types/game';

	interface Props {
		x: number;
		y: number;
		player: Player;
		isLastMove?: boolean;
		isOpenRuleInvalid?: boolean;
		cellSize: number;
		onclick?: () => void;
		onkeydown?: (e: KeyboardEvent) => void;
	}

	let { x, y, player, isLastMove = false, isOpenRuleInvalid = false, cellSize, onclick, onkeydown }: Props = $props();
</script>

<button
	onclick={onclick}
	onkeydown={onkeydown}
	class="font-bold hover:bg-amber-200 active:bg-amber-300 transition-colors border border-amber-300 {player === 'red'
		? 'text-red-600'
		: ''} {player === 'blue' ? 'text-blue-600' : ''} {isLastMove ? 'bg-amber-300' : ''} {isOpenRuleInvalid ? 'bg-red-50 opacity-50' : ''}"
	style="width: {cellSize}px; height: {cellSize}px; min-width: {cellSize}px; min-height: {cellSize}px; font-size: {cellSize * 0.5}px; display: flex; align-items: center; justify-content: center; position: relative;"
	aria-label="Cell {x},{y}"
	data-x={x}
	data-y={y}
>
	{#if player === 'red'}
		O
	{:else if player === 'blue'}
		X
	{/if}
	{#if isLastMove && player !== 'none'}
		<span
			class="absolute inset-0 border-2 {player === 'red' ? 'border-red-400' : 'border-blue-400'} rounded-sm pointer-events-none"
		></span>
	{/if}
</button>
