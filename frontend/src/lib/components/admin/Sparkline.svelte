<script lang="ts" module>
	let uid = 0;
</script>

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

	const gradId = `spark-${uid++}`;
	const W = 100;
	const H = 40;

	const shape = $derived.by(() => {
		if (values.length < 2) return null;
		const step = W / (values.length - 1);
		const y = (v: number) => H - Math.max(0, Math.min(1, v / max)) * (H - 2) - 1;
		const pts = values.map((v, i) => [i * step, y(v)] as const);
		const line = pts.map(([x, py]) => `${x.toFixed(2)},${py.toFixed(2)}`).join(' ');
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
	<defs>
		<linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
			<stop offset="0%" stop-color={color} stop-opacity="0.35" />
			<stop offset="100%" stop-color={color} stop-opacity="0" />
		</linearGradient>
	</defs>
	{#if shape}
		<polygon points={shape.area} fill="url(#{gradId})" />
		<polyline
			points={shape.line}
			fill="none"
			stroke={color}
			stroke-width="2"
			vector-effect="non-scaling-stroke"
			stroke-linejoin="round"
			stroke-linecap="round"
		/>
	{/if}
</svg>
