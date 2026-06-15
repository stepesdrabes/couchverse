<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Film, Info, Loader2, Plus, Search, Trash2, UploadCloud } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { Episode, MediaFile, Season } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { SubtitleInfo } from '$lib/features/library/api';
	import type { Upload } from '$lib/features/uploads/uploader.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import EpisodeInfoModal from './EpisodeInfoModal.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		titleId,
		seasons,
		mediaFiles,
		subtitlesByFile,
		importing = false,
		languages = []
	}: {
		titleId: string;
		seasons: Season[];
		mediaFiles: MediaFile[];
		subtitlesByFile: Record<string, SubtitleInfo[]>;
		importing?: boolean;
		languages?: string[];
	} = $props();

	const fileByEpisode = $derived(
		new Map(mediaFiles.filter((f) => f.episodeId).map((f) => [f.episodeId as string, f]))
	);

	// client-side episode filters (data is already loaded)
	let nameQuery = $state('');
	let fileFilter = $state('all'); // all | with | without
	let subFilter = $state('all'); // all | with | without

	const fileTabs = $derived([
		{ value: 'all', label: m.common_all() },
		{ value: 'with', label: m.library_filter_with_file() },
		{ value: 'without', label: m.library_filter_without_file() }
	]);
	const subItems = $derived([
		{ value: 'all', label: m.common_all() },
		{ value: 'with', label: m.library_filter_with_subs() },
		{ value: 'without', label: m.library_filter_without_subs() }
	]);

	const filtersActive = $derived(
		nameQuery.trim() !== '' || fileFilter !== 'all' || subFilter !== 'all'
	);
	const hasAnyEpisode = $derived(seasons.some((s) => s.episodes.length > 0));

	function episodeMatches(ep: Episode): boolean {
		const file = fileByEpisode.get(ep.id);
		if (fileFilter === 'with' && !file) return false;
		if (fileFilter === 'without' && file) return false;
		const hasSubs = !!file && (subtitlesByFile[file.id] ?? []).length > 0;
		if (subFilter === 'with' && !hasSubs) return false;
		if (subFilter === 'without' && hasSubs) return false;
		const q = nameQuery.trim().toLowerCase();
		if (q && !`${ep.name} e${ep.episodeNumber}`.toLowerCase().includes(q)) return false;
		return true;
	}

	// seasons with their matching episodes; empty seasons drop out while filtering
	const filteredSeasons = $derived(
		seasons
			.map((season) => ({ season, episodes: season.episodes.filter(episodeMatches) }))
			.filter((g) => !filtersActive || g.episodes.length > 0)
	);

	let newEpisodeName = $state<Record<string, string>>({});
	let uploads = $state<Record<string, Upload>>({});
	let fileInputs: Record<string, HTMLInputElement | undefined> = {};

	let infoEpisodeId = $state<string | null>(null);
	let infoOpen = $state(false);
	const infoEpisode = $derived(
		seasons.flatMap((s) => s.episodes).find((e) => e.id === infoEpisodeId) ?? null
	);
	const infoFile = $derived(infoEpisode ? (fileByEpisode.get(infoEpisode.id) ?? null) : null);

	async function addSeason() {
		const nextNumber = seasons.length + 1;
		try {
			await libraryApi.createSeason(
				titleId,
				nextNumber,
				m.library_season_number({ number: nextNumber })
			);
			invalidateAll();
		} catch {
			toast.error(m.library_add_season_failed());
		}
	}

	async function removeSeason(season: Season) {
		try {
			await libraryApi.deleteSeason(season.id);
			invalidateAll();
		} catch {
			toast.error(m.library_delete_season_failed());
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
			toast.error(m.library_add_episode_failed());
		}
	}

	async function removeEpisode(id: string) {
		try {
			await libraryApi.deleteEpisode(id);
			invalidateAll();
		} catch {
			toast.error(m.library_delete_episode_failed());
		}
	}

	async function uploadFor(ep: Episode, list: FileList | null) {
		const file = list?.[0];
		if (!file) return;
		const [created] = await uploadQueue.add([file], 'series', {
			assign: { titleId, episodeId: ep.id },
			onDone: () => invalidateAll()
		});
		uploads[ep.id] = created;
	}

	function openInfo(ep: Episode) {
		infoEpisodeId = ep.id;
		infoOpen = true;
	}

	const uploadActive = (u: Upload | undefined) =>
		u !== undefined && u.status !== 'done' && u.status !== 'error';
</script>

