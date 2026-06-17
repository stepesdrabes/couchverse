<script lang="ts">
	import type { PageProps } from './$types';
	import CouchPlayerPage from '$lib/features/couch/pages/CouchPlayerPage.svelte';

	let { data }: PageProps = $props();
</script>

<CouchPlayerPage snapshot={data.snapshot} player={data.player} jitSessionId={data.jitSessionId} />
