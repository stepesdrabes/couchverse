<script lang="ts">
	import SegmentBar, { type Segment } from '$lib/components/ui/SegmentBar.svelte';
	import { xpSourceLabel } from '../labels';
	import { xpSourceColor } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { XpResult } from '../types';

	let { xp }: { xp: XpResult } = $props();

	const segments = $derived<Segment[]>(
		xp.sources.map((s) => ({
			key: s.key,
			value: s.xp,
			color: xpSourceColor(s.key),
			label: xpSourceLabel(s.key)
		}))
	);
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<h2 class="mb-3 text-sm font-semibold text-muted">{m.rank_sources_heading()}</h2>

	{#if xp.total === 0}
		<p class="py-6 text-center text-xs text-faint">{m.profiles_no_activity()}</p>
	{:else}
		<SegmentBar
			{segments}
			total={xp.total}
			class="h-2"
			title={(s) => `${s.label}: ${m.rank_xp_value({ xp: s.value })}`}
		/>
		<ul class="mt-3 space-y-1.5 text-xs">
			{#each segments as segment (segment.key)}
				<li class="flex items-center justify-between gap-3">
					<span class="flex min-w-0 items-center gap-2 text-muted">
						<span class="size-2 shrink-0 rounded-full" style="background: {segment.color}"></span>
						<span class="truncate">{segment.label}</span>
					</span>
					<span class="shrink-0 text-faint tnum">{m.rank_xp_value({ xp: segment.value })}</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>
