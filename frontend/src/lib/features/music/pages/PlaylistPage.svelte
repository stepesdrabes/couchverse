<script lang="ts">
	import type { getPlaylist } from '$lib/features/music/api';
	import { playlistCache } from '$lib/features/music/cache.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import PlaylistContent from './PlaylistContent.svelte';
	import MusicDetailSkeleton from '$lib/features/music/components/MusicDetailSkeleton.svelte';

	let { data }: { data: { id: string; fresh: Promise<Awaited<ReturnType<typeof getPlaylist>>> } } =
		$props();
</script>

<CachedView value={playlistCache.get(data.id)} fresh={data.fresh}>
	{#snippet content(playlist)}
		<PlaylistContent data={playlist} />
	{/snippet}
	{#snippet skeleton()}
		<MusicDetailSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound />
	{/snippet}
</CachedView>
