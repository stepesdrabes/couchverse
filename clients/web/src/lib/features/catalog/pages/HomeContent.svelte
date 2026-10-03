<script lang="ts">
	import type { HomeView } from '$lib/generated/core';
	import ContinueWatchingCard from '$lib/features/catalog/components/ContinueWatchingCard.svelte';
	import HeroMarquee from '$lib/features/catalog/components/HeroMarquee.svelte';
	import MediaRow from '$lib/features/catalog/components/MediaRow.svelte';
	import TitleCard from '$lib/features/catalog/components/TitleCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import { rowLabel } from '../labels';
	import * as m from '$lib/paraglide/messages';

	let { view }: { view: HomeView } = $props();
</script>

<svelte:head>
	<title>{m.catalog_home_title()}</title>
</svelte:head>

{#if view.featured.length > 0}
	<HeroMarquee items={view.featured} />

	<div class="relative z-10 -mt-10 space-y-10 pb-16">
		{#each view.rows as row (row.id)}
			<MediaRow label={rowLabel(row)}>
				{#each row.continueWatching as card (card.play.kind + card.play.id)}
					<ContinueWatchingCard {card} />
				{/each}
				{#each row.cards as card (card.titleId)}
					<TitleCard {card} />
				{/each}
			</MediaRow>
		{/each}
	</div>
{:else}
	<div class="flex min-h-dvh items-center justify-center">
		<EmptyState
			title={m.catalog_library_empty_title()}
			message={m.catalog_library_empty_message()}
		/>
	</div>
{/if}
