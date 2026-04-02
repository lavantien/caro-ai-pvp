<script lang="ts">
	import type { Player } from '$lib/types/game';
	import { UIConfig } from '$lib/config/uiConfig';

	interface Props {
		x: number;
		y: number;
		player: Player;
		isLastMove?: boolean;
		onclick?: () => void;
		onkeydown?: (e: KeyboardEvent) => void;
	}

	let { x, y, player, isLastMove = false, onclick, onkeydown }: Props = $props();
</script>

<button
	onclick={onclick}
	onkeydown={onkeydown}
	class="text-2xl font-bold hover:bg-amber-200 active:bg-amber-300 transition-colors {player === 'red'
		? 'text-red-600'
		: ''} {player === 'blue' ? 'text-blue-600' : ''} {isLastMove ? 'bg-amber-300' : ''}"
	style="width: {UIConfig.cellSize}px; height: {UIConfig.cellSize}px; min-width: {UIConfig.cellSize}px; min-height: {UIConfig.cellSize}px; display: flex; align-items: center; justify-content: center; position: relative;"
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
