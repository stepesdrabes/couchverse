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
	import LanguageChips from '$lib/features/library/components/LanguageChips.svelte';
	import * as m from '$lib/paraglide/messages';

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

	let languages = $state<string[]>(['en']);

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
				toast.error(err instanceof Error ? err.message : m.library_tmdb_search_failed());
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
				overview: result.overview,
				metadataLanguages: languages
			});
			await libraryApi.applyTmdb(title.id, result.tmdbId);
			toast.success(m.library_added_fetching({ name: result.name }));
			open = false;
			goto(`/admin/library/${title.id}`);
		} catch {
			toast.error(m.library_create_title_failed());
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
				year: manualYear ? Number(manualYear) : null,
				metadataLanguages: languages
			});
			open = false;
			goto(`/admin/library/${title.id}`);
		} catch {
			toast.error(m.library_create_title_failed());
		} finally {
			creating = false;
		}
	}
</script>

<Modal
	bind:open
	title={m.library_new_title()}
	description={m.library_new_title_description()}
	size="lg"
>
	<div class="mb-4">
		<Tabs
			bind:value={kind}
			items={[
				{ value: 'movie', label: m.library_kind_movie() },
				{ value: 'series', label: m.library_kind_series() }
			]}
		/>
	</div>

	<div class="mb-4">
		<p class="mb-1.5 text-xs font-medium text-muted">{m.library_content_languages()}</p>
		<LanguageChips bind:selected={languages} />
	</div>

	<form onsubmit={search} class="flex gap-2">
		<div class="relative flex-1">
			<Search class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
			<input
				bind:value={query}
				class="h-9 w-full rounded-full border border-edge bg-surface pr-4 pl-9 text-sm
					placeholder:text-faint focus:border-accent focus:outline-none"
				placeholder={m.library_search_tmdb_placeholder()}
			/>
		</div>
		<Button type="submit" size="sm" loading={searching}>{m.common_search()}</Button>
	</form>

	{#if noTmdbKey}
		<p class="mt-3 rounded-input border border-edge bg-surface px-3 py-2 text-xs text-faint">
			{m.library_no_tmdb_key_before()}
			<a href="/admin/settings" class="text-accent hover:underline">{m.library_settings_link()}</a>
			{m.library_no_tmdb_key_after()}
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
						{m.common_add()}
					</Button>
				</li>
			{/each}
		</ul>
	{/if}

	<div class="mt-5 border-t border-edge/60 pt-4">
		<p class="mb-3 text-[11px] font-semibold tracking-widest text-faint uppercase">
			{m.library_or_create_manually()}
		</p>
		<form onsubmit={createManually} class="flex items-end gap-2">
			<Input label={m.common_name()} bind:value={manualName} required class="flex-1" />
			<Input label={m.library_year()} type="number" bind:value={manualYear} class="w-24" />
			<Button type="submit" variant="secondary" loading={creating}>{m.common_create()}</Button>
		</form>
	</div>
</Modal>
