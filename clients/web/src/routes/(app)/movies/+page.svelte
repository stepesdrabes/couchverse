<script lang="ts">
	import type { PageProps } from './$types';
	import MoviesPage from '$lib/features/catalog/pages/MoviesPage.svelte';

	let { data }: PageProps = $props();
</script>

<MoviesPage {data} />
