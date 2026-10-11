<script lang="ts">
	import { SvelteMap } from 'svelte/reactivity';
	import { fly } from 'svelte/transition';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { artworkUrl, artworkVer } from '$lib/features/catalog/api';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import { adminGetTitle, adminListLibrary, type AdminTitle, type LibraryRow } from '../api';
	import BroadcastPlayer from '../components/BroadcastPlayer.svelte';
	import {
		buildSpec,
		DEFAULT_OPTIONS,
		NEEDS_TITLE,
		previewTitle,
		type PreviewOptions,
		type PreviewTemplate
	} from '../preview';

	let rows = $state<LibraryRow[]>([]);
	let loaded = $state(false);
	let mainId = $state('');
	let nextId = $state('');
	const titles = new SvelteMap<string, AdminTitle>();
	let o = $state<PreviewOptions>({ ...DEFAULT_OPTIONS, taglines: [...DEFAULT_OPTIONS.taglines] });

	async function loadLibrary() {
		try {
			rows = (await adminListLibrary({ sort: 'added', pageSize: 50 })).items;
			mainId = rows[0]?.id ?? '';
			nextId = rows[1]?.id ?? rows[0]?.id ?? '';
		} finally {
			loaded = true;
		}
	}
	loadLibrary();

	// the lineup shows the chosen title and the next ones in the library
	const lineupIds = $derived(
		[mainId, ...rows.map((r) => r.id).filter((id) => id !== mainId)].filter(Boolean).slice(0, 4)
	);

	$effect(() => {
		for (const id of new Set([mainId, nextId, ...lineupIds])) {
			if (!id || titles.has(id)) continue;
			adminGetTitle(id)
				.then((t) => titles.set(id, t))
				.catch(() => undefined);
		}
	});

	const preview = (id: string) => {
		const t = titles.get(id);
		return t ? previewTitle(t, (a, size) => artworkUrl(a.id, artworkVer(a.createdAt), size)) : null;
	};

	// only the titles the chosen template shows, so loading the others never reloads it
	const spec = $derived(
		buildSpec(
			o,
			preview(mainId),
			o.template === 'squeeze' ? preview(nextId) : null,
			o.template === 'lineup' ? lineupIds.map(preview).filter((t) => t !== null) : []
		)
	);

	const templates = $derived<{ value: PreviewTemplate; label: string }[]>([
		{ value: 'identFull', label: m.admin_couchtv_tpl_ident_full() },
		{ value: 'identShort', label: m.admin_couchtv_tpl_ident_short() },
		{ value: 'sting', label: m.admin_couchtv_tpl_sting() },
		{ value: 'upNext', label: m.admin_couchtv_tpl_up_next() },
		{ value: 'promo', label: m.admin_couchtv_tpl_promo() },
		{ value: 'evergreenEnd', label: m.admin_couchtv_tpl_evergreen() },
		{ value: 'stripPromo', label: m.admin_couchtv_tpl_strip() },
		{ value: 'shortPromo', label: m.admin_couchtv_tpl_short() },
		{ value: 'lineup', label: m.admin_couchtv_tpl_lineup() },
		{ value: 'squeeze', label: m.admin_couchtv_tpl_squeeze() },
		{ value: 'standby', label: m.admin_couchtv_tpl_standby() }
	]);

	const titleItems = $derived(
		rows.map((r) => ({ value: r.id, label: r.year ? `${r.name} (${r.year})` : r.name }))
	);

	const weekdays = $derived(
		// 2024-01-01 was a Monday
		Array.from({ length: 7 }, (_, d) => ({
			value: String(d),
			label: new Date(Date.UTC(2024, 0, 1 + d)).toLocaleDateString(currentLang(), {
				weekday: 'long',
				timeZone: 'UTC'
			})
		}))
	);

	const t = $derived(o.template);
	const hasBed = $derived(
		['upNext', 'promo', 'evergreenEnd', 'stripPromo', 'shortPromo', 'squeeze'].includes(t)
	);
	const hasAiring = $derived(
		['upNext', 'promo', 'stripPromo', 'shortPromo', 'lineup', 'squeeze'].includes(t)
	);
	const airsItems = $derived(
		t === 'stripPromo'
			? [
					{ value: 'weekdays', label: m.admin_couchtv_airs_weekdays() },
					{ value: 'nightly', label: m.admin_couchtv_airs_nightly() }
				]
			: [
					{ value: 'tonight', label: m.admin_couchtv_airs_tonight() },
					{ value: 'tomorrow', label: m.admin_couchtv_airs_tomorrow() },
					{ value: 'day', label: m.admin_couchtv_airs_day() },
					{ value: 'premiere', label: m.admin_couchtv_airs_premiere() }
				]
	);
	const showsAirs = $derived(['promo', 'stripPromo', 'shortPromo'].includes(t) && !o.premiere);
	const showsDay = $derived(
		(['promo', 'shortPromo'].includes(t) &&
			(o.premiere || o.airs === 'day' || o.airs === 'premiere')) ||
			(t === 'stripPromo' && preview(mainId)?.kind !== 'series')
	);

	function choose(template: PreviewTemplate) {
		o.template = template;
		// a strip airs on weekdays or nightly; every other promo on a date
		if (template === 'stripPromo' && !['weekdays', 'nightly'].includes(o.airs)) o.airs = 'weekdays';
		if (template !== 'stripPromo' && ['weekdays', 'nightly'].includes(o.airs)) o.airs = 'tonight';
	}
