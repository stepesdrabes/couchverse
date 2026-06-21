<script lang="ts">
	import {
		Boxes,
		Clapperboard,
		Cpu,
		Disc3,
		Film,
		Gauge,
		ListVideo,
		MemoryStick,
		MonitorPlay,
		Music,
		Sofa,
		Sparkles,
		Timer,
		Tv,
		Users,
		Zap
	} from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import * as jobsApi from '$lib/features/jobs/api';
	import type {
		AnalyticsInfo,
		LiveStats,
		OverviewInfo,
		StorageInfo,
		SystemStats
	} from '$lib/features/jobs/api';
	import BarChart from '$lib/features/admin/components/BarChart.svelte';
	import Sparkline from '$lib/features/admin/components/Sparkline.svelte';
	import StorageBar from '$lib/features/admin/components/StorageBar.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { categoryStyle } from '$lib/features/admin/components/storageColors';
	import { jobAction, jobSubjectLabel } from '$lib/features/jobs/job-label';
	import { formatBytes, formatDate, formatUptime } from '$lib/utils/format';
	import { usageColor } from '$lib/utils/usage-color';
	import * as m from '$lib/paraglide/messages';

	let overview = $state<OverviewInfo | null>(null);
	let storage = $state<StorageInfo | null>(null);
	let system = $state<SystemStats | null>(null);
	let analytics = $state<AnalyticsInfo | null>(null);
	let live = $state<LiveStats | null>(null);
	let cpuHistory = $state<number[]>([]);
	let memHistory = $state<number[]>([]);

	$effect(() => {
		jobsApi.getOverview().then((o) => (overview = o));
		jobsApi.getStorage().then((s) => (storage = s));
		jobsApi.getAnalytics(30).then((a) => (analytics = a));

		async function pollSystem() {
			if (document.visibilityState === 'hidden') return;
			try {
				const [s, l] = await Promise.all([jobsApi.getSystem(), jobsApi.getLive()]);
				system = s;
				live = l;
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

	const lib = $derived(overview?.library ?? null);
	const contentHours = $derived(lib ? Math.round(lib.totalRuntimeSeconds / 3600) : 0);
	const QUALITY = [
		{ key: 'uhd', label: '4K', color: 'var(--color-accent)' },
		{ key: 'fhd', label: '1080p', color: '#38bdf8' },
		{ key: 'hd', label: '720p', color: '#f5b14c' },
		{ key: 'sd', label: 'SD', color: '#5b6072' }
	] as const;
	const qualityBars = $derived(
		lib
			? QUALITY.map((q) => ({
					key: q.key,
					label: q.label,
					color: q.color,
					count: lib.quality[q.key]
				}))
			: []
	);
	const qualityTotal = $derived(qualityBars.reduce((sum, q) => sum + q.count, 0));

	const liveCards = $derived(
		live
			? [
					{ label: m.admin_streaming_now(), value: live.streams, icon: MonitorPlay, sub: '' },
					{
						label: m.admin_on_couch(),
						value: live.couchSessions,
						icon: Sofa,
						sub: live.couchViewers > 0 ? m.admin_couch_watching({ count: live.couchViewers }) : ''
					},
					{ label: m.admin_instant_play(), value: live.transcodes, icon: Zap, sub: '' }
				]
			: []
	);
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

{#if live}
	<div class="mt-6" in:fly|global={{ y: 20, duration: 400 }}>
		<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_live()}</h2>
		<div class="grid gap-4 sm:grid-cols-3">
			{#each liveCards as c (c.label)}
				<div
					class="flex items-center gap-4 rounded-card border bg-surface/40 p-5 transition-colors
						{c.value > 0 ? 'border-accent/50' : 'border-edge'}"
				>
					<span
						class="relative flex size-12 shrink-0 items-center justify-center rounded-xl
							{c.value > 0 ? 'bg-accent-soft' : 'bg-surface-2'}"
					>
						<c.icon class="size-5 {c.value > 0 ? 'text-accent' : 'text-faint'}" />
						{#if c.value > 0}
							<span class="absolute -top-0.5 -right-0.5 flex size-2.5">
								<span class="absolute inline-flex size-full animate-ping rounded-full bg-accent/70"
								></span>
								<span class="relative inline-flex size-2.5 rounded-full bg-accent"></span>
							</span>
						{/if}
					</span>
					<span class="min-w-0">
						<span
							class="block text-3xl font-extrabold tracking-tight tnum {c.value > 0
								? ''
								: 'text-faint'}"
						>
							{c.value}
						</span>
						<span class="block text-xs font-medium text-muted">{c.label}</span>
						{#if c.sub}
							<span class="block text-[11px] text-faint tnum">{c.sub}</span>
						{/if}
					</span>
				</div>
			{/each}
		</div>
	</div>
{/if}

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

{#if lib}
	<div
		class="mt-6 rounded-card border border-edge bg-surface/40 p-6"
		in:fly|global={{ y: 20, duration: 400 }}
	>
		<h2 class="mb-4 text-sm font-semibold text-muted">{m.admin_library()}</h2>
		<div class="grid gap-6 sm:grid-cols-2">
			<div class="flex gap-8">
				<div>
					<div class="flex items-center gap-1.5 text-[11px] text-faint">
						<Film class="size-3" />
						{m.admin_content_runtime()}
					</div>
					<p class="mt-1 text-3xl font-extrabold tracking-tight tnum">
						{contentHours}<span class="ml-1 text-base font-semibold text-muted">h</span>
					</p>
				</div>
				<div>
					<div class="flex items-center gap-1.5 text-[11px] text-faint">
						<Sparkles class="size-3" />
						{m.admin_added_recently()}
					</div>
					<p class="mt-1 text-3xl font-extrabold tracking-tight tnum">{lib.addedLast30Days}</p>
				</div>
			</div>
			{#if qualityTotal > 0}
				<div>
					<div class="mb-2 flex items-center justify-between text-[11px] text-faint">
						<span>{m.admin_quality_mix()}</span>
						{#if lib.hdr > 0}
							<span class="rounded bg-accent-soft px-1.5 py-0.5 font-semibold text-accent">
								{m.admin_hdr({ count: lib.hdr })}
							</span>
						{/if}
					</div>
					<div class="flex h-2 overflow-hidden rounded-full bg-surface-2">
						{#each qualityBars as q (q.key)}
							{#if q.count > 0}
								<div style="width: {(q.count / qualityTotal) * 100}%; background: {q.color}"></div>
							{/if}
						{/each}
					</div>
					<ul class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-[11px]">
						{#each qualityBars as q (q.key)}
							<li class="flex items-center gap-1.5 text-muted">
								<span class="size-2 rounded-full" style="background: {q.color}"></span>
								{q.label}
								<span class="text-faint tnum">{q.count}</span>
							</li>
						{/each}
					</ul>
				</div>
			{/if}
		</div>
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

{#if analytics && (analytics.topUsers.length > 0 || analytics.totals.couchSeconds > 0)}
	<div class="mt-6 grid gap-6 lg:grid-cols-2" in:fly|global={{ y: 20, duration: 400 }}>
		{#if analytics.topUsers.length > 0}
			<div class="rounded-card border border-edge bg-surface/40 p-6">
				<h2 class="mb-3 text-sm font-semibold text-muted">{m.admin_top_viewers()}</h2>
				<ul class="space-y-2.5 text-xs">
					{#each analytics.topUsers.slice(0, 6) as u, i (u.userId)}
						<li class="flex items-center gap-3">
							<span class="w-4 shrink-0 text-center font-semibold text-faint tnum">{i + 1}</span>
							<UserAvatar
								name={u.displayName}
								avatarId={u.avatarId}
								seed={u.userId}
								class="size-7 rounded-lg text-[10px]"
							/>
							<span class="min-w-0 flex-1 truncate font-medium">{u.displayName}</span>
							<span class="shrink-0 text-faint tnum">{formatUptime(u.seconds)}</span>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
		{#if analytics.totals.couchSeconds > 0}
			<div class="rounded-card border border-edge bg-surface/40 p-6">
				<div class="mb-1 flex items-baseline justify-between">
					<h2 class="text-sm font-semibold text-muted">{m.admin_couch_watch_time()}</h2>
					<span class="text-xs text-faint tnum">
						{m.admin_total_value({ value: formatUptime(analytics.totals.couchSeconds) })}
					</span>
				</div>
				<p class="mb-3 text-[11px] text-faint">{m.admin_couch_watch_hint()}</p>
				{#if analytics.topCouchTitles.length > 0}
					<ul class="space-y-2.5 text-xs">
						{#each analytics.topCouchTitles.slice(0, 6) as title (title.titleId)}
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
										style="width: {(title.seconds / analytics.topCouchTitles[0].seconds) * 100}%"
									></div>
								</div>
							</li>
						{/each}
					</ul>
				{/if}
			</div>
		{/if}
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
