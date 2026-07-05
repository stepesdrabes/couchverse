<script lang="ts">
	import type { getAlbum } from '$lib/features/music/api';
	import { albumCache } from '$lib/features/music/cache.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import AlbumContent from './AlbumContent.svelte';
	import MusicDetailSkeleton from '$lib/features/music/components/MusicDetailSkeleton.svelte';

	let { data }: { data: { id: string; fresh: Promise<Awaited<ReturnType<typeof getAlbum>>> } } =
		$props();
</script>

<CachedView value={albumCache.get(data.id)} fresh={data.fresh}>
	{#snippet content(album)}
		<AlbumContent data={album} />
	{/snippet}
	{#snippet skeleton()}
		<MusicDetailSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound />
	{/snippet}
</CachedView>
