<script lang="ts">
	import BarChart, { type Bar } from '$lib/features/admin/components/BarChart.svelte';
	import { categoryStyle } from '$lib/features/admin/components/storageColors';
	import * as libraryApi from '$lib/features/library/api';
	import type { TitleStorage } from '$lib/features/library/api';
	import { formatBytes } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	let { titleId, kind }: { titleId: string; kind: 'movie' | 'series' } = $props();

	let data = $state<TitleStorage | null>(null);

	$effect(() => {
		libraryApi.getTitleStorage(titleId).then((s) => (data = s));
	});

	// source bars take the title's catalog colour, transcodes the storage emerald
	const sourceColor = $derived(
		kind === 'series' ? categoryStyle.series.color : categoryStyle.movies.color
	);
	const transcodedColor = categoryStyle.transcodes.color;

	const bars = $derived<Bar[]>(
		(data?.items ?? []).map((it) => ({
			label: it.label,
			segments: [
				{ name: m.library_storage_source(), value: it.sourceBytes, color: sourceColor },
				{ name: m.library_storage_transcoded(), value: it.transcodedBytes, color: transcodedColor }
			]
		}))
	);

	const total = $derived((data?.sourceBytes ?? 0) + (data?.transcodedBytes ?? 0));
</script>

{#if data}
	<section class="rounded-card border border-edge bg-surface/40 p-6">
		<div class="mb-4 flex items-center justify-between">
			<h2 class="text-sm font-semibold text-muted">{m.library_storage()}</h2>
			{#if total > 0}<span class="text-xs text-faint tnum">{formatBytes(total)}</span>{/if}
		</div>
		{#if data.items.length > 0}
			<BarChart {bars} format={formatBytes} class="h-32" />
			<div class="mt-3 flex flex-wrap gap-4 text-xs">
				<span class="flex items-center gap-1.5 text-muted">
					<span class="size-2 rounded-full" style="background: {sourceColor}"></span>
					{m.library_storage_source()}
					<span class="text-faint tnum">{formatBytes(data.sourceBytes)}</span>
				</span>
				<span class="flex items-center gap-1.5 text-muted">
					<span class="size-2 rounded-full" style="background: {transcodedColor}"></span>
					{m.library_storage_transcoded()}
					<span class="text-faint tnum">{formatBytes(data.transcodedBytes)}</span>
				</span>
			</div>
		{:else}
			<p class="text-xs text-faint">{m.library_storage_empty()}</p>
		{/if}
	</section>
{/if}
