<script lang="ts">
	import { Plus } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { problemMessage } from '$lib/api/problem';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';
	import { adminListLibrary, adminUpdateCouchTv, type CouchTvOverview } from '../api';
	import { dayLabel } from '../labels';
	import {
		configOf,
		DAYS,
		draftOf,
		freshKey,
		problems,
		ROTATION_KEYS,
		type Day,
		type ProblemCode
	} from '../template';
	import SlotEditor from './SlotEditor.svelte';

	// The channel's tunable surface: its time zone, the rotation knobs and the weekly
	// template. A save replans every day that is not frozen yet.
	let { overview }: { overview: CouchTvOverview } = $props();

	// the editor starts from what the server had and adopts its answer to every save
	// svelte-ignore state_referenced_locally
	let base = $state(overview.config);
	let draft = $state(draftOf(base));
	let saving = $state(false);
	let day = $state<Day | 'all'>('all');
	let names = $state<Record<string, string>>({});
	const form = new FormState(() => draft);
	form.reset();

	const lang = $derived(currentLang());
	const issues = $derived(problems(draft.slots));
	const dayIssues = $derived(issues.filter((p) => p.slot === undefined || p.day));
	// a slot shows each kind of problem it has once, the day ones included
	const slotIssues = (i: number): ProblemCode[] => [
		...new Set(issues.filter((p) => p.slot === i).map((p) => p.code))
	];
	const visible = $derived(
		draft.slots.map((s, i) => ({ s, i })).filter(({ s }) => day === 'all' || s.days.includes(day))
	);
	const zones =
		typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : [];
	const dayChoices: (Day | 'all')[] = ['all', ...DAYS];

	const rotation = $derived<{ key: (typeof ROTATION_KEYS)[number]; label: string; hint: string }[]>(
		[
			{
				key: 'primeRepeatMinDays',
				label: m.admin_couchtv_rot_prime_days(),
				hint: m.admin_couchtv_rot_prime_days_hint()
			},
			{
				key: 'primeRepeatFactor',
				label: m.admin_couchtv_rot_prime_factor(),
				hint: m.admin_couchtv_rot_prime_factor_hint()
			},
			{
				key: 'movieGapDays',
				label: m.admin_couchtv_rot_movie_gap(),
				hint: m.admin_couchtv_rot_movie_gap_hint()
			},
			{
				key: 'rerunGapDays',
				label: m.admin_couchtv_rot_rerun_gap(),
				hint: m.admin_couchtv_rot_rerun_gap_hint()
			},
			{
				key: 'premiereWindowDays',
				label: m.admin_couchtv_rot_premiere(),
				hint: m.admin_couchtv_rot_premiere_hint()
			},
			{
				key: 'fillerMinMinutes',
				label: m.admin_couchtv_rot_filler(),
				hint: m.admin_couchtv_rot_filler_hint()
			}
		]
	);

	onMount(async () => {
		try {
			const page = await adminListLibrary({ pageSize: 200, sort: 'name' });
			names = { ...Object.fromEntries(page.items.map((r) => [r.id, r.name])), ...names };
		} catch {
			// pinned titles show their ids
		}
	});

	/** Moves a slot past its neighbour among those shown (the day's, or all). */
	function move(at: number, by: -1 | 1) {
		const k = visible.findIndex((v) => v.i === at);
		const other = visible[k + by];
		if (!other) return;
		const slots = [...draft.slots];
		[slots[at], slots[other.i]] = [slots[other.i], slots[at]];
		draft.slots = slots;
	}

	function add() {
		draft.slots = [
			...draft.slots,
			{
				key: freshKey(draft.slots, 'reruns'),
				days: day === 'all' ? [...DAYS] : [day],
				start: '',
				anchor: false,
				fill: 'reruns',
				pick: 'any',
				count: '',
				titles: []
			}
		];
	}

	function discard() {
		draft = draftOf(base);
		form.reset();
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		try {
			const saved = await adminUpdateCouchTv(configOf(draft, base));
			base = saved.config;
			draft = draftOf(saved.config);
			form.reset();
			toast.success(m.admin_couchtv_template_saved());
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_template_save_failed()));
		} finally {
			saving = false;
		}
	}
</script>

