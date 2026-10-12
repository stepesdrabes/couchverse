<script lang="ts">
	import StreamedView from '$lib/components/StreamedView.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import * as m from '$lib/paraglide/messages';
	import type { CouchTvOverview } from '../api';
	import GuestLinkPanel from '../components/GuestLinkPanel.svelte';
	import TemplateEditor from '../components/TemplateEditor.svelte';

	let { data }: { data: { fresh: Promise<CouchTvOverview> } } = $props();
</script>

<svelte:head>
	<title>{m.admin_couchtv_template_page_title()}</title>
</svelte:head>

<StreamedView key="couchtv-template" data={data.fresh}>
	{#snippet content(overview)}
		<div class="space-y-6">
			<GuestLinkPanel link={overview.config.guestLink} />
			<TemplateEditor {overview} />
		</div>
	{/snippet}
	{#snippet skeleton()}
		<div class="space-y-4">
			<Skeleton class="h-32 w-full rounded-card" />
			<Skeleton class="h-28 w-full rounded-card" />
			<Skeleton class="h-40 w-full rounded-card" />
			<Skeleton class="h-72 w-full rounded-card" />
		</div>
	{/snippet}
</StreamedView>
