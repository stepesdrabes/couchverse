<script lang="ts">
	import type { PageProps } from './$types';
	import TitleDetailPage from '$lib/features/catalog/pages/TitleDetailPage.svelte';

	let { data }: PageProps = $props();
</script>

<TitleDetailPage {data} />
