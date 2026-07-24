<script lang="ts">
	import { Award, Sparkles, Trophy, Users } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import StatTile from '$lib/components/ui/StatTile.svelte';
	import SegmentBar, { type Segment } from '$lib/components/ui/SegmentBar.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import AchievementBadge from '../components/AchievementBadge.svelte';
	import { achievementName, achievementTierName, tierName } from '../labels';
	import { rankColor } from '../tiers';
	import * as adminApi from '../admin-api';
	import type { AdminRanksOverview, RankConfig } from '../admin-api';
	import { formatUptime } from '$lib/utils/format';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';

	type RateKey = keyof adminApi.RankRates;

	let data = $state<AdminRanksOverview | null>(null);
	// number inputs are kept as strings and converted on save, matching the
	// transcode settings form
	let rates = $state<Record<RateKey, string> | null>(null);
	let tiers = $state<string[]>([]);
	let saving = $state(false);
	const form = new FormState(() => ({ rates, tiers }));

	async function load() {
		data = await adminApi.getAdminRanks();
		adopt(data.config);
	}

	function adopt(config: RankConfig) {
		rates = Object.fromEntries(
			Object.entries(config.rates).map(([key, value]) => [key, String(value)])
		) as Record<RateKey, string>;
		tiers = config.tiers.map(String);
		form.reset();
	}

	load();

	// The XP a member has is always derived, so a saved change re-levels everyone
	// on their next read; there is nothing to recompute.
	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (!rates) return;
		saving = true;
		try {
			const numeric = Object.fromEntries(
				Object.entries(rates).map(([key, value]) => [key, Number(value) || 0])
			);
			const payload: RankConfig = {
				rates: numeric as unknown as adminApi.RankRates,
				tiers: tiers.map((t) => Number(t) || 0)
			};
			adopt(await adminApi.putRankConfig(payload));
			toast.success(m.admin_ranks_saved());
			data = await adminApi.getAdminRanks();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.common_save_failed());
		} finally {
			saving = false;
		}
	}

	const rateFields = $derived(
		rates
			? ([
					{ key: 'videoMinute', label: m.admin_ranks_rate_video() },
					{ key: 'musicMinute', label: m.admin_ranks_rate_music() },
					{ key: 'movie', label: m.admin_ranks_rate_movie() },
					{ key: 'episode', label: m.admin_ranks_rate_episode() },
					{ key: 'couchHost', label: m.admin_ranks_rate_couch_host() },
					{ key: 'couchJoin', label: m.admin_ranks_rate_couch_join() },
					{ key: 'bronze', label: achievementTierName('bronze') },
					{ key: 'silver', label: achievementTierName('silver') },
					{ key: 'gold', label: achievementTierName('gold') },
					{ key: 'platinum', label: achievementTierName('platinum') }
				] as const)
			: []
	);

	const levelSegments = $derived<Segment[]>(
		(data?.tiers ?? [])
			.filter((t) => t.members > 0)
			.map((t) => ({
				key: t.code,
				value: t.members,
				color: rankColor(t.code),
				label: tierName(t.code)
			}))
	);

	// a locked badge with a real progress value reads better than a bare zero
	const asAchievement = (stat: adminApi.AchievementStat) => ({
		code: stat.code,
		category: stat.category,
		tier: stat.tier,
		xp: 0,
		target: 1,
		value: stat.unlocked > 0 ? 1 : 0,
		percent: stat.unlocked > 0 ? 100 : 0,
		unlocked: stat.unlocked > 0,
		unlockedAt: null
	});
</script>

<svelte:head>
	<title>{m.admin_ranks_page_title()}</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">{m.admin_nav_ranks()}</h1>

