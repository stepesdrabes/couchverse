<script lang="ts">
	import * as jobsApi from '$lib/features/jobs/api';
	import type { StorageInfo } from '$lib/features/jobs/api';
	import { formatBytes } from '$lib/utils/format';

	let storage = $state<StorageInfo | null>(null);

	$effect(() => {
		jobsApi.getStorage().then((s) => (storage = s));
	});

	const usedPct = $derived(
		storage?.disk.total ? ((storage.disk.used ?? 0) / storage.disk.total) * 100 : 0
	);
</script>

{#if storage?.disk.total}
	<div class="rounded-card border border-edge bg-surface p-3">
		<div class="mb-2 flex items-center justify-between text-xs">
			<span class="font-medium text-muted">Storage</span>
			<span class="text-faint tnum">
				{formatBytes(storage.disk.used ?? 0)} / {formatBytes(storage.disk.total)}
			</span>
		</div>
		<div class="h-1.5 overflow-hidden rounded-full bg-surface-2">
			<div
				class="h-full rounded-full {usedPct > 90 ? 'bg-danger' : 'bg-accent'}"
				style="width: {usedPct}%"
			></div>
		</div>
	</div>
{/if}
