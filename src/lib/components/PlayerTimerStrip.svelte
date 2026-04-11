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
		if (!isActive) return Math.round(serverTimeBase);
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
		seconds = Math.round(seconds);
		const mins = Math.floor(seconds / 60);
		const secs = seconds % 60;
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}

	const isLowTime = $derived(displayTime() < UIConfig.lowTimeThresholdSeconds);
</script>

<div
	class="flex items-center gap-2 px-3 py-1.5 rounded-md w-full max-w-[1024px] mx-auto transition-colors {isActive
		? player === 'red'
			? 'bg-red-50 border border-red-200'
			: 'bg-blue-50 border border-blue-200'
		: 'bg-gray-50 border border-gray-200 opacity-60'}"
>
	<span class="w-2.5 h-2.5 rounded-full shrink-0 {player === 'red' ? 'bg-red-500' : 'bg-blue-500'}"></span>
	<span class="text-sm font-semibold {player === 'red' ? 'text-red-700' : 'text-blue-700'}">
		{player === 'red' ? 'Red' : 'Blue'}
	</span>
	<span
		class="text-lg font-mono font-bold ml-auto {isLowTime && isActive
			? 'text-red-500 animate-pulse'
			: 'text-gray-800'}"
	>
		{formatTime(displayTime())}
	</span>
</div>
