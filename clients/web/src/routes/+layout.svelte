<script lang="ts">
	import '../app.css';
	import 'flag-icons/css/flag-icons.min.css';
	import { afterNavigate, onNavigate } from '$app/navigation';
	import { page } from '$app/state';
	import { Toaster } from 'svelte-sonner';
	import { core } from '$lib/core';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchBar from '$lib/features/couch/components/CouchBar.svelte';
	import NavProgress from '$lib/components/layout/NavProgress.svelte';
	import UploadDock from '$lib/features/uploads/components/UploadDock.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import { applyPalette } from '$lib/theme';
	import TvShell from '$lib/tv/TvShell.svelte';
	import { isTV } from '$lib/tv/tv';

	let { children } = $props();

	// the site accent is the core session's, the login screen's included; an error page
	// can render without a started core
	$effect(() => {
		if (core.started) applyPalette(core.session.accent);
	});

	const onWatch = $derived(page.route.id?.includes('/watch/') ?? false);
	const onAuth = $derived(page.route.id?.includes('(auth)') ?? false);
	// the couch player is fully immersive, like /watch
	const onCouch = $derived(page.route.id?.includes('/couch/') ?? false);

	// the upload dock hides over the player and the login screen, but the unload
	// guard below stays; TVs never upload
	const showUploadDock = $derived(!onWatch && !onCouch && !onAuth && !isTV);

	// the on-screen video player hosts the couch bar itself (so it survives
	// fullscreen); the layout only shows it when no player is mounted
	const showCouchBar = $derived(couch.active && !couch.playerMounted);

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

	// soft cross-fade between pages via the View Transitions API (not on TVs, whose
	// GPUs stutter through it)
	onNavigate((navigation) => {
		if (isTV || !document.startViewTransition) return;
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

<NavProgress />

{#if isTV}
	<TvShell />
{/if}

{@render children()}

<!-- bottom-right corner stack: upload dock on top, couch bar at the very bottom -->
{#if showUploadDock || showCouchBar}
	<div data-tv-pin class="fixed right-4 bottom-4 z-40 flex flex-col items-end gap-3">
		{#if showUploadDock}
			<UploadDock />
		{/if}
		{#if showCouchBar}
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
