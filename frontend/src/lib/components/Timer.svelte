<script lang="ts">
	import type { Player } from '$lib/types/game';
	import { UIConfig } from '$lib/config/uiConfig';

	interface Props {
		player: Player;
		timeRemaining: number;
		isActive: boolean;
		onTimeOut?: () => void;
	}

	let { player, isActive, onTimeOut, timeRemaining: propTimeRemaining }: Props = $props();

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
		tick;
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
