<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.pcss';
	import { page } from '$app/stores';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<link rel="stylesheet" href={''} />
</svelte:head>

<nav class="bg-slate-900 border-b border-slate-700">
	<div class="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between">
		<a href="/" class="text-xl font-bold text-white hover:text-slate-300 transition-colors">
			Caro AI
		</a>
		<div class="flex gap-4">
			<a
				href="/game"
				class="px-3 py-2 rounded-lg transition-colors {$page.url.pathname === '/game'
					? 'bg-blue-600 text-white'
					: 'text-slate-300 hover:bg-slate-800'}">
				Play Game
			</a>
			<a
				href="/tournament"
				class="px-3 py-2 rounded-lg transition-colors {$page.url.pathname === '/tournament'
					? 'bg-purple-600 text-white'
					: 'text-slate-300 hover:bg-slate-800'}">
				Tournament
			</a>
		</div>
	</div>
</nav>

{@render children()}
