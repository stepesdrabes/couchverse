<script lang="ts">
	import type { getArtist } from '$lib/features/music/api';
	import AlbumCard from '$lib/features/music/components/AlbumCard.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: Awaited<ReturnType<typeof getArtist>> } = $props();
</script>

<svelte:head>
	<title>{m.music_artist_title({ name: data.artist.name })}</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-6 pt-28 pb-16">
	<p class="eyebrow mb-2">{m.music_artist()}</p>
	<h1 class="animate-slide-up text-4xl font-extrabold tracking-tight">{data.artist.name}</h1>
	<p class="mt-2 text-sm text-faint tnum">
		{m.music_album_count({ count: data.artist.albumCount })}
	</p>

	<div class="mt-10 flex flex-wrap gap-5">
		{#each data.albums as album (album.id)}
			<AlbumCard {album} />
		{/each}
	</div>
</div>
