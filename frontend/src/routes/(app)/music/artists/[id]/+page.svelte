<script lang="ts">
	import type { PageProps } from './$types';
	import ArtistPage from '$lib/features/music/pages/ArtistPage.svelte';

	let { data }: PageProps = $props();
</script>

<ArtistPage {data} />
