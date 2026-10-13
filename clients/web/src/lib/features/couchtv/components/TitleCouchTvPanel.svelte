<script lang="ts">
	import { ScanSearch } from 'lucide-svelte';
	import { onDestroy } from 'svelte';
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { problemMessage } from '$lib/api/problem';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import { formatRelative } from '$lib/utils/format';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';
	import {
		adminAnalyzeCouchTvTitle,
		adminGetCouchTvTitle,
		adminSearchCouchTvSoundtracks,
		adminUpdateCouchTvTitle,
		type AdminTitle,
		type CouchTvShot,
		type CouchTvTitle,
		type CouchTvTitleUpdate
	} from '../api';
	import { moodLabel } from '../labels';
	import { airedFiles } from '../files';
	import { Grants } from '../media';
	import MarkersEditor from './MarkersEditor.svelte';
	import PromoAudioPicker, { type AudioPick } from './PromoAudioPicker.svelte';
	import ShotsEditor from './ShotsEditor.svelte';
	import SoundtrackPicker from './SoundtrackPicker.svelte';

	// A title's Couch TV tools in its editor: off the channel, the promo's mood, its music (a
	// soundtrack from the web or an excerpt of its own audio) and footage, and its files'
	// intro and credits markers.
	let { title }: { title: AdminTitle } = $props();

	let ct = $state<CouchTvTitle | null>(null);
	let failed = $state(false);
	let saving = $state(false);
	let excluded = $state(false);
	let mood = $state('');
	let promo = $state<AudioPick | null>(null);
	let promoAdmin = $state(false);
	let soundtrack = $state<string | null>(null);
	let soundtrackAdmin = $state(false);
	let shots = $state<CouchTvShot[]>([]);
	let shotsAdmin = $state(false);
	const form = new FormState(() => ({
		excluded,
		mood,
		promo: promoAdmin ? promo : null,
		soundtrack: soundtrackAdmin ? soundtrack : null,
		shots: shotsAdmin ? shots : null
	}));

	const grants = new Grants();
	const files = $derived(airedFiles(title));
	const lang = $derived(currentLang());
	const moods = $derived([
		{ value: '', label: m.admin_couchtv_mood_auto() },
		...(['cinematic', 'grit', 'bright'] as const).map((v) => ({ value: v, label: moodLabel(v) }))
	]);

	function adopt(next: CouchTvTitle) {
		// the picker opens again on the excerpt the server now has (an analysis may have moved it)
		if (JSON.stringify(next.promoAudio) !== JSON.stringify(ct?.promoAudio)) adopted++;
		ct = next;
		excluded = next.excluded;
		mood = next.mood ?? '';
		promo = next.promoAudio
			? {
					mediaFileId: next.promoAudio.mediaFileId,
					startMs: next.promoAudio.startMs,
					durationMs: next.promoAudio.durationMs
				}
			: null;
		promoAdmin = next.promoAudio?.source === 'admin';
		soundtrack = next.soundtrack?.soundtrackId ?? null;
		soundtrackAdmin = next.soundtrack?.source === 'admin';
		shots = next.shots;
		shotsAdmin = next.shotsSource === 'admin';
		form.reset();
	}
	let adopted = $state(0);

	let poll: ReturnType<typeof setTimeout> | undefined;
	async function load() {
		try {
			const next = await adminGetCouchTvTitle(title.title.id);
			// an analysis or a search landing does not overwrite edits in progress
			if (!form.loaded || !form.dirty) adopt(next);
			else if (ct) {
				ct = {
					...ct,
					analyzing: next.analyzing,
					promoBed: next.promoBed,
					soundtrack: next.soundtrack,
					soundtracks: next.soundtracks,
					soundtrackQuery: next.soundtrackQuery,
					soundtrackSearchedAt: next.soundtrackSearchedAt,
					searchingSoundtrack: next.searchingSoundtrack
				};
			}
			failed = false;
			clearTimeout(poll);
			if (next.analyzing || next.searchingSoundtrack) poll = setTimeout(load, 5000);
		} catch {
			failed = true;
		}
	}

	$effect(() => {
		void title.title.id;
		load();
	});
	onDestroy(() => clearTimeout(poll));

	async function save() {
		saving = true;
		const body: CouchTvTitleUpdate = { excluded };
		if (mood) body.mood = mood as CouchTvTitleUpdate['mood'];
		if (promoAdmin && promo) body.promoAudio = promo;
		else if (soundtrackAdmin && soundtrack) body.soundtrackId = soundtrack;
		if (shotsAdmin && shots.length) {
			body.shots = shots.map((s) => ({
				mediaFileId: s.mediaFileId,
				startMs: s.startMs,
				durationMs: s.durationMs
			}));
		}
		try {
			adopt(await adminUpdateCouchTvTitle(title.title.id, body));
			toast.success(m.common_saved());
			if (ct?.analyzing || ct?.searchingSoundtrack) poll = setTimeout(load, 5000);
		} catch (err) {
			toast.error(problemMessage(err, m.common_save_failed()));
		} finally {
			saving = false;
		}
	}

	async function analyze() {
		try {
			const next = await adminAnalyzeCouchTvTitle(title.title.id);
			if (ct) ct = { ...ct, analyzing: next.analyzing };
			toast.success(m.admin_couchtv_analysis_queued());
			clearTimeout(poll);
			poll = setTimeout(load, 5000);
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_analysis_failed()));
		}
	}

	async function searchSoundtrack(query: string) {
		try {
			const next = await adminSearchCouchTvSoundtracks(title.title.id, query ? { query } : {});
			if (ct) ct = { ...ct, searchingSoundtrack: next.searchingSoundtrack };
			toast.success(m.admin_couchtv_soundtrack_search_queued());
			clearTimeout(poll);
			poll = setTimeout(load, 3000);
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_soundtrack_search_failed()));
		}
	}

	const source = (admin: boolean) =>
		admin ? m.admin_couchtv_picked_by_admin() : m.admin_couchtv_picked_by_analysis();

	// what the promos play once saved: an admin's pick, else what the finder or the
	// analysis picked on its own
	const bed = $derived.by(() => {
		if (promoAdmin) return m.admin_couchtv_bed_excerpt_admin();
		if (soundtrackAdmin) return m.admin_couchtv_bed_soundtrack_admin();
		if (ct?.soundtrack?.source === 'auto' && ct.soundtrack.ready)
			return m.admin_couchtv_bed_soundtrack_auto();
		if (ct?.promoAudio?.source === 'auto') return m.admin_couchtv_bed_excerpt_auto();
		return m.admin_couchtv_bed_nothing();
	});
