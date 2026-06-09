<script lang="ts">
	import type { CardItem } from '$lib/features/catalog/types';
	import Artwork from './Artwork.svelte';

	let { item }: { item: CardItem } = $props();

	const meta = $derived(
		[item.kind === 'series' ? 'Series' : 'Movie', item.year].filter(Boolean).join(' · ')
	);
</script>

<a href="/title/{item.titleId}" class="group w-48 shrink-0 snap-start sm:w-56">
	<div
		class="aspect-video overflow-hidden rounded-xl border border-edge/50 transition-all
			duration-300 group-hover:scale-[1.04] group-hover:border-accent/60 group-hover:shadow-lg
			group-hover:shadow-black/40"
	>
		<Artwork artworkId={item.backdropId ?? item.posterId} name={item.name} />
	</div>
	<p class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent">
		{item.name}
	</p>
	<p class="text-xs text-faint">{meta}</p>
</a>
