<script lang="ts">
	import * as jobsApi from '$lib/features/jobs/api';
	import type { StorageInfo } from '$lib/features/jobs/api';
	import StorageBar from './StorageBar.svelte';
	import { formatBytes } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	let storage = $state<StorageInfo | null>(null);

	$effect(() => {
		jobsApi.getStorage().then((s) => (storage = s));
	});
</script>

{#if storage && storage.diskTotal > 0}
	<div class="rounded-card border border-edge bg-surface p-3">
		<div class="mb-2 flex items-center justify-between text-xs">
			<span class="font-medium text-muted">{m.admin_storage()}</span>
			<span class="text-faint tnum">
				{formatBytes(storage.used)} / {formatBytes(storage.budget)}
			</span>
		</div>
		<StorageBar {storage} />
	</div>
{/if}
