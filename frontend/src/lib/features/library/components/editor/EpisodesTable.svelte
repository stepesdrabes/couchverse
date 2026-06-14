<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Film, Info, Loader2, Plus, Trash2, UploadCloud } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { Episode, MediaFile, Season } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { SubtitleInfo } from '$lib/features/library/api';
	import type { Upload } from '$lib/features/uploads/uploader.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
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

	{#each seasons as season (season.id)}
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
				{#each season.episodes as ep (ep.id)}
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
		{#if importing}
			<div class="space-y-2">
				{#each [0, 1, 2] as i (i)}
					<Skeleton class="h-12 w-full" />
				{/each}
			</div>
		{:else}
			<p class="text-xs text-faint">{m.library_no_seasons()}</p>
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
