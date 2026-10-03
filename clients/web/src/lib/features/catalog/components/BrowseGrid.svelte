<script lang="ts">
	import { fly } from 'svelte/transition';
	import * as catalog from '$lib/features/catalog/api';
	import { useScreen } from '$lib/core/screen.svelte';
	import { surfaceKey } from '$lib/core/runtime.svelte';
	import {
		BrowseSort,
		type BrowseKey,
		type BrowseView,
		type GenresView
	} from '$lib/generated/core';
	import PosterCard from './PosterCard.svelte';
	import PosterGridSkeleton from './PosterGridSkeleton.svelte';
	import LoadFailed from './LoadFailed.svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import * as m from '$lib/paraglide/messages';

	// `listing` is the page's own key (a kind or a genre); the visitor picks the sort and, on a
	// kind's page, a genre
	let { heading, listing }: { heading: string; listing: BrowseKey } = $props();

	let selectedGenre = $state('');
	let sort = $state<BrowseSort>(BrowseSort.Added);

	const key = $derived<BrowseKey>({
		kind: listing.kind,
		genre: listing.genre ?? (selectedGenre || undefined),
		sort
	});
	const screen = $derived(catalog.browseScreen(key));
	const browse = useScreen<BrowseView>(() => screen);
	const view = $derived(catalog.shown(browse.view));
	// the genre filter only shows on a kind's page
	const genres = useScreen<GenresView>(() => (listing.genre ? undefined : catalog.genresScreen));

	// the next page loads as the end of the grid scrolls into reach
	function nearEnd(node: HTMLElement) {
		if (!view?.more || view.loadingMore) return;
		const target = key;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries.some((e) => e.isIntersecting)) void catalog.browseMore(target);
			},
			{ rootMargin: '800px 0px' }
		);
		observer.observe(node);
		return () => observer.disconnect();
	}
</script>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<div class="mb-8 flex flex-wrap items-center gap-4">
		<h1 class="text-2xl font-bold">{heading}</h1>
		<div class="ml-auto flex items-center gap-3">
			{#if !listing.genre}
				<Select
					bind:value={selectedGenre}
					label={m.catalog_filter_genre()}
					placeholder={m.catalog_filter_all()}
					items={[
						{ value: '', label: m.catalog_filter_all() },
						...(genres.view?.genres ?? []).map((g) => ({ value: g.name, label: g.label }))
					]}
				/>
			{/if}
			<Select
				bind:value={sort}
				label={m.catalog_sort_label()}
				items={[
					{ value: BrowseSort.Added, label: m.catalog_sort_recently_added() },
					{ value: BrowseSort.Name, label: m.catalog_sort_name() },
					{ value: BrowseSort.Year, label: m.catalog_sort_year() }
				]}
			/>
		</div>
	</div>

	<CachedView value={view} status={browse.view?.status}>
		{#snippet content(grid)}
			{#if grid.cards.length === 0}
				<EmptyState
					title={m.catalog_browse_empty_title()}
					message={m.catalog_browse_empty_message()}
				/>
			{:else}
				<!-- a new filter or sort plays the entrance again; more pages only add cards -->
				{#key surfaceKey(screen)}
					<div
						class="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6"
						data-testid="browse-grid"
					>
						{#each grid.cards as card, i (card.titleId)}
							<div in:fly|global={{ y: 14, duration: 300, delay: Math.min(i * 30, 360) }}>
								<PosterCard {card} />
							</div>
						{/each}
					</div>
				{/key}
				<div {@attach nearEnd} class="h-px"></div>
				{#if grid.loadingMore}
					<div class="mt-8"><PosterGridSkeleton count={6} /></div>
				{/if}
			{/if}
		{/snippet}
		{#snippet skeleton()}
			<PosterGridSkeleton />
		{/snippet}
		{#snippet failed()}
			<LoadFailed {screen} />
		{/snippet}
	</CachedView>
</div>
