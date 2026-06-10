<script lang="ts">
	import { Music, Play, Shuffle } from 'lucide-svelte';
	import TrackList from '$lib/features/music/components/TrackList.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import { musicPlayer as player } from '$lib/features/music/player.svelte';

	let { data } = $props();

	const totalSeconds = $derived(data.tracks.reduce((sum, t) => sum + t.durationSeconds, 0));

	function playAll() {
		player.playQueue(data.tracks);
	}

	function shufflePlay() {
		player.playQueue(data.tracks, Math.floor(Math.random() * data.tracks.length));
		if (!player.shuffle) player.toggleShuffle();
	}
</script>

<svelte:head>
	<title>{data.album.name} — Couchverse</title>
</svelte:head>

<div class="mx-auto max-w-4xl px-6 pt-28 pb-16">
	<div class="mb-8 flex flex-col items-start gap-6 sm:flex-row sm:items-end">
		<div
			class="size-44 shrink-0 animate-slide-up overflow-hidden rounded-card border border-edge/60
				shadow-2xl shadow-black/50"
		>
			{#if data.album.coverId}
				<img src={artworkUrl(data.album.coverId)} alt="" class="size-full object-cover" />
			{:else}
				<div
					class="flex size-full items-center justify-center bg-gradient-to-br from-accent-soft to-surface-2"
				>
					<Music class="size-10 text-accent/60" />
				</div>
			{/if}
		</div>
		<div class="min-w-0 animate-slide-up">
			<p class="eyebrow mb-2">Album</p>
			<h1 class="text-3xl font-extrabold tracking-tight sm:text-4xl">{data.album.name}</h1>
			<p class="mt-2 text-sm text-muted">
				<a href="/music/artists/{data.album.artistId}" class="font-semibold hover:text-accent">
					{data.album.artistName}
				</a>
				{#if data.album.year}· {data.album.year}{/if}
				· {data.tracks.length} tracks · {Math.round(totalSeconds / 60)} min
			</p>
			<div class="mt-5 flex gap-2">
				<Button onclick={playAll}>
					<Play class="size-4 fill-current" />
					Play
				</Button>
				<Button variant="secondary" onclick={shufflePlay}>
					<Shuffle class="size-4" />
					Shuffle
				</Button>
			</div>
		</div>
	</div>

	<TrackList tracks={data.tracks} />
</div>
