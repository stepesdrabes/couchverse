<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { waitForJob } from '$lib/features/jobs/api';
	import * as libraryApi from '$lib/features/library/api';
	import TmdbSearchModal from '$lib/features/library/components/TmdbSearchModal.svelte';
	import EditorHero from '$lib/features/library/components/editor/EditorHero.svelte';
	import EpisodesTable from '$lib/features/library/components/editor/EpisodesTable.svelte';
	import ImportEpisodesModal from '$lib/features/library/components/editor/ImportEpisodesModal.svelte';
	import MovieFilesPanel from '$lib/features/library/components/editor/MovieFilesPanel.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import LanguageChips from '$lib/features/library/components/LanguageChips.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: Awaited<ReturnType<typeof libraryApi.getTitle>> } = $props();

	let name = $state('');
	let year = $state('');
	let contentRating = $state('');
	let overview = $state('');
	let status = $state<string>('draft');
	let genres = $state('');
	let runtime = $state('');
	let languages = $state<string[]>([]);
	let saving = $state(false);
	const form = new FormState(() => ({
		name,
		year,
		contentRating,
		overview,
		status,
		genres,
		runtime,
		languages: languages.join('|')
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
			languages = title.metadataLanguages ?? [];
			form.reset();
		});
	});

	let confirmDeleteTitle = $state(false);
	let tmdbOpen = $state(false);
	let importOpen = $state(false);
	// a TMDB metadata/import job (or the chained pair) is running
	let jobActive = $state(false);

	// Apply TMDB metadata + artwork. For a series, automatically chain into the
	// episode import so the whole show fills in from one action, holding the
	// in-page loading indicator across both jobs.
	async function runTmdb(tmdbId: number) {
		jobActive = true;
		const pending = toast.loading(m.library_tmdb_fetching());
		try {
			const { jobId } = await libraryApi.applyTmdb(data.title.id, tmdbId);
			const job = await waitForJob(jobId);
			if (job?.status === 'failed') {
				toast.error(m.library_tmdb_fetch_failed(), { id: pending });
				return;
			}
			await invalidateAll(); // pull in the new poster/backdrop/metadata + tmdbId
			if (data.title.kind === 'series') {
				toast.loading(m.library_importing_episodes(), { id: pending });
				await importEpisodeJob(pending);
			} else {
				toast.success(m.library_tmdb_applied(), { id: pending });
			}
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.library_tmdb_apply_failed(), {
				id: pending
			});
		} finally {
			jobActive = false;
		}
	}

	// Manual "Import episodes" path (re-import or pick specific seasons).
	async function runImport(seasons?: number[]) {
		jobActive = true;
		const pending = toast.loading(m.library_importing_episodes());
		try {
			await importEpisodeJob(pending, seasons);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.library_import_failed(), { id: pending });
		} finally {
			jobActive = false;
		}
	}

	// Run the import job and refresh, then refresh once more after a short delay:
	// episode stills download best-effort *after* the job completes, so the
	// delayed reload pulls in thumbnails that land just afterwards (Artwork shows
	// a letter fallback until then).
	async function importEpisodeJob(toastId: string | number, seasons?: number[]) {
		const { jobId } = await libraryApi.importEpisodes(data.title.id, seasons);
		const job = await waitForJob(jobId);
		await invalidateAll();
		if (job?.status === 'failed') {
			toast.error(m.library_episode_import_failed(), { id: toastId });
			return;
		}
		toast.success(m.library_episodes_imported(), { id: toastId });
		setTimeout(() => invalidateAll(), 4000);
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		// a newly-added content language has no TMDB text yet - re-fetch to pull it
		const addedLang =
			data.title.tmdbId != null &&
			languages.some((l) => !(data.title.metadataLanguages ?? []).includes(l));
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
					.filter(Boolean),
				metadataLanguages: languages
			});
			form.reset();
			toast.success(m.common_saved());
			await invalidateAll();
			if (addedLang && data.title.tmdbId != null) {
				await runTmdb(data.title.tmdbId);
			}
		} catch {
			toast.error(m.common_save_failed());
		} finally {
			saving = false;
		}
	}

	async function deleteTitle() {
		try {
			await libraryApi.deleteTitle(data.title.id);
			toast.success(m.library_title_deleted());
			goto('/admin/library');
		} catch {
			toast.error(m.common_delete_failed());
		}
	}
</script>

<svelte:head>
	<title>{m.library_editor_title({ name: data.title.name })}</title>
</svelte:head>

<EditorHero
	title={data.title}
	artwork={data.artwork}
	busy={jobActive}
	onFetchTmdb={() => (tmdbOpen = true)}
	onImportEpisodes={() => (importOpen = true)}
	onDelete={() => (confirmDeleteTitle = true)}
/>

<div class="mx-auto max-w-7xl space-y-8 px-8 pb-12">
	<form onsubmit={save} class="space-y-4 rounded-card border border-edge bg-surface/40 p-6">
		<h2 class="text-sm font-semibold text-muted">{m.library_metadata()}</h2>
		<div class="grid gap-4 sm:grid-cols-2">
			<Input label={m.common_name()} bind:value={name} required />
			<Input label={m.library_year()} type="number" bind:value={year} />
			<Input
				label={m.library_content_rating()}
				bind:value={contentRating}
				placeholder="TV-14, PG-13…"
			/>
			{#if data.title.kind === 'movie'}
				<Input label={m.library_runtime_minutes()} type="number" bind:value={runtime} />
			{/if}
		</div>
		<Textarea label={m.library_overview()} bind:value={overview} />
		<Input
			label={m.library_genres()}
			bind:value={genres}
			placeholder={m.library_genres_placeholder()}
		/>
		<div>
			<p class="mb-1.5 text-xs font-medium text-muted">{m.library_content_languages()}</p>
			<LanguageChips bind:selected={languages} />
			<p class="mt-1.5 text-[11px] text-faint">{m.library_content_languages_help()}</p>
		</div>
		<div class="flex items-center justify-between pt-2">
			<Select
				bind:value={status}
				label={m.common_status()}
				items={[
					{ value: 'draft', label: m.library_status_draft() },
					{ value: 'published', label: m.library_status_published() },
					{ value: 'hidden', label: m.library_status_hidden() }
				]}
			/>
			<Button type="submit" loading={saving} disabled={!form.dirty}
				>{m.library_save_changes()}</Button
			>
		</div>
	</form>

	{#if data.title.kind === 'series'}
		<EpisodesTable
			titleId={data.title.id}
			seasons={data.seasons ?? []}
			mediaFiles={data.mediaFiles}
			subtitlesByFile={data.subtitlesByFile ?? {}}
			importing={jobActive}
		/>
	{:else}
		<MovieFilesPanel
			titleId={data.title.id}
			mediaFiles={data.mediaFiles}
			subtitlesByFile={data.subtitlesByFile ?? {}}
		/>
	{/if}
</div>

<Confirm
	bind:open={confirmDeleteTitle}
	title={m.library_delete_title_confirm({ name: data.title.name })}
	message={m.library_delete_title_message()}
	onconfirm={deleteTitle}
/>

<TmdbSearchModal bind:open={tmdbOpen} title={data.title} onApply={runTmdb} />

{#if data.title.kind === 'series'}
	<ImportEpisodesModal bind:open={importOpen} titleId={data.title.id} onImport={runImport} />
{/if}
