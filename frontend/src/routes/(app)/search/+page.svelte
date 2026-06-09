<script lang="ts">
	import { Music, Search } from 'lucide-svelte';
	import * as catalog from '$lib/features/catalog/api';
	import type { SearchResults } from '$lib/features/catalog/types';
	import PosterCard from '$lib/components/media/PosterCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';

	let query = $state('');
	let results = $state<SearchResults | null>(null);
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
			return;
		}
		controller = new AbortController();
		try {
			results = await catalog.search(query.trim(), controller.signal);
		} catch {
			// aborted or failed — keep previous results
		}
	}

	const musicHits = $derived(
		results ? [...results.artists, ...results.albums, ...results.tracks] : []
	);
	const empty = $derived(results !== null && results.titles.length === 0 && musicHits.length === 0);
</script>

<svelte:head>
	<title>Search — Couchverse</title>
</svelte:head>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<div class="relative mx-auto mb-10 max-w-xl">
		<Search class="absolute top-1/2 left-4 size-5 -translate-y-1/2 text-faint" />
		<!-- svelte-ignore a11y_autofocus -->
		<input
			bind:value={query}
			oninput={onInput}
			autofocus
			placeholder="Search movies, series, music…"
			class="h-12 w-full rounded-full border border-edge bg-surface pr-5 pl-12 text-[15px]
				transition-colors placeholder:text-faint focus:border-accent focus:outline-none"
		/>
	</div>

	{#if empty}
		<EmptyState title="No results" message={`Nothing in the library matches “${query}”.`} />
	{:else if results}
		{#if results.titles.length > 0}
			<h2 class="eyebrow mb-4">Movies & Series</h2>
			<div
				class="mb-10 grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6"
			>
				{#each results.titles as item (item.titleId)}
					<PosterCard {item} />
				{/each}
			</div>
		{/if}

		{#if musicHits.length > 0}
			<h2 class="eyebrow mb-4">Music</h2>
			<ul class="max-w-xl divide-y divide-edge/50 rounded-card border border-edge bg-surface/40">
				{#each musicHits as hit (hit.name + hit.subtitle)}
					<li class="flex items-center gap-3 px-4 py-3">
						<Music class="size-4 text-faint" />
						<div class="min-w-0">
							<p class="truncate text-sm font-medium">{hit.name}</p>
							{#if hit.subtitle}
								<p class="truncate text-xs text-faint">{hit.subtitle}</p>
							{/if}
						</div>
					</li>
				{/each}
			</ul>
			<p class="mt-3 text-xs text-faint">Music playback arrives with the music milestone.</p>
		{/if}
	{/if}
</div>
