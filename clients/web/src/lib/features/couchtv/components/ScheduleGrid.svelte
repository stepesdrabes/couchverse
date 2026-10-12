<script lang="ts">
	import { Lock, RefreshCw } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';
	import type { CouchTvAiring } from '../api';
	import { episodeCode, fillLabel, renderColor, renderLabel } from '../labels';
	import { DAY_MINUTES, nowLine, place, type DayColumn } from '../schedule';
	import { formatClock, formatDay } from '../time';

	// The week as a TV guide: a column per day, time running down from the hour the
	// broadcast day opens; breaks run in a thin lane beside the programs.
	let {
		columns,
		airings,
		axis,
		timeZone,
		lang,
		now,
		today,
		selected = null,
		swapping = false,
		onselect,
		onregenerate
	}: {
		columns: DayColumn[];
		airings: CouchTvAiring[];
		/** minutes after midnight the columns start */
		axis: number;
		timeZone: string;
		lang: string;
		now: number;
		/** the date of the broadcast day on air */
		today: string;
		selected?: string | null;
		swapping?: boolean;
		onselect: (airing: CouchTvAiring) => void;
		onregenerate: (date: string) => void;
	} = $props();

	const PX = 1.5; // per minute
	const height = DAY_MINUTES * PX;

	const hours = $derived(
		Array.from({ length: 24 }, (_, h) => {
			const minutes = (axis + h * 60) % DAY_MINUTES;
			return `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`;
		})
	);

	const laid = $derived(
		columns.map((c) => ({
			column: c,
			programs: place(c, airings, 'program'),
			breaks: place(c, airings, 'break'),
			now: nowLine(c, now)
		}))
	);

	let scroller = $state<HTMLDivElement>();
	let scrolled = false;
	// open on what is on air, with a couple of hours before it in view
	$effect(() => {
		const line = laid.find((d) => d.now !== null)?.now;
		if (!scroller || scrolled || line == null) return;
		scrolled = true;
		scroller.scrollTop = Math.max(0, (line - 120) * PX);
	});

	const span = (a: CouchTvAiring) =>
		`${formatClock(Date.parse(a.startsAt), timeZone, lang)}-${formatClock(Date.parse(a.endsAt), timeZone, lang)}`;

	function programLabel(a: CouchTvAiring) {
		const parts = [a.titleName ?? '', episodeCode(a.seasonNumber, a.episodeNumber), span(a)];
		if (a.locked) parts.push(m.admin_couchtv_locked());
		if (!a.ready) parts.push(m.admin_couchtv_not_prepared());
		return parts.filter(Boolean).join(', ');
	}
</script>

<div
	bind:this={scroller}
	class="max-h-[72vh] overflow-auto rounded-card border border-edge bg-surface/40"
	role="region"
	aria-label={m.admin_couchtv_schedule_grid()}
>
	<div
		class="grid min-w-[52rem]"
		style="grid-template-columns: 3.25rem repeat({columns.length}, minmax(6.5rem, 1fr))"
	>
		<!-- day headers -->
		<div class="sticky top-0 z-20 border-b border-edge bg-surface"></div>
		{#each laid as day (day.column.date)}
			<div
				class="sticky top-0 z-20 flex items-center justify-between gap-1 border-b border-l border-edge bg-surface px-2 py-2"
			>
				<span
					class="truncate text-xs font-semibold {day.column.date === today
						? 'text-accent-ink'
						: ''}"
				>
					{formatDay(day.column.date, lang)}
				</span>
				<button
					type="button"
					class="rounded-full p-1 text-faint transition-colors hover:bg-surface-2 hover:text-text"
					aria-label={m.admin_couchtv_regenerate_day_named({
						day: formatDay(day.column.date, lang, 'long')
					})}
					title={m.admin_couchtv_regenerate_day()}
					onclick={() => onregenerate(day.column.date)}
				>
					<RefreshCw class="size-3" />
				</button>
			</div>
		{/each}

		<!-- the hours -->
		<div class="relative" style="height: {height}px">
			{#each hours as label, h (h)}
				<span
					class="absolute right-1.5 -translate-y-1/2 text-[10px] text-faint tnum"
					style="top: {h * 60 * PX}px"
				>
					{h === 0 ? '' : label}
				</span>
			{/each}
		</div>

		{#each laid as day (day.column.date)}
			<!-- an hour line every 60 minutes -->
			<div
				role="group"
				aria-label={formatDay(day.column.date, lang, 'long')}
				class="relative border-l border-edge"
				style="height: {height}px; background-image: repeating-linear-gradient(to bottom,
					color-mix(in srgb, var(--color-edge) 40%, transparent) 0 1px, transparent 1px {60 * PX}px)"
			>
				{#each day.breaks as b (b.airing.id)}
					<div
						class="absolute left-0.5 w-1.5 rounded-full"
						style="top: {b.top * PX}px; height: {Math.max(b.height * PX, 2)}px;
							background: {renderColor[b.airing.render ?? 'uncomposed']}"
						title="{m.admin_couchtv_break()}: {renderLabel(b.airing.render)} ({span(b.airing)})"
					></div>
				{/each}

				{#each day.programs as p (p.airing.id)}
					{@const a = p.airing}
					{@const tall = p.height * PX >= 30}
					<button
						type="button"
						onclick={() => onselect(a)}
						aria-label={programLabel(a)}
						aria-pressed={selected === a.id}
						class="absolute right-1 left-2.5 overflow-hidden border px-1.5 text-left transition-colors
							{p.cutStart ? 'rounded-t-none' : 'rounded-t-md'} {p.cutEnd ? 'rounded-b-none' : 'rounded-b-md'}
							{selected === a.id
							? 'z-10 border-accent bg-accent-soft'
							: a.source === 'admin'
								? 'border-accent/60 bg-surface-2 hover:border-accent'
								: 'border-edge bg-surface-2 hover:border-faint'}
							{swapping ? 'cursor-crosshair' : ''}
							{a.state === 'aired' ? 'opacity-55' : ''}"
						style="top: {p.top * PX}px; height: {Math.max(p.height * PX - 1, 2)}px"
					>
						{#if p.height * PX >= 13}
							<span class="flex items-center gap-1 text-[11px] leading-tight font-semibold">
								<span
									class="size-1.5 shrink-0 rounded-full {a.ready
										? 'bg-success'
										: 'border border-faint'}"
								></span>
								<span class="truncate">{a.titleName}</span>
								{#if a.locked}<Lock class="size-2.5 shrink-0 text-muted" />{/if}
							</span>
						{/if}
						{#if tall}
							<span class="block truncate text-[10px] leading-tight text-muted">
								{#if a.episodeId}{episodeCode(a.seasonNumber, a.episodeNumber)}
								{/if}{fillLabel(a.fill)}
							</span>
						{/if}
					</button>
				{/each}

				{#if day.now !== null}
					<div
						class="pointer-events-none absolute inset-x-0 z-10 h-0.5 bg-accent"
						style="top: {day.now * PX}px"
						aria-hidden="true"
					>
						<span class="absolute -top-[3px] -left-1 size-2 rounded-full bg-accent"></span>
					</div>
				{/if}
			</div>
		{/each}
	</div>
</div>
