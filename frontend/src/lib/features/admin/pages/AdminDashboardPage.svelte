<script lang="ts">
	import {
		Boxes,
		Clapperboard,
		Cpu,
		Disc3,
		Gauge,
		ListVideo,
		MemoryStick,
		Music,
		Timer,
		Tv,
		Users
	} from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import * as jobsApi from '$lib/features/jobs/api';
	import type {
		AnalyticsInfo,
		OverviewInfo,
		StorageInfo,
		SystemStats
	} from '$lib/features/jobs/api';
	import BarChart from '$lib/features/admin/components/BarChart.svelte';
	import Sparkline from '$lib/features/admin/components/Sparkline.svelte';
	import StorageBar from '$lib/features/admin/components/StorageBar.svelte';
	import { categoryStyle } from '$lib/features/admin/components/storageColors';
	import { jobAction, jobSubjectLabel } from '$lib/features/jobs/job-label';
	import { formatBytes, formatDate, formatUptime } from '$lib/utils/format';
	import { usageColor } from '$lib/utils/usage-color';
	import * as m from '$lib/paraglide/messages';

	let overview = $state<OverviewInfo | null>(null);
	let storage = $state<StorageInfo | null>(null);
	let system = $state<SystemStats | null>(null);
	let analytics = $state<AnalyticsInfo | null>(null);
	let cpuHistory = $state<number[]>([]);
	let memHistory = $state<number[]>([]);

	$effect(() => {
		jobsApi.getOverview().then((o) => (overview = o));
		jobsApi.getStorage().then((s) => (storage = s));
		jobsApi.getAnalytics(30).then((a) => (analytics = a));

		async function pollSystem() {
			if (document.visibilityState === 'hidden') return;
			try {
				const s = await jobsApi.getSystem();
				system = s;
				if (s.cpuPercent >= 0) {
					cpuHistory = [...cpuHistory, s.cpuPercent].slice(-40);
				}
				if (s.memTotal > 0) {
					memHistory = [...memHistory, (s.memUsed / s.memTotal) * 100].slice(-40);
				}
			} catch {
				// transient; the next tick retries
			}
		}
		pollSystem();
		const t = setInterval(pollSystem, 3000);
		return () => clearInterval(t);
	});

	const memPercent = $derived(
		system && system.memTotal > 0 ? (system.memUsed / system.memTotal) * 100 : 0
	);
	// load relative to core count: <0.7/core healthy, <1.0 busy, ≥1.0 saturated
	const loadPerCore = $derived(
		system && system.load1 >= 0 && system.cpuCores > 0 ? system.load1 / system.cpuCores : -1
	);
	const loadLabel = $derived(
		loadPerCore < 0
			? ''
			: loadPerCore < 0.7
				? m.admin_load_healthy()
				: loadPerCore < 1
					? m.admin_load_busy()
					: m.admin_load_saturated()
	);

	const cards = $derived(
		overview
			? [
					{
						label: m.nav_movies(),
						value: overview.counts.movies,
						icon: Clapperboard,
						href: '/admin/library?type=movie'
					},
					{
						label: m.nav_series(),
						value: overview.counts.series,
						icon: Tv,
						href: '/admin/library?type=series'
					},
					{
						label: m.admin_episodes(),
						value: overview.counts.episodes,
						icon: ListVideo,
						href: '/admin/library?type=series'
					},
					{
						label: m.admin_albums(),
						value: overview.counts.albums,
						icon: Disc3,
						href: '/admin/jobs'
					},
					{
						label: m.admin_tracks(),
						value: overview.counts.tracks,
						icon: Music,
						href: '/admin/jobs'
					},
					{
						label: m.admin_nav_users(),
						value: overview.counts.users,
						icon: Users,
						href: '/admin/users'
					}
				]
			: []
	);

	const statusColor: Record<string, string> = {
		pending: 'text-muted',
		running: 'text-accent',
		done: 'text-success',
		failed: 'text-danger',
		cancelled: 'text-faint'
	};

	const MUSIC_COLOR = '#f5b14c'; // matches the storage music segment
	const watchBars = $derived(
		(analytics?.daily ?? []).map((d) => ({
			label: formatDate(d.day),
			segments: [
				{ name: m.admin_video(), value: d.videoSeconds, color: 'var(--color-accent)' },
				{ name: m.nav_music(), value: d.musicSeconds, color: MUSIC_COLOR }
			]
		}))
	);
	const watchTotal = $derived(
		analytics ? analytics.totals.videoSeconds + analytics.totals.musicSeconds : 0
	);
	const avgActiveUsers = $derived.by(() => {
		if (!analytics || analytics.daily.length === 0) return 0;
		return analytics.daily.reduce((sum, d) => sum + d.activeUsers, 0) / analytics.daily.length;
	});