</script>

<svelte:head>
	<title>{m.admin_couchtv_preview_page_title()}</title>
</svelte:head>

<h1 class="mb-2 text-2xl font-bold">{m.admin_couchtv_preview_title()}</h1>
<p class="mb-6 max-w-3xl text-sm text-muted">{m.admin_couchtv_english_hint()}</p>

{#if loaded && NEEDS_TITLE[t] && rows.length === 0}
	<EmptyState title={m.admin_couchtv_no_titles()} />
{/if}

<div class="grid gap-6 lg:grid-cols-[minmax(0,1fr)_17rem]" in:fly|global={{ y: 16, duration: 350 }}>
	<section class="min-w-0 space-y-2">
		<BroadcastPlayer {spec} />
		<p class="text-[11px] text-faint">{m.admin_couchtv_keys_hint()}</p>
	</section>

	<aside class="space-y-5 rounded-card border border-edge bg-surface/40 p-4">
		<div>
			<h2 class="mb-2 text-xs font-medium text-muted">{m.admin_couchtv_template()}</h2>
			<div class="flex flex-col gap-0.5">
				{#each templates as item (item.value)}
					<button
						type="button"
						aria-pressed={t === item.value}
						onclick={() => choose(item.value)}
						class="rounded-full px-3 py-1.5 text-left text-[13px] font-medium transition-colors
							{t === item.value ? 'bg-accent-soft text-text' : 'text-muted hover:bg-surface-2 hover:text-text'}"
					>
						{item.label}
					</button>
				{/each}
			</div>
		</div>

		{#if NEEDS_TITLE[t] && rows.length}
			<div class="space-y-2">
				<p class="text-xs font-medium text-muted">{m.admin_couchtv_title()}</p>
				<Select items={titleItems} bind:value={mainId} class="w-full justify-between" />
				{#if t === 'squeeze'}
					<p class="pt-1 text-xs font-medium text-muted">{m.admin_couchtv_next_title()}</p>
					<Select items={titleItems} bind:value={nextId} class="w-full justify-between" />
				{/if}
			</div>
		{/if}

		{#if t === 'promo' || t === 'evergreenEnd' || t === 'stripPromo'}
			<div class="space-y-2">
				<p class="text-xs font-medium text-muted">{m.admin_couchtv_mood()}</p>
				{#if t === 'stripPromo'}
					<Select
						items={[
							{ value: 'bright', label: m.admin_couchtv_mood_bright() },
							{ value: 'dark', label: m.admin_couchtv_mood_dark() }
						]}
						bind:value={o.stripMood}
						class="w-full justify-between"
					/>
				{:else}
					<Select
						items={[
							{ value: 'cinematic', label: m.admin_couchtv_mood_cinematic() },
							{ value: 'grit', label: m.admin_couchtv_mood_grit() },
							{ value: 'bright', label: m.admin_couchtv_mood_bright() }
						]}
						bind:value={o.promoMood}
						class="w-full justify-between"
					/>
				{/if}
			</div>
		{/if}

		{#if t === 'promo'}
			<div class="space-y-2">
				<p class="text-xs font-medium text-muted">{m.admin_couchtv_part()}</p>
				<Select
					items={[
						{ value: 'full', label: m.admin_couchtv_part_full() },
						{ value: 'body', label: m.admin_couchtv_part_body() },
						{ value: 'end', label: m.admin_couchtv_part_end() }
					]}
					bind:value={o.part}
					class="w-full justify-between"
				/>
				<div class="flex items-center justify-between pt-1">
					<span class="text-xs font-medium text-muted">{m.admin_couchtv_premiere()}</span>
					<Switch bind:checked={o.premiere} label={m.admin_couchtv_premiere()} />
				</div>
			</div>
		{/if}

		{#if t === 'sting'}
			<div class="space-y-2">
				<p class="text-xs font-medium text-muted">{m.admin_couchtv_length()}</p>
				<Select
					items={[2, 3, 5].map((s) => ({
						value: String(s),
						label: m.admin_couchtv_seconds({ seconds: s })
					}))}
					value={String(o.stingLength)}
					onchange={(v) => (o.stingLength = Number(v))}
					class="w-full justify-between"
				/>
			</div>
		{/if}

		{#if t === 'squeeze'}
			<div class="space-y-2">
				<p class="text-xs font-medium text-muted">{m.admin_couchtv_handover()}</p>
				<Select
					items={[
						{ value: 'expand', label: m.admin_couchtv_handover_expand() },
						{ value: 'line', label: m.admin_couchtv_handover_line() }
					]}
					bind:value={o.handover}
					class="w-full justify-between"
				/>
			</div>
		{/if}

		{#if hasAiring}
			<div class="space-y-2">
				{#if showsAirs}
					<p class="text-xs font-medium text-muted">{m.admin_couchtv_airs()}</p>
					<Select
						items={airsItems}
						value={o.airs}
						onchange={(v) => (o.airs = v as PreviewOptions['airs'])}
						class="w-full justify-between"
					/>
				{/if}
				<Input type="time" label={m.admin_couchtv_time()} bind:value={o.time} />
				{#if showsDay}
					<p class="text-xs font-medium text-muted">{m.admin_couchtv_day()}</p>
					<Select
						items={weekdays}
						value={String(o.day)}
						onchange={(v) => (o.day = Number(v))}
						class="w-full justify-between"
					/>
				{/if}
			</div>
		{/if}

		{#if t === 'promo' || t === 'evergreenEnd'}
			<div class="space-y-2">
				<p class="text-xs font-medium text-muted">{m.admin_couchtv_tagline()}</p>
				<Input label={m.admin_couchtv_tagline_first()} bind:value={o.taglines[0]} />
				<Input label={m.admin_couchtv_tagline_second()} bind:value={o.taglines[1]} />
			</div>
		{/if}

		{#if hasBed}
			<div class="space-y-2">
				<p class="text-xs font-medium text-muted">{m.admin_couchtv_bed()}</p>
				<Select
					items={[
						{ value: 'synth:groove', label: m.admin_couchtv_bed_groove() },
						{ value: 'synth:pulse', label: m.admin_couchtv_bed_pulse() },
						{ value: 'synth:drone', label: m.admin_couchtv_bed_drone() },
						{ value: 'none', label: m.admin_couchtv_bed_none() }
					]}
					value={o.bed}
					onchange={(v) => (o.bed = v as PreviewOptions['bed'])}
					class="w-full justify-between"
				/>
			</div>
		{/if}
	</aside>
</div>
