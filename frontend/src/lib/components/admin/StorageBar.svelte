<script lang="ts">
	import type { StorageInfo } from '$lib/features/jobs/api';
	import { categoryStyle } from './storageColors';

	// Segments are sized against `budget` (Couchverse usage + free), so the
	// leftover track shows the free space available to Couchverse.
	let { storage, class: cls = 'h-1.5' }: { storage: StorageInfo; class?: string } = $props();

	const pct = (bytes: number) => (storage.budget > 0 ? (bytes / storage.budget) * 100 : 0);
</script>

<div class="flex w-full overflow-hidden rounded-full bg-surface-2 {cls}">
	{#each storage.categories as cat (cat.kind)}
		{#if cat.bytes > 0}
			<div
				class="h-full shrink-0 transition-all duration-500 first:rounded-l-full"
				style="width: {pct(cat.bytes)}%; background: {categoryStyle[cat.kind].color}"
				title="{categoryStyle[cat.kind].label}: {cat.bytes} bytes"
			></div>
		{/if}
	{/each}
</div>
