<script lang="ts">
	import type { PageProps } from './$types';
	import GuidePage from '$lib/features/couchtv/pages/GuidePage.svelte';

	let { data }: PageProps = $props();
</script>

<GuidePage {data} />
