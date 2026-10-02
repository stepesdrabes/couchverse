<script lang="ts" module>
	export interface Segment {
		key: string;
		value: number;
		color: string;
		label: string;
	}
</script>

<script lang="ts">
	// Segments are sized against `total` rather than their own sum, so the
	// leftover track can carry meaning of its own (free disk, xp still to earn).
	let {
		segments,
		total,
		class: cls = 'h-1.5',
		title
	}: {
		segments: Segment[];
		total: number;
		class?: string;
		// per-segment tooltip text; omitted means no title attribute at all
		title?: (segment: Segment) => string;
	} = $props();

	const pct = (value: number) => (total > 0 ? (value / total) * 100 : 0);
</script>

<div class="flex w-full overflow-hidden rounded-full bg-surface-2 {cls}">
	{#each segments as segment (segment.key)}
		{#if segment.value > 0}
			<div
				class="h-full shrink-0 transition-all duration-500 first:rounded-l-full"
				style="width: {pct(segment.value)}%; background: {segment.color}"
				title={title?.(segment)}
			></div>
		{/if}
	{/each}
</div>
