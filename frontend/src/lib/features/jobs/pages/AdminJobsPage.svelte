<script lang="ts">
	import { RotateCcw, X } from 'lucide-svelte';
	import * as jobsApi from '$lib/features/jobs/api';
	import type { Job } from '$lib/features/jobs/api';
	import { jobAction, jobSubjectLabel } from '$lib/features/jobs/job-label';
	import Button from '$lib/components/ui/Button.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import * as m from '$lib/paraglide/messages';

	let jobs = $state<Job[]>([]);

	async function refresh() {
		jobs = await jobsApi.listJobs();
	}

	$effect(() => {
		refresh();
		const t = setInterval(refresh, 3000);
		return () => clearInterval(t);
	});

	const statusColor: Record<Job['status'], string> = {
		pending: 'text-muted',
		running: 'text-accent',
		done: 'text-success',
		failed: 'text-danger',
		cancelled: 'text-faint'
	};

	const statusLabel: Record<Job['status'], () => string> = {
		pending: m.jobs_status_pending,
		running: m.jobs_status_running,
		done: m.jobs_status_done,
		failed: m.common_failed,
		cancelled: m.jobs_status_cancelled
	};

	// subject comes from the backend; older payload shapes are the fallback
	const jobSubject = (job: Job) => {
		const subject = jobSubjectLabel(job);
		if (subject) return subject;
		if (typeof job.payload.mediaFileId === 'string')
			return m.jobs_file_fallback({ id: job.payload.mediaFileId });
		return null;
	};

	// collapse jobs onto the content they belong to: all jobs for one media file
	// (e.g. the 1080p/720p/480p transcodes of an episode) become one entry
	const groupKey = (job: Job): string => {
		const s = job.subject;
		if (s?.mediaFileId) return `mf:${s.mediaFileId}`;
		if (s?.titleId) return `title:${s.titleId}`;
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
			.map((s) => m.jobs_status_count({ count: counts[s], status: statusLabel[s]() }))
			.join(' · ');
	};
</script>

<svelte:head>
	<title>{m.jobs_page_title()}</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">{m.jobs_heading()}</h1>

{#snippet jobRow(job: Job)}
	<div class="flex items-center gap-3 px-4 py-2.5 text-sm">
		<div class="min-w-0 flex-1">
			<div class="flex items-center gap-2">
				<span class="truncate text-muted">{jobAction(job)}</span>
				{#if job.attempts > 1}
					<span class="text-[11px] text-faint">{m.jobs_attempt({ n: job.attempts })}</span>
				{/if}
			</div>
			{#if job.lastError && (job.status === 'failed' || job.status === 'pending')}
				<p class="mt-0.5 max-w-md truncate text-xs text-danger" title={job.lastError}>
					{job.lastError}
				</p>
			{/if}
		</div>
		<span class="w-20 shrink-0 text-xs font-semibold capitalize {statusColor[job.status]}">
			{statusLabel[job.status]()}
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
					{m.common_retry()}
				</Button>
			{:else if job.status === 'pending' || job.status === 'running'}
				<Button variant="ghost" size="sm" onclick={() => jobsApi.cancelJob(job.id).then(refresh)}>
					<X class="size-3.5" />
					{m.common_cancel()}
				</Button>
			{/if}
		</div>
	</div>
{/snippet}

<section>
	<h2 class="mb-3 text-sm font-semibold text-muted">{m.jobs_queue()}</h2>
	<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
		{#if groups.length === 0}
			<EmptyState title={m.jobs_empty_title()} message={m.jobs_empty_message()} />
		{:else}
			<ul class="divide-y divide-edge/50">
				{#each groups as group (group.key)}
					<li>
						<div class="flex items-baseline justify-between gap-3 px-4 pt-3 pb-1">
							<p class="min-w-0 truncate text-sm font-semibold" title={group.label ?? undefined}>
								{group.label ?? m.jobs_system()}
							</p>
							<span class="shrink-0 text-[11px] text-faint">
								{#if group.jobs.length > 1}{m.jobs_job_count({ count: group.jobs.length })} ·
								{/if}{groupSummary(group)}
							</span>
						</div>
						<div class="pb-1">
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
