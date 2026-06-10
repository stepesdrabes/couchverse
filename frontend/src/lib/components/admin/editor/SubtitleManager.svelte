<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { Plus, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import type { MediaFile } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';
	import type { SubtitleInfo } from '$lib/features/library/api';

	let { mediaFile, subtitles }: { mediaFile: MediaFile; subtitles: SubtitleInfo[] } = $props();

	let lang = $state('en');
	let fileInput = $state<HTMLInputElement>();

	async function upload(files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			await libraryApi.uploadSubtitle(mediaFile.id, lang.trim() || 'und', file);
			toast.success('Subtitle added');
			invalidateAll();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'upload failed');
		}
	}

	async function remove(sub: SubtitleInfo) {
		try {
			await libraryApi.deleteSubtitle(sub.id);
			invalidateAll();
		} catch {
			toast.error('failed to delete subtitle');
		}
	}
</script>

<div>
	<div class="mb-2 flex items-center justify-between gap-2">
		<label class="flex items-center gap-1.5 text-xs text-faint">
			Lang
			<input
				bind:value={lang}
				class="h-7 w-14 rounded-lg border border-edge bg-surface px-2 text-center text-xs
					focus:border-accent focus:outline-none"
				maxlength="5"
			/>
		</label>
		<button
			class="inline-flex shrink-0 items-center gap-1 text-[11px] font-medium text-accent hover:underline"
			onclick={() => fileInput?.click()}
		>
			<Plus class="size-3" />
			Add .srt/.vtt
		</button>
	</div>

	{#if subtitles.length}
		<ul class="divide-y divide-edge/40 rounded-input border border-edge/60">
			{#each subtitles as sub (sub.id)}
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