{#if data}
	<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
		{#each [{ key: 'members', icon: Users, value: data.members, label: m.admin_ranks_members() }, { key: 'xp', icon: Sparkles, value: data.totalXp.toLocaleString(), label: m.admin_ranks_total_xp() }, { key: 'unlocks', icon: Award, value: data.totalUnlocks, label: m.admin_ranks_total_unlocks() }, { key: 'level', icon: Trophy, value: data.averageLevel.toFixed(1), label: m.admin_ranks_avg_level() }] as card, i (card.key)}
			<div in:fly|global={{ y: 16, duration: 350, delay: Math.min(i * 55, 300) }}>
				<StatTile icon={card.icon} value={card.value} label={card.label} size="sm" />
			</div>
		{/each}
	</div>

	<div class="mt-6" in:fly|global={{ y: 20, duration: 400 }}>
		<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_ranks_distribution()}</h2>
		<div class="rounded-card border border-edge bg-surface/40 p-6">
			{#if levelSegments.length === 0}
				<p class="text-xs text-faint">{m.admin_ranks_no_members()}</p>
			{:else}
				<SegmentBar
					segments={levelSegments}
					total={data.members}
					class="h-2"
					title={(s) => `${s.label}: ${s.value}`}
				/>
				<ul class="mt-3 grid gap-1.5 text-xs sm:grid-cols-2 lg:grid-cols-3">
					{#each data.tiers as tier (tier.code)}
						<li class="flex items-center justify-between gap-3">
							<span class="flex min-w-0 items-center gap-2 text-muted">
								<span class="size-2 shrink-0 rounded-full" style="background: {tier.colour}"></span>
								<span class="truncate">
									{m.rank_level({ level: tier.level })} · {tierName(tier.code)}
								</span>
							</span>
							<span class="shrink-0 text-faint tnum">{tier.members}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	</div>

	<div class="mt-6" in:fly|global={{ y: 20, duration: 400, delay: 80 }}>
		<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_ranks_rarity()}</h2>
		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<div class="grid gap-x-4 gap-y-5 sm:grid-cols-4 lg:grid-cols-6">
				{#each data.achievements as stat (stat.code)}
					<div class="flex flex-col items-center text-center" class:opacity-40={stat.hidden}>
						<AchievementBadge achievement={asAchievement(stat)} size="sm" />
						<p class="mt-1.5 line-clamp-2 text-[11px] font-medium">
							{achievementName(stat.code)}
						</p>
						<p class="text-[11px] text-faint tnum">
							{m.admin_ranks_holders({ count: stat.unlocked })}
						</p>
					</div>
				{/each}
			</div>
		</div>
	</div>

	<div class="mt-6" in:fly|global={{ y: 20, duration: 400, delay: 120 }}>
		<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_ranks_members()}</h2>
		<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b border-edge text-[11px] tracking-wider text-faint uppercase">
						<th scope="col" class="px-4 py-2.5">{m.leaderboard_col_member()}</th>
						<th scope="col" class="px-4 py-2.5">{m.leaderboard_col_level()}</th>
						<th scope="col" class="px-4 py-2.5 text-right">{m.rank_xp()}</th>
						<th scope="col" class="px-4 py-2.5 text-right">{m.achievement_heading()}</th>
						<th scope="col" class="hidden px-4 py-2.5 text-right lg:table-cell">
							{m.profiles_stat_watch_time()}
						</th>
					</tr>
				</thead>
				<tbody>
					{#each data.rows as row (row.userId)}
						<tr
							class="border-b border-edge/50 transition-colors last:border-0 hover:bg-surface-2/40"
						>
							<td class="px-4 py-3">
								<div class="flex min-w-0 items-center gap-3">
									<UserAvatar
										name={row.displayName}
										avatarId={row.avatarId}
										seed={row.username}
										class="size-8 shrink-0 rounded-lg text-[10px]"
									/>
									<div class="min-w-0">
										<a
											href="/u/{row.username}"
											class="block truncate font-medium transition-colors hover:text-accent"
										>
											{row.displayName}
										</a>
										<span class="block truncate text-[11px] text-faint">
											@{row.username}
											{#if !row.public}
												· {m.admin_ranks_hidden()}
											{/if}
										</span>
									</div>
								</div>
							</td>
							<td class="px-4 py-3">
								<span
									class="rounded px-1.5 py-0.5 text-[10px] font-bold whitespace-nowrap"
									style="background: color-mix(in srgb, {rankColor(row.tierCode)} 18%, transparent);
										color: {rankColor(row.tierCode)}"
								>
									{m.rank_level({ level: row.level })} · {tierName(row.tierCode)}
								</span>
							</td>
							<td class="px-4 py-3 text-right font-semibold tnum">{row.xp.toLocaleString()}</td>
							<td class="px-4 py-3 text-right text-faint tnum">{row.achievements}</td>
							<td class="hidden px-4 py-3 text-right text-faint tnum lg:table-cell">
								{formatUptime(row.watchSeconds)}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	</div>

	{#if rates}
		<form onsubmit={save} class="mt-6" in:fly|global={{ y: 20, duration: 400, delay: 160 }}>
			<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_ranks_config()}</h2>
			<div class="space-y-6 rounded-card border border-edge bg-surface/40 p-6">
				<div>
					<p class="mb-3 text-[11px] text-faint">{m.admin_ranks_rates_hint()}</p>
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
						{#each rateFields as field (field.key)}
							<Input label={field.label} type="number" min="0" bind:value={rates[field.key]} />
						{/each}
					</div>
				</div>

				<div>
					<p class="mb-3 text-[11px] text-faint">{m.admin_ranks_tiers_hint()}</p>
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
						{#each data.tiers as tier, i (tier.code)}
							<Input
								label="{m.rank_level({ level: tier.level })} · {tierName(tier.code)}"
								type="number"
								min="0"
								disabled={i === 0}
								bind:value={tiers[i]}
							/>
						{/each}
					</div>
				</div>

				<div class="flex justify-end">
					<Button type="submit" loading={saving} disabled={!form.dirty}>{m.common_save()}</Button>
				</div>
			</div>
		</form>
	{/if}
{/if}
