<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	const rows = [0, 1, 2, 3, 4, 5, 6];
</script>

<div class="mx-auto max-w-4xl px-6 pt-28 pb-16">
	<div class="mb-8 flex flex-col items-start gap-6 sm:flex-row sm:items-end">
		<Skeleton class="size-44 shrink-0 rounded-card" />
		<div class="min-w-0 flex-1 space-y-3">
			<Skeleton class="h-3 w-16" />
			<Skeleton class="h-9 w-64 max-w-full sm:h-10" />
			<Skeleton class="h-4 w-48" />
			<div class="flex gap-2 pt-2">
				<Skeleton class="h-10 w-28 rounded-full" />
				<Skeleton class="h-10 w-28 rounded-full" />
			</div>
		</div>
	</div>

	<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
		{#each rows as row (row)}
			<div class="flex items-center gap-4 border-b border-edge/40 px-4 py-3 last:border-0">
				<Skeleton class="size-4 shrink-0 rounded" />
				<div class="min-w-0 flex-1 space-y-1.5">
					<Skeleton class="h-3.5 w-1/2" />
					<Skeleton class="h-3 w-1/3" />
				</div>
				<Skeleton class="h-3 w-8 shrink-0" />
			</div>
		{/each}
	</div>
</div>
