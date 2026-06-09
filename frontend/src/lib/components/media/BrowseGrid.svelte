<script lang="ts">
	import * as catalog from '$lib/features/catalog/api';
	import type { CardItem, Genre } from '$lib/features/catalog/types';
	import PosterCard from './PosterCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Select from '$lib/components/ui/Select.svelte';

	let {
		heading,
		kind = '',
		genre = ''
	}: { heading: string; kind?: string; genre?: string } = $props();

	let items = $state<CardItem[]>([]);
	let genres = $state<Genre[]>([]);
	let selectedGenre = $state(genre);
	let sort = $state('added');
	let loaded = $state(false);

	$effect(() => {
		catalog.listGenres().then((g) => (genres = g));
	});

	$effect(() => {
		catalog
			.browse({ kind, genre: selectedGenre, sort })
			.then((res) => {
				items = res.items;
				loaded = true;
			})
			.catch(() => (loaded = true));
	});
</script>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<div class="mb-8 flex flex-wrap items-center gap-4">
		<h1 class="text-2xl font-bold">{heading}</h1>
		<div class="ml-auto flex items-center gap-3">
			{#if !genre}
				<Select
					bind:value={selectedGenre}
					label="Genre"
					placeholder="All"
					items={[
						{ value: '', label: 'All' },
						...genres.map((g) => ({ value: g.name, label: g.name }))
					]}
				/>
			{/if}
			<Select
				bind:value={sort}
				label="Sort"
				items={[
					{ value: 'added', label: 'Recently added' },
					{ value: 'name', label: 'Name' },
					{ value: 'year', label: 'Year' }
				]}
			/>
		</div>
	</div>

	{#if loaded && items.length === 0}
		<EmptyState title="Nothing here yet" message="Published titles will show up in this view." />
	{:else}
		<div class="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
			{#each items as item (item.titleId)}
				<PosterCard {item} />
			{/each}
		</div>
	{/if}
</div>
