<script lang="ts">
	import type { CardItem } from '$lib/features/catalog/types';
	import Artwork from './Artwork.svelte';
	import * as m from '$lib/paraglide/messages';

	let { item }: { item: CardItem } = $props();

	const meta = $derived(
		[item.kind === 'series' ? m.catalog_kind_series() : m.catalog_kind_movie(), item.year]
			.filter(Boolean)
			.join(' · ')
	);

	const art = $derived(
		item.backdropId
			? { id: item.backdropId, v: item.backdropVer, accent: item.backdropAccent }
			: { id: item.posterId, v: item.posterVer, accent: item.posterAccent }
	);

	const cardAccent = $derived(art.accent?.startsWith('#') ? `--card-accent:${art.accent}` : '');
</script>

<a href="/title/{item.slug}" class="group w-48 shrink-0 snap-start sm:w-56" style={cardAccent}>
	<div
		class="aspect-video overflow-hidden rounded-xl border border-edge/50 transition-all duration-300
			group-hover:scale-[1.02] group-hover:shadow-lg group-hover:shadow-black/40 group-hover:ring-2
			group-hover:ring-offset-2"
		style="--tw-ring-color:var(--card-accent,var(--color-accent));--tw-ring-offset-color:var(--color-bg)"
	>
		<Artwork artworkId={art.id} v={art.v} name={item.name} />
	</div>
	<p class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent">
		{item.name}
	</p>
	<p class="text-xs text-faint">{meta}</p>
</a>
