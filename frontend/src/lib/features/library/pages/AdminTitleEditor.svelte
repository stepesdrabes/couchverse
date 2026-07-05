<script lang="ts">
	import type { getTitle } from '$lib/features/library/api';
	import StreamedView from '$lib/components/StreamedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import AdminTitleEditorPage from './AdminTitleEditorPage.svelte';
	import AdminEditorSkeleton from '$lib/features/library/components/AdminEditorSkeleton.svelte';

	let { data }: { data: { id: string; fresh: Promise<Awaited<ReturnType<typeof getTitle>>> } } =
		$props();
</script>

<StreamedView key={data.id} data={data.fresh}>
	{#snippet content(title)}
		<AdminTitleEditorPage data={title} />
	{/snippet}
	{#snippet skeleton()}
		<AdminEditorSkeleton />
	{/snippet}
	{#snippet notFound()}
		<NotFound />
	{/snippet}
</StreamedView>
