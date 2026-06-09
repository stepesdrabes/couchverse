<script lang="ts">
	import type { ContinueItem } from '$lib/features/catalog/types';
	import ContinueWatchingCard from '$lib/components/media/ContinueWatchingCard.svelte';
	import HeroMarquee from '$lib/components/media/HeroMarquee.svelte';
	import MediaRow from '$lib/components/media/MediaRow.svelte';
	import TitleCard from '$lib/components/media/TitleCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';

	let { data } = $props();

	const visibleRows = $derived(data.rows.filter((r) => r.items.length > 0));
</script>

<svelte:head>
	<title>Home — Couchverse</title>
</svelte:head>

{#if data.featured}
	<HeroMarquee featured={data.featured} />

	<div class="relative z-10 -mt-10 space-y-10 pb-16">
		{#each visibleRows as row (row.label)}
			<MediaRow label={row.label}>
				{#if row.kind === 'continue_watching'}
					{#each row.items as ContinueItem[] as item (item.playbackKind + item.playbackId)}
						<ContinueWatchingCard {item} />
					{/each}
				{:else}
					{#each row.items as item (item.titleId)}
						<TitleCard {item} />
					{/each}
				{/if}
			</MediaRow>
		{/each}
	</div>
{:else}
	<div class="flex min-h-dvh items-center justify-center">
		<EmptyState
			title="The library is empty"
			message="Once the admin publishes movies or series, they show up here."
		/>
	</div>
{/if}
