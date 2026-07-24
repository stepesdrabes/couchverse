<script lang="ts">
	import { onMount } from 'svelte';
	import { cubicOut } from 'svelte/easing';
	import { formatUptime, formatYearDate } from '$lib/utils/format';
	import { HEAT_FILLS, heatLevel, heatThresholds } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { Activity } from '../types';

	let { activity }: { activity: Activity } = $props();

	const CELL = 10; // px, plus a 3px gap = the 13px pitch the month labels assume
	const PITCH = 13;

	const thresholds = $derived(heatThresholds(activity.days));

	const DAY_MS = 86_400_000;

	const cells = $derived.by(() => {
		// day offsets in ms rather than mutating a Date: the run is short enough
		// that a DST shift cannot walk it onto the wrong day
		const start = new Date(activity.from + 'T12:00:00').getTime();
		return activity.days.map((seconds, i) => ({
			seconds,
			date: new Date(start + i * DAY_MS),
			level: heatLevel(seconds, thresholds)
		}));
	});

	// The grid is column-major (a column is a week), so a month label sits above
	// the column where that month first appears.
	const columns = $derived(Math.ceil(cells.length / 7));
	const monthLabels = $derived.by(() => {
		const out: { col: number; label: string }[] = [];
		let lastMonth = -1;
		for (let col = 0; col < columns; col++) {
			const cell = cells[col * 7];
			if (!cell) break;
			const month = cell.date.getMonth();
			if (month !== lastMonth) {
				lastMonth = month;
				out.push({
					col,
					label: cell.date.toLocaleDateString(undefined, { month: 'short' })
				});
			}
		}
		return out;
	});

	const totalSeconds = $derived(activity.days.reduce((sum, s) => sum + s, 0));
	const activeDays = $derived(activity.days.filter((s) => s > 0).length);
	const summary = $derived(
		m.profiles_activity_summary({
			hours: Math.round(totalSeconds / 3600),
			days: activeDays
		})
	);

	// A year of weeks is wider than the card, so start scrolled to today: the
	// recent end is the part anyone actually looks at.
	let scroller = $state<HTMLDivElement>();
	onMount(() => {
		if (scroller) scroller.scrollLeft = scroller.scrollWidth;
	});

	// One delegated listener and one shared tooltip: 371 bits-ui providers or 371
	// listeners would be a real cost on a Pi, and this reads identically.
	let hover = $state<{ x: number; text: string } | null>(null);

	function onMove(event: PointerEvent) {
		const target = (event.target as HTMLElement)?.closest<HTMLElement>('[data-i]');
		if (!target) {
			hover = null;
			return;
		}
		const cell = cells[Number(target.dataset.i)];
		if (!cell) return;
		const date = formatYearDate(cell.date.toISOString());
		hover = {
			x: (Number(target.dataset.col) * PITCH + CELL / 2) / (columns * PITCH),
			text:
				cell.seconds > 0
					? m.profiles_activity_cell({ value: formatUptime(cell.seconds), date })
					: m.profiles_activity_cell_none({ date })
		};
	}

	// A single mask wipe instead of 371 staggered cells: one composited element
	// rather than a few hundred. Being a css transition, the global
	// prefers-reduced-motion clamp collapses it automatically.
	function wipeX(_node: Element, { duration = 750 } = {}) {
		return {
			duration,
			easing: cubicOut,
			css: (t: number) =>
				`-webkit-mask-image: linear-gradient(90deg, #000 0 ${t * 130}%, transparent ${t * 130}%);
				 mask-image: linear-gradient(90deg, #000 0 ${t * 130}%, transparent ${t * 130}%);`
		};
	}
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<div class="mb-3 flex flex-wrap items-baseline justify-between gap-2">
		<h2 class="text-sm font-semibold text-muted">{m.profiles_activity_heading()}</h2>
		<div class="flex items-center gap-1.5 text-[11px] text-faint">
			<span>{m.profiles_activity_less()}</span>
			{#each HEAT_FILLS as fill, i (i)}
				<span class="size-2.5 rounded-[3px]" style="background: {fill}"></span>
			{/each}
			<span>{m.profiles_activity_more()}</span>
		</div>
	</div>

	<div bind:this={scroller} class="relative overflow-x-auto scrollbar-none">
		<div class="relative h-4" style="width: {columns * PITCH}px">
			{#each monthLabels as label (label.col)}
				<span class="absolute text-[10px] text-faint" style="left: {label.col * PITCH}px">
					{label.label}
				</span>
			{/each}
		</div>

		<div
			class="grid grid-flow-col grid-rows-7 gap-[3px]"
			style="width: {columns * PITCH}px"
			role="img"
			aria-label={summary}
			onpointermove={onMove}
			onpointerleave={() => (hover = null)}
			transition:wipeX
		>
			{#each cells as cell, i (i)}
				<div
					data-i={i}
					data-col={Math.floor(i / 7)}
					class="size-2.5 rounded-[3px]"
					style="background: {HEAT_FILLS[cell.level]}"
					aria-hidden="true"
				></div>
			{/each}
		</div>

		{#if hover}
			<div
				class="pointer-events-none absolute top-0 z-10 -translate-x-1/2 rounded-input border
					border-edge bg-surface-2 px-2.5 py-1.5 text-[11px] whitespace-nowrap shadow-xl"
				style="left: {Math.min(92, Math.max(8, hover.x * 100))}%"
			>
				{hover.text}
			</div>
		{/if}
	</div>

	<p class="mt-3 text-[11px] text-faint tnum">{summary}</p>
</div>
