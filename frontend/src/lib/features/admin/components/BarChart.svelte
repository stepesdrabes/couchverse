<script module lang="ts">
	export interface BarSegment {
		name: string;
		value: number;
		color: string;
	}
	export interface Bar {
		label: string;
		segments: BarSegment[];
	}
</script>

<script lang="ts">
	let {
		bars,
		format = (v: number) => String(v),
		class: className = ''
	}: {
		bars: Bar[];
		format?: (v: number) => string;
		class?: string;
	} = $props();

	let hover = $state<number | null>(null);

	const W = 100;
	const H = 40;
	const total = (bar: Bar) => bar.segments.reduce((sum, s) => sum + s.value, 0);
	const max = $derived(Math.max(1, ...bars.map(total)));
	const step = $derived(W / Math.max(1, bars.length));

	// stack segments bottom-up: [x, y, width, height] per segment
	const rects = $derived(
		bars.map((bar, i) => {
			let top = H;
			return bar.segments.map((seg) => {
				const h = (seg.value / max) * (H - 1);
				top -= h;
				return { x: i * step + step * 0.15, y: top, w: step * 0.7, h, color: seg.color };
			});
		})
	);

	// keep the tooltip away from the card edges
	const tooltipLeft = $derived(
		hover === null ? 0 : Math.min(92, Math.max(8, ((hover + 0.5) / bars.length) * 100))
	);
</script>

<div class="relative {className}">
	<svg viewBox="0 0 {W} {H}" preserveAspectRatio="none" class="size-full" role="img">
		{#each rects as bar, i (i)}
			{#each bar as rect, j (j)}
				{#if rect.h > 0}
					<rect x={rect.x} y={rect.y} width={rect.w} height={rect.h} fill={rect.color} />
				{/if}
			{/each}
			<rect
				role="presentation"
				x={i * step}
				y="0"
				width={step}
				height={H}
				fill="transparent"
				onpointerenter={() => (hover = i)}
				onpointerleave={() => (hover = null)}
			/>
		{/each}
	</svg>
	{#if hover !== null && bars[hover]}
		<div
			class="pointer-events-none absolute top-0 -translate-x-1/2 -translate-y-full rounded-input
				border border-edge bg-surface-2 px-2.5 py-1.5 text-[11px] whitespace-nowrap shadow-xl"
			style="left: {tooltipLeft}%"
		>
			<p class="font-semibold">{bars[hover].label}</p>
			{#each bars[hover].segments as seg (seg.name)}
				<p class="flex items-center gap-1.5 text-muted">
					<span class="size-1.5 rounded-full" style="background: {seg.color}"></span>
					{seg.name}
					<span class="tnum">{format(seg.value)}</span>
				</p>
			{/each}
		</div>
	{/if}
</div>
