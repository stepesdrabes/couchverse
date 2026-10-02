<script lang="ts">
	import type { PageProps } from './$types';
	import CouchPlayerPage from '$lib/features/couch/pages/CouchPlayerPage.svelte';

	let { data }: PageProps = $props();
</script>

<CouchPlayerPage token={data.token} info={data.info} />
