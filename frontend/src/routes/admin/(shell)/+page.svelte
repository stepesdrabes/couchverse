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
	import * as jobsApi from '$lib/features/jobs/api';
	import type { OverviewInfo, StorageInfo, SystemStats } from '$lib/features/jobs/api';
	import Sparkline from '$lib/components/admin/Sparkline.svelte';
	import StorageBar from '$lib/components/admin/StorageBar.svelte';
	import { categoryStyle } from '$lib/components/admin/storageColors';
	import { formatBytes, formatUptime } from '$lib/utils/format';

	let overview = $state<OverviewInfo | null>(null);
	let storage = $state<StorageInfo | null>(null);
	let system = $state<SystemStats | null>(null);
	let cpuHistory = $state<number[]>([]);
	let memHistory = $state<number[]>([]);

	$effect(() => {
		jobsApi.getOverview().then((o) => (overview = o));
		jobsApi.getStorage().then((s) => (storage = s));

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
	const loadColor = $derived(
		loadPerCore < 0
			? 'text-faint'
			: loadPerCore < 0.7
				? 'text-success'
				: loadPerCore < 1
					? 'text-amber-400'
					: 'text-danger'
	);
	const loadLabel = $derived(
		loadPerCore < 0 ? '' : loadPerCore < 0.7 ? 'Healthy' : loadPerCore < 1 ? 'Busy' : 'Saturated'
	);

	const cards = $derived(
		overview
			? [
					{
						label: 'Movies',
						value: overview.counts.movies,
						icon: Clapperboard,
						href: '/admin/library?type=movie'
					},
					{
						label: 'Series',
						value: overview.counts.series,
						icon: Tv,
						href: '/admin/library?type=series'
					},
					{
						label: 'Episodes',
						value: overview.counts.episodes,
						icon: ListVideo,
						href: '/admin/library?type=series'
					},
					{ label: 'Albums', value: overview.counts.albums, icon: Disc3, href: '/admin/jobs' },
					{ label: 'Tracks', value: overview.counts.tracks, icon: Music, href: '/admin/jobs' },
					{ label: 'Users', value: overview.counts.users, icon: Users, href: '/admin/users' }
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
</script>

<svelte:head>
	<title>Overview — Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Overview</h1>

<div class="grid gap-4 sm:grid-cols-3 xl:grid-cols-6">
	{#each cards as card (card.label)}
		<a
			href={card.href}
			class="group rounded-card border border-edge bg-surface/40 p-5 transition-colors hover:border-faint"
		>
			<card.icon class="mb-3 size-5 text-accent" />
			<p class="text-2xl font-bold tnum">{card.value}</p>
			<p class="text-xs font-medium text-muted group-hover:text-text">{card.label}</p>
		</a>
	{/each}
</div>

{#if system}
	<div class="mt-6">
		<h2 class="mb-3 text-sm font-semibold text-muted">System</h2>
		<div class="grid gap-4 lg:grid-cols-2">
			<div class="rounded-card border border-edge bg-surface/40 p-5">
				<div class="mb-3 flex items-start justify-between">
					<span class="flex items-center gap-2 text-sm font-medium text-muted">
						<Cpu class="size-4 text-accent" /> CPU
					</span>
					{#if system.cpuPercent >= 0}
						<span class="text-2xl leading-none font-bold tnum">{system.cpuPercent.toFixed(0)}%</span
						>
					{/if}
				</div>
				{#if system.cpuPercent >= 0}
					<Sparkline values={cpuHistory} max={100} class="h-20 w-full" />
					<p class="mt-2 text-[11px] text-faint">{system.cpuCores} cores</p>
				{:else}
					<p class="py-6 text-xs text-faint">Host CPU stats are unavailable on this platform.</p>
				{/if}
			</div>

			<div class="rounded-card border border-edge bg-surface/40 p-5">
				<div class="mb-3 flex items-start justify-between">
					<span class="flex items-center gap-2 text-sm font-medium text-muted">
						<MemoryStick class="size-4 text-accent" /> Memory
					</span>
					{#if system.memTotal > 0}
						<span class="text-2xl leading-none font-bold tnum">{memPercent.toFixed(0)}%</span>
					{/if}
				</div>
				{#if system.memTotal > 0}
					<Sparkline
						values={memHistory}
						max={100}
						class="h-20 w-full"
						color="var(--color-success)"
					/>
					<p class="mt-2 text-[11px] text-faint tnum">
						{formatBytes(system.memUsed)} of {formatBytes(system.memTotal)} used
					</p>
				{:else}
					<p class="py-6 text-xs text-faint">Host memory stats are unavailable on this platform.</p>
				{/if}
			</div>
		</div>

		<dl class="mt-4 grid grid-cols-2 gap-4 sm:grid-cols-4">
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<Gauge class="size-3" /> Load (1m)
				</dt>
				{#if system.load1 >= 0}
					<dd class="mt-1 flex items-baseline gap-2">
						<span class="text-lg font-bold tnum {loadColor}">{system.load1.toFixed(2)}</span>
						<span class="text-[11px] {loadColor}">{loadLabel}</span>
					</dd>
					<div class="mt-1.5 h-1 overflow-hidden rounded-full bg-surface-2">
						<div
							class="h-full rounded-full bg-current {loadColor}"
							style="width: {Math.min(100, loadPerCore * 100)}%"
						></div>
					</div>
				{:else}
					<dd class="mt-1 text-lg font-bold text-faint">n/a</dd>
				{/if}
			</div>
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<Boxes class="size-3" /> Goroutines
				</dt>
				<dd class="mt-1 text-lg font-bold tnum">{system.goroutines}</dd>
			</div>
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<MemoryStick class="size-3" /> Go heap
				</dt>
				<dd class="mt-1 text-lg font-bold tnum">{formatBytes(system.goHeapBytes)}</dd>
			</div>
			<div class="rounded-card border border-edge bg-surface/40 px-4 py-3">
				<dt class="flex items-center gap-1.5 text-[11px] text-faint">
					<Timer class="size-3" /> Uptime
				</dt>
				<dd class="mt-1 text-lg font-bold tnum">{formatUptime(system.uptimeSeconds)}</dd>
			</div>
		</dl>
	</div>
{/if}

<div class="mt-6 grid gap-6 lg:grid-cols-2">
	{#if storage && storage.diskTotal > 0}
		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<div class="mb-3 flex items-baseline justify-between">
				<h2 class="text-sm font-semibold text-muted">Storage</h2>
				<span class="text-xs text-faint tnum">
					{formatBytes(storage.used)} of {formatBytes(storage.budget)} available
				</span>
			</div>
			<StorageBar {storage} class="mb-4 h-2" />
			<ul class="space-y-1.5 text-xs">
				{#each storage.categories as cat (cat.kind)}
					<li class="flex items-center justify-between">
						<span class="flex items-center gap-2 text-muted">
							<span class="size-2 rounded-full" style="background: {categoryStyle[cat.kind].color}"
							></span>
							{categoryStyle[cat.kind].label}
						</span>
						<span class="text-faint tnum">{formatBytes(cat.bytes)}</span>
					</li>
				{/each}
				<li class="flex items-center justify-between border-t border-edge/50 pt-1.5">
					<span class="flex items-center gap-2 text-muted">
						<span class="size-2 rounded-full bg-surface-2 ring-1 ring-edge"></span>
						Free
					</span>
					<span class="text-faint tnum">{formatBytes(storage.free)}</span>
				</li>
			</ul>
		</div>
	{/if}

	{#if overview}
		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<div class="mb-3 flex items-baseline justify-between">
				<h2 class="text-sm font-semibold text-muted">Activity</h2>
				<a href="/admin/jobs" class="text-xs text-accent hover:underline">
					{overview.pendingJobs} job{overview.pendingJobs === 1 ? '' : 's'} in queue
				</a>
			</div>
			{#if overview.recentJobs.length === 0}
				<p class="text-xs text-faint">No recent activity.</p>
			{:else}
				<ul class="space-y-2 text-xs">
					{#each overview.recentJobs as job (job.id)}
						<li class="flex items-center justify-between gap-3">
							<span class="truncate text-muted">{job.type.replaceAll('_', ' ')} #{job.id}</span>
							<span class="font-semibold capitalize {statusColor[job.status]}">{job.status}</span>
						</li>
					{/each}
				</ul>
			{/if}
		</div>
	{/if}
</div>
