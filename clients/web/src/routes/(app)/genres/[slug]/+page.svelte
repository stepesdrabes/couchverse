<script lang="ts">
	import type { PageProps } from './$types';
	import GenrePage from '$lib/features/catalog/pages/GenrePage.svelte';

	let { data }: PageProps = $props();
</script>

<GenrePage {data} />
