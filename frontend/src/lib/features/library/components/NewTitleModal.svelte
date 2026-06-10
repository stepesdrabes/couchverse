<script lang="ts">
	import { goto } from '$app/navigation';
	import { Plus, Search } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '$lib/api/client';
	import type { TitleKind } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { TmdbResult } from '$lib/features/library/api';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	let kind = $state<string>('movie');
	let query = $state('');
	let results = $state<TmdbResult[]>([]);
	let searching = $state(false);
	let noTmdbKey = $state(false);
	let adding = $state<number | null>(null);

	let manualName = $state('');
	let manualYear = $state('');
	let creating = $state(false);

	async function search(e?: SubmitEvent) {
		e?.preventDefault();
		if (!query.trim()) return;
		searching = true;
		try {
			results = await libraryApi.searchTmdb(query.trim(), kind as TitleKind);
			noTmdbKey = false;
		} catch (err) {
			results = [];
			if (err instanceof ApiError && err.status === 412) {
				noTmdbKey = true;
			} else {
				toast.error(err instanceof Error ? err.message : 'TMDB search failed');
			}
		} finally {
			searching = false;
		}
	}

	// create from a TMDB result and pull its full metadata + artwork
	async function addFromTmdb(result: TmdbResult) {
		adding = result.tmdbId;
		try {
			const title = await libraryApi.createTitle({
				kind,
				name: result.name,
				year: result.year || null,
				overview: result.overview
			});
			await libraryApi.applyTmdb(title.id, result.tmdbId);
			toast.success(`Added “${result.name}” - fetching metadata & artwork`);
			open = false;
			goto(`/admin/library/${title.id}`);
		} catch {
			toast.error('Failed to create title');
		} finally {
			adding = null;
		}
	}

	async function createManually(e: SubmitEvent) {
		e.preventDefault();
		creating = true;
		try {
			const title = await libraryApi.createTitle({
				kind,
				name: manualName,
				year: manualYear ? Number(manualYear) : null
			});
			open = false;
			goto(`/admin/library/${title.id}`);
		} catch {
			toast.error('Failed to create title');
		} finally {
			creating = false;
		}
	}
</script>

<Modal bind:open title="New title" description="Search TMDB or create one from scratch." size="lg">
	<div class="mb-4">
		<Tabs
			bind:value={kind}
			items={[
				{ value: 'movie', label: 'Movie' },
				{ value: 'series', label: 'Series' }
			]}
		/>
	</div>

	<form onsubmit={search} class="flex gap-2">
		<div class="relative flex-1">
			<Search class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
			<input
				bind:value={query}
				class="h-9 w-full rounded-full border border-edge bg-surface pr-4 pl-9 text-sm
					placeholder:text-faint focus:border-accent focus:outline-none"
				placeholder="Search TMDB…"
			/>
		</div>
		<Button type="submit" size="sm" loading={searching}>Search</Button>
	</form>

	{#if noTmdbKey}
		<p class="mt-3 rounded-input border border-edge bg-surface px-3 py-2 text-xs text-faint">
			No TMDB API key configured - add one under
			<a href="/admin/settings" class="text-accent hover:underline">Settings</a>
			to search, or create the title manually below.
		</p>
	{:else if results.length > 0}
		<ul class="mt-3 max-h-72 space-y-2 overflow-y-auto pr-1">
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
						loading={adding === result.tmdbId}
						onclick={() => addFromTmdb(result)}
					>
						<Plus class="size-3.5" />
						Add
					</Button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="mt-5 border-t border-edge/60 pt-4">
		<p class="mb-3 text-[11px] font-semibold tracking-widest text-faint uppercase">
			Or create manually
		</p>
		<form onsubmit={createManually} class="flex items-end gap-2">
			<Input label="Name" bind:value={manualName} required class="flex-1" />
			<Input label="Year" type="number" bind:value={manualYear} class="w-24" />
			<Button type="submit" variant="secondary" loading={creating}>Create</Button>
		</form>
	</div>
</Modal>
