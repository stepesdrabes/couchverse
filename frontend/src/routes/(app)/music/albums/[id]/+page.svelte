<script lang="ts">
	import type { PageProps } from './$types';
	import AlbumPage from '$lib/features/music/pages/AlbumPage.svelte';

	let { data }: PageProps = $props();
</script>

<AlbumPage {data} />