</script>

<!-- not a form: the markers and the pickers inside it act on their own -->
<section
	class="space-y-6 rounded-card border border-edge bg-surface/40 p-6"
	aria-labelledby="couchtv-panel"
>
	<div class="flex flex-wrap items-start justify-between gap-3">
		<div>
			<h2 id="couchtv-panel" class="text-sm font-semibold text-muted">{m.admin_nav_couch_tv()}</h2>
			{#if ct}
				<p class="mt-0.5 text-xs text-faint">
					{#if ct.analyzing}
						{m.admin_couchtv_analyzing()}
					{:else if ct.analyzedAt}
						{m.admin_couchtv_analyzed({ when: formatRelative(ct.analyzedAt, lang) })}
						{#if ct.loudnessLufs != null}· {m.admin_couchtv_loudness({
								lufs: ct.loudnessLufs.toFixed(1)
							})}{/if}
					{:else}
						{m.admin_couchtv_not_analyzed()}
					{/if}
				</p>
			{/if}
		</div>
		<Button
			type="button"
			variant="secondary"
			size="sm"
			disabled={!ct || ct.analyzing || excluded || files.length === 0}
			onclick={analyze}
		>
			<ScanSearch class="size-4" />{m.admin_couchtv_analyze()}
		</Button>
	</div>

	{#if failed && !ct}
		<p class="text-xs text-muted">{m.admin_couchtv_title_failed()}</p>
	{:else if ct}
		<div class="flex flex-wrap items-center justify-between gap-4">
			<div>
				<p class="text-sm font-medium">{m.admin_couchtv_excluded()}</p>
				<p class="text-xs text-faint">{m.admin_couchtv_excluded_hint()}</p>
			</div>
			<Switch bind:checked={excluded} label={m.admin_couchtv_excluded()} />
		</div>

		<div class="space-y-6 {excluded ? 'pointer-events-none opacity-40' : ''}" inert={excluded}>
			<div class="flex flex-wrap items-center gap-3">
				<span class="text-sm font-medium">{m.admin_couchtv_mood()}</span>
				<Select items={moods} bind:value={mood} class="w-40 justify-between" />
			</div>

			{#if files.length === 0}
				<p class="text-xs text-faint">{m.admin_couchtv_no_files()}</p>
			{:else}
				<section class="space-y-5 border-t border-edge/50 pt-5">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<h3 class="text-xs font-semibold tracking-widest text-faint uppercase">
							{m.admin_couchtv_promo_audio()}
						</h3>
						<span class="flex items-center gap-2 text-xs text-muted">
							{bed}
							{#if promoAdmin || soundtrackAdmin}
								<button
									type="button"
									class="text-accent-ink hover:underline"
									onclick={() => {
										promoAdmin = false;
										soundtrackAdmin = false;
									}}
								>
									{m.admin_couchtv_back_to_automatic()}
								</button>
							{/if}
						</span>
					</div>
					{#if ct.soundtrack && !ct.soundtrack.ready}
						<p class="text-[11px] text-faint">{m.admin_couchtv_bed_preparing()}</p>
					{/if}
					<SoundtrackPicker
						title={ct}
						chosen={soundtrackAdmin ? soundtrack : null}
						onchoose={(id) => {
							soundtrack = id;
							soundtrackAdmin = true;
							// one bed: an admin's excerpt would play instead
							promoAdmin = false;
						}}
						onsearch={searchSoundtrack}
					/>
					<div class="space-y-3">
						<div class="flex flex-wrap items-baseline justify-between gap-2">
							<h4 class="text-xs font-medium text-muted">{m.admin_couchtv_excerpt_heading()}</h4>
							<span class="text-[11px] text-faint">
								{promo ? source(promoAdmin) : m.admin_couchtv_promo_audio_none()}
							</span>
						</div>
						<p class="text-[11px] text-faint">{m.admin_couchtv_excerpt_hint()}</p>
						{#key `${title.title.id}:${adopted}`}
							<PromoAudioPicker
								{files}
								{grants}
								pick={promo}
								onpick={(p) => {
									promo = p;
									promoAdmin = true;
									soundtrackAdmin = false;
								}}
							/>
						{/key}
					</div>
				</section>

				<section class="space-y-3 border-t border-edge/50 pt-5">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<h3 class="text-xs font-semibold tracking-widest text-faint uppercase">
							{m.admin_couchtv_shots()}
						</h3>
						<span class="flex items-center gap-2 text-xs text-muted">
							{shots.length ? source(shotsAdmin) : ''}
							{#if shotsAdmin}
								<button
									type="button"
									class="text-accent-ink hover:underline"
									onclick={() => (shotsAdmin = false)}
								>
									{m.admin_couchtv_back_to_analysis()}
								</button>
							{/if}
						</span>
					</div>
					<ShotsEditor
						{shots}
						{files}
						{grants}
						onchange={(next) => {
							shots = next;
							shotsAdmin = true;
						}}
					/>
				</section>

				<section class="space-y-3 border-t border-edge/50 pt-5">
					<h3 class="text-xs font-semibold tracking-widest text-faint uppercase">
						{m.admin_couchtv_markers()}
					</h3>
					<MarkersEditor {files} movie={title.title.kind === 'movie'} />
				</section>
			{/if}
		</div>

		<div class="flex items-center justify-end gap-3 border-t border-edge/50 pt-4">
			<p class="mr-auto text-[11px] text-faint">{m.admin_couchtv_title_save_hint()}</p>
			<Button type="button" loading={saving} disabled={!form.dirty} onclick={save}>
				{m.library_save_changes()}
			</Button>
		</div>
	{/if}
</section>
