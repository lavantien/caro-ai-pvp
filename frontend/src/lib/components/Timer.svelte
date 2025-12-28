<script lang="ts">
	import type { Player } from '$lib/types/game';
	import { onMount, onDestroy } from 'svelte';

	interface Props {
		player: Player;
		timeRemaining: number; // seconds
		isActive: boolean;
		onTimeOut?: () => void;
	}

	let { player, timeRemaining: initialTime, isActive, onTimeOut }: Props = $props();

	// Local state for countdown
	let timeRemaining = $state(initialTime);

	// Update when initial time changes (e.g., after backend sync)
	$effect(() => {
		timeRemaining = initialTime;
	});

	let interval: ReturnType<typeof setInterval> | null = null;

	// Start countdown when active
	$effect(() => {
		if (isActive) {
			interval = setInterval(() => {
				timeRemaining--;

				if (timeRemaining <= 0) {
					if (interval) clearInterval(interval);
					if (onTimeOut) onTimeOut();
				}
			}, 1000);
		} else {
			if (interval) {
				clearInterval(interval);
				interval = null;
			}
		}

		return () => {
			if (interval) clearInterval(interval);
		};
	});

	function formatTime(seconds: number): string {
		if (seconds < 0) seconds = 0;
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
