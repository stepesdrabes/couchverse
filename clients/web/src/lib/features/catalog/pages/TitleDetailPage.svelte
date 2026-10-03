<script lang="ts">
	import type { Surface, TitleView } from '$lib/generated/core';
	import { useScreen } from '$lib/core/screen.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import TitleDetailContent from './TitleDetailContent.svelte';
	import LoadFailed from '$lib/features/catalog/components/LoadFailed.svelte';
	import TitleDetailSkeleton from '$lib/features/catalog/components/TitleDetailSkeleton.svelte';

	let { data }: { data: { screen: Surface } } = $props();

	const title = useScreen<TitleView>(() => data.screen, { revalidate: true });
</script>

<CachedView value={title.view?.detail} status={title.view?.status}>
	{#snippet content(detail)}
		<TitleDetailContent {detail} />
	{/snippet}
	{#snippet skeleton()}
		<TitleDetailSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound />
	{/snippet}
	{#snippet failed()}
		<LoadFailed screen={data.screen} />
	{/snippet}
</CachedView>
