<script lang="ts">
	import type { PageProps } from './$types';
	import AdminAlbumPage from '$lib/features/library/pages/AdminAlbumPage.svelte';

	let { data }: PageProps = $props();
</script>

<AdminAlbumPage {data} />
