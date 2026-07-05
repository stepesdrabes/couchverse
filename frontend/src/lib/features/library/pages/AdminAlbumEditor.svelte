<script lang="ts">
	import type { getAdminAlbum } from '$lib/features/library/api';
	import StreamedView from '$lib/components/StreamedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import AdminAlbumPage from './AdminAlbumPage.svelte';
	import AdminEditorSkeleton from '$lib/features/library/components/AdminEditorSkeleton.svelte';

	let {
		data
	}: { data: { id: string; fresh: Promise<Awaited<ReturnType<typeof getAdminAlbum>>> } } = $props();
</script>

<StreamedView key={data.id} data={data.fresh}>
	{#snippet content(album)}
		<AdminAlbumPage data={album} />
	{/snippet}
	{#snippet skeleton()}
		<AdminEditorSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound />
	{/snippet}
</StreamedView>