<form onsubmit={save} class="space-y-6">
	<section class="rounded-card border border-edge bg-surface/40 p-5">
		<h2 class="mb-4 text-sm font-semibold text-muted">{m.admin_couchtv_channel()}</h2>
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<Input
				label={m.admin_couchtv_time_zone()}
				bind:value={draft.timeZone}
				list="couchtv-zones"
				required
				spellcheck={false}
			/>
			<datalist id="couchtv-zones">
				{#each zones as zone (zone)}<option value={zone}></option>{/each}
			</datalist>
			<Input
				label={m.admin_couchtv_break_min()}
				type="number"
				min="5"
				max="600"
				required
				bind:value={draft.breakMinSeconds}
			/>
		</div>
	</section>

	<section class="rounded-card border border-edge bg-surface/40 p-5">
		<h2 class="mb-1 text-sm font-semibold text-muted">{m.admin_couchtv_rotation()}</h2>
		<p class="mb-4 text-xs text-faint">{m.admin_couchtv_rotation_hint()}</p>
		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			{#each rotation as knob (knob.key)}
				<div>
					<Input
						label={knob.label}
						type="number"
						min="0"
						step={knob.key === 'primeRepeatFactor' ? '0.1' : '1'}
						required
						bind:value={draft.rotation[knob.key]}
					/>
					<p class="mt-1 text-[11px] text-faint">{knob.hint}</p>
				</div>
			{/each}
		</div>
	</section>

	<section>
		<div class="mb-3 flex flex-wrap items-center justify-between gap-3">
			<div>
				<h2 class="text-sm font-semibold text-muted">{m.admin_couchtv_week_template()}</h2>
				<p class="text-xs text-faint">{m.admin_couchtv_week_template_hint()}</p>
			</div>
			<div class="flex flex-wrap gap-1" role="group" aria-label={m.admin_couchtv_show_day()}>
				{#each dayChoices as d (d)}
					<button
						type="button"
						aria-pressed={day === d}
						onclick={() => (day = d)}
						class="rounded-full border px-2.5 py-1 text-xs transition-colors
							{day === d
							? 'border-accent bg-accent-soft text-text'
							: 'border-edge text-muted hover:border-faint'}"
					>
						{d === 'all' ? m.common_all() : dayLabel(d, lang)}
					</button>
				{/each}
			</div>
		</div>

		{#if dayIssues.length}
			<ul
				class="mb-3 space-y-1 rounded-card border border-danger/50 bg-danger/10 px-4 py-3 text-xs text-danger"
			>
				{#each dayIssues as issue, n (n)}
					<li>
						{#if issue.day}<span class="font-semibold">{dayLabel(issue.day, lang, 'long')}:</span
							>{/if}
						{problemMessage({ code: issue.code }, m.admin_couchtv_template_invalid())}
						{#if issue.slot !== undefined}({draft.slots[issue.slot]?.key}){/if}
					</li>
				{/each}
			</ul>
		{/if}

		<ol class="space-y-3">
			{#each visible as v, k (v.i)}
				<SlotEditor
					bind:slot={draft.slots[v.i]}
					{names}
					problems={slotIssues(v.i)}
					first={k === 0}
					last={k === visible.length - 1}
					onmove={(by) => move(v.i, by)}
					onremove={() => (draft.slots = draft.slots.filter((_, i) => i !== v.i))}
					onnamed={(id, name) => (names = { ...names, [id]: name })}
				/>
			{/each}
		</ol>
		<Button type="button" variant="secondary" size="sm" class="mt-3" onclick={add}>
			<Plus class="size-4" />{m.admin_couchtv_slot_add()}
		</Button>
	</section>

	<div
		class="sticky bottom-4 flex items-center justify-end gap-2 rounded-card border border-edge bg-surface-2/95 px-4 py-3 shadow-xl shadow-black/30 backdrop-blur"
	>
		{#if form.dirty}
			<span class="mr-auto text-xs text-muted">{m.admin_couchtv_unsaved()}</span>
			<Button type="button" variant="ghost" size="sm" onclick={discard}
				>{m.admin_couchtv_discard()}</Button
			>
		{/if}
		<Button type="submit" loading={saving} disabled={!form.dirty || issues.length > 0}>
			{m.common_save()}
		</Button>
	</div>
</form>
