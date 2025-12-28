<script lang="ts">
	import favicon from '$lib/assets/favicon.svg';
	import '../app.pcss';

	let { children } = $props();
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<link rel="stylesheet" href={''} />
</svelte:head>

{@render children()}
