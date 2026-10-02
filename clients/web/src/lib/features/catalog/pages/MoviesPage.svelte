<script lang="ts">
	import BrowseGrid from '$lib/features/catalog/components/BrowseGrid.svelte';
	import * as m from '$lib/paraglide/messages';
</script>

<svelte:head>
	<title>{m.catalog_movies_title()}</title>
</svelte:head>

<BrowseGrid heading={m.nav_movies()} kind="movie" />
