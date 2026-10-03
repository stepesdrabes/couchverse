<script lang="ts">
	import type { PageProps } from './$types';
	import SeriesPage from '$lib/features/catalog/pages/SeriesPage.svelte';

	let { data }: PageProps = $props();
</script>

<SeriesPage {data} />
