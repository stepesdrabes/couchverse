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
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import { formatBytes, formatYearDate, qualityLabel } from '$lib/utils/format';
	import SubtitleManager from './SubtitleManager.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		titleId,
		episode,
		file,
		subtitles
	}: {
		open?: boolean;
		titleId: string;
		episode: Episode | null;
		file: MediaFile | null;
		subtitles: SubtitleInfo[];
	} = $props();

	let name = $state('');
	let overview = $state('');
	let saving = $state(false);
	let upload = $state<Upload | null>(null);
	let fileInput = $state<HTMLInputElement>();
	let loadedId = '';
	let jobActive = $state(false);
	const form = new FormState(() => ({ name, overview }));

	$effect(() => {
		if (episode && episode.id !== loadedId) {
			loadedId = episode.id;
			name = episode.name;
			overview = episode.overview;
			upload = null;
			form.reset();
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
				<Input label={m.common_name()} bind:value={name} />
				<Textarea label={m.library_overview()} bind:value={overview} rows={4} />
				<div class="flex justify-end">
					<Button loading={saving} disabled={!form.dirty} onclick={save}>{m.common_save()}</Button>
				</div>
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
