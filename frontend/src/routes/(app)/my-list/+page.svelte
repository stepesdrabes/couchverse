<script lang="ts">
	import * as catalog from '$lib/features/catalog/api';
	import type { CardItem } from '$lib/features/catalog/types';
	import PosterCard from '$lib/components/media/PosterCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';

	let items = $state<CardItem[]>([]);
	let loaded = $state(false);

	$effect(() => {
		catalog
			.myList()
			.then((res) => {
				items = res;
				loaded = true;
			})
			.catch(() => (loaded = true));
	});
</script>

<svelte:head>
	<title>My List — Couchverse</title>
</svelte:head>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<h1 class="mb-8 text-2xl font-bold">My List</h1>

	{#if loaded && items.length === 0}
		<EmptyState
			title="Your list is empty"
			message="Save movies and series with the + button to find them here."
		/>
	{:else}
		<div class="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
			{#each items as item (item.titleId)}
				<PosterCard {item} />
			{/each}
		</div>
	{/if}
</div>
