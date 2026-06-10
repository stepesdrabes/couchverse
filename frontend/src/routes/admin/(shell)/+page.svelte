<script lang="ts">
	import { Clapperboard, Disc3, ListVideo, Music, Tv, Users } from 'lucide-svelte';
	import * as jobsApi from '$lib/features/jobs/api';
	import type { OverviewInfo, StorageInfo } from '$lib/features/jobs/api';
	import StorageBar from '$lib/components/admin/StorageBar.svelte';
	import { categoryStyle } from '$lib/components/admin/storageColors';
	import { formatBytes } from '$lib/utils/format';

	let overview = $state<OverviewInfo | null>(null);
	let storage = $state<StorageInfo | null>(null);

	$effect(() => {
		jobsApi.getOverview().then((o) => (overview = o));
		jobsApi.getStorage().then((s) => (storage = s));
	});

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
