<script lang="ts">
	import { ApiError } from '$lib/api/client';
	import * as libraryApi from '$lib/features/library/api';
	import type { TmdbSeasonPreview } from '$lib/features/library/api';
	import Button from '$lib/components/ui/Button.svelte';
	import Checkbox from '$lib/components/ui/Checkbox.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		titleId,
		onImport
	}: { open?: boolean; titleId: string; onImport: (seasons?: number[]) => void } = $props();

	let seasons = $state<TmdbSeasonPreview[]>([]);
	let picks = $state<Record<number, boolean>>({});
	let allSeasons = $state(true);
	let loading = $state(false);
	let errorMsg = $state('');

	$effect(() => {
		if (open) fetchSeasons();
	});

	async function fetchSeasons() {
		loading = true;
		errorMsg = '';
		seasons = [];
		allSeasons = true;
		try {
			seasons = await libraryApi.getTmdbSeasons(titleId);
			picks = Object.fromEntries(seasons.map((s) => [s.seasonNumber, s.seasonNumber !== 0]));
		} catch (err) {
			if (err instanceof ApiError && err.code === 'no_tmdb_id') {
				errorMsg = m.library_import_no_tmdb_id();
			} else if (err instanceof ApiError && err.code === 'no_tmdb_key') {
				errorMsg = m.library_import_no_tmdb_key();
			} else {
				errorMsg = err instanceof Error ? err.message : m.library_load_seasons_failed();
			}
		} finally {
			loading = false;
		}
	}

	const selectedSeasons = $derived(seasons.map((s) => s.seasonNumber).filter((n) => picks[n]));

	// the editor page runs the import job and shows progress in-page; this modal
	// just hands back the season selection and closes
	function startImport() {
		open = false;
		onImport(allSeasons ? undefined : selectedSeasons);
	}
</script>

<Modal bind:open title={m.library_import_episodes()} description={m.library_import_episodes_desc()}>
	{#if loading}
		<p class="py-6 text-center text-sm text-faint">{m.library_loading_seasons()}</p>
	{:else if errorMsg}
		<p class="py-6 text-center text-sm text-danger">{errorMsg}</p>
	{:else if seasons.length === 0}
		<p class="py-6 text-center text-sm text-faint">{m.library_no_seasons_found()}</p>
	{:else}
		<label
			class="flex cursor-pointer items-center gap-2.5 border-b border-edge/60 pb-3 text-sm font-medium"
		>
			<Checkbox bind:checked={allSeasons} />
			{m.library_all_seasons()}
		</label>
		<ul
			class="mt-2 max-h-64 space-y-0.5 overflow-y-auto
				{allSeasons ? 'pointer-events-none opacity-50' : ''}"
		>
			{#each seasons as season (season.seasonNumber)}
				<li>
					<label
						class="flex cursor-pointer items-center gap-2.5 rounded-lg px-1 py-1.5 text-sm
							transition-colors hover:bg-surface-2/50"
					>
						<Checkbox bind:checked={picks[season.seasonNumber]} />
						<span class="min-w-0 flex-1 truncate">
							{season.name || m.library_season_number({ number: season.seasonNumber })}
						</span>
						<span class="shrink-0 text-xs text-faint tnum"
							>{m.library_episode_count({ count: season.episodeCount })}</span
						>
					</label>
				</li>
			{/each}
		</ul>
	{/if}

	{#snippet footer()}
		<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
		<Button
			disabled={loading ||
				errorMsg !== '' ||
				seasons.length === 0 ||
				(!allSeasons && selectedSeasons.length === 0)}
			onclick={startImport}
		>
			{m.library_import()}
		</Button>
	{/snippet}
</Modal>
