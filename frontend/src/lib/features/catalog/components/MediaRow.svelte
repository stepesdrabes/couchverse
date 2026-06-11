<script lang="ts">
	import { ChevronLeft, ChevronRight } from 'lucide-svelte';
	import type { Snippet } from 'svelte';

	let { label, children }: { label: string; children: Snippet } = $props();

	let scroller = $state<HTMLDivElement>();

	function scrollBy(dir: number) {
		scroller?.scrollBy({ left: dir * scroller.clientWidth * 0.85, behavior: 'smooth' });
	}
</script>

<section class="group/row">
	<div class="mb-3 flex items-center justify-between px-6 lg:px-12">
		<h2 class="eyebrow">{label}</h2>
		<div
			class="hidden gap-1 opacity-0 transition-opacity group-hover/row:opacity-100
				[@media(pointer:fine)]:flex"
		>
			<button
				class="rounded-full border border-edge bg-surface/80 p-1.5 text-muted transition-colors hover:text-text"
				onclick={() => scrollBy(-1)}
				aria-label="Scroll left"
			>
				<ChevronLeft class="size-4" />
			</button>
			<button
				class="rounded-full border border-edge bg-surface/80 p-1.5 text-muted transition-colors hover:text-text"
				onclick={() => scrollBy(1)}
				aria-label="Scroll right"
			>
				<ChevronRight class="size-4" />
			</button>
		</div>
	</div>
	<div
		bind:this={scroller}
		class="flex snap-x snap-mandatory gap-4 overflow-x-auto scroll-smooth scroll-px-6 px-6 pt-1 pb-4
			scrollbar-none lg:scroll-px-12 lg:px-12"
	>
		{@render children()}
	</div>
</section>
