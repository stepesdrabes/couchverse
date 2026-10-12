<script lang="ts">
	import { ChevronLeft, ChevronRight } from 'lucide-svelte';
	import { onMount, untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { fly } from 'svelte/transition';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { problemMessage } from '$lib/api/problem';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import {
		adminGetCouchTv,
		adminListAirings,
		adminRegenerateDay,
		adminUpdateAiring,
		type CouchTvAiring,
		type CouchTvOverview
	} from '../api';
	import AiringPanel from '../components/AiringPanel.svelte';
	import ScheduleGrid from '../components/ScheduleGrid.svelte';
	import { renderColor } from '../labels';
	import { axisStart, columnDate, columns, readiness } from '../schedule';
	import { addDays, formatClock, formatDay } from '../time';

	const DAYS = 7;
	const HOUR = 3_600_000;

	let overview = $state<CouchTvOverview | null>(null);
	let airings = $state<CouchTvAiring[]>([]);
	let loaded = $state(false);
	let failed = $state(false);
	let now = $state(Date.now());
	let offset = $state(0); // weeks from this one
	let selectedId = $state<string | null>(null);
	let swapFrom = $state<CouchTvAiring | null>(null);
	let regenerateDate = $state<string | null>(null);
	let regenerateOpen = $state(false);

	const timeZone = $derived(overview?.config.timeZone ?? 'Europe/Prague');
	const axis = $derived(axisStart(overview?.config.template.slots ?? []));
	const today = $derived(columnDate(now, axis, timeZone));
	const first = $derived(addDays(today, offset * DAYS));
	const cols = $derived(overview ? columns(first, DAYS, axis, timeZone) : []);
	const selected = $derived(airings.find((a) => a.id === selectedId) ?? null);
	const lang = $derived(currentLang());
	const ahead = $derived(readiness(airings, now, now + 48 * HOUR));

	let generation = 0;
	async function load() {
		const mine = ++generation;
		try {
			if (!overview) {
				// the config places the columns; the effect below loads the week once it is in
				overview = await adminGetCouchTv();
				return;
			}
			const range = columns(first, DAYS, axis, timeZone);
			const schedule = await adminListAirings({
				from: new Date(range[0].start).toISOString(),
				to: new Date(range[range.length - 1].end).toISOString()
			});
			if (mine !== generation) return;
			airings = schedule.airings;
			failed = false;
			loaded = true;
		} catch {
			if (mine === generation) {
				failed = true;
				loaded = true;
			}
		}
	}

	// the week shown follows the arrows (and the config once it is in)
	$effect(() => {
		void cols;
		untrack(load);
	});

	onMount(() => {
		// the now-line moves, and render states change while the sidecar works
		const clock = setInterval(() => (now = Date.now()), 30_000);
		const poll = setInterval(load, 60_000);
		return () => {
			clearInterval(clock);
			clearInterval(poll);
		};
	});

	// a new config (how far it is planned) reloads the week through the effect above
	async function refresh() {
		overview = await adminGetCouchTv();
	}

	function askRegenerate(date: string) {
		regenerateDate = date;
		regenerateOpen = true;
	}

	function select(a: CouchTvAiring) {
		if (swapFrom) {
			if (a.id !== swapFrom.id) swap(swapFrom, a);
			return;
		}
		selectedId = a.id;
	}

	async function swap(a: CouchTvAiring, b: CouchTvAiring) {
		swapFrom = null;
		try {
			await adminUpdateAiring(a.id, { action: 'swap', swapWith: b.id });
			toast.success(m.admin_couchtv_swapped());
			selectedId = a.id;
			await load();
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_edit_failed()));
		}
	}

	async function regenerate() {
		const date = regenerateDate;
		if (!date) return;
		try {
			await adminRegenerateDay(date);
			toast.success(m.admin_couchtv_regenerated({ day: formatDay(date, lang, 'long') }));
			await refresh();
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_regenerate_failed()));
		}
	}

	const legend = $derived([
		{ color: renderColor.ready, label: m.admin_couchtv_render_ready() },
		{ color: renderColor.composed, label: m.admin_couchtv_render_composed() },
		{ color: renderColor.rendering, label: m.admin_couchtv_render_rendering() },
		{ color: renderColor.failed, label: m.admin_couchtv_render_failed() },
		{ color: renderColor.uncomposed, label: m.admin_couchtv_render_uncomposed() }
	]);
</script>

<svelte:head>
	<title>{m.admin_couchtv_schedule_page_title()}</title>
</svelte:head>

