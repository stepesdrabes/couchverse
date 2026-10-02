<script lang="ts">
	import type { HomeData } from '$lib/features/catalog/types';
	import { homeCache } from '$lib/features/catalog/cache.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import HomeContent from './HomeContent.svelte';
	import HomeSkeleton from '$lib/features/catalog/components/HomeSkeleton.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: { lang: string; fresh: Promise<HomeData> } } = $props();
</script>

<CachedView value={homeCache.get(data.lang)} fresh={data.fresh}>
	{#snippet content(home)}
		<HomeContent data={home} />
	{/snippet}
	{#snippet skeleton()}
		<HomeSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound title={m.error_page_title()} />
	{/snippet}
</CachedView>
