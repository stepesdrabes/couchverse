<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { ImagePlus, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { artworkUrl } from '$lib/features/catalog/api';
	import type { ArtworkRef } from '$lib/features/catalog/types';
	import * as libraryApi from '$lib/features/library/api';

	let { titleId, artwork }: { titleId: string; artwork: ArtworkRef[] } = $props();

	const slots: { kind: 'poster' | 'backdrop'; label: string; aspect: string }[] = [
		{ kind: 'poster', label: 'Poster', aspect: 'aspect-[2/3] w-24' },
		{ kind: 'backdrop', label: 'Backdrop', aspect: 'aspect-video w-40' }
	];

	let inputs: Record<string, HTMLInputElement | undefined> = {};

	const existing = (kind: string) => artwork.find((a) => a.kind === kind);

	async function upload(kind: 'poster' | 'backdrop', files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			await libraryApi.uploadArtwork('title', titleId, kind, file);
			toast.success(`${kind} updated`);
			invalidateAll();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'upload failed');
		}
	}

	async function remove(art: ArtworkRef) {
		try {
			await libraryApi.deleteArtwork(art.id);
			invalidateAll();
		} catch {
			toast.error('failed to delete artwork');
		}
	}
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<h2 class="mb-4 text-sm font-semibold text-muted">Artwork</h2>
	<div class="flex flex-wrap gap-5">
		{#each slots as slot (slot.kind)}
			{@const art = existing(slot.kind)}
			<div>
				<p class="mb-2 text-xs font-medium text-faint">{slot.label}</p>
				<button
					type="button"
					class="group relative block overflow-hidden rounded-input border border-edge bg-surface-2
						transition-colors hover:border-accent {slot.aspect}"
					onclick={() => inputs[slot.kind]?.click()}
					title="Upload {slot.label}"
				>
					{#if art}
						<img
							src="{artworkUrl(art.id)}?size=w342"
							alt={slot.label}
							class="size-full object-cover"
						/>
						<span
							class="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0
								transition-opacity group-hover:opacity-100"
						>
							<ImagePlus class="size-5 text-white" />
						</span>
					{:else}
						<span class="flex size-full items-center justify-center">
							<ImagePlus class="size-5 text-faint" />
						</span>
					{/if}
				</button>
				{#if art}
					<button
						class="mt-1.5 inline-flex items-center gap-1 text-[11px] text-faint transition-colors hover:text-danger"
						onclick={() => remove(art)}
					>
						<Trash2 class="size-3" />
						Remove
					</button>
				{/if}
				<input
					bind:this={inputs[slot.kind]}
					type="file"
					accept=".jpg,.jpeg,.png,.webp"
					class="hidden"
					onchange={(e) => {
						upload(slot.kind, e.currentTarget.files);
						e.currentTarget.value = '';
					}}
				/>
			</div>
		{/each}
	</div>
</div>
