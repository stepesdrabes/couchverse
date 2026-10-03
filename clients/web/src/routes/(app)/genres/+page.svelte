<script lang="ts">
	import type { PageProps } from './$types';
	import GenresPage from '$lib/features/catalog/pages/GenresPage.svelte';

	let { data }: PageProps = $props();
</script>

<GenresPage {data} />
