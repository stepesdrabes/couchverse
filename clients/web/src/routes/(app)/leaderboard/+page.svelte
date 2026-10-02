<script lang="ts">
	import LeaderboardPage from '$lib/features/ranks/pages/LeaderboardPage.svelte';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
</script>

<LeaderboardPage {data} />
