<script lang="ts" module>
	export interface PickedProgram {
		titleId: string;
		episodeId?: string;
		/** the title, and the episode's place and name */
		label: string;
	}
</script>

<script lang="ts">
	import { Film, Search, Tv } from 'lucide-svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import * as m from '$lib/paraglide/messages';
	import { adminGetTitle, adminListLibrary, type AdminTitle, type LibraryRow } from '../api';
	import { episodeCode } from '../labels';

	// Finds a published title (and, for a series, one of its episodes) to air.
	let {
		kind = undefined,
		episodes = true,
		exclude = [],
		onpick
	}: {
		/** only titles of this kind */
		kind?: 'movie' | 'series';
		/** a series needs an episode picked (the schedule); off for the template's pinned titles */
		episodes?: boolean;
		/** titles already chosen, left out of the results */
		exclude?: string[];
		onpick: (program: PickedProgram) => void;
	} = $props();

	let query = $state('');
	let results = $state<LibraryRow[]>([]);
	let series = $state<AdminTitle | null>(null);
	let episodeId = $state('');
	let timer: ReturnType<typeof setTimeout> | undefined;
	let generation = 0;

	async function search() {
		const mine = ++generation;
		try {
			const page = await adminListLibrary({
				q: query.trim() || undefined,
				type: kind,
				status: 'published',
				sort: 'name',
				pageSize: 12
			});
			if (mine === generation) results = page.items.filter((r) => !exclude.includes(r.id));
		} catch {
			if (mine === generation) results = [];
		}
	}

	$effect(() => {
		void query;
		clearTimeout(timer);
		timer = setTimeout(search, 200);
		return () => clearTimeout(timer);
	});

	async function choose(row: LibraryRow) {
		if (row.kind === 'movie' || !episodes) {
			onpick({ titleId: row.id, label: row.name });
			return;
		}
		series = await adminGetTitle(row.id);
		episodeId = '';
	}

	const episodeItems = $derived(
		(series?.seasons ?? []).flatMap((s) =>
			s.episodes.map((e) => ({
				value: e.id,
				label: `${episodeCode(s.seasonNumber, e.episodeNumber)} ${e.name}`.trim()
			}))
		)
	);

	function pickEpisode(id: string) {
		const e = episodeItems.find((i) => i.value === id);
		if (!series || !e) return;
		onpick({ titleId: series.title.id, episodeId: id, label: `${series.title.name} ${e.label}` });
	}
</script>

{#if series}
	<div class="space-y-2">
		<div class="flex items-center justify-between gap-2">
			<p class="truncate text-sm font-medium">{series.title.name}</p>
			<button
				type="button"
				class="shrink-0 text-xs text-muted hover:text-text"
				onclick={() => (series = null)}
			>
				{m.admin_couchtv_picker_back()}
			</button>
		</div>
		{#if episodeItems.length}
			<Select
				items={episodeItems}
				bind:value={episodeId}
				label={m.admin_couchtv_episode()}
				placeholder={m.admin_couchtv_picker_episode()}
				class="w-full justify-between"
				onchange={pickEpisode}
			/>
		{:else}
			<p class="text-xs text-faint">{m.admin_couchtv_picker_no_episodes()}</p>
		{/if}
	</div>
{:else}
	<div class="space-y-2">
		<div class="relative">
			<Search class="pointer-events-none absolute top-3 left-3 size-4 text-faint" />
			<Input
				bind:value={query}
				type="search"
				aria-label={m.admin_couchtv_picker_search()}
				placeholder={m.admin_couchtv_picker_search()}
				class="[&_input]:pl-9"
			/>
		</div>
		<ul class="max-h-56 space-y-0.5 overflow-y-auto">
			{#each results as row (row.id)}
				<li>
					<button
						type="button"
						onclick={() => choose(row)}
						class="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-sm transition-colors hover:bg-surface-2"
					>
						{#if row.kind === 'movie'}
							<Film class="size-3.5 shrink-0 text-faint" />
						{:else}
							<Tv class="size-3.5 shrink-0 text-faint" />
						{/if}
						<span class="truncate">{row.name}</span>
						{#if row.year}<span class="shrink-0 text-xs text-faint">{row.year}</span>{/if}
					</button>
				</li>
			{:else}
				<li class="px-2 py-1.5 text-xs text-faint">{m.admin_couchtv_picker_none()}</li>
			{/each}
		</ul>
	</div>
{/if}