<div class="mb-4 flex flex-wrap items-center justify-between gap-3">
	<div class="flex items-center gap-2">
		<Button
			variant="secondary"
			size="sm"
			aria-label={m.admin_couchtv_week_previous()}
			onclick={() => offset--}
		>
			<ChevronLeft class="size-4" />
		</Button>
		<Button variant="secondary" size="sm" disabled={offset === 0} onclick={() => (offset = 0)}>
			{m.admin_couchtv_week_this()}
		</Button>
		<Button
			variant="secondary"
			size="sm"
			aria-label={m.admin_couchtv_week_next()}
			onclick={() => offset++}
		>
			<ChevronRight class="size-4" />
		</Button>
		{#if cols.length}
			<span class="ml-1 text-xs text-muted">
				{formatDay(cols[0].date, lang)} - {formatDay(cols[cols.length - 1].date, lang)} · {timeZone}
			</span>
		{/if}
	</div>
	{#if overview}
		<p class="text-xs text-muted">
			{#if overview.plannedUntil}
				{m.admin_couchtv_planned_until({
					when: `${formatDay(columnDate(Date.parse(overview.plannedUntil), axis, timeZone), lang, 'long')} ${formatClock(Date.parse(overview.plannedUntil), timeZone, lang)}`
				})}
			{:else}
				{m.admin_couchtv_not_planned()}
			{/if}
		</p>
	{/if}
</div>

<div class="mb-4 flex flex-wrap items-center gap-x-6 gap-y-2 text-xs text-muted">
	<span>
		{m.admin_couchtv_ready_programs({ ready: ahead.prepared, total: ahead.programs })}
	</span>
	<span>
		{m.admin_couchtv_ready_breaks({ ready: ahead.rendered, total: ahead.breaks })}
	</span>
	<span class="flex flex-wrap items-center gap-3">
		<span class="flex items-center gap-1.5">
			<span class="size-1.5 rounded-full bg-success"></span>{m.admin_couchtv_prepared()}
		</span>
		<span class="flex items-center gap-1.5">
			<span class="size-1.5 rounded-full border border-faint"
			></span>{m.admin_couchtv_not_prepared()}
		</span>
		{#each legend as item (item.label)}
			<span class="flex items-center gap-1.5">
				<span class="h-2.5 w-1 rounded-full" style="background: {item.color}"></span>{item.label}
			</span>
		{/each}
	</span>
</div>

{#if swapFrom}
	<div
		class="mb-3 flex items-center justify-between gap-3 rounded-card border border-accent/50 bg-accent-soft px-4 py-2.5 text-sm"
		role="status"
	>
		<span>{m.admin_couchtv_swap_hint({ name: swapFrom.titleName ?? '' })}</span>
		<Button variant="ghost" size="sm" onclick={() => (swapFrom = null)}>{m.common_cancel()}</Button>
	</div>
{/if}

{#if !loaded}
	<Skeleton class="h-[60vh] w-full rounded-card" />
{:else if failed && airings.length === 0}
	<div class="rounded-card border border-edge bg-surface/40 p-8 text-center text-sm text-muted">
		<p>{m.admin_couchtv_schedule_failed()}</p>
		<Button variant="secondary" size="sm" class="mt-3" onclick={load}>{m.common_retry()}</Button>
	</div>
{:else}
	<div in:fly|global={{ y: 16, duration: 350 }}>
		{#if airings.length === 0}
			<p class="mb-3 text-sm text-muted">{m.admin_couchtv_week_empty()}</p>
		{/if}
		<ScheduleGrid
			columns={cols}
			{airings}
			{axis}
			{timeZone}
			{lang}
			{now}
			{today}
			selected={selectedId}
			swapping={swapFrom !== null}
			onselect={select}
			onregenerate={askRegenerate}
		/>
	</div>
{/if}

{#if selected && overview && !swapFrom}
	<AiringPanel
		airing={selected}
		{airings}
		{axis}
		{timeZone}
		breakMinSeconds={overview.config.breakMinSeconds}
		{lang}
		{now}
		onchanged={load}
		onclose={() => (selectedId = null)}
		onswap={() => (swapFrom = selected)}
		onregenerate={askRegenerate}
	/>
{/if}

<Confirm
	bind:open={regenerateOpen}
	title={m.admin_couchtv_regenerate_title({
		day: regenerateDate ? formatDay(regenerateDate, lang, 'long') : ''
	})}
	message={m.admin_couchtv_regenerate_message()}
	confirmLabel={m.admin_couchtv_regenerate_day()}
	onconfirm={regenerate}
/>
