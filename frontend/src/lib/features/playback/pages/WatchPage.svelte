<script lang="ts">
	import type { PlaybackInfo, PlaybackKind } from '$lib/features/playback/api';
	import { invalidate } from '$app/navigation';
	import { ArrowLeft, House, Loader } from 'lucide-svelte';
	import VideoPlayer from '$lib/features/playback/components/VideoPlayer.svelte';
	import * as m from '$lib/paraglide/messages';

	type Resolved = { info: PlaybackInfo; jitSessionId: string | null };

	let {
		data
	}: {
		data: { kind: PlaybackKind; id: string; playback: Promise<Resolved> };
	} = $props();

	// Resolve the streamed playback into local state instead of {#await}, so a
	// transcode-poll re-fetch (same media) updates in place without flashing the
	// loader; only a genuine media change (new id) clears back to the loader.
	let resolved = $state<Resolved | null>(null);
	let failed = $state(false);
	let shownId = '';

	$effect(() => {
		const { id, playback } = data;
		if (id !== shownId) {
			resolved = null;
			failed = false;
			shownId = id;
		}
		let live = true;
		playback.then(
			(p) => {
				if (live) {
					resolved = p;
					failed = false;
				}
			},
			() => {
				if (live && !resolved) failed = true;
			}
		);
		return () => {
			live = false;
		};
	});

	const info = $derived(resolved?.info ?? null);
	const preparing = $derived(info?.mode === 'preparing');

	// while transcoding, poll until the stream becomes playable. Targeted invalidate
	// re-runs only this load, not the layout's session/features/preferences fetch.
	$effect(() => {
		if (!preparing) return;
		const t = setInterval(() => invalidate('app:playback'), 3000);
		return () => clearInterval(t);
	});
</script>

<svelte:head>
	<title>{info ? m.player_page_title({ title: info.display.title }) : 'Couchverse'}</title>
</svelte:head>

{#if failed}
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
{:else if !resolved || !info}
	<div class="flex min-h-dvh items-center justify-center">
		<Loader class="size-7 animate-spin text-accent" />
	</div>
{:else if info.mode === 'direct' || info.mode === 'hls'}
	{#key data.id}
		<VideoPlayer
			{info}
			titleId={data.kind === 'movie' ? data.id : null}
			episodeId={data.kind === 'episode' ? data.id : null}
			jitSessionId={resolved.jitSessionId}
		/>
	{/key}
{:else if info.mode === 'preparing'}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-5 px-6 text-center">
		<Loader class="size-7 animate-spin text-accent" />
		<h1 class="text-xl font-bold">{m.player_preparing_title()}</h1>
		<p class="max-w-md text-sm text-muted">
			{m.player_preparing_description()}
		</p>
		<div class="h-1.5 w-64 overflow-hidden rounded-full bg-surface-2">
			<div
				class="h-full rounded-full bg-accent transition-all duration-700"
				style="width: {info.jobProgress ?? 0}%"
			></div>
		</div>
	</div>
{:else}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-4 px-6 text-center">
		<h1 class="text-xl font-bold">{m.player_needs_transcoding_title()}</h1>
		<p class="max-w-md text-sm text-muted">
			{m.player_needs_transcoding_description()}
		</p>
		<a
			href="/title/{info.display.titleSlug}"
			class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60
				px-5 text-sm font-semibold transition-colors hover:border-faint hover:bg-surface-2"
		>
			<ArrowLeft class="size-4" />
			{m.player_back_to_title()}
		</a>
	</div>
{/if}
