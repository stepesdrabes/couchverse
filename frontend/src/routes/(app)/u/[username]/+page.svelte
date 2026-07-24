<script lang="ts">
	import PublicProfilePage from '$lib/features/ranks/pages/PublicProfilePage.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<PublicProfilePage {data} />
