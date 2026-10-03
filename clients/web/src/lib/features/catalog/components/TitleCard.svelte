<script lang="ts">
	import type { Card } from '$lib/generated/core';
	import Artwork from './Artwork.svelte';
	import { kindLabel } from '../labels';

	let { card }: { card: Card } = $props();

	const meta = $derived([kindLabel(card.kind), card.year].filter(Boolean).join(' · '));
	const art = $derived(card.backdrop ?? card.poster);
	const cardAccent = $derived(art?.accent?.startsWith('#') ? `--card-accent:${art.accent}` : '');
</script>

<a
	href="/title/{card.slug}"
	class="group w-48 shrink-0 snap-start outline-none sm:w-56"
	style={cardAccent}
>
	<div
		class="aspect-video overflow-hidden rounded-xl border border-edge/50 transition-all duration-300
			group-hover:scale-[1.02] group-hover:shadow-lg group-hover:shadow-black/40 group-hover:ring-2
			group-hover:ring-offset-2 group-focus-visible:scale-[1.02] group-focus-visible:shadow-lg
			group-focus-visible:shadow-black/40 group-focus-visible:ring-3 group-focus-visible:ring-offset-2"
		style="--tw-ring-color:var(--card-accent,var(--color-accent));--tw-ring-offset-color:var(--color-bg)"
	>
		<Artwork src={art?.url} name={card.name} />
	</div>
	<p
		class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent group-focus-visible:text-accent"
	>
		{card.name}
	</p>
	<p class="text-xs text-faint">{meta}</p>
</a>
