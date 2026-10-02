<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	// visually 2-1-3, mirroring the real podium so nothing jumps
	const PLINTHS = [
		{ order: 'order-2', height: 'h-28', avatar: 'size-16' },
		{ order: 'order-1', height: 'h-20', avatar: 'size-14' },
		{ order: 'order-3', height: 'h-16', avatar: 'size-14' }
	];
</script>

<div class="mb-10 flex items-end justify-center gap-4 sm:gap-8">
	{#each PLINTHS as plinth, i (i)}
		<div class="flex w-24 flex-col items-center sm:w-32 {plinth.order}">
			<Skeleton class="{plinth.avatar} rounded-2xl" />
			<Skeleton class="mt-2 h-3 w-16" />
			<Skeleton class="mt-3 w-full rounded-t-xl {plinth.height}" />
		</div>
	{/each}
</div>

<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
	{#each { length: 8 }}
		<div class="flex items-center gap-3 border-b border-edge/50 px-4 py-3 last:border-0">
			<Skeleton class="h-3 w-4" />
			<Skeleton class="size-8 rounded-lg" />
			<Skeleton class="h-4 w-40" />
			<Skeleton class="ml-auto h-4 w-16" />
		</div>
	{/each}
</div>
