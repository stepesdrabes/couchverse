<script lang="ts">
	import { artworkUrl } from '$lib/features/catalog/api';
	import type { CardItem } from '$lib/features/catalog/types';
	import { hoverAccent } from '$lib/utils/palette.svelte';
	import Artwork from './Artwork.svelte';

	let { item }: { item: CardItem } = $props();

	const ca = hoverAccent(() => {
		const id = item.posterId ?? item.backdropId;
		return id ? artworkUrl(id) : null;
	});
	// --card-accent inherits to the ring div (Tailwind's --tw-ring-color does not);
	// it falls back to the themed accent until the poster's colour is extracted
	const cardAccent = $derived(ca.accent ? `--card-accent:${ca.accent}` : '');
</script>

<a href="/title/{item.slug}" class="group" onpointerenter={ca.load} style={cardAccent}>
	<div
		class="aspect-[2/3] overflow-hidden rounded-xl border border-edge/50 transition-all duration-300
			group-hover:scale-[1.02] group-hover:shadow-lg group-hover:shadow-black/40 group-hover:ring-2
			group-hover:ring-offset-2"
		style="--tw-ring-color:var(--card-accent,var(--color-accent));--tw-ring-offset-color:var(--color-bg)"
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
