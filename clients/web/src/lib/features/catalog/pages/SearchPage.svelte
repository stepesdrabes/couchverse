<script lang="ts">
	import { Loader2, Search } from 'lucide-svelte';
	import { untrack } from 'svelte';
	import { fly } from 'svelte/transition';
	import * as catalog from '$lib/features/catalog/api';
	import { useScreen } from '$lib/core/screen.svelte';
	import { LoadStatus, type SearchView } from '$lib/generated/core';
	import PosterCard from '$lib/features/catalog/components/PosterCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import * as m from '$lib/paraglide/messages';

	const search = useScreen<SearchView>(() => catalog.searchScreen);
	const view = $derived(search.view);

	// the core keeps the last search, so coming back to the page shows it again
	let query = $state(untrack(() => search.view?.query ?? ''));

	// the previous results stay up while the next search runs
	const searching = $derived(
		view?.status === LoadStatus.Loading || (view?.status === LoadStatus.Stale && !view.problem)
	);
	const results = $derived(catalog.shown(view)?.cards);
	const empty = $derived(view?.status === LoadStatus.Loaded && view.cards.length === 0);
</script>

<svelte:head>
	<title>{m.catalog_search_title()}</title>
</svelte:head>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<div class="relative mx-auto mb-10 max-w-xl">
		<Search class="absolute top-1/2 left-4 size-5 -translate-y-1/2 text-faint" />
		<!-- svelte-ignore a11y_autofocus -->
		<input
			bind:value={query}
			oninput={() => catalog.searchFor(query)}
			autofocus
			placeholder={m.catalog_search_placeholder()}
			class="h-12 w-full rounded-full border border-edge bg-surface pr-12 pl-12 text-[15px]
				transition-colors placeholder:text-faint focus:border-accent focus:outline-none"
		/>
		{#if searching}
			<Loader2 class="absolute top-1/2 right-4 size-5 -translate-y-1/2 animate-spin text-faint" />
		{/if}
	</div>

	{#if empty}
		<EmptyState
			title={m.catalog_search_empty_title()}
			message={m.catalog_search_empty_message({ query: view?.query.trim() ?? '' })}
		/>
	{:else if results?.length}
		<h2 class="eyebrow mb-4">{m.catalog_search_movies_series()}</h2>
		{#key results}
			<div
				class="mb-10 grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6"
			>
				{#each results as card, i (card.titleId)}
					<div in:fly|global={{ y: 14, duration: 300, delay: Math.min(i * 35, 350) }}>
						<PosterCard {card} />
					</div>
				{/each}
			</div>
		{/key}
	{/if}
</div>
