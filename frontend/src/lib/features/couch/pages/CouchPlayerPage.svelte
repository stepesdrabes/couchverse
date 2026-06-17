<script lang="ts">
	import { onMount } from 'svelte';
	import { fade } from 'svelte/transition';
	import { Loader } from 'lucide-svelte';
	import VideoPlayer from '$lib/features/playback/components/VideoPlayer.svelte';
	import type { CouchInfo } from '$lib/features/couch/api';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchJoinScreen from '$lib/features/couch/components/CouchJoinScreen.svelte';
	import HostAwayOverlay from '$lib/features/couch/components/HostAwayOverlay.svelte';

	let { token, info }: { token: string; info: CouchInfo } = $props();

	let joining = $state(false);
	let joinError = $state(false);

	onMount(() => () => couch.leave());

	// the join happens on this click so the browser allows autoplay
	async function start() {
		joining = true;
		joinError = false;
		try {
			await couch.joinByToken(token);
		} catch {
			joinError = true;
			joining = false;
		}
	}

	const ids = $derived.by(() => {
		const ref = couch.hostState?.media;
		if (ref?.kind === 'movie') return { titleId: ref.titleId ?? null, episodeId: null };
		if (ref?.kind === 'episode') return { titleId: null, episodeId: ref.episodeId ?? null };
		return { titleId: null, episodeId: null };
	});

	const choosing = $derived((couch.hostState?.media.kind ?? '') === '');
</script>

{#if !couch.active}
	<CouchJoinScreen {info} {joining} error={joinError} onstart={start} />
{:else if couch.playerInfo}
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
	<div class="flex h-dvh w-full items-center justify-center bg-black" transition:fade>
		<Loader class="size-7 animate-spin text-accent" />
	</div>
{/if}
