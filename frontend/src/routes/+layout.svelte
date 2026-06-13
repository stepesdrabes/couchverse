<script lang="ts">
	import '../app.css';
	import { onNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { Toaster } from 'svelte-sonner';
	import PlayerBar from '$lib/features/music/components/PlayerBar.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import UploadDock from '$lib/features/uploads/components/UploadDock.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';

	let { children } = $props();

	const onWatch = $derived(page.route.id?.includes('/watch/') ?? false);
	const onAuth = $derived(page.route.id?.includes('(auth)') ?? false);

	// the music bar yields to the video player and the login screen
	const showPlayerBar = $derived(!onWatch && !onAuth && features.musicEnabled);
	// the upload dock hides over the player too, but the unload guard below stays
	const showUploadDock = $derived(!onWatch && !onAuth);

	// warn before closing/reloading the tab while an upload could be lost. Lives
	// in the always-mounted root layout so it holds even on the /watch player.
	$effect(() => {
		if (!uploadQueue.inFlight) return;
		const onBeforeUnload = (e: BeforeUnloadEvent) => {
			e.preventDefault();
			e.returnValue = ''; // some browsers need returnValue set to show the prompt
		};
		window.addEventListener('beforeunload', onBeforeUnload);
		return () => window.removeEventListener('beforeunload', onBeforeUnload);
	});

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
	<title>Couchverse</title>
</svelte:head>

{@render children()}

{#if showPlayerBar}
	<PlayerBar />
{/if}

{#if showUploadDock}
	<UploadDock playerBarVisible={showPlayerBar} />
{/if}

<Toaster
	theme="dark"
	position="bottom-right"
	toastOptions={{
		style:
			'background: var(--color-surface-2); border: 1px solid var(--color-edge); color: var(--color-text); border-radius: var(--radius-card);'
	}}
/>
