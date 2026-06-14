<script lang="ts">
	import { ArrowDown, ArrowUp, Plus, Trash2 } from 'lucide-svelte';
	import { flip } from 'svelte/animate';
	import { toast } from 'svelte-sonner';
	import { listGenres } from '$lib/features/catalog/api';
	import type { Genre } from '$lib/features/catalog/types';
	import * as jobsApi from '$lib/features/jobs/api';
	import type { HomeRowConfig } from '$lib/features/jobs/api';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';

	let rows = $state<HomeRowConfig[]>([]);
	let genres = $state<Genre[]>([]);
	let saving = $state(false);
	let nextTempID = -1;
	const form = new FormState(() => rows);

	$effect(() => {
		jobsApi.getHomeRows().then((r) => {
			rows = r;
			form.reset();
		});
		listGenres().then((g) => (genres = g));
	});

	const kindLabel: Record<HomeRowConfig['kind'], () => string> = {
		continue_watching: () => m.settings_home_row_continue_watching(),
		recently_added: () => m.settings_home_row_recently_added(),
		genre: () => m.settings_home_row_genre(),
		recently_played_music: () => m.settings_home_row_recently_played_music()
	};

	function move(index: number, dir: -1 | 1) {
		const target = index + dir;
		if (target < 0 || target >= rows.length) return;
		const next = [...rows];
		[next[index], next[target]] = [next[target], next[index]];
		rows = next;
	}

	function addGenreRow() {
		if (genres.length === 0) {
			toast.error(m.settings_home_rows_no_genres());
			return;
		}
		rows = [
			...rows,
			{
				id: nextTempID--,
				position: rows.length + 1,
				kind: 'genre',
				genreId: genres[0].id,
				label: genres[0].name,
				enabled: true
			}
		];
	}

	function removeRow(index: number) {
		rows = rows.filter((_, i) => i !== index);
	}

	async function save() {
		saving = true;
		try {
			rows = await jobsApi.putHomeRows(rows);
			form.reset();
			toast.success(m.settings_home_rows_saved());
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.settings_home_rows_save_failed());
		} finally {
			saving = false;
		}
	}
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<div class="mb-4 flex items-center justify-between">
		<h2 class="text-sm font-semibold text-muted">{m.settings_home_rows_heading()}</h2>
		<Button variant="secondary" size="sm" onclick={addGenreRow}>
			<Plus class="size-3.5" />
			{m.settings_home_row_genre()}
		</Button>
	</div>

	<ul class="space-y-2">
		{#each rows as row, i (row.id)}
			<li
				animate:flip={{ duration: 200 }}
				class="flex items-center gap-3 rounded-input border border-edge/60 bg-surface px-3 py-2"
			>
				<Switch bind:checked={row.enabled} />
				<div class="min-w-0 flex-1">
					{#if row.kind === 'genre'}
						<div class="flex items-center gap-2">
							<Select
								value={String(row.genreId ?? '')}
								onchange={(v) => {
									row.genreId = Number(v);
									row.label = genres.find((g) => g.id === Number(v))?.name ?? row.label;
								}}
								items={genres.map((g) => ({ value: String(g.id), label: g.name }))}
							/>
						</div>
					{:else}
						<input
							bind:value={row.label}
							class="w-full rounded-lg border border-transparent bg-transparent px-1 py-0.5 text-sm
								focus:border-edge focus:outline-none"
						/>
						<p class="px-1 text-[10px] text-faint">{kindLabel[row.kind]()}</p>
					{/if}
				</div>
				<span class="flex gap-0.5">
					<button
						class="rounded-full p-1.5 text-faint hover:text-text disabled:opacity-30"
						disabled={i === 0}
						onclick={() => move(i, -1)}
						aria-label={m.settings_home_rows_move_up()}
					>
						<ArrowUp class="size-3.5" />
					</button>
					<button
						class="rounded-full p-1.5 text-faint hover:text-text disabled:opacity-30"
						disabled={i === rows.length - 1}
						onclick={() => move(i, 1)}
						aria-label={m.settings_home_rows_move_down()}
					>
						<ArrowDown class="size-3.5" />
					</button>
					{#if row.kind === 'genre'}
						<button
							class="rounded-full p-1.5 text-faint hover:text-danger"
							onclick={() => removeRow(i)}
							aria-label={m.settings_home_rows_remove_row()}
						>
							<Trash2 class="size-3.5" />
						</button>
					{/if}
				</span>
			</li>
		{/each}
	</ul>

	<div class="mt-4 flex justify-end">
		<Button onclick={save} loading={saving} disabled={!form.dirty}
			>{m.settings_home_rows_save()}</Button
		>
	</div>
</div>
