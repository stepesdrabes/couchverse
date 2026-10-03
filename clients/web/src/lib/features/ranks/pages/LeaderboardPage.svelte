<script lang="ts">
	import { goto } from '$app/navigation';
	import CachedView from '$lib/components/CachedView.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Tooltip from '$lib/components/ui/Tooltip.svelte';
	import { Info } from 'lucide-svelte';
	import { useScreen } from '$lib/core/screen.svelte';
	import LoadFailed from '$lib/features/catalog/components/LoadFailed.svelte';
	import { shown } from '$lib/features/catalog/api';
	import { Metric, Period, type LeaderboardView, type Surface } from '$lib/generated/core';
	import LeaderboardContent from '../components/LeaderboardContent.svelte';
	import LeaderboardSkeleton from '../components/LeaderboardSkeleton.svelte';
	import { metricLabel } from '../labels';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: { period: Period; metric: Metric; screen: Surface } } = $props();

	const board = useScreen<LeaderboardView>(() => data.screen);

	// The switchers are chrome driven by URL params, not by the payload, so they
	// live outside CachedView - inside it they would turn into shimmer blocks at
	// exactly the moment the user is looking at them.
	let metric = $state<Metric>(data.metric);
	let period = $state<Period>(data.period);

	// back/forward navigation changes the URL without touching the controls, so
	// re-adopt whatever the loader resolved
	$effect(() => {
		if (metric !== data.metric) metric = data.metric;
		if (period !== data.period) period = data.period;
	});

	const metrics = $derived([
		{ value: Metric.Xp, label: metricLabel(Metric.Xp) },
		{ value: Metric.Watch, label: metricLabel(Metric.Watch) },
		{ value: Metric.Achievements, label: metricLabel(Metric.Achievements) }
	]);

	const periods = $derived([
		{ value: Period.All, label: m.leaderboard_period_all() },
		{ value: Period.Month, label: m.leaderboard_period_month() },
		{ value: Period.Week, label: m.leaderboard_period_week() }
	]);

	// XP is lifetime by definition (completions carry no date), so the period
	// selector is meaningless on that board rather than silently ignored.
	const periodApplies = $derived(metric !== Metric.Xp);

	function apply(next: { metric?: Metric; period?: Period }) {
		const m2 = next.metric ?? metric;
		const p2 = next.period ?? period;
		if (m2 === metric && p2 === period) return;
		goto(`?metric=${m2}&period=${p2}`, { replaceState: true, noScroll: true, keepFocus: true });
	}
</script>

<svelte:head>
	<title>{m.leaderboard_page_title()}</title>
</svelte:head>

<div class="mx-auto max-w-4xl px-6 pt-28 pb-16">
	<div class="mb-6 flex flex-wrap items-center gap-3">
		<h1 class="text-2xl font-bold">{m.leaderboard_heading()}</h1>
		<div class="ml-auto flex flex-wrap items-center gap-3">
			<Tabs bind:value={metric} items={metrics} onchange={(v) => apply({ metric: v as Metric })} />
			{#if periodApplies}
				<Select
					bind:value={period}
					items={periods}
					label={m.leaderboard_period()}
					onchange={(v) => apply({ period: v as Period })}
				/>
			{:else}
				<Tooltip label={m.leaderboard_period_locked()}>
					{#snippet trigger(props)}
						<span
							{...props}
							class="flex items-center gap-1.5 rounded-full border border-edge bg-surface px-4
								py-2 text-xs text-faint"
						>
							<Info class="size-3.5" />
							{m.leaderboard_period_all()}
						</span>
					{/snippet}
				</Tooltip>
			{/if}
		</div>
	</div>

	<CachedView value={shown(board.view)} status={board.view?.status}>
		{#snippet content(view)}
			<LeaderboardContent board={view} />
		{/snippet}
		{#snippet skeleton()}
			<LeaderboardSkeleton />
		{/snippet}
		{#snippet failed()}
			<LoadFailed screen={data.screen} />
		{/snippet}
	</CachedView>
</div>
