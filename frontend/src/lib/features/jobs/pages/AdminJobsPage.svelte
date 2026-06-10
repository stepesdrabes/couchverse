<script lang="ts">
	import { FolderSearch, RefreshCw, RotateCcw, X } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as jobsApi from '$lib/features/jobs/api';
	import type { Job, Library } from '$lib/features/jobs/api';
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

	const jobLabel = (job: Job) => {
		switch (job.type) {
			case 'scan_library': {
				const lib = libraries.find((l) => l.id === job.payload.libraryId);
				return `Scan ${lib ? `“${lib.name}”` : `library #${job.payload.libraryId}`}`;
			}
			case 'probe':
				return `Analyze file #${job.payload.mediaFileId}`;
			default:
				return job.type;
		}
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

<section>
	<h2 class="mb-3 text-sm font-semibold text-muted">Job queue</h2>
	<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
		{#if jobs.length === 0}
			<EmptyState title="No jobs yet" message="Library scans and file analysis show up here." />
		{:else}
			<table class="w-full text-left text-sm">
				<thead>
					<tr class="border-b border-edge text-[11px] tracking-wider text-faint uppercase">
						<th class="px-4 py-3 font-semibold">Job</th>
						<th class="py-3 pr-4 font-semibold">Status</th>
						<th class="w-44 py-3 pr-4 font-semibold">Progress</th>
						<th class="py-3 pr-4"></th>
					</tr>
				</thead>
				<tbody>
					{#each jobs as job (job.id)}
						<tr class="border-b border-edge/50 last:border-0">
							<td class="px-4 py-3">
								<p class="font-medium">{jobLabel(job)}</p>
								{#if job.lastError && (job.status === 'failed' || job.status === 'pending')}
									<p class="mt-0.5 max-w-md truncate text-xs text-danger" title={job.lastError}>
										{job.lastError}
									</p>
								{/if}
							</td>
							<td class="py-3 pr-4">
								<span class="text-xs font-semibold capitalize {statusColor[job.status]}">
									{job.status}
									{#if job.attempts > 1}
										<span class="font-normal text-faint">(attempt {job.attempts})</span>
									{/if}
								</span>
							</td>
							<td class="py-3 pr-4">
								{#if job.status === 'running' || job.status === 'pending'}
									<div class="h-1.5 w-36 overflow-hidden rounded-full bg-surface-2">
										<div
											class="h-full rounded-full bg-accent transition-all duration-500"
											style="width: {job.progress}%"
										></div>
									</div>
								{:else}
									<span class="text-xs text-faint">-</span>
								{/if}
							</td>
							<td class="py-3 pr-4 text-right">
								{#if job.status === 'failed' || job.status === 'cancelled'}
									<Button
										variant="ghost"
										size="sm"
										onclick={() => jobsApi.retryJob(job.id).then(refresh)}
									>
										<RotateCcw class="size-3.5" />
										Retry
									</Button>
								{:else if job.status === 'pending' || job.status === 'running'}
									<Button
										variant="ghost"
										size="sm"
										onclick={() => jobsApi.cancelJob(job.id).then(refresh)}
									>
										<X class="size-3.5" />
										Cancel
									</Button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		{/if}
	</div>
</section>
