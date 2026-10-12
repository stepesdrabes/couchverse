<script lang="ts">
	import type { CouchTvWaveform } from '../api';

	// A stretch of a file's loudness with a selection dragged across it (at most max ms).
	let {
		data,
		start,
		length,
		max,
		playhead = null,
		label,
		onselect
	}: {
		data: CouchTvWaveform;
		/** the selection, in ms of the file */
		start: number;
		length: number;
		max: number;
		playhead?: number | null;
		label: string;
		onselect: (start: number, length: number) => void;
	} = $props();

	const W = 1000;
	const H = 120;
	const span = $derived(Math.max(data.toMs - data.fromMs, 1));
	const x = (ms: number) => ((ms - data.fromMs) / span) * W;
	const barW = $derived(W / Math.max(data.loudness.length, 1));
	// momentary loudness runs from -70 (silence) to 0 LUFS; the curve lifts quiet passages
	const level = (lufs: number) => Math.pow(Math.max(0, (lufs + 70) / 70), 1.6);

	let svg = $state<SVGSVGElement>();
	let anchor: number | null = null;
	let moved = false;

	function at(e: PointerEvent): number {
		const box = svg!.getBoundingClientRect();
		const f = Math.min(Math.max((e.clientX - box.left) / box.width, 0), 1);
		return Math.round(data.fromMs + f * span);
	}

	function down(e: PointerEvent) {
		svg!.setPointerCapture(e.pointerId);
		anchor = at(e);
		moved = false;
	}

	function move(e: PointerEvent) {
		if (anchor === null) return;
		const t = at(e);
		if (Math.abs(t - anchor) < span / 200) return;
		moved = true;
		const from = Math.min(anchor, t);
		const to = Math.min(Math.max(anchor, t), from + max);
		onselect(from, Math.max(to - from, 1000));
	}

	function up(e: PointerEvent) {
		if (anchor === null) return;
		// a click moves the selection there, keeping its length
		if (!moved) onselect(Math.min(at(e), data.toMs - length), length);
		anchor = null;
	}

	// the arrows move the selection a second at a time, ten with Shift
	function key(e: KeyboardEvent) {
		const step =
			(e.shiftKey ? 10_000 : 1_000) * (e.key === 'ArrowLeft' ? -1 : e.key === 'ArrowRight' ? 1 : 0);
		if (!step) return;
		e.preventDefault();
		onselect(Math.min(Math.max(start + step, data.fromMs), data.toMs - length), length);
	}
</script>

<svg
	bind:this={svg}
	viewBox="0 0 {W} {H}"
	preserveAspectRatio="none"
	class="h-28 w-full cursor-col-resize touch-none rounded-lg bg-surface select-none"
	role="slider"
	tabindex="0"
	aria-label={label}
	aria-valuemin={data.fromMs / 1000}
	aria-valuemax={data.toMs / 1000}
	aria-valuenow={Math.round(start / 1000)}
	onpointerdown={down}
	onpointermove={move}
	onpointerup={up}
	onpointercancel={() => (anchor = null)}
	onkeydown={key}
>
	<rect
		x={x(start)}
		y="0"
		width={Math.max(x(start + length) - x(start), 1)}
		height={H}
		fill="var(--color-accent)"
		opacity="0.18"
	/>
	{#each data.loudness as v, i (i)}
		{@const h = level(v) * (H - 8)}
		{@const t = data.fromMs + i * data.stepMs}
		<rect
			x={i * barW + barW * 0.15}
			y={(H - h) / 2}
			width={barW * 0.7}
			height={Math.max(h, 1)}
			fill={t >= start && t < start + length ? 'var(--color-accent-ink)' : 'var(--color-muted)'}
		/>
	{/each}
	{#if playhead !== null && playhead >= data.fromMs && playhead <= data.toMs}
		<line
			x1={x(playhead)}
			x2={x(playhead)}
			y1="0"
			y2={H}
			stroke="var(--color-text)"
			stroke-width="2"
		/>
	{/if}
</svg>
