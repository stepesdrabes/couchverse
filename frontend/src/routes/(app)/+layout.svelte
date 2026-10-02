<script lang="ts">
	import { page } from '$app/state';
	import TopNav from '$lib/components/layout/TopNav.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import AchievementWatcher from '$lib/features/ranks/components/AchievementWatcher.svelte';

	let { children } = $props();

	// the player page is fully immersive
	const watching = $derived(page.route.id?.includes('/watch/') ?? false);
</script>

{#if !watching}
	<TopNav />
{/if}

{#if features.rankingsEnabled}
	<!-- renders nothing; turns queued unlocks into toasts everywhere except
	     inside the player, which mounts its own fullscreen-safe overlay -->
	<AchievementWatcher />
{/if}

<main class="min-h-dvh">
	{@render children()}
</main>
