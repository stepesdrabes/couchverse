<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { CircleCheck, Languages, UploadCloud, X } from 'lucide-svelte';
	import type { MediaFile } from '$lib/features/catalog/types';
	import type { SubtitleInfo } from '$lib/features/library/api';
	import type { Upload } from '$lib/features/uploads/uploader.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import MediaFileJobs from '$lib/features/jobs/components/MediaFileJobs.svelte';
	import FileVariants from '$lib/features/library/components/FileVariants.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { formatBytes, qualityLabel } from '$lib/utils/format';
	import SubtitlesModal from './SubtitlesModal.svelte';

	let {
		titleId,
		mediaFiles,
		subtitlesByFile
	}: {
		titleId: string;
		mediaFiles: MediaFile[];
		subtitlesByFile: Record<string, SubtitleInfo[]>;
	} = $props();

	// only the uploads started from this panel
	let mine = $state<Upload[]>([]);
	let fileInput = $state<HTMLInputElement>();
	let subsFor = $state<MediaFile | null>(null);
	let subsOpen = $state(false);

	async function addFiles(list: FileList | null) {
		if (!list?.length) return;
		const created = await uploadQueue.add([...list], 'movies', {
			assign: { titleId },
			onDone: () => invalidateAll()
		});
		mine = [...mine, ...created];
	}

	function openSubtitles(file: MediaFile) {
		subsFor = file;
		subsOpen = true;
	}
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<div class="mb-4 flex items-center justify-between gap-3">
		<h2 class="text-sm font-semibold text-muted">Files</h2>
		<Button variant="secondary" size="sm" onclick={() => fileInput?.click()}>
			<UploadCloud class="size-3.5" />
			Upload file
		</Button>
	</div>

	{#if mine.length > 0}
		<ul class="mb-4 space-y-2">
			{#each mine as upload (upload)}
				<li class="text-xs">
					<div class="flex items-center gap-2">
						<span class="min-w-0 flex-1 truncate font-medium">{upload.file.name}</span>
						{#if upload.status === 'done'}
							<CircleCheck class="size-4 shrink-0 text-success" />
						{:else if upload.status === 'error'}
							<span class="shrink-0 text-danger">{upload.error}</span>
						{:else}
							<span class="shrink-0 text-faint tnum">
								{formatBytes(upload.offset)} / {formatBytes(upload.file.size)}
							</span>
							<button
								class="shrink-0 rounded-full p-0.5 text-faint hover:text-danger"
								onclick={() => upload.abort().then(() => (mine = mine.filter((u) => u !== upload)))}
								aria-label="Cancel upload"
							>
								<X class="size-3.5" />
							</button>
						{/if}
					</div>
					{#if upload.status !== 'done' && upload.status !== 'error'}
						<div class="mt-1 h-1 overflow-hidden rounded-full bg-surface-2">
							<div
								class="h-full rounded-full bg-accent transition-all duration-300"
								style="width: {upload.progress * 100}%"
							></div>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{/if}

	{#if mediaFiles.length === 0}
		<p class="text-xs leading-relaxed text-faint">
			No media files yet - upload one or scan a library folder.
		</p>
	{:else}
		<ul class="space-y-4">
			{#each mediaFiles as file (file.id)}
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
					<div class="mt-1.5 flex flex-wrap items-center gap-1.5">
						{#each subtitlesByFile[file.id] ?? [] as sub (sub.id)}
							<Badge>{sub.lang}</Badge>
						{/each}
						<button
							class="inline-flex items-center gap-1 text-[11px] font-medium text-accent hover:underline"
							onclick={() => openSubtitles(file)}
						>
							<Languages class="size-3" />
							Subtitles
						</button>
					</div>
					<FileVariants {file} />
					<div class="mt-3">
						<p class="mb-1 text-[11px] font-medium text-faint">Jobs</p>
						<MediaFileJobs mediaFileId={file.id} />
					</div>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<input
	bind:this={fileInput}
	type="file"
	multiple
	class="hidden"
	onchange={(e) => {
		addFiles(e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>

<SubtitlesModal
	bind:open={subsOpen}
	mediaFile={subsFor}
	subtitles={subsFor ? (subtitlesByFile[subsFor.id] ?? []) : []}
/>