<section class="rounded-card border border-edge bg-surface/40 p-6">
	<div class="mb-4 flex items-center justify-between">
		<div class="flex items-center gap-2.5">
			<h2 class="text-sm font-semibold text-muted">{m.library_seasons_episodes()}</h2>
			{#if importing}
				<span class="flex items-center gap-1.5 text-xs font-medium text-accent">
					<Loader2 class="size-3.5 animate-spin" />
					{m.library_importing_from_tmdb()}
				</span>
			{/if}
		</div>
		<Button variant="secondary" size="sm" onclick={addSeason}>
			<Plus class="size-3.5" />
			{m.library_add_season()}
		</Button>
	</div>

	{#if hasAnyEpisode}
		<div class="mb-4 flex flex-wrap items-center gap-3">
			<div class="relative">
				<Search class="absolute top-1/2 left-3 size-3.5 -translate-y-1/2 text-faint" />
				<input
					bind:value={nameQuery}
					placeholder={m.library_filter_episode_name()}
					class="h-9 w-48 rounded-full border border-edge bg-surface pr-3 pl-8 text-xs text-text
						placeholder:text-faint focus:border-accent focus:outline-none"
				/>
			</div>
			<Tabs bind:value={fileFilter} items={fileTabs} />
			<Select bind:value={subFilter} label={m.library_filter_subtitles()} items={subItems} />
		</div>
	{/if}

	{#each filteredSeasons as { season, episodes } (season.id)}
		<div class="mb-5 last:mb-0">
			<div class="mb-2 flex items-center justify-between">
				<h3 class="text-sm font-semibold">
					{season.name || m.library_season_number({ number: season.seasonNumber })}
					<span class="ml-2 text-xs font-normal text-faint tnum">
						{m.library_episode_count({ count: season.episodes.length })}
					</span>
				</h3>
				<button
					class="rounded-full p-1.5 text-faint transition-colors hover:bg-danger/15 hover:text-danger"
					title={m.library_delete_season()}
					onclick={() => removeSeason(season)}
				>
					<Trash2 class="size-3.5" />
				</button>
			</div>
			<ul class="divide-y divide-edge/50 rounded-input border border-edge/70">
				{#each episodes as ep (ep.id)}
					{@const file = fileByEpisode.get(ep.id)}
					{@const upload = uploads[ep.id]}
					<li class="flex items-center gap-3 px-3 py-2 text-sm">
						<span class="w-8 shrink-0 text-xs text-faint tnum">E{ep.episodeNumber}</span>
						<div class="min-w-0 flex-1">
							<div class="flex items-center gap-2">
								<span class="truncate">{ep.name || m.common_untitled()}</span>
								{#if file}
									<span title={m.library_has_video_file()} class="shrink-0 text-success">
										<Film class="size-3" />
									</span>
									{#each subtitlesByFile[file.id] ?? [] as sub (sub.id)}
										<Badge>{sub.lang}</Badge>
									{/each}
								{/if}
							</div>
							{#if uploadActive(upload)}
								<div class="mt-1.5 flex items-center gap-2">
									<div class="h-1 w-40 overflow-hidden rounded-full bg-surface-2">
										<div
											class="h-full rounded-full bg-accent transition-all duration-300"
											style="width: {upload.progress * 100}%"
										></div>
									</div>
									<span class="text-[10px] text-faint tnum">
										{Math.round(upload.progress * 100)}%
									</span>
								</div>
							{:else if upload?.status === 'error'}
								<p class="mt-0.5 text-[11px] text-danger">{upload.error}</p>
							{/if}
						</div>
						<button
							class="rounded-full p-1.5 text-faint transition-colors hover:bg-surface-2 hover:text-text"
							title={m.library_upload_episode_file()}
							onclick={() => fileInputs[ep.id]?.click()}
						>
							<UploadCloud class="size-3.5" />
						</button>
						<button
							class="rounded-full p-1.5 text-faint transition-colors hover:bg-surface-2 hover:text-text"
							title={m.library_episode_details()}
							onclick={() => openInfo(ep)}
						>
							<Info class="size-3.5" />
						</button>
						<button
							class="rounded-full p-1.5 text-faint transition-colors hover:bg-danger/15 hover:text-danger"
							title={m.library_delete_episode()}
							onclick={() => removeEpisode(ep.id)}
						>
							<Trash2 class="size-3" />
						</button>
						<input
							bind:this={fileInputs[ep.id]}
							type="file"
							class="hidden"
							onchange={(e) => {
								uploadFor(ep, e.currentTarget.files);
								e.currentTarget.value = '';
							}}
						/>
					</li>
				{/each}
				<li class="flex items-center gap-2 px-3 py-2">
					<input
						bind:value={newEpisodeName[season.id]}
						placeholder={m.library_new_episode_name()}
						class="h-8 flex-1 rounded-lg border border-transparent bg-transparent px-2 text-sm
							placeholder:text-faint focus:border-edge focus:outline-none"
						onkeydown={(e) => e.key === 'Enter' && (e.preventDefault(), addEpisode(season))}
					/>
					<Button variant="ghost" size="sm" onclick={() => addEpisode(season)}>
						<Plus class="size-3.5" />
						{m.common_add()}
					</Button>
				</li>
			</ul>
		</div>
	{:else}
		{#if seasons.length === 0}
			{#if importing}
				<div class="space-y-2">
					{#each [0, 1, 2] as i (i)}
						<Skeleton class="h-12 w-full" />
					{/each}
				</div>
			{:else}
				<p class="text-xs text-faint">{m.library_no_seasons()}</p>
			{/if}
		{:else}
			<p class="text-xs text-faint">{m.library_no_episodes_match()}</p>
		{/if}
	{/each}
</section>

<EpisodeInfoModal
	bind:open={infoOpen}
	{titleId}
	episode={infoEpisode}
	file={infoFile}
	subtitles={infoFile ? (subtitlesByFile[infoFile.id] ?? []) : []}
	{languages}
/>
