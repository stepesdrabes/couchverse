<script lang="ts">
	import { onMount } from 'svelte';
	import * as m from '$lib/paraglide/messages';
	import type { RankProgress } from '../types';

	let { rank, class: cls = '' }: { rank: RankProgress; class?: string } = $props();

	// scaleX on a full-width track is compositor-only; animating width would
	// force layout on every frame, which matters with a hundred of these on the
	// leaderboard.
	let ready = $state(false);
	onMount(() => {
		ready = true;
	});

	const fill = $derived(ready ? Math.min(100, Math.max(0, rank.percent)) / 100 : 0);
</script>

<div class={cls}>
	<div class="h-2 w-full overflow-hidden rounded-full bg-surface-2">
		<div
			class="h-full origin-left rounded-full bg-accent transition-transform duration-[900ms] ease-out"
			style="transform: scaleX({fill})"
		></div>
	</div>
	<div class="mt-1.5 flex justify-between text-[11px] text-faint tnum">
		{#if rank.next}
			<span>{m.rank_xp_progress({ into: rank.intoTier, need: rank.tierSpan })}</span>
			<span>{m.rank_to_next_level({ level: rank.next.level })}</span>
		{:else}
			<span>{m.rank_xp_value({ xp: rank.xp })}</span>
			<span>{m.rank_max_level()}</span>
		{/if}
	</div>
</div>
