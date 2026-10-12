<script lang="ts">
	import {
		ArrowLeftRight,
		CalendarClock,
		Lock,
		LockOpen,
		Plus,
		RefreshCw,
		Replace,
		Trash2,
		X
	} from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { fly } from 'svelte/transition';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import { problemMessage } from '$lib/api/problem';
	import * as m from '$lib/paraglide/messages';
	import {
		adminInsertAiring,
		adminUpdateAiring,
		type CouchTvAiring,
		type CouchTvAiringEdit
	} from '../api';
	import { episodeCode, fillLabel, renderColor, renderLabel, stateLabel } from '../labels';
	import { afterTime, atClock, columnDate, neighbours } from '../schedule';
	import { clockOf, formatClock, formatDay, formatLength } from '../time';
	import ProgramPicker, { type PickedProgram } from './ProgramPicker.svelte';

	// A program's details and what an admin can do to it. Every change replans the rest
	// of the week around it, so the page reloads the schedule after one.
	let {
		airing,
		airings,
		axis,
		timeZone,
		breakMinSeconds,
		lang,
		now,
		onchanged,
		onclose,
		onswap,
		onregenerate
	}: {
		airing: CouchTvAiring;
		airings: CouchTvAiring[];
		axis: number;
		timeZone: string;
		breakMinSeconds: number;
		lang: string;
		now: number;
		onchanged: () => Promise<void>;
		onclose: () => void;
		onswap: () => void;
		onregenerate: (date: string) => void;
	} = $props();

	type Mode = 'details' | 'replace' | 'move' | 'before' | 'after';
	let mode = $state<Mode>('details');
	let picked = $state<PickedProgram | null>(null);
	let time = $state('');
	let busy = $state(false);
	let confirmRemove = $state(false);

	const start = $derived(Date.parse(airing.startsAt));
	const end = $derived(Date.parse(airing.endsAt));
	const started = $derived(start <= now);
	const date = $derived(columnDate(start, axis, timeZone));
	const near = $derived(neighbours(airings, airing.id));

	// a fresh program opens on its details
	$effect(() => {
		void airing.id;
		mode = 'details';
		picked = null;
	});

	function open(next: Mode) {
		mode = next;
		picked = null;
		if (next === 'move' || next === 'before') time = clockOf(start, timeZone);
		if (next === 'after') time = clockOf(afterTime(airing, breakMinSeconds), timeZone);
	}

	async function run(action: () => Promise<unknown>, done: string) {
		busy = true;
		try {
			await action();
			toast.success(done);
			mode = 'details';
			await onchanged();
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_edit_failed()));
		} finally {
			busy = false;
		}
	}

	const edit = (body: CouchTvAiringEdit, done: string) =>
		run(() => adminUpdateAiring(airing.id, body), done);

	const startsAt = () => {
		const at = atClock(date, time, axis, timeZone);
		return Number.isNaN(at) ? undefined : new Date(at).toISOString();
	};

	function submit() {
		if (mode === 'replace' && picked) {
			edit(
				{ action: 'pin', titleId: picked.titleId, episodeId: picked.episodeId },
				m.admin_couchtv_replaced()
			);
		} else if (mode === 'move') {
			const at = startsAt();
			if (at) edit({ action: 'pin', startsAt: at }, m.admin_couchtv_moved());
		} else if ((mode === 'before' || mode === 'after') && picked) {
			const at = startsAt();
			const p = picked;
			if (at)
				run(
					() => adminInsertAiring({ titleId: p.titleId, episodeId: p.episodeId, startsAt: at }),
					m.admin_couchtv_inserted()
				);
		}
	}

	const titles: Record<Exclude<Mode, 'details'>, () => string> = {
		replace: m.admin_couchtv_action_replace,
		move: m.admin_couchtv_action_move,
		before: m.admin_couchtv_action_insert_before,
		after: m.admin_couchtv_action_insert_after
	};
</script>

<aside
	transition:fly={{ x: 24, duration: 200 }}
	aria-label={m.admin_couchtv_program_details()}
	class="fixed inset-y-0 right-0 z-40 flex w-full max-w-sm flex-col border-l border-edge bg-surface-2 shadow-2xl shadow-black/50"
