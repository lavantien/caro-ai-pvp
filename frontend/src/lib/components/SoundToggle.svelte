<script lang="ts">
	import { soundManager } from '$lib/utils/sound';
	import { onMount } from 'svelte';

	let isMuted = $state(soundManager.isMuted());

	function toggleSound() {
		soundManager.toggleMute();
		isMuted = soundManager.isMuted();
	}

	// Initialize isMuted state from soundManager
	onMount(() => {
		isMuted = soundManager.isMuted();
	});
</script>

<button
	onclick={toggleSound}
	class="p-2 rounded-full hover:bg-gray-200 transition-colors focus:outline-none focus:ring-2 focus:ring-blue-500"
	aria-label={isMuted ? 'Unmute' : 'Mute'}
	title={isMuted ? 'Unmute sounds' : 'Mute sounds'}
>
	{#if isMuted}
		<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6">
			<path stroke-linecap="round" stroke-linejoin="round" d="M17.25 9.75 19.5 12m0 0 2.25 2.25M19.5 12l2.25-2.25M19.5 12l-2.25 2.25m-10.5-6 4.72-4.72a.75.75 0 0 1 1.28.53v15.88a.75.75 0 0 1-1.28.53L6.75 15.75H4.5a.75.75 0 0 1-.75-.75V9a.75.75 0 0 1 .75-.75h2.25Z" />
		</svg>
	{:else}
		<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="w-6 h-6">
			<path stroke-linecap="round" stroke-linejoin="round" d="M19.114 5.636a9 9 0 0 1 0 12.728M16.463 8.288a5.25 5.25 0 0 1 0 7.424M6.75 8.25l4.72-4.72a.75.75 0 0 1 1.28.53v15.88a.75.75 0 0 1-1.28.53L6.75 15.75H4.5a.75.75 0 0 1-.75-.75V9a.75.75 0 0 1 .75-.75h2.25Z" />
		</svg>
	{/if}
</button>
