<script lang="ts">
	import { Play } from 'lucide-svelte';
	import type { ContinueItem } from '$lib/features/catalog/types';
	import Artwork from './Artwork.svelte';

	let { item }: { item: ContinueItem } = $props();

	const pct = $derived(
		item.durationSeconds > 0 ? (item.positionSeconds / item.durationSeconds) * 100 : 0
	);
</script>

<a
	href="/watch/{item.playbackKind}/{item.playbackId}"
	class="group w-48 shrink-0 snap-start sm:w-56"
>
	<div
		class="relative aspect-video overflow-hidden rounded-xl border border-edge/50 transition-all
			duration-300 group-hover:scale-[1.04] group-hover:border-accent/60 group-hover:shadow-lg
			group-hover:shadow-black/40"
	>
		<Artwork artworkId={item.backdropId ?? item.posterId} name={item.name} />
		<div
			class="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0
				transition-opacity group-hover:opacity-100"
		>
			<span class="rounded-full bg-accent p-3 text-white shadow-lg">
				<Play class="size-5 fill-current" />
			</span>
		</div>
		<div class="absolute inset-x-0 bottom-0 h-1 bg-black/50">
			<div class="h-full bg-accent" style="width: {pct}%"></div>
		</div>
	</div>
	<p class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent">
		{item.name}
	</p>
	<p class="truncate text-xs text-faint">{item.episodeLabel || 'Continue watching'}</p>
</a>
