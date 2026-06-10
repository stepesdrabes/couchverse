<script lang="ts">
	// Rolling area+line sparkline. `values` are plotted left→right; `max` fixes
	// the vertical scale (e.g. 100 for a percentage) so the line doesn't rescale
	// on every tick.
	let {
		values,
		max,
		class: className = '',
		color = 'var(--color-accent)'
	}: {
		values: number[];
		max: number;
		class?: string;
		color?: string;
	} = $props();

	const W = 100;
	const H = 32;

	const points = $derived.by(() => {
		if (values.length < 2) return null;
		const step = W / (values.length - 1);
		const y = (v: number) => H - Math.max(0, Math.min(1, v / max)) * H;
		const line = values.map((v, i) => `${(i * step).toFixed(2)},${y(v).toFixed(2)}`).join(' ');
		const area = `0,${H} ${line} ${W},${H}`;
		return { line, area };
	});
</script>

<svg
	class={className}
	viewBox="0 0 {W} {H}"
	preserveAspectRatio="none"
	role="img"
	aria-label="trend"
>
	{#if points}
		<polygon points={points.area} fill={color} opacity="0.12" />
		<polyline
			points={points.line}
			fill="none"
			stroke={color}
			stroke-width="1.5"
			vector-effect="non-scaling-stroke"
			stroke-linejoin="round"
		/>
	{/if}
</svg>
