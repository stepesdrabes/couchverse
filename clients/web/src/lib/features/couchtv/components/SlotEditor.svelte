<script lang="ts">
	import { ArrowDown, ArrowUp, Pin, Trash2, X } from 'lucide-svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { problemMessage } from '$lib/api/problem';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import { dayLabel, fillLabel, pickLabel } from '../labels';
	import {
		airsSeries,
		DAYS,
		FILLS,
		inOrder,
		PICKS,
		type ProblemCode,
		type SlotDraft
	} from '../template';
	import ProgramPicker from './ProgramPicker.svelte';

	// One slot of the weekly template: when it starts, what it airs and on which days.
	let {
		slot = $bindable(),
		names,
		problems,
		first,
		last,
		onmove,
		onremove,
		onnamed
	}: {
		slot: SlotDraft;
		/** title names by id, for the pinned titles */
		names: Record<string, string>;
		/** a title picked here, so its name shows */
		onnamed: (id: string, name: string) => void;
		problems: ProblemCode[];
		first: boolean;
		last: boolean;
		onmove: (by: -1 | 1) => void;
		onremove: () => void;
	} = $props();

	let pinning = $state(false);
	const lang = $derived(currentLang());
	const fills = $derived(FILLS.map((f) => ({ value: f, label: fillLabel(f) })));
	const picks = $derived(PICKS.map((p) => ({ value: p, label: pickLabel(p) })));
	const maxPins = $derived(inOrder(slot.fill) ? 1 : 50);

	function toggleDay(day: SlotDraft['days'][number]) {
		slot.days = slot.days.includes(day) ? slot.days.filter((d) => d !== day) : [...slot.days, day];
	}

	// pinned titles must suit the fill: a series for strips and reruns, a movie otherwise
	function changeFill(fill: string) {
		const wasSeries = airsSeries(slot.fill);
		slot.fill = fill as SlotDraft['fill'];
		if (airsSeries(slot.fill) !== wasSeries) slot.titles = [];
		if (inOrder(slot.fill)) slot.titles = slot.titles.slice(0, 1);
	}
</script>

<li
	class="rounded-card border bg-surface/40 p-4 {problems.length
		? 'border-danger/60'
		: 'border-edge'}"
	aria-label={m.admin_couchtv_slot_named({ key: slot.key })}
>
	<div class="flex flex-wrap items-end gap-3">
		<Input
			label={m.admin_couchtv_slot_key()}
			bind:value={slot.key}
			class="w-40"
			maxlength={40}
			spellcheck={false}
		/>
		<label class="block">
			<span class="mb-1.5 block text-xs font-medium text-muted">{m.admin_couchtv_slot_start()}</span
			>
			<input
				type="time"
				bind:value={slot.start}
				class="h-10 w-28 rounded-input border border-edge bg-surface px-3 text-sm focus:border-accent focus:outline-none"
			/>
		</label>
		<div class="flex h-10 items-center gap-2">
			<Switch bind:checked={slot.anchor} label={m.admin_couchtv_slot_anchor()} />
			<span class="text-xs text-muted">{m.admin_couchtv_slot_anchor()}</span>
		</div>
		<div class="flex flex-col gap-1.5">
			<span class="text-xs font-medium text-muted">{m.admin_couchtv_slot_fill()}</span>
			<Select items={fills} value={slot.fill} onchange={changeFill} class="w-36 justify-between" />
		</div>
		<div class="flex flex-col gap-1.5">
			<span class="text-xs font-medium text-muted">{m.admin_couchtv_slot_pick()}</span>
			<Select items={picks} bind:value={slot.pick} class="w-32 justify-between" />
		</div>
		{#if inOrder(slot.fill)}
			<Input
				label={m.admin_couchtv_slot_count()}
				type="number"
				min="0"
				max="20"
				placeholder="1"
				bind:value={slot.count}
				class="w-24"
			/>
		{/if}
		<div class="ml-auto flex items-center gap-1">
			<button
				type="button"
				class="rounded-full p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text disabled:opacity-30"
				aria-label={m.admin_couchtv_slot_up()}
				disabled={first}
				onclick={() => onmove(-1)}
			>
				<ArrowUp class="size-4" />
			</button>
			<button
				type="button"
				class="rounded-full p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text disabled:opacity-30"
				aria-label={m.admin_couchtv_slot_down()}
				disabled={last}
				onclick={() => onmove(1)}
			>
				<ArrowDown class="size-4" />
			</button>
			<button
				type="button"
				class="rounded-full p-2 text-danger transition-colors hover:bg-danger/15"
				aria-label={m.admin_couchtv_slot_remove()}
				onclick={onremove}
			>
				<Trash2 class="size-4" />
			</button>
		</div>
	</div>

	<div
		class="mt-3 flex flex-wrap items-center gap-1"
		role="group"
		aria-label={m.admin_couchtv_slot_days()}
	>
		{#each DAYS as day (day)}
			<button
				type="button"
				aria-pressed={slot.days.includes(day)}
				onclick={() => toggleDay(day)}
				class="rounded-full border px-2.5 py-1 text-xs transition-colors
					{slot.days.includes(day)
					? 'border-accent bg-accent-soft text-text'
					: 'border-edge text-muted hover:border-faint'}"
			>
				{dayLabel(day, lang)}
			</button>
		{/each}
	</div>

	<div class="mt-3 flex flex-wrap items-center gap-1.5">
		<span class="text-xs text-muted">
			{airsSeries(slot.fill)
				? m.admin_couchtv_slot_pinned_series()
				: m.admin_couchtv_slot_pinned_movies()}
		</span>
		{#each slot.titles as id (id)}
			<span
				class="inline-flex items-center gap-1 rounded-full border border-edge bg-surface-2 py-0.5 pr-1 pl-2.5 text-xs"
			>
				{names[id] ?? id.slice(0, 8)}
				<button
					type="button"
					class="rounded-full p-0.5 text-muted hover:text-text"
					aria-label={m.admin_couchtv_slot_unpin({ name: names[id] ?? id })}
					onclick={() => (slot.titles = slot.titles.filter((t) => t !== id))}
				>
					<X class="size-3" />
				</button>
			</span>
		{:else}
			<span class="text-xs text-faint">{m.admin_couchtv_slot_by_rule()}</span>
		{/each}
		{#if slot.titles.length < maxPins}
			<button
				type="button"
				class="inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs text-accent-ink hover:bg-surface-2"
				aria-expanded={pinning}
				onclick={() => (pinning = !pinning)}
			>
				<Pin class="size-3" />{m.admin_couchtv_slot_pin()}
			</button>
		{/if}
	</div>
	{#if pinning}
		<div class="mt-2 max-w-sm rounded-lg border border-edge bg-surface p-3">
			<ProgramPicker
				kind={airsSeries(slot.fill) ? 'series' : 'movie'}
				episodes={false}
				exclude={slot.titles}
				onpick={(p) => {
					onnamed(p.titleId, p.label);
					slot.titles = [...slot.titles, p.titleId];
					pinning = false;
				}}
			/>
		</div>
	{/if}

	{#if problems.length}
		<ul class="mt-3 space-y-0.5 text-xs text-danger">
			{#each problems as code (code)}
				<li>{problemMessage({ code }, m.admin_couchtv_template_invalid())}</li>
			{/each}
		</ul>
	{/if}
</li>
