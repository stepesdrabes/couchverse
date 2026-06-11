<script lang="ts">
	import { artworkUrl } from '$lib/features/catalog/api';
	import type { CardItem } from '$lib/features/catalog/types';
	import { hoverAccent } from '$lib/utils/palette.svelte';
	import Artwork from './Artwork.svelte';

	let { item }: { item: CardItem } = $props();

	const meta = $derived(
		[item.kind === 'series' ? 'Series' : 'Movie', item.year].filter(Boolean).join(' · ')
	);

	const ca = hoverAccent(() => {
		const id = item.backdropId ?? item.posterId;
		return id ? artworkUrl(id) : null;
	});
	const cardAccent = $derived(ca.accent ? `--card-accent:${ca.accent}` : '');
</script>

<a
	href="/title/{item.slug}"
	class="group w-48 shrink-0 snap-start sm:w-56"
	onpointerenter={ca.load}
	style={cardAccent}
>
	<div
		class="aspect-video overflow-hidden rounded-xl border border-edge/50 transition-all duration-300
			group-hover:scale-[1.02] group-hover:shadow-lg group-hover:shadow-black/40 group-hover:ring-2
			group-hover:ring-offset-2"
		style="--tw-ring-color:var(--card-accent,var(--color-accent));--tw-ring-offset-color:var(--color-bg)"
	>
		<Artwork artworkId={item.backdropId ?? item.posterId} name={item.name} />
	</div>
	<p class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent">
		{item.name}
	</p>
	<p class="text-xs text-faint">{meta}</p>
</a>
