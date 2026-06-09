<script lang="ts">
	import { page } from '$app/state';
	import TopNav from '$lib/components/layout/TopNav.svelte';

	let { children } = $props();

	// the player page is fully immersive
	const watching = $derived(page.route.id?.includes('/watch/') ?? false);
</script>

{#if !watching}
	<TopNav />
{/if}

<main class="min-h-dvh">
	{@render children()}
</main>
