<script lang="ts">
	import { Clock } from 'lucide-svelte';
	import { onMount, tick } from 'svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import type { CouchTvGuideView, CouchTvProgram } from '$lib/generated/core';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import {
		clock,
		dayName,
		episodeLabel,
		fillBadge,
		guideHours,
		guideOffset,
		guideRows,
		HOUR_REM
	} from '../channel';
	import ProgramSheet from './ProgramSheet.svelte';

	// The week as a timeline grid: a row per broadcast day, its programs placed by time, the
	// ones that aired dimmed and a line at now. It scrolls sideways; the day names stay put.
	let { view }: { view: CouchTvGuideView } = $props();

	const lang = $derived(currentLang());
	const rows = $derived(guideRows(view.days));
	const hours = $derived(guideHours(rows));
	const ticks = $derived(Array.from({ length: hours }, (_, i) => i));

	// the guide's own clock: a now-line and dimming need no more than the minute
	let now = $state(Date.now());
	onMount(() => {
		const timer = setInterval(() => (now = Date.now()), 30_000);
		return () => clearInterval(timer);
	});

	let scroller = $state<HTMLDivElement>();
	let selected = $state<CouchTvProgram>();
	let selectedDay = $state('');

	/** Scrolls today's row to a little before now. */
	function toNow(behavior: ScrollBehavior) {
		const today = rows[0];
		const at = today && guideOffset(today, now);
		if (!scroller || at === undefined) return;
		const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
		scroller.scrollTo({ left: Math.max(0, (at - HOUR_REM / 2) * rem), behavior });
	}

	onMount(async () => {
		await tick();
		toNow('instant');
	});

	function open(program: CouchTvProgram, day: string) {
		selected = program;
		selectedDay = day;
	}
</script>

{#if rows.every((r) => r.programs.length === 0)}
	<EmptyState title={m.couchtv_nothing_planned()} />
{:else}
	<div class="mb-3 flex justify-end">
		<button
			class="inline-flex h-8 items-center gap-1.5 rounded-full border border-edge bg-surface/60 px-3.5
				text-xs font-semibold text-muted transition-colors hover:border-faint hover:text-text"
			onclick={() => toNow('smooth')}
		>
			<Clock class="size-3.5" />
			{m.couchtv_back_to_now()}
		</button>
	</div>

	<div
		bind:this={scroller}
		class="relative overflow-x-auto rounded-card border border-edge bg-surface/40 scrollbar-none"
		role="region"
		aria-label={m.couchtv_guide_days()}
	>
		<div
			class="timeline"
			style="--hour: {HOUR_REM}rem; width: calc(var(--day) + {hours} * var(--hour))"
		>
			<!-- the hours, as today's run (a day with a clock change shifts after it) -->
			<div class="flex h-9 border-b border-edge" aria-hidden="true">
				<div class="day-name sticky left-0 z-20 shrink-0 bg-surface"></div>
				<div class="relative flex-1">
					{#each ticks as hour (hour)}
						<span
							class="absolute top-0 flex h-full items-center border-l border-edge/60 pl-2 text-[11px]
								font-semibold text-muted tnum"
							style="left: calc({hour} * var(--hour))"
						>
							{clock(rows[0].startsAtMs + hour * 3_600_000, lang)}
						</span>
					{/each}
				</div>
			</div>

			{#each rows as row, i (row.date)}
				{@const name = dayName(row.date, i, lang)}
				{@const at = guideOffset(row, now)}
				<div
					class="flex h-20 border-b border-edge/60 last:border-b-0"
					role="group"
					aria-label={name}
				>
					<div
						class="day-name sticky left-0 z-20 flex shrink-0 items-center border-r border-edge bg-surface
							px-3 text-sm font-semibold"
					>
						{name}
					</div>
					<div class="relative flex-1">
						{#each row.programs as { program, left, width } (program.id)}
							{@const onNow = program.startsAtMs <= now && now < program.endsAtMs}
							{@const aired = program.endsAtMs <= now}
							{@const badge = fillBadge(program.fill)}
							<button
								class="program group absolute inset-y-1.5 overflow-hidden rounded-lg border px-2 py-1.5
									text-left transition-colors focus-visible:z-10 focus-visible:ring-2
									focus-visible:ring-accent
									{onNow
									? 'border-accent bg-accent-soft hover:border-accent-ink'
									: 'border-edge bg-surface-2 hover:border-faint'}"
								class:opacity-50={aired}
								class:tiny={width < 1}
								style="left: {left}rem; width: {width}rem"
								aria-current={onNow ? 'time' : undefined}
								data-tv-autofocus={onNow || undefined}
								onclick={() => open(program, name)}
							>
								<span class="block truncate text-xs font-semibold">{program.title}</span>
								<span class="block truncate text-[11px] text-muted">
									<span class="tnum">{clock(program.startsAtMs, lang)}</span>
									{#if badge}
										<span class="font-semibold text-accent-ink">{badge}</span>
									{:else if program.episode}
										<span>{episodeLabel(program.episode)}</span>
									{/if}
								</span>
							</button>
						{/each}
						{#if at !== undefined}
							<div
								class="pointer-events-none absolute inset-y-0 z-10 w-0.5 bg-accent"
								style="left: {at}rem"
								aria-hidden="true"
							></div>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	</div>

	<ProgramSheet bind:program={selected} day={selectedDay} {now} />
{/if}

<style>
	.timeline {
		--day: 7rem;
	}

	.day-name {
		width: var(--day);
	}

	/* a program too short to read keeps its place on the line, without padding to widen it */
	.tiny {
		padding: 0;
		border-width: 0;
	}
</style>
