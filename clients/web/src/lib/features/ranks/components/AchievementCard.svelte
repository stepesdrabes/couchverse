<script lang="ts">
	import AchievementBadge from './AchievementBadge.svelte';
	import { achievementDesc, achievementName } from '../labels';
	import { ACHIEVEMENT_TIERS } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { Achievement } from '../types';

	// One visual with two mounts: the sonner toast off-player and the in-player
	// overlay, which has to live inside the fullscreen subtree.
	let {
		achievement,
		live = false
	}: {
		achievement: Achievement;
		// announce it ourselves; the toast path lets sonner's live region do it
		live?: boolean;
	} = $props();

	const metal = $derived(ACHIEVEMENT_TIERS[achievement.tier] ?? ACHIEVEMENT_TIERS.bronze);
</script>

<div
	class="achievement-celebrate flex w-[22rem] max-w-[calc(100vw-2rem)] items-center gap-4
		rounded-card border p-4 shadow-2xl shadow-black/50 backdrop-blur"
	style="border-color: color-mix(in srgb, {metal.ring} 45%, var(--color-edge));
		background: linear-gradient(120deg,
			color-mix(in srgb, {metal.from} 12%, var(--color-surface-2)),
			var(--color-surface-2) 60%)"
	role="status"
	aria-live={live ? 'polite' : 'off'}
>
	<AchievementBadge {achievement} size="lg" celebrate />
	<div class="min-w-0">
		<p class="eyebrow" style="color: {metal.ring}">{m.achievement_unlocked()}</p>
		<p class="mt-0.5 truncate text-sm font-bold">{achievementName(achievement.code)}</p>
		<p class="mt-0.5 line-clamp-2 text-xs text-muted">{achievementDesc(achievement.code)}</p>
		<p class="mt-1 text-[11px] font-semibold tnum" style="color: {metal.ring}">
			{m.achievement_reward({ xp: achievement.xp })}
		</p>
	</div>
</div>
