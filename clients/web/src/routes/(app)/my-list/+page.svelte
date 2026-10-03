<script lang="ts">
	import type { PageProps } from './$types';
	import MyListPage from '$lib/features/catalog/pages/MyListPage.svelte';

	let { data }: PageProps = $props();
</script>

<MyListPage {data} />
