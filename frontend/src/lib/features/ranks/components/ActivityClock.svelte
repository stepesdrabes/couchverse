<script lang="ts">
	import { formatUptime } from '$lib/utils/format';
	import { XP_SOURCE_COLORS } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { HourBucket } from '../types';

	// A 24-slice radial histogram of when this person actually watches, built from
	// the hour-of-day rollup. Midnight is at the top and the day runs clockwise,
	// so the shape reads like a clock face.
	let { hours }: { hours: HourBucket[] } = $props();

	const OUTER = 46;
	const INNER = 16;
	const GAP_DEG = 2;

	const peak = $derived(Math.max(1, ...hours.map((h) => h.videoSeconds + h.musicSeconds)));
	const total = $derived(hours.reduce((sum, h) => sum + h.videoSeconds + h.musicSeconds, 0));

	const polar = (radius: number, degrees: number) => {
		const rad = ((degrees - 90) * Math.PI) / 180;
		return [50 + radius * Math.cos(rad), 50 + radius * Math.sin(rad)];
	};

	/** an annulus wedge for one hour, scaled by that hour's share of the peak */
	function wedge(hour: number, fraction: number, from = 0) {
		const start = hour * 15 + GAP_DEG / 2;
		const end = (hour + 1) * 15 - GAP_DEG / 2;
		const span = OUTER - INNER;
		const r0 = INNER + span * from;
		const r1 = INNER + span * (from + fraction);
		const [ax, ay] = polar(r1, start);
		const [bx, by] = polar(r1, end);
		const [cx, cy] = polar(r0, end);
		const [dx, dy] = polar(r0, start);
		return `M${ax} ${ay} A${r1} ${r1} 0 0 1 ${bx} ${by} L${cx} ${cy} A${r0} ${r0} 0 0 0 ${dx} ${dy} Z`;
	}

	const slices = $derived(
		hours.map((h) => {
			const video = h.videoSeconds / peak;
			const music = h.musicSeconds / peak;
			return { ...h, video, music, any: h.videoSeconds + h.musicSeconds > 0 };
		})
	);

	let hover = $state<HourBucket | null>(null);

	function onMove(event: PointerEvent) {
		const target = (event.target as Element)?.closest?.('[data-hour]') as HTMLElement | null;
		hover = target ? (hours[Number(target.dataset.hour)] ?? null) : null;
	}

	const summary = $derived(`${m.profiles_clock_heading()}: ${formatUptime(total)}`);
</script>

<div class="rounded-card border border-edge bg-surface/40 p-6">
	<h2 class="mb-3 text-sm font-semibold text-muted">{m.profiles_clock_heading()}</h2>

	{#if total === 0}
		<p class="py-8 text-center text-xs text-faint">{m.profiles_clock_empty()}</p>
	{:else}
		<div class="relative mx-auto max-w-[15rem]">
			<svg
				viewBox="0 0 100 100"
				class="size-full overflow-visible"
				role="img"
				aria-label={summary}
				onpointermove={onMove}
				onpointerleave={() => (hover = null)}
			>
				<circle
					cx="50"
					cy="50"
					r={OUTER}
					fill="none"
					stroke="var(--color-edge)"
					stroke-width="0.5"
				/>
				{#each slices as slice (slice.hour)}
					<!-- a full-height invisible wedge is the hover target, so a quiet
					     hour is still readable rather than a 1px sliver; one delegated
					     listener on the svg reads the hour off it -->
					<path d={wedge(slice.hour, 1)} fill="transparent" data-hour={slice.hour} />
					{#if slice.video > 0}
						<path
							d={wedge(slice.hour, slice.video)}
							fill={XP_SOURCE_COLORS.video}
							class="pointer-events-none"
						/>
					{/if}
					{#if slice.music > 0}
						<path
							d={wedge(slice.hour, slice.music, slice.video)}
							fill={XP_SOURCE_COLORS.music}
							class="pointer-events-none"
						/>
					{/if}
				{/each}
				{#each [0, 6, 12, 18] as mark (mark)}
					{@const [x, y] = polar(OUTER + 4.5, mark * 15 + 7.5)}
					<text
						{x}
						{y}
						text-anchor="middle"
						dominant-baseline="middle"
						class="fill-[var(--color-faint)] text-[5px]"
					>
						{mark}
					</text>
				{/each}
			</svg>

			{#if hover}
				<div
					class="pointer-events-none absolute inset-0 flex flex-col items-center justify-center
						text-center"
				>
					<span class="text-sm font-bold tnum">{m.profiles_clock_hour({ hour: hover.hour })}</span>
					<span class="text-[11px] text-faint tnum">
						{formatUptime(hover.videoSeconds + hover.musicSeconds)}
					</span>
				</div>
			{/if}
		</div>
	{/if}
</div>
