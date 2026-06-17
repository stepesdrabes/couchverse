<script lang="ts">
	import '../app.css';
	import 'flag-icons/css/flag-icons.min.css';
	import { afterNavigate, onNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { Toaster } from 'svelte-sonner';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchBar from '$lib/features/couch/components/CouchBar.svelte';
	import { musicPlayer } from '$lib/features/music/player.svelte';
	import PlayerBar from '$lib/features/music/components/PlayerBar.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import UploadDock from '$lib/features/uploads/components/UploadDock.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';

	let { children } = $props();

	const onWatch = $derived(page.route.id?.includes('/watch/') ?? false);
	const onAuth = $derived(page.route.id?.includes('(auth)') ?? false);
	// the couch player is fully immersive, like /watch
	const onCouch = $derived(page.route.id?.includes('/couch/') ?? false);

	// the music bar yields to the video player and the login screen
	const showPlayerBar = $derived(!onWatch && !onCouch && !onAuth && features.musicEnabled);
	// the full-width music bar is only actually on screen when a track is loaded;
	// the corner stack clears it only then (otherwise it sits at the bottom)
	const musicBarVisible = $derived(showPlayerBar && !!musicPlayer.current);
	// the upload dock hides over the player too, but the unload guard below stays
	const showUploadDock = $derived(!onWatch && !onCouch && !onAuth);

	// where the bottom-right corner stack sits: on the immersive player it rises
	// above the controls while they're on screen so the couch bar never covers them
	const immersive = $derived(onWatch || onCouch);
	const stackBottom = $derived(
		immersive
			? couch.playerControlsVisible
				? '5.5rem'
				: '1rem'
			: musicBarVisible
				? '5.75rem'
				: '1rem'
	);

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

	// while hosting a couch, keep the session's media in step with the host's
	// navigation: switching episode/title propagates, leaving the player -> "choosing"
	afterNavigate(() => {
		if (!couch.isHost) return;
		if (page.route.id?.includes('/watch/')) {
			couch.setHostMedia(page.params.kind ?? '', page.params.id ?? '');
		} else {
			couch.setHostMedia('', '');
		}
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

<!-- bottom-right corner stack: upload dock on top, couch bar at the very bottom.
	The whole stack clears the music bar only when a track is actually playing, so
	with no couch session and no music it sits at the bottom. -->
{#if showUploadDock || couch.active}
	<div
		class="fixed right-4 z-40 flex flex-col items-end gap-3"
		style="bottom: {stackBottom}; transition: bottom 0.25s ease;"
	>
		{#if showUploadDock}
			<UploadDock />
		{/if}
		{#if couch.active}
			<CouchBar />
		{/if}
	</div>
{/if}

<Toaster
	theme="dark"
	position="bottom-right"
	toastOptions={{
		style:
			'background: var(--color-surface-2); border: 1px solid var(--color-edge); color: var(--color-text); border-radius: var(--radius-card);'
	}}
/>
