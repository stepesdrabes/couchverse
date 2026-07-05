<script lang="ts">
	import type { MusicHome } from '$lib/features/music/api';
	import { musicHomeCache } from '$lib/features/music/cache.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import MusicHomeContent from './MusicHomeContent.svelte';
	import MusicHomeSkeleton from '$lib/features/music/components/MusicHomeSkeleton.svelte';

	let { data }: { data: { lang: string; fresh: Promise<MusicHome> } } = $props();
</script>

<CachedView value={musicHomeCache.get(data.lang)} fresh={data.fresh}>
	{#snippet content(home)}
		<MusicHomeContent data={home} />
	{/snippet}
	{#snippet skeleton()}
		<MusicHomeSkeleton />
	{/snippet}
</CachedView>
