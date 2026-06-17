<script lang="ts">
	import { onMount } from 'svelte';
	import { Loader } from 'lucide-svelte';
	import VideoPlayer from '$lib/features/playback/components/VideoPlayer.svelte';
	import type { PlaybackInfo } from '$lib/features/playback/api';
	import { couch } from '$lib/features/couch/couch.svelte';
	import HostAwayOverlay from '$lib/features/couch/components/HostAwayOverlay.svelte';
	import type { Snapshot } from '$lib/features/couch/types';

	let {
		snapshot,
		player,
		jitSessionId
	}: { snapshot: Snapshot; player: PlaybackInfo | null; jitSessionId: string | null } = $props();

	onMount(() => {
		couch.joinFromSnapshot(snapshot, player, jitSessionId);
		return () => couch.leave();
	});

	// the host's current media drives which title/episode the player reports
	const ids = $derived.by(() => {
		const ref = couch.hostState?.media;
		if (ref?.kind === 'movie') return { titleId: ref.titleId ?? null, episodeId: null };
		if (ref?.kind === 'episode') return { titleId: null, episodeId: ref.episodeId ?? null };
		return { titleId: null, episodeId: null };
	});

	const choosing = $derived((couch.hostState?.media.kind ?? '') === '');
</script>

{#if couch.playerInfo}
	{#key couch.mediaKey}
		<VideoPlayer
			info={couch.playerInfo}
			titleId={ids.titleId}
			episodeId={ids.episodeId}
			jitSessionId={couch.jitSessionId}
		/>
	{/key}
{:else if choosing}
	<div class="relative h-dvh w-full bg-black">
		<HostAwayOverlay />
	</div>
{:else}
	<div class="flex h-dvh w-full items-center justify-center bg-black">
		<Loader class="size-7 animate-spin text-accent" />
	</div>
{/if}
