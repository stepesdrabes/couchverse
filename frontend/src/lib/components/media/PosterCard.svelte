<script lang="ts">
	import type { CardItem } from '$lib/features/catalog/types';
	import Artwork from './Artwork.svelte';

	let { item }: { item: CardItem } = $props();
</script>

<a href="/title/{item.slug}" class="group">
	<div
		class="aspect-[2/3] overflow-hidden rounded-xl border border-edge/50 transition-all
			duration-300 group-hover:scale-[1.04] group-hover:border-accent/60 group-hover:shadow-lg
			group-hover:shadow-black/40"
	>
		<Artwork artworkId={item.posterId ?? item.backdropId} name={item.name} />
	</div>
	<p class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent">
		{item.name}
	</p>
	<p class="text-xs text-faint">
		{[item.kind === 'series' ? 'Series' : 'Movie', item.year].filter(Boolean).join(' · ')}
	</p>
</a>
