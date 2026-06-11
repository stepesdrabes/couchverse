<script lang="ts">
	import type { Job } from '$lib/features/jobs/api';
	import * as jobsApi from '$lib/features/jobs/api';
	import { jobAction } from '$lib/features/jobs/job-label';
	import { formatYearDate } from '$lib/utils/format';

	// `active` is a write-only bindable consumed by the parent
	// eslint-disable-next-line no-useless-assignment
	let { mediaFileId, active = $bindable(false) }: { mediaFileId: string; active?: boolean } =
		$props();

	let jobs = $state<Job[]>([]);
	let loaded = $state(false);

	const statusColor: Record<Job['status'], string> = {
		pending: 'text-muted',
		running: 'text-accent',
		done: 'text-success',
		failed: 'text-danger',
		cancelled: 'text-faint'
	};

	$effect(() => {
		const id = mediaFileId;
		loaded = false;
		async function refresh() {
			if (document.visibilityState === 'hidden') return;
			try {
				jobs = await jobsApi.listJobs({ mediaFileId: id, limit: 8 });
				active = jobs.some((j) => j.status === 'pending' || j.status === 'running');
				loaded = true;
			} catch {
				// transient; the next tick retries
			}
		}
		refresh();
		const t = setInterval(refresh, 3000);
		return () => clearInterval(t);
	});
</script>

{#if loaded && jobs.length === 0}
	<p class="text-xs text-faint">No jobs for this file yet.</p>
{:else if jobs.length > 0}
	<ul class="space-y-1.5">
		{#each jobs as job (job.id)}
			<li class="text-xs">
				<div class="flex items-center gap-2">
					<span class="min-w-0 flex-1 truncate font-medium">{jobAction(job)}</span>
					<span class="shrink-0 font-semibold capitalize {statusColor[job.status]}">
						{job.status}
					</span>
					<span class="shrink-0 text-faint tnum">{formatYearDate(job.createdAt)}</span>
				</div>
				{#if job.status === 'running' || job.status === 'pending'}
					<div class="mt-1 h-1 overflow-hidden rounded-full bg-surface-2">
						<div
							class="h-full rounded-full bg-accent transition-all duration-500"
							style="width: {job.progress}%"
						></div>
					</div>
				{:else if job.lastError && job.status === 'failed'}
					<p class="mt-0.5 truncate text-[11px] text-danger" title={job.lastError}>
						{job.lastError}
					</p>
				{/if}
			</li>
		{/each}
	</ul>
{/if}
