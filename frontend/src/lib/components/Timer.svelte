<script lang="ts">
	import type { Player } from '$lib/types/game';

	interface Props {
		player: Player;
		timeRemaining: number; // seconds
		isActive: boolean;
	}

	let { player, timeRemaining, isActive }: Props = $props();

	function formatTime(seconds: number): string {
		const mins = Math.floor(seconds / 60);
		const secs = seconds % 60;
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}

	const isLowTime = $derived(timeRemaining < 60);
</script>

<div
	class="flex items-center gap-2 p-3 rounded {isActive ? 'bg-opacity-100' : 'bg-opacity-50'} {player === 'red'
		? 'bg-red-100'
		: 'bg-blue-100'}"
>
	<span class="font-semibold {player === 'red' ? 'text-red-700' : 'text-blue-700'}">
		{player === 'red' ? 'Red' : 'Blue'}
	</span>
	<span
		class="text-xl font-mono {isLowTime && isActive ? 'text-red-500 animate-pulse' : 'text-gray-700'}"
	>
		{formatTime(timeRemaining)}
	</span>
</div>
