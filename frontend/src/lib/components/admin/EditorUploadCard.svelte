<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { CircleCheck, UploadCloud, X } from 'lucide-svelte';
	import type { LibraryKind } from '$lib/features/uploads/api';
	import type { Upload } from '$lib/features/uploads/uploader.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import { formatBytes } from '$lib/utils/format';

	let {
		kind,
		titleId = null,
		hint
	}: { kind: LibraryKind; titleId?: number | null; hint: string } = $props();

	// only the uploads started from this card
	let mine = $state<Upload[]>([]);
	let fileInput = $state<HTMLInputElement>();
	let dragging = $state(false);

	async function addFiles(list: FileList | null) {
		if (!list?.length) return;
		const created = await uploadQueue.add([...list], kind, {
			assign: titleId ? { titleId } : {},
			onDone: () => invalidateAll()
		});
		mine = [...mine, ...created];
	}
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<h2 class="mb-3 text-sm font-semibold text-muted">Upload media</h2>
	<button
		type="button"
		class="flex w-full flex-col items-center gap-1.5 rounded-input border border-dashed px-4 py-6
			transition-colors
			{dragging ? 'border-accent bg-accent-soft/30' : 'border-edge hover:border-faint'}"
		ondragover={(e) => {
			e.preventDefault();
			dragging = true;
		}}
		ondragleave={() => (dragging = false)}
		ondrop={(e) => {
			e.preventDefault();
			dragging = false;
			addFiles(e.dataTransfer?.files ?? null);
		}}
		onclick={() => fileInput?.click()}
	>
		<UploadCloud class="size-5 text-accent" />
		<span class="text-xs font-semibold">Drop files or click</span>
		<span class="text-[11px] text-faint">{hint}</span>
	</button>

	{#if mine.length > 0}
		<ul class="mt-3 space-y-2">
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
