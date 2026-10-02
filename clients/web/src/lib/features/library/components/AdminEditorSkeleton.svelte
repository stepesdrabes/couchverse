<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	const rows = [0, 1, 2, 3, 4];
</script>

<div class="mx-auto max-w-5xl px-6 py-10">
	<Skeleton class="h-8 w-32 rounded-full" />
	<div class="mt-8 flex flex-col gap-6 sm:flex-row">
		<Skeleton class="h-64 w-44 shrink-0 rounded-card" />
		<div class="flex-1 space-y-3">
			<Skeleton class="h-8 w-2/3" />
			<Skeleton class="h-4 w-40" />
			<Skeleton class="h-20 w-full max-w-xl" />
			<div class="flex gap-2 pt-2">
				<Skeleton class="h-9 w-24 rounded-full" />
				<Skeleton class="h-9 w-24 rounded-full" />
			</div>
		</div>
	</div>
	<div class="mt-10 space-y-2">
		{#each rows as row (row)}
			<Skeleton class="h-14 w-full rounded-card" />
		{/each}
	</div>
</div>
