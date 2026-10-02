<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	let { count = 12 }: { count?: number } = $props();
	const cells = $derived([...Array(count).keys()]);
</script>

<div class="grid grid-cols-2 gap-x-4 gap-y-8 sm:grid-cols-3 md:grid-cols-4 xl:grid-cols-6">
	{#each cells as cell (cell)}
		<div>
			<Skeleton class="aspect-[2/3] w-full rounded-xl" />
			<Skeleton class="mt-2 h-4 w-3/4" />
			<Skeleton class="mt-1.5 h-3 w-1/2" />
		</div>
	{/each}
</div>
