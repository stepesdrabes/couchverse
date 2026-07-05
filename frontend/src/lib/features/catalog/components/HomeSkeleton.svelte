<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	// mirrors HomeContent: a hero band then a couple of card rows
	const cards = [0, 1, 2, 3, 4, 5];
	const rows = [0, 1];
</script>

<div
	class="relative flex min-h-[72vh] items-center justify-center overflow-hidden px-6 md:min-h-[82vh]"
>
	<div
		class="pointer-events-none absolute inset-0 bg-gradient-to-br from-surface-2/40 via-bg to-bg"
	></div>
	<div class="relative flex flex-col items-center gap-5">
		<Skeleton class="h-3 w-40" />
		<Skeleton class="h-12 w-80 max-w-[80vw] md:h-16 md:w-[28rem]" />
		<Skeleton class="h-4 w-72 max-w-[70vw]" />
		<div class="mt-2 flex gap-3">
			<Skeleton class="h-11 w-32 rounded-full" />
			<Skeleton class="h-11 w-32 rounded-full" />
		</div>
	</div>
</div>

<div class="relative z-10 -mt-10 space-y-10 pb-16">
	{#each rows as row (row)}
		<section>
			<div class="mb-3 px-6 lg:px-12"><Skeleton class="h-3 w-32" /></div>
			<div class="flex gap-4 overflow-hidden px-6 lg:px-12">
				{#each cards as card (card)}
					<div class="w-48 shrink-0 sm:w-56">
						<Skeleton class="aspect-video w-full rounded-xl" />
						<Skeleton class="mt-2 h-4 w-3/4" />
						<Skeleton class="mt-1.5 h-3 w-1/2" />
					</div>
				{/each}
			</div>
		</section>
	{/each}
</div>
