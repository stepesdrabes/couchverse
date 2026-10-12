<script lang="ts">
	import type { PageProps } from './$types';
	import TvJoinPage from '$lib/features/couchtv/pages/TvJoinPage.svelte';

	let { data }: PageProps = $props();
</script>

<TvJoinPage code={data.code} />
