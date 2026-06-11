<script lang="ts">
	import { FolderSearch, RefreshCw, RotateCcw, X } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as jobsApi from '$lib/features/jobs/api';
	import type { Job, Library } from '$lib/features/jobs/api';
	import { jobAction, jobSubjectLabel } from '$lib/features/jobs/job-label';
	import Button from '$lib/components/ui/Button.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import { formatYearDate } from '$lib/utils/format';

	let libraries = $state<Library[]>([]);
	let jobs = $state<Job[]>([]);

	async function refresh() {
		[libraries, jobs] = await Promise.all([jobsApi.listLibraries(), jobsApi.listJobs()]);
	}

	$effect(() => {
		refresh();
		const t = setInterval(refresh, 3000);
		return () => clearInterval(t);
	});

	async function scan(lib: Library) {
		try {
			await jobsApi.scanLibrary(lib.id);
			toast.success(`Scanning “${lib.name}”`);
			refresh();
		} catch {
			toast.error('Failed to start scan');
		}
	}

	async function scanAll() {
		try {
			await jobsApi.scanAllLibraries();
			toast.success('Scanning all libraries');
			refresh();
		} catch {
			toast.error('Failed to start scans');
		}
	}

	const statusColor: Record<Job['status'], string> = {
		pending: 'text-muted',
		running: 'text-accent',
		done: 'text-success',
		failed: 'text-danger',
		cancelled: 'text-faint'
	};

	// subject comes from the backend; older payload shapes are the fallback
	const jobSubject = (job: Job) => {
		const subject = jobSubjectLabel(job);
		if (subject) return subject;
		if (job.type === 'scan_library') {
			const lib = libraries.find((l) => l.id === job.payload.libraryId);
			return lib ? `“${lib.name}”` : `library #${job.payload.libraryId}`;
		}
		if (typeof job.payload.mediaFileId === 'string') return `file #${job.payload.mediaFileId}`;
		return null;
	};

	// collapse jobs onto the content they belong to: all jobs for one media file
	// (e.g. the 1080p/720p/480p transcodes of an episode) become one entry
	const groupKey = (job: Job): string => {
		const s = job.subject;
		if (s?.mediaFileId) return `mf:${s.mediaFileId}`;
		if (s?.titleId) return `title:${s.titleId}`;
		const lib = job.payload.libraryId;
		if (lib !== undefined && lib !== null) return `lib:${lib}`;
		return `job:${job.id}`;
	};

	interface JobGroup {
		key: string;
		label: string | null;
		jobs: Job[];
		latestId: number;
	}

	const groups = $derived.by(() => {
		// transient within the derived, recomputed each run - not reactive state
		// eslint-disable-next-line svelte/prefer-svelte-reactivity
		const map = new Map<string, JobGroup>();
		for (const job of jobs) {
			const key = groupKey(job);
			let group = map.get(key);
			if (!group) {
				group = { key, label: jobSubject(job), jobs: [], latestId: job.id };
				map.set(key, group);
			}
			group.jobs.push(job);
			group.latestId = Math.max(group.latestId, job.id);
		}
		// newest activity first
		return [...map.values()].sort((a, b) => b.latestId - a.latestId);
	});

	const groupSummary = (group: JobGroup): string => {
		const counts: Record<string, number> = {};
		for (const job of group.jobs) counts[job.status] = (counts[job.status] ?? 0) + 1;
		const order: Job['status'][] = ['running', 'pending', 'failed', 'cancelled', 'done'];
		return order
			.filter((s) => counts[s])
			.map((s) => `${counts[s]} ${s}`)
			.join(' · ');
	};
</script>

<svelte:head>
	<title>Jobs & Storage - Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Jobs & Storage</h1>

