<script lang="ts">
	import {
		ChevronDown,
		CircleCheck,
		Loader2,
		Pause,
		Play,
		RotateCw,
		UploadCloud,
		X
	} from 'lucide-svelte';
	import { fly, slide } from 'svelte/transition';
	import type { Upload } from '$lib/features/uploads/uploader.svelte';
	import { uploadQueue } from '$lib/features/uploads/uploader.svelte';
	import { formatBytes, formatEta } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	// positioning (bottom offset, stacking vs the music + couch bars) is owned by
	// the root layout's corner stack, so this is just the card
	let collapsed = $state(false);

	const uploads = $derived(uploadQueue.uploads);
	const activeCount = $derived(
		uploads.filter(
			(u) => u.status === 'uploading' || u.status === 'completing' || u.status === 'queued'
		).length
	);
	const doneCount = $derived(uploads.filter((u) => u.status === 'done').length);
	const errorCount = $derived(uploads.filter((u) => u.status === 'error').length);

	// aggregate progress by bytes across the whole queue
	const aggregatePct = $derived.by(() => {
		const total = uploads.reduce((sum, u) => sum + u.file.size, 0);
		if (total === 0) return 0;
		const loaded = uploads.reduce(
			(sum, u) => sum + (u.status === 'done' ? u.file.size : u.offset),
			0
		);
		return Math.round((loaded / total) * 100);
	});

	// live transfer rate + ETA across everything still uploading
	const aggregateSpeed = $derived(
		uploads.filter((u) => u.status === 'uploading').reduce((sum, u) => sum + u.bytesPerSec, 0)
	);
	const remainingBytes = $derived.by(() => {
		const total = uploads.reduce((sum, u) => sum + u.file.size, 0);
		const loaded = uploads.reduce(
			(sum, u) => sum + (u.status === 'done' ? u.file.size : u.offset),
			0
		);
		return Math.max(0, total - loaded);
	});
	const etaSeconds = $derived(aggregateSpeed > 0 ? remainingBytes / aggregateSpeed : null);

	const header = $derived.by(() => {
		if (activeCount > 0) {
			return m.uploads_uploading_progress({
				done: Math.min(doneCount + 1, uploads.length),
				total: uploads.length,
				pct: aggregatePct
			});
		}
		if (errorCount > 0) return m.uploads_failed_done({ failed: errorCount, done: doneCount });
		return uploads.length === 1
			? m.uploads_complete_one()
			: m.uploads_complete_many({ count: doneCount });
	});

	function cancel(upload: Upload) {
		upload.abort().finally(() => uploadQueue.remove(upload));
	}

	function clearFinished() {
		for (const u of uploads.filter((x) => x.status === 'done' || x.status === 'error')) {
			uploadQueue.remove(u);
		}
	}
</script>

