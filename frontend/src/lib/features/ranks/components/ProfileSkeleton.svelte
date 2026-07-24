<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
</script>

<!-- mirrors ProfileContent 1:1 so nothing shifts when the real payload lands -->
<div class="mx-auto max-w-5xl px-6 pt-28 pb-16">
	<div class="rounded-card border border-edge bg-surface/40 p-6 sm:p-8">
		<div class="flex flex-col items-center gap-6 sm:flex-row sm:gap-8">
			<Skeleton class="size-32 shrink-0 rounded-full" />
			<div class="w-full max-w-sm space-y-3">
				<Skeleton class="h-3 w-24" />
				<Skeleton class="h-9 w-56" />
				<Skeleton class="h-3 w-40" />
				<Skeleton class="h-2 w-full rounded-full" />
			</div>
		</div>
	</div>

	<div class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		{#each { length: 8 }}
			<Skeleton class="h-[5.5rem] rounded-card" />
		{/each}
	</div>

	<div class="mt-6 grid gap-6 lg:grid-cols-3">
		<Skeleton class="h-52 rounded-card lg:col-span-2" />
		<Skeleton class="h-52 rounded-card" />
	</div>

	<div class="mt-6 grid gap-6 lg:grid-cols-2">
		<Skeleton class="h-56 rounded-card" />
		<Skeleton class="h-56 rounded-card" />
	</div>

	<div class="mt-10">
		<Skeleton class="mb-4 h-3 w-28" />
		<div class="grid grid-cols-3 gap-x-4 gap-y-5 sm:grid-cols-5 lg:grid-cols-8">
			{#each { length: 16 }}
				<Skeleton class="size-16 rounded-full" />
			{/each}
		</div>
	</div>
</div>
