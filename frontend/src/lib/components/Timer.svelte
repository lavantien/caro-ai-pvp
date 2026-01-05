<script lang="ts">
	import type { Player } from '$lib/types/game';
	import { onMount, onDestroy } from 'svelte';

	interface Props {
		player: Player;
		timeRemaining: number; // seconds from server ( authoritative )
		isActive: boolean;
		onTimeOut?: () => void;
		gameId?: string; // Optional: used for periodic server sync
	}

	let { player, isActive, onTimeOut, timeRemaining: propTimeRemaining, gameId }: Props = $props();

	// Server time base and timestamp - used to calculate display time
	let serverTimeBase = $state(propTimeRemaining);
	let serverTimeTimestamp = $state(Date.now());

	// Sync with prop value when it changes (after move, undo, etc.)
	$effect(() => {
		serverTimeBase = propTimeRemaining;
		serverTimeTimestamp = Date.now();
	});

	let displayInterval: ReturnType<typeof setInterval> | null = null;
	let hasTriggeredTimeout = $state(false);

	// Calculate display time based on elapsed time since last server sync
	const displayTime = $derived(() => {
		if (!isActive) return serverTimeBase;
		const elapsed = (Date.now() - serverTimeTimestamp) / 1000;
		const calculated = Math.max(0, Math.round(serverTimeBase - elapsed));
		return calculated;
	});

	// Trigger timeout when display time reaches 0
	$effect(() => {
		const current = displayTime();
		if (current <= 0 && isActive && !hasTriggeredTimeout) {
			hasTriggeredTimeout = true;
			if (onTimeOut) onTimeOut();
		}
	});

	// High-frequency update for smooth display (100ms)
	$effect(() => {
		if (isActive) {
			displayInterval = setInterval(() => {
				// Force reactivity by reading displayTime
				const _ = displayTime();
			}, 100);
		} else {
			if (displayInterval) {
				clearInterval(displayInterval);
				displayInterval = null;
			}
		}

		return () => {
			if (displayInterval) clearInterval(displayInterval);
		};
	});

	// Optional: Periodic server sync if gameId is provided
	// This keeps the timer in sync with server time
	let syncInterval: ReturnType<typeof setInterval> | null = null;

	$effect(() => {
		if (isActive && gameId) {
			syncInterval = setInterval(async () => {
				try {
					const apiUrl = import.meta.env.VITE_API_BASE_URL || 'http://localhost:5207';
					const response = await fetch(`${apiUrl}/api/game/${gameId}`);
					if (response.ok) {
						const data = await response.json();
						const serverTime = player === 'red' ? data.state.redTimeRemaining : data.state.blueTimeRemaining;
						serverTimeBase = serverTime;
						serverTimeTimestamp = Date.now();
					}
				} catch {
					// Ignore sync errors - continue with local calculation
				}
			}, 500); // Sync every 500ms
		} else {
			if (syncInterval) {
				clearInterval(syncInterval);
				syncInterval = null;
			}
		}

		return () => {
			if (syncInterval) clearInterval(syncInterval);
		};
	});

	function formatTime(seconds: number): string {
		if (seconds < 0) seconds = 0;
		const mins = Math.floor(seconds / 60);
		const secs = seconds % 60;
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}

	const isLowTime = $derived(displayTime() < 60);
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
		{formatTime(displayTime())}
	</span>
</div>
