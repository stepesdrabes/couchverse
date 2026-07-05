<script lang="ts">
	import { fly } from 'svelte/transition';
	import * as catalog from '$lib/features/catalog/api';
	import { browseCache } from '$lib/features/catalog/cache.svelte';
	import type { Genre } from '$lib/features/catalog/types';
	import PosterCard from './PosterCard.svelte';
	import PosterGridSkeleton from './PosterGridSkeleton.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		heading,
		kind = '',
		genre = ''
	}: { heading: string; kind?: string; genre?: string } = $props();

	let genres = $state<Genre[]>([]);
	let selectedGenre = $state(genre);
	let sort = $state('added');

	// cached results paint instantly on revisit/filter-change; a cold key shows the
	// skeleton grid until its fetch lands, then updates in place
	const browseKey = $derived(`${kind}|${selectedGenre}|${sort}`);
	const items = $derived(browseCache.get(browseKey));
	let failed = $state(false);

	$effect(() => {
		catalog.listGenres().then((g) => (genres = g));
	});

	$effect(() => {
		const key = browseKey;
		failed = false;
		catalog
			.browse({ kind, genre: selectedGenre, sort })
			.then((res) => browseCache.set(key, res.items))
			.catch(() => {
				if (browseCache.get(key) === undefined) failed = true;
			});
	});
</script>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<div class="mb-8 flex flex-wrap items-center gap-4">
		<h1 class="text-2xl font-bold">{heading}</h1>
		<div class="ml-auto flex items-center gap-3">
			{#if !genre}
				<Select
					bind:value={selectedGenre}
					label={m.catalog_filter_genre()}
					placeholder={m.catalog_filter_all()}
					items={[
						{ value: '', label: m.catalog_filter_all() },
						...genres.map((g) => ({ value: g.name, label: g.label }))
					]}
				/>
			{/if}
			<Select
				bind:value={sort}
				label={m.catalog_sort_label()}
				items={[
					{ value: 'added', label: m.catalog_sort_recently_added() },
					{ value: 'name', label: m.catalog_sort_name() },
					{ value: 'year', label: m.catalog_sort_year() }
				]}
			/>
		</div>
	</div>

	{#if items === undefined && !failed}
		<PosterGridSkeleton />
	{:else if !items || items.length === 0}
		<EmptyState title={m.catalog_browse_empty_title()} message={m.catalog_browse_empty_message()} />
	{:else}
		{#key items}
			<div class="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
				{#each items as item, i (item.titleId)}
					<div in:fly|global={{ y: 14, duration: 300, delay: Math.min(i * 30, 360) }}>
						<PosterCard {item} />
					</div>
				{/each}
			</div>
		{/key}
	{/if}
</div>
