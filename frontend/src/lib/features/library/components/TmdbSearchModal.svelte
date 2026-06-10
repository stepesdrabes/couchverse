<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Search } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as libraryApi from '$lib/features/library/api';
	import type { TmdbResult } from '$lib/features/library/api';
	import type { Title } from '$lib/features/catalog/types';
	import Button from '$lib/components/ui/Button.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';

	let { open = $bindable(false), title }: { open?: boolean; title: Title } = $props();

	let query = $state(title.name);
	let results = $state<TmdbResult[]>([]);
	let searching = $state(false);
	let applying = $state<number | null>(null);
	let error = $state('');

	async function search(e?: SubmitEvent) {
		e?.preventDefault();
		searching = true;
		error = '';
		try {
			results = await libraryApi.searchTmdb(query, title.kind);
		} catch (err) {
			error = err instanceof Error ? err.message : 'search failed';
			results = [];
		} finally {
			searching = false;
		}
	}

	$effect(() => {
		if (open && results.length === 0 && !error) search();
	});

	async function apply(result: TmdbResult) {
		applying = result.tmdbId;
		try {
			await libraryApi.applyTmdb(title.id, result.tmdbId);
			toast.success('Fetching metadata & artwork from TMDB…');
			open = false;
			// the fetch job usually lands within a few seconds
			setTimeout(() => invalidateAll(), 4000);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'failed to apply');
		} finally {
			applying = null;
		}
	}
</script>

<Modal bind:open title="Search TMDB" description="Pulls overview, year, genres and artwork.">
	<form onsubmit={search} class="mb-4 flex gap-2">
		<div class="relative flex-1">
			<Search class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
			<input
				bind:value={query}
				class="h-9 w-full rounded-full border border-edge bg-surface pr-4 pl-9 text-sm
					placeholder:text-faint focus:border-accent focus:outline-none"
				placeholder="Search by name…"
			/>
		</div>
		<Button type="submit" size="sm" loading={searching}>Search</Button>
	</form>

	{#if error}
		<p class="py-6 text-center text-sm text-danger">{error}</p>
	{:else if results.length === 0 && !searching}
		<p class="py-6 text-center text-sm text-faint">No results.</p>
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
					<Button
						variant="secondary"
						size="sm"
						class="self-center"
						loading={applying === result.tmdbId}
						onclick={() => apply(result)}
					>
						Apply
					</Button>
				</li>
			{/each}
		</ul>
	{/if}
</Modal>
