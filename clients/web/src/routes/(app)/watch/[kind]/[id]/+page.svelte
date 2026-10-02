<script lang="ts">
	import type { PageProps } from './$types';
	import WatchPage from '$lib/features/playback/pages/WatchPage.svelte';

	let { data }: PageProps = $props();
</script>

<WatchPage {data} />
