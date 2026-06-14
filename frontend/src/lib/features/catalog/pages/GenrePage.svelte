<script lang="ts">
	import { page } from '$app/state';
	import BrowseGrid from '$lib/features/catalog/components/BrowseGrid.svelte';
	import * as m from '$lib/paraglide/messages';

	const genre = $derived(decodeURIComponent(page.params.slug ?? ''));
</script>

<svelte:head>
	<title>{m.catalog_genre_title({ genre })}</title>
</svelte:head>

{#key genre}
	<BrowseGrid heading={genre} {genre} />
{/key}
