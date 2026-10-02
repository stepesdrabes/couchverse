<script lang="ts">
	import type { PageProps } from './$types';
	import HomePage from '$lib/features/catalog/pages/HomePage.svelte';

	let { data }: PageProps = $props();
</script>

<HomePage {data} />
