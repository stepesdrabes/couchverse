<script lang="ts">
	import type { BrowseKey } from '$lib/generated/core';
	import BrowseGrid from '$lib/features/catalog/components/BrowseGrid.svelte';

	let { data }: { data: { key: BrowseKey } } = $props();

	// the URL carries the genre's English name; the grid shows its label once the core has it
	const genre = $derived(data.key.genre ?? '');
</script>

{#key genre}
	<BrowseGrid heading={genre} listing={data.key} />
{/key}
