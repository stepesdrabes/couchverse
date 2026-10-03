<script lang="ts">
	import type { BrowseKey } from '$lib/generated/core';
	import BrowseGrid from '$lib/features/catalog/components/BrowseGrid.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: { key: BrowseKey } } = $props();

	// the URL carries the genre's English name, as the grid's heading always has
	const genre = $derived(data.key.genre ?? '');
</script>

<svelte:head>
	<title>{m.catalog_genre_title({ genre })}</title>
</svelte:head>

{#key genre}
	<BrowseGrid heading={genre} listing={data.key} />
{/key}
