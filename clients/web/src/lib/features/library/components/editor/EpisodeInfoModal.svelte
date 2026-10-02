<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Loader2, UploadCloud } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { Episode, MediaFile } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { SubtitleInfo } from '$lib/features/library/api';
	import type { Upload } from '$lib/features/uploads/uploader.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import MediaFileJobs from '$lib/features/jobs/components/MediaFileJobs.svelte';
	import FileVariants from '$lib/features/library/components/FileVariants.svelte';
	import AudioLangControl from './AudioLangControl.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import { formatBytes, formatYearDate, qualityLabel } from '$lib/utils/format';
	import { langLabel } from '$lib/i18n/content-langs';
	import SubtitleManager from './SubtitleManager.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		titleId,
		episode,
		file,
		subtitles,
		languages = []
	}: {
		open?: boolean;
		titleId: string;
		episode: Episode | null;
		file: MediaFile | null;
		subtitles: SubtitleInfo[];
		languages?: string[];
	} = $props();

	let name = $state('');
	let overview = $state('');
	let saving = $state(false);
	let upload = $state<Upload | null>(null);
	let fileInput = $state<HTMLInputElement>();
	let loadedId = '';
	let jobActive = $state(false);
	const form = new FormState(() => ({ name, overview }));

	// per-language editing: base language edits the plain columns, others edit the
	// episode's translation (fetched on open, works for non-TMDB episodes too).
	const baseLang = $derived(languages[0] ?? 'en');
	const editLangs = $derived(languages.length ? languages : [baseLang]);
	let editLang = $state('');
	let translations = $state<Record<string, { name?: string; overview?: string }>>({});
	let tName = $state('');
	let tOverview = $state('');
	let tLoadedName = $state('');
	let tLoadedOverview = $state('');
	const tDirty = $derived(tName !== tLoadedName || tOverview !== tLoadedOverview);

	function pickLang(lang: string) {
		editLang = lang;
		if (lang !== baseLang) {
			const tr = translations[lang] ?? {};
			tName = tr.name ?? '';
			tOverview = tr.overview ?? '';
			tLoadedName = tName;
			tLoadedOverview = tOverview;
		}
	}

	async function saveTranslation() {
		if (!episode) return;
		saving = true;
		try {
			await libraryApi.setEpisodeTranslation(episode.id, editLang, {
				name: tName,
				overview: tOverview
			});
			translations = { ...translations, [editLang]: { name: tName, overview: tOverview } };
			tLoadedName = tName;
			tLoadedOverview = tOverview;
			toast.success(m.library_episode_saved());
		} catch {
			toast.error(m.library_save_episode_failed());
		} finally {
			saving = false;
		}
	}

	$effect(() => {
		if (episode && episode.id !== loadedId) {
			loadedId = episode.id;
			name = episode.name;
			overview = episode.overview;
			upload = null;
			form.reset();
			editLang = baseLang;
			translations = {};
			tName = '';
			tOverview = '';
			tLoadedName = '';
			tLoadedOverview = '';
			libraryApi
				.getEpisodeTranslations(episode.id)
				.then((t) => (translations = t ?? {}))
				.catch(() => {});
		}
	});

	async function save() {
		if (!episode) return;
		saving = true;
		try {
			await libraryApi.updateEpisode(episode.id, { name, overview });
			form.reset();
			toast.success(m.library_episode_saved());
			invalidateAll();
		} catch {
			toast.error(m.library_save_episode_failed());
		} finally {
			saving = false;
		}
	}

	async function uploadFile(list: FileList | null) {
		const picked = list?.[0];
		if (!picked || !episode) return;
		const [created] = await uploadQueue.add([picked], 'series', {
			assign: { titleId, episodeId: episode.id },
			onDone: () => invalidateAll()
		});
		upload = created;
	}

	const uploadActive = $derived(
		upload !== null && upload.status !== 'done' && upload.status !== 'error'
	);
</script>

