<script lang="ts">
	import type { PageProps } from './$types';
	import PlaylistPage from '$lib/features/music/pages/PlaylistPage.svelte';

	let { data }: PageProps = $props();
</script>

<PlaylistPage {data} />