<section class="mb-8">
	<div class="mb-3 flex items-center justify-between">
		<h2 class="text-sm font-semibold text-muted">Libraries</h2>
		<Button variant="secondary" size="sm" onclick={scanAll}>
			<RefreshCw class="size-3.5" />
			Re-scan all
		</Button>
	</div>
	<div class="grid gap-3 lg:grid-cols-3">
		{#each libraries as lib (lib.id)}
			<div class="rounded-card border border-edge bg-surface/40 p-4">
				<div class="flex items-start justify-between gap-2">
					<div class="min-w-0">
						<p class="font-semibold">{lib.name}</p>
						<p class="mt-0.5 truncate text-xs text-faint" title={lib.path}>{lib.path}</p>
					</div>
					<Button variant="ghost" size="sm" onclick={() => scan(lib)}>
						<FolderSearch class="size-3.5" />
						Scan
					</Button>
				</div>
				<p class="mt-3 text-[11px] text-faint">
					{lib.lastScannedAt
						? `Last scanned ${formatYearDate(lib.lastScannedAt)}`
						: 'Never scanned'}
				</p>
			</div>
		{/each}
	</div>
	<p class="mt-2 text-xs text-faint">
		Drop files into a library folder (SMB/SFTP) and hit Scan - series like
		<span class="font-mono">Show/Season 01/Show S01E01.mkv</span>, movies like
		<span class="font-mono">Name (2024)/Name (2024).mkv</span>, music sorted by its tags.
	</p>
</section>

{#snippet jobRow(job: Job)}
	<div class="flex items-center gap-3 px-4 py-2.5 text-sm">
		<div class="min-w-0 flex-1">
			<div class="flex items-center gap-2">
				<span class="truncate font-medium">{jobAction(job)}</span>
				{#if job.attempts > 1}
					<span class="text-[11px] text-faint">attempt {job.attempts}</span>
				{/if}
			</div>
			{#if job.lastError && (job.status === 'failed' || job.status === 'pending')}
				<p class="mt-0.5 max-w-md truncate text-xs text-danger" title={job.lastError}>
					{job.lastError}
				</p>
			{/if}
		</div>
		<span class="w-20 shrink-0 text-xs font-semibold capitalize {statusColor[job.status]}">
			{job.status}
		</span>
		<div class="w-28 shrink-0">
			{#if job.status === 'running' || job.status === 'pending'}
				<div class="h-1.5 overflow-hidden rounded-full bg-surface-2">
					<div
						class="h-full rounded-full bg-accent transition-all duration-500"
						style="width: {job.progress}%"
					></div>
				</div>
			{:else}
				<span class="text-xs text-faint">-</span>
			{/if}
		</div>
		<div class="w-24 shrink-0 text-right">
			{#if job.status === 'failed' || job.status === 'cancelled'}
				<Button variant="ghost" size="sm" onclick={() => jobsApi.retryJob(job.id).then(refresh)}>
					<RotateCcw class="size-3.5" />
					Retry
				</Button>
			{:else if job.status === 'pending' || job.status === 'running'}
				<Button variant="ghost" size="sm" onclick={() => jobsApi.cancelJob(job.id).then(refresh)}>
					<X class="size-3.5" />
					Cancel
				</Button>
			{/if}
		</div>
	</div>
{/snippet}

<section>
	<h2 class="mb-3 text-sm font-semibold text-muted">Job queue</h2>
	<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
		{#if groups.length === 0}
			<EmptyState title="No jobs yet" message="Library scans and file analysis show up here." />
		{:else}
			<ul class="divide-y divide-edge/50">
				{#each groups as group (group.key)}
					<li>
						<div class="flex items-baseline justify-between gap-3 px-4 pt-3 pb-1">
							<p class="min-w-0 truncate text-sm font-semibold" title={group.label ?? undefined}>
								{group.label ?? 'System'}
							</p>
							<span class="shrink-0 text-[11px] text-faint">
								{#if group.jobs.length > 1}{group.jobs.length} jobs ·
								{/if}{groupSummary(group)}
							</span>
						</div>
						<div class="divide-y divide-edge/30">
							{#each group.jobs as job (job.id)}
								{@render jobRow(job)}
							{/each}
						</div>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>