{#if episode}
	<Modal
		bind:open
		size="xl"
		title={m.library_episode_number({ number: episode.episodeNumber })}
		description={episode.airDate
			? m.library_episode_aired({ date: formatYearDate(episode.airDate) })
			: ''}
	>
		<div class="space-y-6">
			<div class="space-y-4">
				{#if editLangs.length > 1}
					<div class="flex flex-wrap gap-1">
						{#each editLangs as lang (lang)}
							<button
								type="button"
								onclick={() => pickLang(lang)}
								class="rounded-full border px-2.5 py-1 text-xs transition-colors
									{editLang === lang
									? 'border-accent bg-accent/15 text-text'
									: 'border-edge text-muted hover:border-faint'}"
							>
								{langLabel(lang)}
							</button>
						{/each}
					</div>
				{/if}
				{#if editLang !== baseLang}
					<Input label={m.common_name()} bind:value={tName} />
					<Textarea label={m.library_overview()} bind:value={tOverview} rows={4} />
					<p class="text-[11px] text-faint">
						{m.library_translation_hint({ lang: langLabel(editLang) })}
					</p>
					<div class="flex justify-end">
						<Button loading={saving} disabled={!tDirty} onclick={saveTranslation}
							>{m.common_save()}</Button
						>
					</div>
				{:else}
					<Input label={m.common_name()} bind:value={name} />
					<Textarea label={m.library_overview()} bind:value={overview} rows={4} />
					<div class="flex justify-end">
						<Button loading={saving} disabled={!form.dirty} onclick={save}>{m.common_save()}</Button
						>
					</div>
				{/if}
			</div>

			{#if file}
				{@const codec = file.videoCodec || file.audioCodec}
				<section>
					<h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-muted">
						{m.library_file()}
						{#if jobActive}
							<span class="flex items-center gap-1.5 text-xs font-medium text-accent">
								<Loader2 class="size-3.5 animate-spin" />
								{m.library_processing()}
							</span>
						{/if}
					</h3>
					<p class="truncate text-sm text-muted" title={file.path}>{file.path}</p>
					<p class="mt-2 flex flex-wrap gap-1">
						{#if qualityLabel(file.height)}
							<Badge>{qualityLabel(file.height)}</Badge>
						{/if}
						{#if file.videoRange !== 'sdr'}
							<Badge>HDR</Badge>
						{/if}
						{#if codec}
							<Badge>{codec}</Badge>
						{/if}
						{#if file.container}
							<Badge>{file.container}</Badge>
						{/if}
						{#if file.sizeBytes > 0}
							<Badge>{formatBytes(file.sizeBytes)}</Badge>
						{/if}
						{#if file.directPlay}
							<Badge>{m.library_direct_play()}</Badge>
						{/if}
					</p>
					<FileVariants {file} />
					<AudioLangControl {file} />
				</section>

				<section>
					<h3 class="mb-2 text-sm font-semibold text-muted">{m.library_jobs()}</h3>
					<MediaFileJobs mediaFileId={file.id} bind:active={jobActive} />
				</section>

				<SubtitleManager mediaFile={file} {subtitles} />
			{:else if !uploadActive}
				<p class="text-xs text-faint">{m.library_no_video_file()}</p>
			{/if}

			{#if upload}
				<div class="text-xs">
					<div class="flex items-center gap-2">
						<span class="min-w-0 flex-1 truncate font-medium">{upload.file.name}</span>
						{#if upload.status === 'error'}
							<span class="shrink-0 text-danger">{upload.error}</span>
						{:else if upload.status !== 'done'}
							<span class="shrink-0 text-faint tnum">
								{formatBytes(upload.offset)} / {formatBytes(upload.file.size)}
							</span>
						{/if}
					</div>
					{#if uploadActive}
						<div class="mt-1 h-1 overflow-hidden rounded-full bg-surface-2">
							<div
								class="h-full rounded-full bg-accent transition-all duration-300"
								style="width: {upload.progress * 100}%"
							></div>
						</div>
					{/if}
				</div>
			{/if}

			<Button variant="secondary" size="sm" onclick={() => fileInput?.click()}>
				<UploadCloud class="size-3.5" />
				{file ? m.library_upload_replacement_file() : m.library_upload_file()}
			</Button>
		</div>
	</Modal>

	<input
		bind:this={fileInput}
		type="file"
		class="hidden"
		onchange={(e) => {
			uploadFile(e.currentTarget.files);
			e.currentTarget.value = '';
		}}
	/>
{/if}