</script>

<svelte:head>
	<title>{m.admin_overview_title()}</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">{m.admin_nav_overview()}</h1>

<div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
	{#each cards as card, i (card.label)}
		<a
			in:fly|global={{ y: 16, duration: 350, delay: Math.min(i * 55, 300) }}
			href={card.href}
			class="group flex items-center gap-4 rounded-card border border-edge bg-surface/40 p-6
				transition-colors hover:border-accent/50 hover:bg-surface/70"
		>
			<span class="flex size-12 shrink-0 items-center justify-center rounded-xl bg-accent-soft">
				<card.icon class="size-5 text-accent" />
			</span>
			<span class="min-w-0">
				<span class="block text-3xl font-extrabold tracking-tight tnum">{card.value}</span>
				<span class="block text-xs font-medium text-muted transition-colors group-hover:text-text">
					{card.label}
				</span>
			</span>
		</a>
	{/each}
</div>

{#if system}
	<div class="mt-6" in:fly|global={{ y: 20, duration: 400 }}>
		<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_system()}</h2>
		<div class="grid gap-4 lg:grid-cols-2">
			<div class="rounded-card border border-edge bg-surface/40 p-5">
				<div class="mb-3 flex items-start justify-between">
					<span class="flex items-center gap-2 text-sm font-medium text-muted">
						<Cpu class="size-4 text-accent" />
						{m.admin_cpu()}
					</span>
					{#if system.cpuPercent >= 0}
						<span
							class="text-2xl leading-none font-bold tnum"
							style="color: {usageColor(system.cpuPercent)}"
						>
							{system.cpuPercent.toFixed(0)}%
						</span>
					{/if}
				</div>
				{#if system.cpuPercent >= 0}
					<Sparkline
						values={cpuHistory}
						max={100}
						class="h-20 w-full"
						color={usageColor(system.cpuPercent)}
					/>
					<div
						class="mt-2 flex flex-wrap items-center justify-between gap-x-3 text-[11px] text-faint"
					>
						<span>{m.admin_cores({ count: system.cpuCores })}</span>
						<span class="flex gap-3 tnum">
							{#if system.app && system.app.cpuPercent >= 0}
								<span>{m.admin_go_app_percent({ percent: system.app.cpuPercent.toFixed(1) })}</span>
							{/if}
							{#if system.ffmpeg && system.ffmpeg.processes > 0}
								<span>
									{m.admin_transcoding_cpu({
										count: system.ffmpeg.processes,
										percent: system.ffmpeg.cpuPercent.toFixed(0)
									})}
								</span>
							{/if}
						</span>
					</div>
				{:else}
					<p class="py-6 text-xs text-faint">{m.admin_cpu_unavailable()}</p>
				{/if}
			</div>

			<div class="rounded-card border border-edge bg-surface/40 p-5">
				<div class="mb-3 flex items-start justify-between">
					<span class="flex items-center gap-2 text-sm font-medium text-muted">
						<MemoryStick class="size-4 text-accent" />
						{m.admin_memory()}
					</span>
					{#if system.memTotal > 0}
						<span
							class="text-2xl leading-none font-bold tnum"
							style="color: {usageColor(memPercent)}"
						>
							{memPercent.toFixed(0)}%
						</span>
					{/if}
				</div>
				{#if system.memTotal > 0}
					<Sparkline
						values={memHistory}
						max={100}
						class="h-20 w-full"
						color={usageColor(memPercent)}
					/>
					<div
						class="mt-2 flex flex-wrap items-center justify-between gap-x-3 text-[11px] text-faint"
					>
						<span class="tnum">
							{m.admin_memory_used({
								used: formatBytes(system.memUsed),
								total: formatBytes(system.memTotal)
							})}
						</span>
						<span class="flex gap-3 tnum">
							{#if system.app && system.app.memBytes > 0}
								<span>{m.admin_go_app_value({ value: formatBytes(system.app.memBytes) })}</span>
							{/if}
							{#if system.ffmpeg && system.ffmpeg.processes > 0}
								<span
									>{m.admin_transcoding_value({ value: formatBytes(system.ffmpeg.memBytes) })}</span
								>
							{/if}
						</span>
					</div>
				{:else}
					<p class="py-6 text-xs text-faint">{m.admin_memory_unavailable()}</p>
				{/if}
			</div>
		</div>

		<dl class="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-4">
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<Gauge class="size-3" />
					{m.admin_load_1m()}
				</dt>
				{#if system.load1 >= 0}
					<dd class="mt-1 flex items-baseline gap-2">
						<span class="text-lg font-bold tnum" style="color: {usageColor(loadPerCore * 100)}">
							{system.load1.toFixed(2)}
						</span>
						<span class="text-[11px]" style="color: {usageColor(loadPerCore * 100)}">
							{loadLabel}
						</span>
					</dd>
					<div class="mt-1.5 h-1 overflow-hidden rounded-full bg-surface-2">
						<div
							class="h-full rounded-full"
							style="width: {Math.min(100, loadPerCore * 100)}%; background: {usageColor(
								loadPerCore * 100
							)}"
						></div>
					</div>
				{:else}
					<dd class="mt-1 text-lg font-bold text-faint">{m.admin_not_available()}</dd>
				{/if}
			</div>
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<Boxes class="size-3" />
					{m.admin_goroutines()}
				</dt>
				<dd class="mt-1 text-lg font-bold tnum">{system.goroutines}</dd>
			</div>
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<MemoryStick class="size-3" />
					{m.admin_go_heap()}
				</dt>
				<dd class="mt-1 text-lg font-bold tnum">{formatBytes(system.goHeapBytes)}</dd>
			</div>
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<Timer class="size-3" />
					{m.admin_uptime()}
				</dt>
				<dd class="mt-1 text-lg font-bold tnum">{formatUptime(system.uptimeSeconds)}</dd>
			</div>
		</dl>
	</div>
{/if}

{#if analytics && watchTotal > 0}
	<div class="mt-6" in:fly|global={{ y: 20, duration: 400 }}>
		<h2 class="mb-3 text-sm font-semibold text-muted">
			{m.admin_analytics_last_days({ days: analytics.days })}
		</h2>
		<div class="grid gap-6 lg:grid-cols-3">
			<div class="rounded-card border border-edge bg-surface/40 p-6 lg:col-span-2">
				<div class="mb-3 flex items-baseline justify-between">
					<h3 class="text-sm font-semibold text-muted">{m.admin_watch_time()}</h3>
					<span class="text-xs text-faint tnum"
						>{m.admin_total_value({ value: formatUptime(watchTotal) })}</span
					>
				</div>
				<BarChart bars={watchBars} format={formatUptime} class="h-36 w-full" />
				<div class="mt-3 flex flex-wrap items-center justify-between gap-x-4 text-[11px]">
					<span class="flex items-center gap-4 text-muted">
						<span class="flex items-center gap-1.5">
							<span class="size-2 rounded-full bg-accent"></span>
							{m.admin_video()}
						</span>
						<span class="flex items-center gap-1.5">
							<span class="size-2 rounded-full" style="background: {MUSIC_COLOR}"></span>
							{m.nav_music()}
						</span>
					</span>
					<span class="text-faint tnum">
						{m.admin_avg_active_users({ count: avgActiveUsers.toFixed(1) })}
					</span>
				</div>
			</div>

			<div class="rounded-card border border-edge bg-surface/40 p-6">
				<h3 class="mb-3 text-sm font-semibold text-muted">{m.admin_top_titles()}</h3>
				{#if analytics.topTitles.length === 0}
					<p class="text-xs text-faint">{m.admin_no_watch_time()}</p>
				{:else}
					<ul class="space-y-2.5 text-xs">
						{#each analytics.topTitles.slice(0, 6) as title (title.titleId)}
							<li>
								<div class="flex items-baseline justify-between gap-3">
									<a
										href="/title/{title.slug}"
										class="truncate font-medium transition-colors hover:text-accent"
									>
										{title.name}
									</a>
									<span class="shrink-0 text-faint tnum">{formatUptime(title.seconds)}</span>
								</div>
								<div class="mt-1 h-1 overflow-hidden rounded-full bg-surface-2">
									<div
										class="h-full rounded-full bg-accent"
										style="width: {(title.seconds / analytics.topTitles[0].seconds) * 100}%"
									></div>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		</div>
	</div>
{/if}

<div class="mt-6 grid gap-6 lg:grid-cols-2" in:fly|global={{ y: 20, duration: 400, delay: 80 }}>
	{#if storage && storage.diskTotal > 0}
		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<div class="mb-3 flex items-baseline justify-between">
				<h2 class="text-sm font-semibold text-muted">{m.admin_storage()}</h2>
				<span class="text-xs text-faint tnum">
					{m.admin_storage_available({
						used: formatBytes(storage.used),
						budget: formatBytes(storage.budget)
					})}
				</span>
			</div>
			<StorageBar {storage} class="mb-4 h-2" />
			<ul class="space-y-1.5 text-xs">
				{#each storage.categories as cat (cat.kind)}
					{@const style = categoryStyle[cat.kind] ?? { label: cat.kind, color: '#5b6072' }}
					<li class="flex items-center justify-between">
						<span class="flex items-center gap-2 text-muted">
							<span class="size-2 rounded-full" style="background: {style.color}"></span>
							{style.label}
						</span>
						<span class="text-faint tnum">{formatBytes(cat.bytes)}</span>
					</li>
				{/each}
				<li class="flex items-center justify-between border-t border-edge/50 pt-1.5">
					<span class="flex items-center gap-2 text-muted">
						<span class="size-2 rounded-full bg-surface-2 ring-1 ring-edge"></span>
						{m.admin_free()}
					</span>
					<span class="text-faint tnum">{formatBytes(storage.free)}</span>
				</li>
			</ul>
		</div>
	{/if}

	{#if overview}
		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<div class="mb-3 flex items-baseline justify-between">
				<h2 class="text-sm font-semibold text-muted">{m.admin_activity()}</h2>
				<a href="/admin/jobs" class="text-xs text-accent hover:underline">
					{m.admin_jobs_in_queue({ count: overview.pendingJobs })}
				</a>
			</div>
			{#if overview.recentJobs.length === 0}
				<p class="text-xs text-faint">{m.admin_no_recent_activity()}</p>
			{:else}
				<ul class="space-y-2 text-xs">
					{#each overview.recentJobs as job (job.id)}
						{@const subject = jobSubjectLabel(job)}
						<li class="flex items-center justify-between gap-3">
							<span class="truncate text-muted">
								{jobAction(job)}{subject ? ` - ${subject}` : ` #${job.id}`}
							</span>
							<span class="font-semibold capitalize {statusColor[job.status]}">{job.status}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</div>
