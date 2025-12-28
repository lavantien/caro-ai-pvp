<script lang="ts">
	import type { Player } from '$lib/types/game';

	interface Props {
		x: number;
		y: number;
		player: Player;
		onclick?: () => void;
		onkeydown?: (e: KeyboardEvent) => void;
	}

	let { x, y, player, onclick, onkeydown }: Props = $props();
</script>

<button
	onclick={onclick}
	onkeydown={onkeydown}
	class="w-10 h-10 text-2xl font-bold hover:bg-amber-200 active:bg-amber-300 transition-colors {player ===
		'red'
		? 'text-red-600'
		: ''} {player === 'blue' ? 'text-blue-600' : ''}"
	style="width: 40px; height: 40px; min-width: 40px; min-height: 40px; display: flex; align-items: center; justify-content: center;"
	aria-label="Cell {x},{y}"
	data-x={x}
	data-y={y}
>
	{#if player === 'red'}
		O
	{:else if player === 'blue'}
		X
	{/if}
</button>