{#if uploads.length > 0}
	<div
		transition:fly={{ y: 24, duration: 250 }}
		class="w-96 max-w-[calc(100vw-2rem)] overflow-hidden rounded-card border border-edge bg-surface-2/95
			shadow-2xl shadow-black/50 backdrop-blur"
	>
		<div class="flex items-center gap-2.5 px-3.5 py-3">
			<UploadCloud class="size-5 shrink-0 {activeCount > 0 ? 'text-accent' : 'text-muted'}" />
			<div class="min-w-0 flex-1">
				<p class="truncate text-sm font-semibold">{header}</p>
				{#if activeCount > 0 && aggregateSpeed > 0}
					<p class="truncate text-[11px] text-faint tnum">
						{formatBytes(aggregateSpeed)}/s{#if etaSeconds !== null}
							· {m.uploads_eta_left({ eta: formatEta(etaSeconds) })}{/if}
					</p>
				{/if}
			</div>
			{#if doneCount > 0 || errorCount > 0}
				<button
					class="shrink-0 rounded px-1.5 py-0.5 text-[11px] text-faint transition-colors hover:text-text"
					onclick={clearFinished}
				>
					{m.common_clear()}
				</button>
			{/if}
			<button
				class="shrink-0 rounded-full p-1.5 text-faint transition-colors hover:bg-surface hover:text-text"
				onclick={() => (collapsed = !collapsed)}
				aria-label={collapsed ? m.uploads_expand() : m.uploads_collapse()}
			>
				<ChevronDown class="size-4 transition-transform {collapsed ? '' : 'rotate-180'}" />
			</button>
		</div>

		{#if activeCount > 0}
			<div class="h-1 bg-surface">
				<div
					class="h-full bg-accent transition-all duration-300"
					style="width: {aggregatePct}%"
				></div>
			</div>
		{/if}

		{#if !collapsed}
			<ul
				transition:slide={{ duration: 200 }}
				class="max-h-72 space-y-0.5 overflow-y-auto border-t border-edge/60 p-2 scrollbar-none"
			>
				{#each uploads as upload (upload)}
					<li class="rounded-lg px-2.5 py-2 hover:bg-surface/60">
						<div class="flex items-center gap-2">
							<span class="min-w-0 flex-1 truncate text-[13px] font-medium">{upload.file.name}</span
							>
							{#if upload.status === 'done'}
								<CircleCheck class="size-4 shrink-0 text-success" />
								<button
									class="dock-btn"
									onclick={() => uploadQueue.remove(upload)}
									aria-label={m.uploads_dismiss()}
								>
									<X class="size-3.5" />
								</button>
							{:else if upload.status === 'error'}
								<button
									class="dock-btn"
									onclick={() => upload.start()}
									aria-label={m.uploads_retry()}
								>
									<RotateCw class="size-3.5" />
								</button>
								<button
									class="dock-btn"
									onclick={() => uploadQueue.remove(upload)}
									aria-label={m.uploads_dismiss()}
								>
									<X class="size-3.5" />
								</button>
							{:else if upload.status === 'uploading'}
								<button
									class="dock-btn"
									onclick={() => upload.pause()}
									aria-label={m.uploads_pause()}
								>
									<Pause class="size-3.5" />
								</button>
								<button
									class="dock-btn"
									onclick={() => cancel(upload)}
									aria-label={m.uploads_cancel()}
								>
									<X class="size-3.5" />
								</button>
							{:else if upload.status === 'paused'}
								<button
									class="dock-btn"
									onclick={() => upload.start()}
									aria-label={m.uploads_resume()}
								>
									<Play class="size-3.5" />
								</button>
								<button
									class="dock-btn"
									onclick={() => cancel(upload)}
									aria-label={m.uploads_cancel()}
								>
									<X class="size-3.5" />
								</button>
							{:else}
								<Loader2 class="size-4 shrink-0 animate-spin text-muted" />
							{/if}
						</div>

						{#if upload.status === 'error'}
							<p class="mt-0.5 text-[11px] text-danger">{upload.error}</p>
						{:else if upload.status !== 'done'}
							<div class="mt-2 flex items-center gap-2">
								<div class="h-1.5 flex-1 overflow-hidden rounded-full bg-surface">
									<div
										class="h-full rounded-full bg-accent transition-all duration-300"
										style="width: {upload.progress * 100}%"
									></div>
								</div>
								<span class="shrink-0 text-[11px] text-faint tnum">
									{Math.round(upload.progress * 100)}% · {formatBytes(upload.offset)} / {formatBytes(
										upload.file.size
									)}
								</span>
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</div>
{/if}

<style lang="scss">
	:global(.dock-btn) {
		flex-shrink: 0;
		border-radius: 9999px;
		padding: 0.25rem;
		color: var(--color-faint);
		transition:
			background-color 0.15s,
			color 0.15s;

		&:hover {
			background-color: var(--color-surface);
			color: var(--color-text);
		}
	}
</style>
