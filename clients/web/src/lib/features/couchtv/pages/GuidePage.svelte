<script lang="ts">
	import { Radio } from 'lucide-svelte';
	import CachedView from '$lib/components/CachedView.svelte';
	import { useScreen } from '$lib/core/screen.svelte';
	import { shown } from '$lib/features/catalog/api';
	import LoadFailed from '$lib/features/catalog/components/LoadFailed.svelte';
	import type { CouchTvGuideView, Surface } from '$lib/generated/core';
	import * as m from '$lib/paraglide/messages';
	import GuideContent from '../components/GuideContent.svelte';
	import GuideSkeleton from '../components/GuideSkeleton.svelte';
	import Lockup from '../components/Lockup.svelte';

	// Couch TV's programs from today's broadcast day to a week ahead, as the core holds them:
	// every visit asks again (what is on moves on) and the last guide shows meanwhile.
	let { data }: { data: { screen: Surface } } = $props();

	const guide = useScreen<CouchTvGuideView>(() => data.screen);
</script>

<svelte:head>
	<title>{m.couchtv_guide_page_title()}</title>
</svelte:head>

<div class="mx-auto max-w-[1700px] px-6 pt-28 pb-16 lg:px-12">
	<!-- the heading and "Tune in" are the page's chrome, not the guide's, so they never shimmer -->
	<div class="mb-6 flex flex-wrap items-center justify-between gap-4">
		<div class="flex items-center gap-4">
			<Lockup size={26} />
			<h1 class="text-2xl font-bold">{m.couchtv_guide()}</h1>
		</div>
		<a
			href="/tv"
			class="inline-flex h-10 items-center gap-2 rounded-full bg-accent px-5 text-sm font-semibold
				text-[var(--color-on-accent)] transition-colors hover:bg-accent-strong"
		>
			<Radio class="size-4" />
			{m.couchtv_tune_in()}
		</a>
	</div>

	<CachedView value={shown(guide.view)} status={guide.view?.status}>
		{#snippet content(view)}
			<GuideContent {view} />
		{/snippet}
		{#snippet skeleton()}
			<GuideSkeleton />
		{/snippet}
		{#snippet failed()}
			<LoadFailed screen={data.screen} />
		{/snippet}
	</CachedView>
</div>
