<script lang="ts">
	import { Loader2, Search } from 'lucide-svelte';
	import * as libraryApi from '$lib/features/library/api';
	import type { TmdbResult } from '$lib/features/library/api';
	import type { Title } from '$lib/features/catalog/types';
	import Button from '$lib/components/ui/Button.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		title,
		onApply
	}: { open?: boolean; title: Title; onApply: (tmdbId: number) => void } = $props();

	let query = $state(title.name);
	let results = $state<TmdbResult[]>([]);
	let searching = $state(false);
	let error = $state('');

	async function search(e?: SubmitEvent) {
		e?.preventDefault();
		searching = true;
		error = '';
		try {
			results = await libraryApi.searchTmdb(query, title.kind);
		} catch (err) {
			error = err instanceof Error ? err.message : m.library_tmdb_search_failed();
			results = [];
		} finally {
			searching = false;
		}
	}

	$effect(() => {
		if (open && results.length === 0 && !error) search();
	});

	// the editor page runs the metadata job and, for a series, chains the episode
	// import - so this modal just hands back the pick and closes
	function apply(result: TmdbResult) {
		open = false;
		onApply(result.tmdbId);
	}
</script>

<Modal bind:open title={m.library_search_tmdb()} description={m.library_tmdb_pull_description()}>
	<form onsubmit={search} class="mb-4 flex gap-2">
		<div class="relative flex-1">
			<Search class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
			<input
				bind:value={query}
				class="h-9 w-full rounded-full border border-edge bg-surface pr-4 pl-9 text-sm
					placeholder:text-faint focus:border-accent focus:outline-none"
				placeholder={m.library_search_by_name_placeholder()}
			/>
		</div>
		<Button type="submit" size="sm" loading={searching}>{m.common_search()}</Button>
	</form>

	{#if searching && results.length === 0}
		<div class="flex justify-center py-10 text-faint">
			<Loader2 class="size-6 animate-spin" />
		</div>
	{:else if error}
		<p class="py-6 text-center text-sm text-danger">{error}</p>
	{:else if results.length === 0}
		<p class="py-6 text-center text-sm text-faint">{m.library_no_results()}</p>
	{:else}
		<ul class="max-h-80 space-y-2 overflow-y-auto pr-1">
			{#each results as result (result.tmdbId)}
				<li class="flex gap-3 rounded-input border border-edge/60 bg-surface p-2.5">
					{#if result.posterUrl}
						<img
							src={result.posterUrl}
							alt=""
							class="h-20 w-14 shrink-0 rounded-md object-cover"
							loading="lazy"
						/>
					{:else}
						<div class="h-20 w-14 shrink-0 rounded-md bg-surface-2"></div>
					{/if}
					<div class="min-w-0 flex-1">
						<p class="text-sm font-semibold">
							{result.name}
							{#if result.year}<span class="font-normal text-faint">({result.year})</span>{/if}
						</p>
						<p class="mt-0.5 line-clamp-2 text-xs text-faint">{result.overview}</p>
					</div>
					<Button variant="secondary" size="sm" class="self-center" onclick={() => apply(result)}>
						{m.library_apply()}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}
</Modal>
