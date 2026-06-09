<script lang="ts">
	import { page } from '$app/state';
	import BrowseGrid from '$lib/components/media/BrowseGrid.svelte';

	const genre = $derived(decodeURIComponent(page.params.slug ?? ''));
</script>

<svelte:head>
	<title>{genre} — Couchverse</title>
</svelte:head>

{#key genre}
	<BrowseGrid heading={genre} {genre} />
{/key}
