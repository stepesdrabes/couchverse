<script lang="ts">
	import type { HomeView, Surface } from '$lib/generated/core';
	import { useScreen } from '$lib/core/screen.svelte';
	import { shown } from '$lib/features/catalog/api';
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import HomeContent from './HomeContent.svelte';
	import HomeSkeleton from '$lib/features/catalog/components/HomeSkeleton.svelte';
	import LoadFailed from '$lib/features/catalog/components/LoadFailed.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: { screen: Surface } } = $props();

	const home = useScreen<HomeView>(() => data.screen, { revalidate: true });
</script>

<CachedView value={shown(home.view)} status={home.view?.status}>
	{#snippet content(view)}
		<HomeContent {view} />
	{/snippet}
	{#snippet skeleton()}
		<HomeSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound title={m.error_page_title()} />
	{/snippet}
	{#snippet failed()}
		<LoadFailed screen={data.screen} />
	{/snippet}
</CachedView>
