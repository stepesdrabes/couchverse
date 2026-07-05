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
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: Awaited<ReturnType<typeof home>> } = $props();

	const visibleRows = $derived(
		data.rows.filter(
			(r) => r.items.length > 0 && (r.kind !== 'recently_played_music' || features.musicEnabled)
		)
	);

	// built-in home rows ship English default labels in the DB; translate those by
	// kind, but respect a label the admin customized in the home-row editor.
	const ROW_DEFAULTS: Record<string, string> = {
		continue_watching: 'Continue Watching',
		recently_added: 'Up on the Marquee',
		recently_played_music: 'Recently Played'
	};
	function rowLabel(row: { kind: string; label: string }): string {
		if (row.label !== ROW_DEFAULTS[row.kind]) return row.label;
		switch (row.kind) {
			case 'continue_watching':
				return m.home_row_continue_watching();
			case 'recently_added':
				return m.home_row_recently_added();
			case 'recently_played_music':
				return m.home_row_recently_played();
			default:
				return row.label;
		}
	}
</script>

<svelte:head>
	<title>{m.catalog_home_title()}</title>
</svelte:head>

{#if data.featured.length > 0}
	<HeroMarquee items={data.featured} />

	<div class="relative z-10 -mt-10 space-y-10 pb-16">
		{#each visibleRows as row (row.label)}
			<MediaRow label={rowLabel(row)}>
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
			title={m.catalog_library_empty_title()}
			message={m.catalog_library_empty_message()}
		/>
	</div>
{/if}
