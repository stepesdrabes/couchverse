<script lang="ts">
	import type { PlaybackInfo, PlaybackKind } from '$lib/features/playback/api';
	import { invalidateAll } from '$app/navigation';
	import { ArrowLeft, Loader } from 'lucide-svelte';
	import VideoPlayer from '$lib/features/playback/components/VideoPlayer.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		data
	}: { data: { info: PlaybackInfo; kind: PlaybackKind; id: string; jitSessionId: string | null } } =
		$props();

	// while transcoding, poll until the stream becomes playable
	$effect(() => {
		if (data.info.mode !== 'preparing') return;
		const t = setInterval(() => invalidateAll(), 3000);
		return () => clearInterval(t);
	});
</script>

<svelte:head>
	<title>{m.player_page_title({ title: data.info.display.title })}</title>
</svelte:head>

{#if data.info.mode === 'direct' || data.info.mode === 'hls'}
	{#key data.id}
		<VideoPlayer
			info={data.info}
			titleId={data.kind === 'movie' ? data.id : null}
			episodeId={data.kind === 'episode' ? data.id : null}
			jitSessionId={data.jitSessionId}
		/>
	{/key}
{:else if data.info.mode === 'preparing'}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-5 px-6 text-center">
		<Loader class="size-7 animate-spin text-accent" />
		<h1 class="text-xl font-bold">{m.player_preparing_title()}</h1>
		<p class="max-w-md text-sm text-muted">
			{m.player_preparing_description()}
		</p>
		<div class="h-1.5 w-64 overflow-hidden rounded-full bg-surface-2">
			<div
				class="h-full rounded-full bg-accent transition-all duration-700"
				style="width: {data.info.jobProgress ?? 0}%"
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
			href="/title/{data.info.display.titleSlug}"
			class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60
				px-5 text-sm font-semibold transition-colors hover:border-faint hover:bg-surface-2"
		>
			<ArrowLeft class="size-4" />
			{m.player_back_to_title()}
		</a>
	</div>
{/if}
