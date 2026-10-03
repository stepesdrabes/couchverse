<script lang="ts">
	import type { Card } from '$lib/generated/core';
	import Artwork from './Artwork.svelte';
	import { kindLabel } from '../labels';

	let { card }: { card: Card } = $props();

	const art = $derived(card.poster ?? card.backdrop);

	// --card-accent inherits to the ring div (Tailwind's --tw-ring-color does not);
	// it falls back to the themed accent when the art has no extracted colour
	const cardAccent = $derived(art?.accent?.startsWith('#') ? `--card-accent:${art.accent}` : '');
</script>

<a href="/title/{card.slug}" class="group outline-none" style={cardAccent}>
	<div
		class="aspect-[2/3] overflow-hidden rounded-xl border border-edge/50 transition-all duration-300
			group-hover:scale-[1.02] group-hover:shadow-lg group-hover:shadow-black/40 group-hover:ring-2
			group-hover:ring-offset-2 group-focus-visible:scale-[1.02] group-focus-visible:shadow-lg
			group-focus-visible:shadow-black/40 group-focus-visible:ring-3 group-focus-visible:ring-offset-2"
		style="--tw-ring-color:var(--card-accent,var(--color-accent));--tw-ring-offset-color:var(--color-bg)"
	>
		<Artwork src={art?.url} name={card.name} />
	</div>
	<p
		class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent-ink group-focus-visible:text-accent-ink"
	>
		{card.name}
	</p>
	<p class="text-xs text-faint">
		{[kindLabel(card.kind), card.year].filter(Boolean).join(' · ')}
	</p>
</a>
