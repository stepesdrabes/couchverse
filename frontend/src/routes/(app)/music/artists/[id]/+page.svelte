<script lang="ts">
	import AlbumCard from '$lib/features/music/components/AlbumCard.svelte';

	let { data } = $props();
</script>

<svelte:head>
	<title>{data.artist.name} — Couchverse</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-6 pt-28 pb-16">
	<p class="eyebrow mb-2">Artist</p>
	<h1 class="animate-slide-up text-4xl font-extrabold tracking-tight">{data.artist.name}</h1>
	<p class="mt-2 text-sm text-faint tnum">
		{data.artist.albumCount} album{data.artist.albumCount === 1 ? '' : 's'}
	</p>

	<div class="mt-10 flex flex-wrap gap-5">
		{#each data.albums as album (album.id)}
			<AlbumCard {album} />
		{/each}
	</div>
</div>
