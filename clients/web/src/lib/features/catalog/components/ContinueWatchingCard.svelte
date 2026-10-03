<script lang="ts">
	import { Play } from 'lucide-svelte';
	import type { ContinueCard } from '$lib/generated/core';
	import Artwork from './Artwork.svelte';
	import * as m from '$lib/paraglide/messages';

	let { card }: { card: ContinueCard } = $props();

	const art = $derived(card.backdrop ?? card.poster);
	const cardAccent = $derived(art?.accent?.startsWith('#') ? `--card-accent:${art.accent}` : '');
</script>

<a
	href="/watch/{card.play.kind}/{card.play.id}"
	data-sveltekit-preload-data="tap"
	class="group w-48 shrink-0 snap-start outline-none sm:w-56"
	style={cardAccent}
>
	<div
		class="relative aspect-video overflow-hidden rounded-xl border border-edge/50 transition-all
			duration-300 group-hover:scale-[1.02] group-hover:shadow-lg group-hover:shadow-black/40
			group-hover:ring-2 group-hover:ring-offset-2 group-focus-visible:scale-[1.02]
			group-focus-visible:shadow-lg group-focus-visible:shadow-black/40 group-focus-visible:ring-3
			group-focus-visible:ring-offset-2"
		style="--tw-ring-color:var(--card-accent,var(--color-accent));--tw-ring-offset-color:var(--color-bg)"
	>
		<Artwork src={art?.url} name={card.name} />
		<div
			class="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0
				transition-opacity group-hover:opacity-100 group-focus-visible:opacity-100"
		>
			<span class="rounded-full bg-accent p-3 text-[var(--color-on-accent)] shadow-lg">
				<Play class="size-5 fill-current" />
			</span>
		</div>
		<div class="absolute inset-x-0 bottom-0 h-1 bg-black/50">
			<div class="h-full bg-accent" style="width: {card.progress * 100}%"></div>
		</div>
	</div>
	<p
		class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent group-focus-visible:text-accent"
	>
		{card.name}
	</p>
	<p class="truncate text-xs text-faint">{card.episodeLabel || m.catalog_continue_watching()}</p>
</a>
