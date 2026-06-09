<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { ArrowLeft, Plus, Sparkles, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as libraryApi from '$lib/features/library/api';
	import type { Season } from '$lib/features/catalog/types';
	import ArtworkCard from '$lib/components/admin/ArtworkCard.svelte';
	import FileVariants from '$lib/components/admin/FileVariants.svelte';
	import SubtitlesCard from '$lib/components/admin/SubtitlesCard.svelte';
	import TmdbSearchModal from '$lib/components/admin/TmdbSearchModal.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import StatusPill from '$lib/components/ui/StatusPill.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import { formatBytes, qualityLabel } from '$lib/utils/format';

	let { data } = $props();

	let name = $state(data.title.name);
	let year = $state(data.title.year?.toString() ?? '');
	let contentRating = $state(data.title.contentRating);
	let overview = $state(data.title.overview);
	let status = $state<string>(data.title.status);
	let genres = $state(data.title.genres.join(', '));
	let runtime = $state(data.title.runtimeMinutes?.toString() ?? '');
	let saving = $state(false);
	let confirmDeleteTitle = $state(false);
	let tmdbOpen = $state(false);

	// season/episode editing state
	let newEpisodeName = $state<Record<number, string>>({});

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
			toast.success('Saved');
			invalidateAll();
		} catch {
			toast.error('Failed to save');
		} finally {
			saving = false;
		}
	}

	async function addSeason() {
		const nextNumber = (data.seasons?.length ?? 0) + 1;
		try {
			await libraryApi.createSeason(data.title.id, nextNumber, `Season ${nextNumber}`);
			invalidateAll();
		} catch {
			toast.error('Failed to add season');
		}
	}

	async function removeSeason(season: Season) {
		try {
			await libraryApi.deleteSeason(season.id);
			invalidateAll();
		} catch {
			toast.error('Failed to delete season');
		}
	}

	async function addEpisode(season: Season) {
		const episodeName = newEpisodeName[season.id]?.trim();
		if (!episodeName) return;
		try {
			await libraryApi.createEpisode(season.id, {
				episodeNumber: season.episodes.length + 1,
				name: episodeName
			});
			newEpisodeName[season.id] = '';
			invalidateAll();
		} catch {
			toast.error('Failed to add episode');
		}
	}

	async function removeEpisode(id: number) {
		try {
			await libraryApi.deleteEpisode(id);
			invalidateAll();
		} catch {
			toast.error('Failed to delete episode');
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

<a
	href="/admin/library"
	class="mb-4 inline-flex items-center gap-1.5 text-xs font-medium text-faint transition-colors hover:text-text"
>
	<ArrowLeft class="size-3.5" />
	Library
</a>

<div class="mb-6 flex items-center gap-4">
	<h1 class="text-2xl font-bold">{data.title.name}</h1>
	<StatusPill status={data.title.status} />
	<span class="text-xs text-faint capitalize">{data.title.kind}</span>
</div>

<div class="grid gap-8 lg:grid-cols-[1fr_320px]">
	<div class="space-y-8">
		<form onsubmit={save} class="space-y-4 rounded-card border border-edge bg-surface/40 p-6">
			<div class="flex items-center justify-between">
				<h2 class="text-sm font-semibold text-muted">Metadata</h2>
				<Button variant="secondary" size="sm" onclick={() => (tmdbOpen = true)}>
					<Sparkles class="size-3.5" />
					Fetch from TMDB
				</Button>
			</div>
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
				<Button type="submit" loading={saving}>Save changes</Button>
			</div>
		</form>

		{#if data.title.kind === 'series'}
			<section class="rounded-card border border-edge bg-surface/40 p-6">
				<div class="mb-4 flex items-center justify-between">
					<h2 class="text-sm font-semibold text-muted">Seasons & episodes</h2>
					<Button variant="secondary" size="sm" onclick={addSeason}>
						<Plus class="size-3.5" />
						Add season
					</Button>
				</div>

				{#each data.seasons ?? [] as season (season.id)}
					<div class="mb-5 last:mb-0">
						<div class="mb-2 flex items-center justify-between">
							<h3 class="text-sm font-semibold">
								{season.name || `Season ${season.seasonNumber}`}
								<span class="ml-2 text-xs font-normal text-faint tnum">
									{season.episodes.length} episodes
								</span>
							</h3>
							<button
								class="rounded-full p-1.5 text-faint transition-colors hover:bg-danger/15 hover:text-danger"
								title="Delete season"
								onclick={() => removeSeason(season)}
							>
								<Trash2 class="size-3.5" />
							</button>
						</div>
						<ul class="divide-y divide-edge/50 rounded-input border border-edge/70">
							{#each season.episodes as ep (ep.id)}
								<li class="flex items-center gap-3 px-3 py-2 text-sm">
									<span class="w-8 text-xs text-faint tnum">E{ep.episodeNumber}</span>
									<span class="flex-1 truncate">{ep.name || 'Untitled'}</span>
									<button
										class="rounded-full p-1.5 text-faint transition-colors hover:bg-danger/15 hover:text-danger"
										onclick={() => removeEpisode(ep.id)}
									>
										<Trash2 class="size-3" />
									</button>
								</li>
							{/each}
							<li class="flex items-center gap-2 px-3 py-2">
								<input
									bind:value={newEpisodeName[season.id]}
									placeholder="New episode name…"
									class="h-8 flex-1 rounded-lg border border-transparent bg-transparent px-2 text-sm
										placeholder:text-faint focus:border-edge focus:outline-none"
									onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addEpisode(season))}
								/>
								<Button variant="ghost" size="sm" onclick={() => addEpisode(season)}>
									<Plus class="size-3.5" />
									Add
								</Button>
							</li>
						</ul>
					</div>
				{:else}
					<p class="text-xs text-faint">No seasons yet.</p>
				{/each}
			</section>
		{/if}
	</div>

	<aside class="space-y-6">
		<ArtworkCard titleId={data.title.id} artwork={data.artwork} />
		<SubtitlesCard mediaFiles={data.mediaFiles} />

		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="mb-3 text-sm font-semibold text-muted">Files</h2>
			{#if data.mediaFiles.length === 0}
				<p class="text-xs leading-relaxed text-faint">
					No media files yet — scan a library folder or upload to attach video to this title.
				</p>
			{:else}
				<ul class="space-y-3">
					{#each data.mediaFiles as file (file.id)}
						<li class="text-xs">
							<p class="truncate font-mono text-muted" title={file.path}>{file.path}</p>
							<p class="mt-1 flex flex-wrap gap-1">
								{#if qualityLabel(file.height)}
									<Badge>{qualityLabel(file.height)}</Badge>
								{/if}
								{#if file.videoRange !== 'sdr'}
									<Badge>HDR</Badge>
								{/if}
								<Badge>{file.videoCodec || file.audioCodec}</Badge>
								<Badge>{file.container}</Badge>
								<Badge>{formatBytes(file.sizeBytes)}</Badge>
								{#if file.directPlay}
									<Badge>direct play</Badge>
								{/if}
							</p>
							<FileVariants {file} />
						</li>
					{/each}
				</ul>
			{/if}
		</div>

		<div class="rounded-card border border-danger/30 bg-danger/5 p-6">
			<h2 class="mb-2 text-sm font-semibold text-danger">Danger zone</h2>
			<p class="mb-4 text-xs text-faint">
				Removes the title and all its metadata. Files on disk are kept.
			</p>
			<Button variant="danger" size="sm" onclick={() => (confirmDeleteTitle = true)}>
				<Trash2 class="size-3.5" />
				Delete title
			</Button>
		</div>
	</aside>
</div>

<Confirm
	bind:open={confirmDeleteTitle}
	title="Delete “{data.title.name}”?"
	message="This removes the title and all its seasons, episodes and metadata."
	onconfirm={deleteTitle}
/>

<TmdbSearchModal bind:open={tmdbOpen} title={data.title} />
