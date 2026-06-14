<script lang="ts">
	import { fly } from 'svelte/transition';
	import * as catalog from '$lib/features/catalog/api';
	import type { Genre } from '$lib/features/catalog/types';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import * as m from '$lib/paraglide/messages';

	let genres = $state<Genre[]>([]);
	let loaded = $state(false);

	$effect(() => {
		catalog
			.listGenres()
			.then((g) => {
				genres = g;
				loaded = true;
			})
			.catch(() => (loaded = true));
	});

	// deterministic accent hue per genre tile
	const hue = (name: string) => [...name].reduce((acc, ch) => acc + ch.charCodeAt(0), 0) % 360;
</script>

<svelte:head>
	<title>{m.catalog_genres_title()}</title>
</svelte:head>

<div class="mx-auto max-w-[1700px] px-6 pt-24 pb-16 lg:px-12">
	<h1 class="mb-8 text-2xl font-bold">{m.nav_genres()}</h1>

	{#if loaded && genres.length === 0}
		<EmptyState title={m.catalog_genres_empty_title()} message={m.catalog_genres_empty_message()} />
	{:else}
		<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
			{#each genres as genre, i (genre.id)}
				<a
					in:fly|global={{ y: 14, duration: 300, delay: Math.min(i * 35, 350) }}
					href="/genres/{encodeURIComponent(genre.name)}"
					class="group relative flex h-28 items-end overflow-hidden rounded-card border
						border-edge/50 p-4 transition-all duration-300 hover:scale-[1.02] hover:border-accent/60"
					style="background: linear-gradient(135deg, hsl({hue(
						genre.name
					)} 40% 16%), var(--color-surface))"
				>
					<span class="text-lg font-bold transition-colors group-hover:text-accent">
						{genre.label}
					</span>
				</a>
			{/each}
		</div>
	{/if}
</div>
