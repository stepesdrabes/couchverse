<script lang="ts">
	import { fade } from 'svelte/transition';
	import { Loader } from 'lucide-svelte';
	import { core } from '$lib/core';
	import { LoadStatus, type PlayerView } from '$lib/generated/core';
	import { problemMessage } from '$lib/api/problem';
	import VideoPlayer from '$lib/features/playback/components/VideoPlayer.svelte';
	import { PLAYER } from '$lib/features/playback/player';
	import type { CouchInfo } from '$lib/features/couch/api';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchJoinScreen from '$lib/features/couch/components/CouchJoinScreen.svelte';
	import CouchRemote from '$lib/features/couch/components/CouchRemote.svelte';
	import HostAwayOverlay from '$lib/features/couch/components/HostAwayOverlay.svelte';
	import * as m from '$lib/paraglide/messages';

	let { token, info }: { token: string; info: CouchInfo } = $props();

	// the core plays the host's media here: it fetches the payload, loads the player and keeps
	// it on the host's timeline
	$effect(() => core.watch(PLAYER));
	const player = $derived(core.view<PlayerView>(PLAYER));

	let joining = $state(false);
	let tried = $state(false);
	$effect(() => () => couch.disconnect());

	// the join happens on this click so the browser allows the video to play
	async function start() {
		joining = true;
		tried = true;
		await couch.join(token);
		joining = false;
	}

	const problem = $derived(tried && !joining && !couch.active ? couch.view?.problem : undefined);
	const playable = $derived(
		player?.status === LoadStatus.Loaded || player?.status === LoadStatus.Stale
	);
</script>

{#if !couch.active}
	<CouchJoinScreen
		{info}
		{joining}
		error={problem ? problemMessage(problem, m.couch_join_failed()) : ''}
		onstart={start}
	/>
{:else if couch.isRemote}
	<!-- the host's account on another device: it steers the host's player, playing nothing -->
	<CouchRemote />
{:else if player && playable}
	<!-- it shows the host's absence itself, so the video stays loaded meanwhile -->
	<VideoPlayer view={player} />
{:else if couch.waiting}
	<div class="relative h-dvh w-full bg-black">
		<HostAwayOverlay />
	</div>
{:else}
	<div class="flex h-dvh w-full items-center justify-center bg-black" transition:fade>
		<Loader class="size-7 animate-spin text-accent-ink" />
	</div>
{/if}
