<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	// the guide's grid while it loads: the hours, then a row per day of program-sized blocks,
	// each row shifted so the days do not line up like a table
	const blocks = ['w-36', 'w-20', 'w-20', 'w-48', 'w-28', 'w-20', 'w-36'];
	const rows = [0, 1, 2, 3, 4, 5].map((row) =>
		blocks.map((_, i) => blocks[(i + row) % blocks.length])
	);
</script>

<div class="mb-3 flex justify-end">
	<Skeleton class="h-8 w-28 rounded-full" />
</div>
<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
	<div class="h-9 border-b border-edge"></div>
	{#each rows as widths, row (row)}
		<div class="flex h-20 items-center gap-2 border-b border-edge/60 px-3 last:border-b-0">
			<Skeleton class="mr-6 h-4 w-16 shrink-0" />
			{#each widths as width, i (i)}
				<Skeleton class="h-14 shrink-0 rounded-lg {width}" />
			{/each}
		</div>
	{/each}
</div>
