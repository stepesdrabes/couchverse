<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import * as libraryApi from '$lib/features/library/api';
	import ArtworkCard from '$lib/components/admin/ArtworkCard.svelte';
	import TmdbSearchModal from '$lib/components/admin/TmdbSearchModal.svelte';
	import EditorHero from '$lib/components/admin/editor/EditorHero.svelte';
	import EpisodesTable from '$lib/components/admin/editor/EpisodesTable.svelte';
	import ImportEpisodesModal from '$lib/components/admin/editor/ImportEpisodesModal.svelte';
	import MovieFilesPanel from '$lib/components/admin/editor/MovieFilesPanel.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';

	let { data } = $props();

	let name = $state('');
	let year = $state('');
	let contentRating = $state('');
	let overview = $state('');
	let status = $state<string>('draft');
	let genres = $state('');
	let runtime = $state('');
	let saving = $state(false);
	const form = new FormState(() => ({
		name,
		year,
		contentRating,
		overview,
		status,
		genres,
		runtime
	}));

	// re-sync after load/invalidateAll, but never clobber in-progress edits
	$effect(() => {
		const title = data.title;
		if (form.loaded && form.dirty) return;
		untrack(() => {
			name = title.name;
			year = title.year?.toString() ?? '';
			contentRating = title.contentRating;
			overview = title.overview;
			status = title.status;
			genres = title.genres.join(', ');
			runtime = title.runtimeMinutes?.toString() ?? '';
			form.reset();
		});
	});

	let confirmDeleteTitle = $state(false);
	let tmdbOpen = $state(false);
	let importOpen = $state(false);

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		try {
			await libraryApi.updateTitle(data.title.id, {
				name,
				year: year ? Number(year) : null,
				contentRating,
				overview,
				status,
				runtimeMinutes: runtime ? Number(runtime) : null,
				genres: genres
					.split(',')
					.map((g) => g.trim())
					.filter(Boolean)
			});
			form.reset();
			toast.success('Saved');
			invalidateAll();
		} catch {
			toast.error('Failed to save');
		} finally {
			saving = false;
		}
	}

	async function deleteTitle() {
		try {
			await libraryApi.deleteTitle(data.title.id);
			toast.success('Title deleted');
			goto('/admin/library');
		} catch {
			toast.error('Failed to delete');
		}
	}
</script>

<svelte:head>
	<title>{data.title.name} — Couchverse admin</title>
</svelte:head>

<EditorHero
	title={data.title}
	artwork={data.artwork}
	onFetchTmdb={() => (tmdbOpen = true)}
	onImportEpisodes={() => (importOpen = true)}
	onDelete={() => (confirmDeleteTitle = true)}
/>

<div class="grid grid-cols-1 gap-8 lg:grid-cols-[1fr_340px]">
	<div class="space-y-8">
		<form onsubmit={save} class="space-y-4 rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="text-sm font-semibold text-muted">Metadata</h2>
			<div class="grid gap-4 sm:grid-cols-2">
				<Input label="Name" bind:value={name} required />
				<Input label="Year" type="number" bind:value={year} />
				<Input label="Content rating" bind:value={contentRating} placeholder="TV-14, PG-13…" />
				{#if data.title.kind === 'movie'}
					<Input label="Runtime (minutes)" type="number" bind:value={runtime} />
				{/if}
			</div>
			<Textarea label="Overview" bind:value={overview} />
			<Input label="Genres" bind:value={genres} placeholder="Sci-Fi, Drama" />
			<div class="flex items-center justify-between pt-2">
				<Select
					bind:value={status}
					label="Status"
					items={[
						{ value: 'draft', label: 'Draft' },
						{ value: 'published', label: 'Published' },
						{ value: 'hidden', label: 'Hidden' }
					]}
				/>
				<Button type="submit" loading={saving} disabled={!form.dirty}>Save changes</Button>
			</div>
		</form>

		{#if data.title.kind === 'series'}
			<EpisodesTable
				titleId={data.title.id}
				seasons={data.seasons ?? []}
				mediaFiles={data.mediaFiles}
				subtitlesByFile={data.subtitlesByFile ?? {}}
			/>
		{/if}
	</div>

	<aside class="order-first space-y-6 lg:order-none">
		<ArtworkCard titleId={data.title.id} artwork={data.artwork} />
		{#if data.title.kind === 'movie'}
			<MovieFilesPanel
				titleId={data.title.id}
				mediaFiles={data.mediaFiles}
				subtitlesByFile={data.subtitlesByFile ?? {}}
			/>
		{/if}
	</aside>
</div>

<Confirm
	bind:open={confirmDeleteTitle}
	title="Delete “{data.title.name}”?"
	message="This removes the title and all its seasons, episodes and metadata."
	onconfirm={deleteTitle}
/>

<TmdbSearchModal bind:open={tmdbOpen} title={data.title} />

{#if data.title.kind === 'series'}
	<ImportEpisodesModal bind:open={importOpen} titleId={data.title.id} />
{/if}
