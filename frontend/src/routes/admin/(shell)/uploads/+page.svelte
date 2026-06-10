<script lang="ts">
	import { CircleCheck, FileVideo, Pause, Play, UploadCloud, X } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import type { LibraryKind } from '$lib/features/uploads/api';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import { formatBytes } from '$lib/utils/format';

	let kind = $state<string>('movies');
	let dragging = $state(false);
	let fileInput = $state<HTMLInputElement>();

	function addFiles(list: FileList | null) {
		if (!list?.length) return;
		uploadQueue.add([...list], kind as LibraryKind);
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragging = false;
		addFiles(e.dataTransfer?.files ?? null);
	}

	const statusLabel = {
		queued: 'Queued',
		uploading: 'Uploading',
		paused: 'Paused',
		completing: 'Finishing…',
		done: 'Done — analyzing in background',
		error: 'Failed'
	};
</script>

<svelte:head>
	<title>Uploads — Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Uploads</h1>

<div class="mb-4 flex items-center gap-4">
	<span class="text-sm text-muted">Upload to:</span>
	<Tabs
		bind:value={kind}
		items={[
			{ value: 'movies', label: 'Movies' },
			{ value: 'series', label: 'Series' },
			{ value: 'music', label: 'Music' }
		]}
	/>
</div>

<button
	type="button"
	class="flex w-full flex-col items-center justify-center gap-3 rounded-card border-2 border-dashed
		px-6 py-16 transition-colors
		{dragging ? 'border-accent bg-accent-soft/30' : 'border-edge bg-surface/40 hover:border-faint'}"
	ondragover={(e) => {
		e.preventDefault();
		dragging = true;
	}}
	ondragleave={() => (dragging = false)}
	ondrop={onDrop}
	onclick={() => fileInput?.click()}
>
	<UploadCloud class="size-8 text-accent" />
	<span class="text-sm font-semibold">Drop files here or click to choose</span>
	<span class="max-w-md text-xs text-faint">
		Movies: <span class="font-mono">Name (2024).mkv</span> · Series:
		<span class="font-mono">Show S01E01.mkv</span> · Music: tagged audio files. Uploads are resumable
		— re-drop the same file to continue an interrupted one.
	</span>
</button>
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

{#if uploadQueue.uploads.length > 0}
	<ul class="mt-6 space-y-3">
		{#each uploadQueue.uploads as upload (upload)}
			<li
				transition:fly={{ y: 12, duration: 200 }}
				class="rounded-card border border-edge bg-surface/40 p-4"
			>
				<div class="flex items-center gap-3">
					<FileVideo class="size-5 shrink-0 text-accent" />
					<div class="min-w-0 flex-1">
						<p class="truncate text-sm font-semibold">{upload.file.name}</p>
						<p class="text-xs text-faint tnum">
							{formatBytes(upload.offset)} / {formatBytes(upload.file.size)}
							· {statusLabel[upload.status]}
							{#if upload.error && upload.status === 'error'}— {upload.error}{/if}
						</p>
					</div>
					{#if upload.status === 'done'}
						<CircleCheck class="size-5 text-success" />
					{:else if upload.status === 'uploading'}
						<Button variant="ghost" size="sm" onclick={() => upload.pause()}>
							<Pause class="size-3.5" />
							Pause
						</Button>
					{:else if upload.status === 'paused' || upload.status === 'error'}
						<Button variant="ghost" size="sm" onclick={() => upload.start()}>
							<Play class="size-3.5" />
							Resume
						</Button>
					{/if}
					{#if upload.status !== 'done'}
						<Button
							variant="ghost"
							size="sm"
							onclick={() => upload.abort().then(() => uploadQueue.remove(upload))}
						>
							<X class="size-3.5" />
						</Button>
					{:else}
						<Button variant="ghost" size="sm" onclick={() => uploadQueue.remove(upload)}>
							<X class="size-3.5" />
						</Button>
					{/if}
				</div>
				{#if upload.status !== 'done'}
					<div class="mt-3 h-1.5 overflow-hidden rounded-full bg-surface-2">
						<div
							class="h-full rounded-full bg-accent transition-all duration-300"
							style="width: {upload.progress * 100}%"
						></div>
					</div>
				{/if}
			</li>
		{/each}
	</ul>
	<p class="mt-3 text-xs text-faint">
		Finished uploads are analyzed automatically and matched into the library — watch the
		<a href="/admin/jobs" class="text-accent hover:underline">job queue</a>.
	</p>
{/if}
