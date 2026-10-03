<script lang="ts">
	import type { MyListView, Surface } from '$lib/generated/core';
	import { useScreen } from '$lib/core/screen.svelte';
	import { shown } from '$lib/features/catalog/api';
	import CachedView from '$lib/components/CachedView.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import LoadFailed from '$lib/features/catalog/components/LoadFailed.svelte';
	import PosterCard from '$lib/features/catalog/components/PosterCard.svelte';
	import PosterGridSkeleton from '$lib/features/catalog/components/PosterGridSkeleton.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: { screen: Surface } } = $props();

	const myList = useScreen<MyListView>(() => data.screen, { revalidate: true });
</script>

<svelte:head>
	<title>{m.catalog_my_list_title()}</title>
</svelte:head>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<h1 class="mb-8 text-2xl font-bold">{m.nav_my_list()}</h1>

	<CachedView value={shown(myList.view)} status={myList.view?.status}>
		{#snippet content(view)}
			{#if view.cards.length === 0}
				<EmptyState
					title={m.catalog_my_list_empty_title()}
					message={m.catalog_my_list_empty_message()}
				/>
			{:else}
				<div class="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
					{#each view.cards as card (card.titleId)}
						<PosterCard {card} />
					{/each}
				</div>
			{/if}
		{/snippet}
		{#snippet skeleton()}
			<PosterGridSkeleton count={6} />
		{/snippet}
		{#snippet failed()}
			<LoadFailed screen={data.screen} />
		{/snippet}
	</CachedView>
</div>
