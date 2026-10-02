<script lang="ts">
	import type { TitleDetail } from '$lib/features/catalog/types';
	import { titleCache } from '$lib/features/catalog/cache.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import TitleDetailContent from './TitleDetailContent.svelte';
	import TitleDetailSkeleton from '$lib/features/catalog/components/TitleDetailSkeleton.svelte';

	let { data }: { data: { slug: string; fresh: Promise<TitleDetail> } } = $props();
</script>

<CachedView value={titleCache.get(data.slug)} fresh={data.fresh}>
	{#snippet content(detail)}
		<TitleDetailContent data={detail} />
	{/snippet}
	{#snippet skeleton()}
		<TitleDetailSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound />
	{/snippet}
</CachedView>
