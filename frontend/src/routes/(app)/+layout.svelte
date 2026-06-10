<script lang="ts">
	import { page } from '$app/state';
	import TopNav from '$lib/components/layout/TopNav.svelte';
	import { musicPlayer } from '$lib/features/music/player.svelte';

	let { children } = $props();

	// the player page is fully immersive
	const watching = $derived(page.route.id?.includes('/watch/') ?? false);
</script>

{#if !watching}
	<TopNav />
{/if}

<main class="min-h-dvh {musicPlayer.current && !watching ? 'pb-24' : ''}">
	{@render children()}
</main>
