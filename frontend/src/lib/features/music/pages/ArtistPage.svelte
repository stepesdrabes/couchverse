<script lang="ts">
	import type { getArtist } from '$lib/features/music/api';
	import { artistCache } from '$lib/features/music/cache.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import ArtistContent from './ArtistContent.svelte';
	import MusicDetailSkeleton from '$lib/features/music/components/MusicDetailSkeleton.svelte';

	let { data }: { data: { id: string; fresh: Promise<Awaited<ReturnType<typeof getArtist>>> } } =
		$props();
</script>

<CachedView value={artistCache.get(data.id)} fresh={data.fresh}>
	{#snippet content(artist)}
		<ArtistContent data={artist} />
	{/snippet}
	{#snippet skeleton()}
		<MusicDetailSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound />
	{/snippet}
</CachedView>
