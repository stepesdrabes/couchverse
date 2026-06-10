<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { UploadCloud } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { Episode, MediaFile } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { SubtitleInfo } from '$lib/features/library/api';
	import type { Upload } from '$lib/features/uploads/uploader.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import FileVariants from '$lib/components/admin/FileVariants.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Textarea from '$lib/components/ui/Textarea.svelte';
	import { formatBytes, formatYearDate, qualityLabel } from '$lib/utils/format';
	import SubtitleManager from './SubtitleManager.svelte';

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

	$effect(() => {
		if (episode && episode.id !== loadedId) {
			loadedId = episode.id;
			name = episode.name;
			overview = episode.overview;
			upload = null;
		}
	});

	async function save() {
		if (!episode) return;
		saving = true;
		try {
			await libraryApi.updateEpisode(episode.id, { name, overview });
			toast.success('Episode saved');
			invalidateAll();
		} catch {
			toast.error('Failed to save episode');
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
		size="lg"
		title="Episode {episode.episodeNumber}"
		description={episode.airDate ? `Aired ${formatYearDate(episode.airDate)}` : ''}
	>
		<div class="space-y-4">
			<Input label="Name" bind:value={name} />
			<Textarea label="Overview" bind:value={overview} />
			<div class="flex justify-end">
				<Button size="sm" loading={saving} onclick={save}>Save</Button>
			</div>

			{#if file}
				<div class="rounded-input border border-edge/70 bg-surface/40 p-3 text-xs">
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
				</div>
				<SubtitleManager mediaFile={file} {subtitles} />
			{:else if !uploadActive}
				<p class="text-xs text-faint">No video file attached to this episode yet.</p>
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
				{file ? 'Upload replacement file' : 'Upload file'}
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
