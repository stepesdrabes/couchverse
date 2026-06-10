<script lang="ts">
	import { Plus, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { MediaFile } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { SubtitleInfo } from '$lib/features/library/api';

	let { mediaFiles }: { mediaFiles: MediaFile[] } = $props();

	const videoFiles = $derived(mediaFiles.filter((f) => f.videoCodec !== ''));

	let subsByFile = $state<Record<string, SubtitleInfo[]>>({});
	let lang = $state('en');
	let fileInput = $state<HTMLInputElement>();
	let targetFile = $state<string | null>(null);

	async function refresh() {
		const entries = await Promise.all(
			videoFiles.map(async (f) => [f.id, await libraryApi.listSubtitles(f.id)] as const)
		);
		subsByFile = Object.fromEntries(entries);
	}

	$effect(() => {
		if (videoFiles.length > 0) refresh();
	});

	function pickFile(mediaFileId: string) {
		targetFile = mediaFileId;
		fileInput?.click();
	}

	async function upload(files: FileList | null) {
		const file = files?.[0];
		if (!file || targetFile === null) return;
		try {
			await libraryApi.uploadSubtitle(targetFile, lang.trim() || 'und', file);
			toast.success('Subtitle added');
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'upload failed');
		}
	}

	async function remove(sub: SubtitleInfo) {
		try {
			await libraryApi.deleteSubtitle(sub.id);
			refresh();
		} catch {
			toast.error('failed to delete subtitle');
		}
	}

	const fileLabel = (f: MediaFile) => f.path.split('/').pop() ?? f.path;
</script>

{#if videoFiles.length > 0}
	<div class="rounded-card border border-edge bg-surface/40 p-6">
		<div class="mb-3 flex items-center justify-between gap-3">
			<h2 class="text-sm font-semibold text-muted">Subtitles</h2>
			<label class="flex items-center gap-1.5 text-xs text-faint">
				Lang
				<input
					bind:value={lang}
					class="h-7 w-14 rounded-lg border border-edge bg-surface px-2 text-center text-xs
						focus:border-accent focus:outline-none"
					maxlength="5"
				/>
			</label>
		</div>

		<div class="space-y-4">
			{#each videoFiles as file (file.id)}
				<div>
					<div class="mb-1.5 flex items-center justify-between gap-2">
						<p class="truncate font-mono text-[11px] text-faint" title={file.path}>
							{fileLabel(file)}
						</p>
						<button
							class="inline-flex shrink-0 items-center gap-1 text-[11px] font-medium text-accent hover:underline"
							onclick={() => pickFile(file.id)}
						>
							<Plus class="size-3" />
							Add .srt/.vtt
						</button>
					</div>
					{#if subsByFile[file.id]?.length}
						<ul class="divide-y divide-edge/40 rounded-input border border-edge/60">
							{#each subsByFile[file.id] as sub (sub.id)}
								<li class="flex items-center gap-2 px-2.5 py-1.5 text-xs">
									<span class="font-semibold uppercase">{sub.lang}</span>
									<span class="truncate text-muted">{sub.label}</span>
									<span class="ml-auto text-[10px] text-faint">{sub.source}</span>
									<button
										class="rounded-full p-1 text-faint transition-colors hover:text-danger"
										onclick={() => remove(sub)}
									>
										<Trash2 class="size-3" />
									</button>
								</li>
							{/each}
						</ul>
					{:else}
						<p class="text-[11px] text-faint">No subtitles.</p>
					{/if}
				</div>
			{/each}
		</div>
	</div>
	<input
		bind:this={fileInput}
		type="file"
		accept=".srt,.vtt"
		class="hidden"
		onchange={(e) => {
			upload(e.currentTarget.files);
			e.currentTarget.value = '';
		}}
	/>
{/if}