>
	<header class="flex items-start justify-between gap-3 border-b border-edge p-5">
		<div class="min-w-0">
			<p class="text-[11px] font-medium text-faint">
				{formatDay(date, lang, 'long')} · {formatClock(start, timeZone, lang)}-{formatClock(
					end,
					timeZone,
					lang
				)}
			</p>
			<h2 class="mt-0.5 truncate text-base font-semibold">{airing.titleName}</h2>
			{#if airing.episodeId}
				<p class="truncate text-xs text-muted">
					{episodeCode(airing.seasonNumber, airing.episodeNumber)}
					{airing.episodeName}
				</p>
			{/if}
		</div>
		<button
			type="button"
			onclick={onclose}
			aria-label={m.admin_couchtv_close()}
			class="rounded-full p-1.5 text-muted transition-colors hover:bg-surface hover:text-text"
		>
			<X class="size-4" />
		</button>
	</header>

	<div class="flex-1 space-y-5 overflow-y-auto p-5">
		{#if mode === 'details'}
			<dl class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 text-xs">
				<dt class="text-faint">{m.admin_couchtv_length_on_air()}</dt>
				<dd>{formatLength(end - start)}</dd>
				{#if airing.fill}
					<dt class="text-faint">{m.admin_couchtv_rule()}</dt>
					<dd>{fillLabel(airing.fill)}{airing.slot ? ` (${airing.slot})` : ''}</dd>
				{/if}
				<dt class="text-faint">{m.admin_couchtv_chosen_by()}</dt>
				<dd>
					{airing.source === 'admin'
						? m.admin_couchtv_source_admin()
						: m.admin_couchtv_source_auto()}
				</dd>
				<dt class="text-faint">{m.common_status()}</dt>
				<dd>{stateLabel(airing.state)}</dd>
				<dt class="text-faint">{m.admin_couchtv_package()}</dt>
				<dd class="flex items-center gap-1.5">
					<span class="size-1.5 rounded-full {airing.ready ? 'bg-success' : 'border border-faint'}"
					></span>
					{airing.ready ? m.admin_couchtv_prepared() : m.admin_couchtv_not_prepared()}
				</dd>
				{#if airing.creditsAtMs != null}
					<dt class="text-faint">{m.admin_couchtv_credits_at()}</dt>
					<dd>{formatLength(airing.creditsAtMs)}</dd>
				{/if}
				{#each [{ label: m.admin_couchtv_break_before(), b: near.breakBefore }, { label: m.admin_couchtv_break_after(), b: near.breakAfter }] as row (row.label)}
					{#if row.b}
						<dt class="text-faint">{row.label}</dt>
						<dd class="flex items-center gap-1.5">
							<span
								class="size-1.5 rounded-full"
								style="background: {renderColor[row.b.render ?? 'uncomposed']}"
							></span>
							{renderLabel(row.b.render)} ·
							{formatLength(Date.parse(row.b.endsAt) - Date.parse(row.b.startsAt))}
						</dd>
					{/if}
				{/each}
			</dl>

			{#if started}
				<p class="rounded-lg bg-surface px-3 py-2 text-xs text-muted">
					{m.admin_couchtv_started_hint()}
				</p>
			{/if}

			<div class="grid gap-2">
				<Button
					variant="secondary"
					disabled={busy}
					onclick={() =>
						edit(
							{ action: airing.locked ? 'unlock' : 'lock' },
							airing.locked ? m.admin_couchtv_unlocked() : m.admin_couchtv_locked()
						)}
				>
					{#if airing.locked}<LockOpen class="size-4" />{m.admin_couchtv_action_unlock()}
					{:else}<Lock class="size-4" />{m.admin_couchtv_action_lock()}{/if}
				</Button>
				<Button variant="secondary" disabled={busy || started} onclick={() => open('replace')}>
					<Replace class="size-4" />{m.admin_couchtv_action_replace()}
				</Button>
				<Button variant="secondary" disabled={busy || started} onclick={() => open('move')}>
					<CalendarClock class="size-4" />{m.admin_couchtv_action_move()}
				</Button>
				<Button variant="secondary" disabled={busy || started} onclick={onswap}>
					<ArrowLeftRight class="size-4" />{m.admin_couchtv_action_swap()}
				</Button>
				<div class="grid grid-cols-2 gap-2">
					<Button variant="secondary" disabled={busy || started} onclick={() => open('before')}>
						<Plus class="size-4" />{m.admin_couchtv_action_insert_before()}
					</Button>
					<Button variant="secondary" disabled={busy} onclick={() => open('after')}>
						<Plus class="size-4" />{m.admin_couchtv_action_insert_after()}
					</Button>
				</div>
				<Button variant="secondary" disabled={busy} onclick={() => onregenerate(date)}>
					<RefreshCw class="size-4" />{m.admin_couchtv_regenerate_day()}
				</Button>
				<Button variant="danger" disabled={busy || started} onclick={() => (confirmRemove = true)}>
					<Trash2 class="size-4" />{m.admin_couchtv_action_remove()}
				</Button>
			</div>
			<p class="text-[11px] text-faint">{m.admin_couchtv_edit_hint()}</p>
		{:else}
			<div class="flex items-center justify-between gap-2">
				<h3 class="text-sm font-semibold">{titles[mode]()}</h3>
				<button
					type="button"
					class="text-xs text-muted hover:text-text"
					onclick={() => (mode = 'details')}
				>
					{m.common_cancel()}
				</button>
			</div>

			{#if mode !== 'move'}
				{#if picked}
					<div
						class="flex items-center justify-between gap-2 rounded-lg bg-surface px-3 py-2 text-sm"
					>
						<span class="truncate">{picked.label}</span>
						<button
							type="button"
							class="shrink-0 text-xs text-muted hover:text-text"
							onclick={() => (picked = null)}
						>
							{m.admin_couchtv_picker_change()}
						</button>
					</div>
				{:else}
					<ProgramPicker onpick={(p) => (picked = p)} />
				{/if}
			{/if}

			{#if mode !== 'replace'}
				<label class="block">
					<span class="mb-1.5 block text-xs font-medium text-muted">
						{m.admin_couchtv_starts_at({ zone: timeZone })}
					</span>
					<input
						type="time"
						bind:value={time}
						class="h-10 w-full rounded-input border border-edge bg-surface px-3.5 text-sm focus:border-accent focus:outline-none"
					/>
				</label>
				<p class="text-[11px] text-faint">{m.admin_couchtv_time_hint()}</p>
			{/if}

			<Button
				class="w-full"
				loading={busy}
				disabled={(mode !== 'move' && !picked) || (mode !== 'replace' && !time)}
				onclick={submit}
			>
				{titles[mode]()}
			</Button>
		{/if}
	</div>
</aside>

<Confirm
	bind:open={confirmRemove}
	title={m.admin_couchtv_remove_title({ name: airing.titleName ?? '' })}
	message={airing.source === 'admin'
		? m.admin_couchtv_remove_admin()
		: m.admin_couchtv_remove_auto()}
	confirmLabel={m.admin_couchtv_action_remove()}
	onconfirm={() => edit({ action: 'remove' }, m.admin_couchtv_removed())}
/>
