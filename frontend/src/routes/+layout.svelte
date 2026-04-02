<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.pcss';
	import { page } from '$app/stores';
	import SoundToggle from '$lib/components/SoundToggle.svelte';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
</svelte:head>

<nav class="bg-slate-900 border-b border-slate-700">
	<div class="max-w-7xl mx-auto px-3 py-2 sm:px-4 sm:py-3 flex items-center justify-between">
		<a href="/" class="text-lg sm:text-xl font-bold text-white hover:text-slate-300 transition-colors">
			Caro AI
		</a>
		<div class="flex items-center gap-2">
			<a
				href="/game"
				class="px-3 py-1.5 rounded-lg text-sm transition-colors {$page.url.pathname === '/game'
					? 'bg-blue-600 text-white'
					: 'text-slate-300 hover:bg-slate-800'}">
				Play
			</a>
			<SoundToggle />
		</div>
	</div>
</nav>

{@render children()}
