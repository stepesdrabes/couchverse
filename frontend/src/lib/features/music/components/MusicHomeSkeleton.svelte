<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	const cards = [0, 1, 2, 3, 4, 5];
	const rows = [0, 1];
</script>

<div class="pt-24 pb-16">
	<div class="mb-8 flex items-center justify-between px-6 lg:px-12">
		<Skeleton class="h-7 w-28" />
		<Skeleton class="h-8 w-32 rounded-full" />
	</div>
	<div class="space-y-10">
		{#each rows as row (row)}
			<section>
				<div class="mb-3 px-6 lg:px-12"><Skeleton class="h-3 w-32" /></div>
				<div class="flex gap-4 overflow-hidden px-6 lg:px-12">
					{#each cards as card (card)}
						<div class="w-40 shrink-0 sm:w-44">
							<Skeleton class="aspect-square w-full rounded-xl" />
							<Skeleton class="mt-2 h-4 w-3/4" />
							<Skeleton class="mt-1.5 h-3 w-1/2" />
						</div>
					{/each}
				</div>
			</section>
		{/each}
	</div>
</div>
