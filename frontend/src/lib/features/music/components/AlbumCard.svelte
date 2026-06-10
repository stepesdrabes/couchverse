<script lang="ts">
	import { Music } from 'lucide-svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import type { AlbumCard } from '$lib/features/music/api';

	let { album }: { album: AlbumCard } = $props();
</script>

<a href="/music/albums/{album.id}" class="group w-40 shrink-0 snap-start sm:w-44">
	<div
		class="aspect-square overflow-hidden rounded-xl border border-edge/50 transition-all
			duration-300 group-hover:scale-[1.04] group-hover:border-accent/60 group-hover:shadow-lg
			group-hover:shadow-black/40"
	>
		{#if album.coverId}
			<img
				src={artworkUrl(album.coverId)}
				alt={album.name}
				loading="lazy"
				class="size-full object-cover"
			/>
		{:else}
			<div
				class="flex size-full items-center justify-center bg-gradient-to-br from-accent-soft to-surface-2"
			>
				<Music class="size-7 text-accent/60" />
			</div>
		{/if}
	</div>
	<p class="mt-2 truncate text-sm font-semibold transition-colors group-hover:text-accent">
		{album.name}
	</p>
	<p class="truncate text-xs text-faint">{album.artistName}</p>
</a>
