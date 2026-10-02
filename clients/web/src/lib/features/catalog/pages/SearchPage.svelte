<script lang="ts">
	import { Loader2, Search } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import * as catalog from '$lib/features/catalog/api';
	import type { SearchResults } from '$lib/features/catalog/types';
	import PosterCard from '$lib/features/catalog/components/PosterCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import * as m from '$lib/paraglide/messages';

	let query = $state('');
	let results = $state<SearchResults | null>(null);
	let searching = $state(false);
	let timer: ReturnType<typeof setTimeout>;
	let controller: AbortController | null = null;

	function onInput() {
		clearTimeout(timer);
		timer = setTimeout(run, 250);
	}

	async function run() {
		controller?.abort();
		if (!query.trim()) {
			results = null;
			searching = false;
			return;
		}
		const mine = new AbortController();
		controller = mine;
		searching = true;
		try {
			results = await catalog.search(query.trim(), mine.signal);
		} catch {
			// aborted or failed - keep previous results
		} finally {
			if (controller === mine) searching = false; // only the latest request clears it
		}
	}

	const empty = $derived(results !== null && results.titles.length === 0);
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
			oninput={onInput}
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
			message={m.catalog_search_empty_message({ query })}
		/>
	{:else if results}
		{#if results.titles.length > 0}
			<h2 class="eyebrow mb-4">{m.catalog_search_movies_series()}</h2>
			{#key results}
				<div
					class="mb-10 grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6"
				>
					{#each results.titles as item, i (item.titleId)}
						<div in:fly|global={{ y: 14, duration: 300, delay: Math.min(i * 35, 350) }}>
							<PosterCard {item} />
						</div>
					{/each}
				</div>
			{/key}
		{/if}
	{/if}
</div>
