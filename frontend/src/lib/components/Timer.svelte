<script lang="ts">
	import type { Player } from '$lib/types/game';
	import { ApiConfig } from '$lib/config/apiConfig';
	import { UIConfig } from '$lib/config/uiConfig';

	interface Props {
		player: Player;
		timeRemaining: number; // seconds from server ( authoritative )
		isActive: boolean;
		onTimeOut?: () => void;
		gameId?: string; // Optional: used for periodic server sync
	}

	let { player, isActive, onTimeOut, timeRemaining: propTimeRemaining, gameId }: Props = $props();

	let serverTimeBase = $state(0);
	let serverTimeTimestamp = $state(Date.now());
	let hasTriggeredTimeout = $state(false);
	let tick = $state(0);

	$effect(() => {
		serverTimeBase = propTimeRemaining;
		serverTimeTimestamp = Date.now();
		hasTriggeredTimeout = false;
	});

	const displayTime = $derived(() => {
		tick; // reactive dependency for timer updates
		if (!isActive) return serverTimeBase;
		const elapsed = (Date.now() - serverTimeTimestamp) / 1000;
		return Math.max(0, Math.round(serverTimeBase - elapsed));
	});

	$effect(() => {
		const current = displayTime();
		if (current <= 0 && isActive && !hasTriggeredTimeout) {
			hasTriggeredTimeout = true;
			if (onTimeOut) onTimeOut();
		}
	});

	$effect(() => {
		if (isActive) {
			const id = setInterval(() => tick++, UIConfig.timerUpdateIntervalMs);
			return () => clearInterval(id);
		}
	});

	// Periodic server sync if gameId is provided
	let syncInterval: ReturnType<typeof setInterval> | null = null;

	$effect(() => {
		if (isActive && gameId) {
			syncInterval = setInterval(async () => {
				try {
					const response = await fetch(`${ApiConfig.baseUrl}${ApiConfig.endpoints.game(gameId!)}`);
					if (response.ok) {
						const data = await response.json();
						const serverTime = player === 'red' ? data.state.redTimeRemaining : data.state.blueTimeRemaining;
						serverTimeBase = serverTime;
						serverTimeTimestamp = Date.now();
					}
				} catch {
					// Ignore sync errors - continue with local calculation
				}
			}, UIConfig.timerSyncIntervalMs);
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

	const isLowTime = $derived(displayTime() < UIConfig.lowTimeThresholdSeconds);
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
