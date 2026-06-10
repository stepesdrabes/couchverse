<script lang="ts">
	import type { PageProps } from './$types';
	import MusicHomePage from '$lib/features/music/pages/MusicHomePage.svelte';

	let { data }: PageProps = $props();
</script>

<MusicHomePage {data} />
