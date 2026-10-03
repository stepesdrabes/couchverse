<script lang="ts">
	import { goto } from '$app/navigation';
	import { ArrowLeft, House, Loader, RotateCw } from 'lucide-svelte';
	import { untrack } from 'svelte';
	import { core } from '$lib/core';
	import { LoadStatus, type PlayTarget, type PlayerView } from '$lib/generated/core';
	import VideoPlayer from '$lib/features/playback/components/VideoPlayer.svelte';
	import { PLAYER, sameTarget, watchPath } from '$lib/features/playback/player';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: { target: PlayTarget } } = $props();

	$effect(() => core.watch(PLAYER));
	const view = $derived(core.view<PlayerView>(PLAYER));

	// What this page last asked the core to play. The URL drives the core (a link, Back), and
	// the core moves the URL on when it starts another episode by itself (next up, a remote).
	let requested: PlayTarget | undefined;

	$effect(() => {
		const target = data.target;
		untrack(() => {
			if (sameTarget(requested, target)) return;
			requested = target;
			if (!sameTarget(view?.target, target)) {
				void core.send({ type: 'playRequested', content: target });
			}
		});
	});

	$effect(() => {
		const playing = view?.target;
		if (!playing || sameTarget(playing, requested)) return;
		requested = playing;
		goto(watchPath(playing), { keepFocus: true, noScroll: true });
	});

	// leaving the player saves progress and frees the stream, and so does closing the tab
	$effect(() => {
		const close = () => void core.send({ type: 'playerClosed' });
		window.addEventListener('pagehide', close);
		return () => {
			window.removeEventListener('pagehide', close);
			close();
		};
	});

	const retry = () => void core.send({ type: 'playRequested', content: data.target });
	const playable = $derived(
		view?.status === LoadStatus.Loaded || view?.status === LoadStatus.Stale
	);
</script>

<svelte:head>
	<title>{view?.title ? m.player_page_title({ title: view.title }) : 'Couchverse'}</title>
</svelte:head>

{#if view && playable}
	<VideoPlayer {view} />
{:else if view?.status === LoadStatus.NotFound}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-4 px-6 text-center">
		<h1 class="text-xl font-bold">{m.error_not_found()}</h1>
		<a
			href="/"
			class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60
				px-5 text-sm font-semibold transition-colors hover:border-faint hover:bg-surface-2"
		>
			<House class="size-4" />
			{m.error_go_home()}
		</a>
	</div>
{:else if view?.status === LoadStatus.Failed && view.problem?.code === 'unsupported'}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-4 px-6 text-center">
		<h1 class="text-xl font-bold">{m.player_needs_transcoding_title()}</h1>
		<p class="max-w-md text-sm text-muted">
			{m.player_needs_transcoding_description()}
		</p>
		<a
			href="/title/{view.titleSlug}"
			class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60
				px-5 text-sm font-semibold transition-colors hover:border-faint hover:bg-surface-2"
		>
			<ArrowLeft class="size-4" />
			{m.player_back_to_title()}
		</a>
	</div>
{:else if view?.status === LoadStatus.Failed}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-4 px-6 text-center">
		<h1 class="text-xl font-bold">{m.error_page_title()}</h1>
		<button
			class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60
				px-5 text-sm font-semibold transition-colors hover:border-faint hover:bg-surface-2"
			onclick={retry}
			data-tv-autofocus
		>
			<RotateCw class="size-4" />
			{m.common_retry()}
		</button>
	</div>
{:else if view?.preparing !== undefined}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-5 px-6 text-center">
		<Loader class="size-7 animate-spin text-accent-ink" />
		<h1 class="text-xl font-bold">{m.player_preparing_title()}</h1>
		<p class="max-w-md text-sm text-muted">
			{m.player_preparing_description()}
		</p>
		<div class="h-1.5 w-64 overflow-hidden rounded-full bg-surface-2">
			<div
				class="h-full rounded-full bg-accent transition-all duration-700"
				style="width: {view.preparing}%"
			></div>
		</div>
	</div>
{:else}
	<div class="flex min-h-dvh items-center justify-center">
		<Loader class="size-7 animate-spin text-accent-ink" />
	</div>
{/if}
