<script lang="ts">
	import type { BrowseKey } from '$lib/generated/core';
	import BrowseGrid from '$lib/features/catalog/components/BrowseGrid.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: { key: BrowseKey } } = $props();
</script>

<svelte:head>
	<title>{m.catalog_series_title()}</title>
</svelte:head>

<BrowseGrid heading={m.nav_series()} listing={data.key} />
