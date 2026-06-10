<script lang="ts">
	import '../app.css';
	import { onNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { Toaster } from 'svelte-sonner';
	import favicon from '$lib/assets/favicon.svg';
	import PlayerBar from '$lib/components/music/PlayerBar.svelte';
	import { features } from '$lib/features/settings/features.svelte';

	let { children } = $props();

	// the music bar yields to the video player and the login screen
	const showPlayerBar = $derived(
		!(page.route.id?.includes('/watch/') ?? false) &&
			!(page.route.id?.includes('(auth)') ?? false) &&
			features.musicEnabled
	);

	// soft cross-fade between pages via the View Transitions API
	onNavigate((navigation) => {
		if (!document.startViewTransition) return;
		return new Promise((resolve) => {
			document.startViewTransition(async () => {
				resolve();
				await navigation.complete;
			});
		});
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} />
	<title>Couchverse</title>
</svelte:head>

{@render children()}

{#if showPlayerBar}
	<PlayerBar />
{/if}

<Toaster
	theme="dark"
	position="bottom-right"
	toastOptions={{
		style:
			'background: var(--color-surface-2); border: 1px solid var(--color-edge); color: var(--color-text); border-radius: var(--radius-card);'
	}}
/>
