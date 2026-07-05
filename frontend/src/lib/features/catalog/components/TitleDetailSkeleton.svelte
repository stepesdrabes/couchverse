<script lang="ts">
	import Skeleton from '$lib/components/ui/Skeleton.svelte';

	// mirrors TitleDetailContent's layout so the cold-load placeholder does not shift
	const rows = [0, 1, 2, 3];
</script>

<div class="relative">
	<div class="absolute inset-x-0 top-0 h-[480px] overflow-hidden">
		<div class="size-full bg-gradient-to-br from-surface-2/50 via-bg to-bg"></div>
		<div class="absolute inset-0 bg-gradient-to-t from-bg via-bg/60 to-bg/30"></div>
	</div>

	<div class="relative mx-auto max-w-5xl px-6 pt-36 pb-16">
		<div class="flex flex-col gap-8 md:flex-row">
			<Skeleton class="hidden h-64 w-44 shrink-0 rounded-card md:block" />

			<div class="min-w-0 flex-1">
				<Skeleton class="h-9 w-2/3 md:h-12" />
				<div class="mt-4 flex gap-2">
					<Skeleton class="h-4 w-12" />
					<Skeleton class="h-4 w-16" />
					<Skeleton class="h-4 w-10" />
				</div>
				<div class="mt-5 max-w-2xl space-y-2.5">
					<Skeleton class="h-3.5 w-full" />
					<Skeleton class="h-3.5 w-11/12" />
					<Skeleton class="h-3.5 w-4/5" />
				</div>
				<div class="mt-6 flex gap-3">
					<Skeleton class="h-11 w-36 rounded-full" />
					<Skeleton class="h-11 w-32 rounded-full" />
				</div>
			</div>
		</div>

		<section class="mt-12">
			<Skeleton class="h-3 w-20" />
			<ul class="mt-4 space-y-2">
				{#each rows as row (row)}
					<li class="flex gap-4 rounded-card border border-edge bg-surface/40 p-3">
						<Skeleton class="aspect-video w-32 shrink-0 rounded-lg sm:w-44" />
						<div class="min-w-0 flex-1 space-y-2 py-1">
							<Skeleton class="h-4 w-1/2" />
							<Skeleton class="h-3 w-full" />
							<Skeleton class="h-3 w-2/3" />
						</div>
					</li>
				{/each}
			</ul>
		</section>
	</div>
</div>
