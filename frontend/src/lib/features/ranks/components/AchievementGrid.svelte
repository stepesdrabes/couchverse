<script lang="ts">
	import { fly } from 'svelte/transition';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import { formatYearDate } from '$lib/utils/format';
	import AchievementBadge from './AchievementBadge.svelte';
	import { achievementDesc, achievementName, achievementTierName } from '../labels';
	import { ACHIEVEMENT_TIERS } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { Achievement } from '../types';

	let { achievements, won }: { achievements: Achievement[]; won: number } = $props();

	let filter = $state('all');
	// One detail surface for both pointers: hover tooltips do not exist on touch,
	// so the badge is a button and the modal is the description everywhere.
	let selected = $state<Achievement | null>(null);
	let open = $state(false);

	const filters = $derived([
		{ value: 'all', label: m.achievement_filter_all() },
		{ value: 'unlocked', label: m.achievement_filter_unlocked() },
		{ value: 'locked', label: m.achievement_filter_locked() }
	]);

	const shown = $derived(
		achievements.filter((a) =>
			filter === 'unlocked' ? a.unlocked : filter === 'locked' ? !a.unlocked : true
		)
	);

	function select(achievement: Achievement) {
		selected = achievement;
		open = true;
	}
</script>

<section class="mt-10">
	<div class="mb-4 flex flex-wrap items-center gap-3">
		<h2 class="text-sm font-semibold text-muted">{m.achievement_heading()}</h2>
		<span
			class="rounded-full border border-edge bg-surface px-2.5 py-0.5 text-xs font-semibold
				text-muted tnum"
		>
			{m.achievement_count({ unlocked: won, total: achievements.length })}
		</span>
		<div class="ml-auto">
			<Tabs bind:value={filter} items={filters} />
		</div>
	</div>

	{#if achievements.length === 0}
		<EmptyState title={m.achievement_empty_title()} message={m.achievement_empty_message()} />
	{:else}
		{#key filter}
			<div
				in:fly|global={{ y: 10, duration: 250 }}
				class="grid grid-cols-3 gap-x-4 gap-y-5 sm:grid-cols-5 lg:grid-cols-8"
			>
				<!-- 28ms rather than the usual 55ms stagger: across 30+ badges the
				     standard cadence would take well over a second to settle -->
				{#each shown as achievement, i (achievement.code)}
					<div
						in:fly|global={{ y: 14, duration: 320, delay: Math.min(i * 28, 420) }}
						class="flex flex-col items-center"
					>
						<AchievementBadge {achievement} size="md" onselect={select} />
						<p
							class="mt-2 line-clamp-2 text-center text-[11px] font-medium
								{achievement.unlocked ? 'text-text' : 'text-faint'}"
						>
							{achievementName(achievement.code)}
						</p>
					</div>
				{/each}
			</div>
		{/key}
	{/if}
</section>

<Modal bind:open title={selected ? achievementName(selected.code) : ''}>
	{#if selected}
		{@const metal = ACHIEVEMENT_TIERS[selected.tier]}
		<div class="flex flex-col items-center gap-4 text-center">
			<AchievementBadge achievement={selected} size="lg" />
			<p class="text-sm text-muted">{achievementDesc(selected.code)}</p>
			<div class="flex items-center gap-2 text-xs">
				<span class="font-semibold" style="color: {metal.ring}">
					{achievementTierName(selected.tier)}
				</span>
				<span class="text-faint">·</span>
				<span class="font-semibold text-faint tnum">
					{m.achievement_reward({ xp: selected.xp })}
				</span>
			</div>

			{#if selected.unlocked}
				<p class="text-xs text-faint">
					{selected.unlockedAt
						? m.achievement_unlocked_on({ date: formatYearDate(selected.unlockedAt) })
						: m.achievement_unlocked()}
				</p>
			{:else}
				<div class="w-full max-w-xs">
					<div class="h-1.5 w-full overflow-hidden rounded-full bg-surface-2">
						<div
							class="h-full rounded-full bg-accent transition-all duration-700"
							style="width: {selected.percent}%"
						></div>
					</div>
					<p class="mt-1.5 text-[11px] text-faint tnum">
						{m.achievement_progress({ current: selected.value, target: selected.target })}
					</p>
				</div>
			{/if}
		</div>
	{/if}
</Modal>
