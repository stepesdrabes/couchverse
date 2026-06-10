<script lang="ts">
	import type { home } from '$lib/features/catalog/api';
	import type { ContinueItem } from '$lib/features/catalog/types';
	import type { AlbumCard as AlbumCardType } from '$lib/features/music/api';
	import ContinueWatchingCard from '$lib/features/catalog/components/ContinueWatchingCard.svelte';
	import HeroMarquee from '$lib/features/catalog/components/HeroMarquee.svelte';
	import MediaRow from '$lib/features/catalog/components/MediaRow.svelte';
	import TitleCard from '$lib/features/catalog/components/TitleCard.svelte';
	import AlbumCard from '$lib/features/music/components/AlbumCard.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import { features } from '$lib/features/settings/features.svelte';

	let { data }: { data: Awaited<ReturnType<typeof home>> } = $props();

	const visibleRows = $derived(
		data.rows.filter(
			(r) => r.items.length > 0 && (r.kind !== 'recently_played_music' || features.musicEnabled)
		)
	);
</script>

<svelte:head>
	<title>Home - Couchverse</title>
</svelte:head>

{#if data.featured}
	<HeroMarquee
		featured={data.featured}
		backdropId={data.featuredBackdropId}
		inList={data.featuredInList}
	/>

	<div class="relative z-10 -mt-10 space-y-10 pb-16">
		{#each visibleRows as row (row.label)}
			<MediaRow label={row.label}>
				{#if row.kind === 'continue_watching'}
					{#each row.items as ContinueItem[] as item (item.playbackKind + item.playbackId)}
						<ContinueWatchingCard {item} />
					{/each}
				{:else if row.kind === 'recently_played_music'}
					{#each row.items as AlbumCardType[] as album (album.id)}
						<AlbumCard {album} />
					{/each}
				{:else}
					{#each row.items as ContinueItem[] as item (item.titleId)}
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
