<script lang="ts">
	import { fly } from 'svelte/transition';
	import { EyeOff } from 'lucide-svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { formatUptime } from '$lib/utils/format';
	import LeaderboardPodium from './LeaderboardPodium.svelte';
	import LeaderboardTable from './LeaderboardTable.svelte';
	import CountUp from './CountUp.svelte';
	import { metricLabel } from '../labels';
	import { musicPlayer } from '$lib/features/music/player.svelte';
	import * as m from '$lib/paraglide/messages';
	import type { Leaderboard, LeaderRow, Metric } from '../types';

	let { board, metric }: { board: Leaderboard; metric: Metric } = $props();

	const SCORE: Record<Metric, (row: LeaderRow) => number> = {
		xp: (r) => r.xp,
		watch: (r) => r.watchSeconds,
		music: (r) => r.musicSeconds,
		achievements: (r) => r.achievements
	};

	const value = $derived(SCORE[metric]);
	const display = $derived(
		metric === 'watch' || metric === 'music'
			? (n: number) => formatUptime(n)
			: metric === 'xp'
				? (n: number) => m.rank_xp_value({ xp: Math.round(n) })
				: (n: number) => String(Math.round(n))
	);

	// Sorting is client-side because one payload carries every metric, so
	// switching board costs no request and no skeleton flash.
	const ranked = $derived([...board.rows].sort((a, b) => value(b) - value(a)));
	const myPosition = $derived(ranked.findIndex((r) => r.isSelf) + 1);
	const showPodium = $derived(ranked.length >= 3 && value(ranked[0]) > 0);
	const empty = $derived(ranked.length === 0);
	const allZero = $derived(!empty && value(ranked[0]) === 0);

	// The fixed music player bar would otherwise sit on top of the sticky row.
	const stickyBottom = $derived(musicPlayer.current ? '6.5rem' : '1rem');
</script>

{#if board.hidden}
	<div
		class="mb-6 flex flex-wrap items-center gap-3 rounded-card border border-edge bg-surface-2/60
			px-4 py-3 text-xs text-muted"
	>
		<EyeOff class="size-4 shrink-0 text-faint" />
		<span>{m.leaderboard_hidden_notice()}</span>
		<a
			href="/profile?tab=privacy"
			class="ml-auto shrink-0 font-semibold text-accent hover:underline"
		>
			{m.leaderboard_hidden_action()}
		</a>
	</div>
{/if}

{#if empty}
	<EmptyState title={m.leaderboard_empty_title()} message={m.leaderboard_empty_message()} />
{:else if allZero}
	<EmptyState title={m.leaderboard_metric_empty()} message={metricLabel(metric)} />
{:else}
	{#key metric}
		<div in:fly|global={{ y: 10, duration: 250 }}>
			{#if showPodium}
				<LeaderboardPodium rows={ranked.slice(0, 3)} {value} {display} />
			{/if}
			<LeaderboardTable rows={ranked} {metric} {value} {display} />
		</div>
	{/key}
{/if}

<!-- Always on rather than observer-driven: it is a persistent "where you stand"
     anchor, and an IntersectionObserver would be pure cost on a Pi. -->
{#if board.me && (board.hidden || myPosition > 3)}
	<div
		class="sticky z-10 mt-4 flex items-center gap-3 rounded-card border border-accent/50
			bg-surface-2/90 px-4 py-2.5 shadow-xl shadow-black/40 backdrop-blur"
		style="bottom: {stickyBottom}"
	>
		<span class="w-6 shrink-0 text-center text-sm font-bold text-accent tnum">
			{board.hidden ? '-' : `#${myPosition}`}
		</span>
		<UserAvatar
			name={board.me.displayName}
			avatarId={board.me.avatarId}
			seed={board.me.username}
			class="size-8 shrink-0 rounded-lg text-[10px]"
		/>
		<span class="min-w-0 flex-1 truncate text-sm font-medium">{board.me.displayName}</span>
		<span class="shrink-0 text-xs text-faint">
			{#if board.hidden}
				{m.leaderboard_unranked()}
			{:else}
				{m.leaderboard_your_position({ rank: myPosition, total: board.total })}
			{/if}
		</span>
		<span class="shrink-0 text-sm font-semibold tnum">
			<CountUp value={value(board.me)} format={display} />
		</span>
	</div>
{/if}
