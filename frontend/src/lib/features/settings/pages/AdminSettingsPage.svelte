<script lang="ts">
	import { page } from '$app/state';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import * as libraryApi from '$lib/features/library/api';
	import * as settingsApi from '$lib/features/settings/api';
	import { features } from '$lib/features/settings/features.svelte';
	import { applyAccent } from '$lib/theme';
	import { FormState } from '$lib/utils/form-state.svelte';
	import HomeRowsEditor from '$lib/features/settings/components/HomeRowsEditor.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Checkbox from '$lib/components/ui/Checkbox.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import * as m from '$lib/paraglide/messages';

	let tab = $state(page.url.searchParams.get('tab') ?? 'general');

	let tmdbKey = $state('');
	let savingTmdb = $state(false);
	const tmdbForm = new FormState(() => ({ tmdbKey }));

	let detectedEncoders = $state<string[]>([]);
	let detecting = $state(false);
	let hwAccel = $state('auto');
	let ladder = $state<string[]>(['720p']);
	let preset = $state('veryfast');
	let maxConcurrent = $state('1');
	let jit = $state('auto');
	let autoPrepare = $state(true);
	let deleteSource = $state(false);
	let savingTranscode = $state(false);
	const transcodeForm = new FormState(() => ({
		hwAccel,
		ladder,
		preset,
		maxConcurrent,
		jit,
		autoPrepare,
		deleteSource
	}));

	let musicEnabled = $state(true);
	let couchEnabled = $state(true);
	let savingFeatures = $state(false);
	const featuresForm = new FormState(() => ({ musicEnabled, couchEnabled }));

	let featuredCount = $state('3');
	let savingHome = $state(false);
	const homeForm = new FormState(() => ({ featuredCount }));

	let accent = $state('#e50914');
	let savingAccent = $state(false);
	const accentForm = new FormState(() => ({ accent }));
	const accentPresets = ['#e50914', '#8b7cf0', '#3b82f6', '#10b981', '#f59e0b', '#ec4899'];

	const allRenditions = ['1080p', '720p', '480p'];

	function applyTranscodeInfo(info: libraryApi.TranscodeInfo) {
		detectedEncoders = info.detectedEncoders;
		detecting = info.detecting;
		if (!transcodeForm.dirty) {
			hwAccel = info.settings.hwAccel;
			ladder = info.settings.ladder;
			preset = info.settings.preset;
			maxConcurrent = String(info.settings.maxConcurrent);
			jit = info.settings.jitEnabled === null ? 'auto' : info.settings.jitEnabled ? 'on' : 'off';
			autoPrepare = info.settings.autoPrepare ?? true;
			deleteSource = info.settings.deleteSourceAfterTranscode ?? false;
			transcodeForm.reset();
		}
	}

	$effect(() => {
		settingsApi
			.getSettings()
			.then((s) => {
				if (!tmdbForm.dirty) {
					tmdbKey = typeof s['tmdb.api_key'] === 'string' ? (s['tmdb.api_key'] as string) : '';
					tmdbForm.reset();
				}
				if (!featuresForm.dirty) {
					const flags = s.features as
						| { musicEnabled?: boolean; couchEnabled?: boolean }
						| undefined;
					musicEnabled = flags?.musicEnabled ?? true;
					couchEnabled = flags?.couchEnabled ?? true;
					featuresForm.reset();
				}
				if (!accentForm.dirty) {
					const appearance = s.appearance as { accent?: string } | undefined;
					accent = appearance?.accent ?? '#e50914';
					accentForm.reset();
				}
				if (!homeForm.dirty) {
					const homeCfg = s.home as { featuredCount?: number } | undefined;
					featuredCount = String(homeCfg?.featuredCount ?? 3);
					homeForm.reset();
				}
			})
			.catch(() => toast.error(m.settings_load_failed()));

		let refetch: ReturnType<typeof setTimeout> | undefined;
		libraryApi
			.transcodeInfo()
			.then((info) => {
				applyTranscodeInfo(info);
				if (info.detecting) {
					// encoder detection runs in the background; pick up late arrivals
					refetch = setTimeout(() => {
						libraryApi
							.transcodeInfo()
							.then(applyTranscodeInfo)
							.catch(() => {});
					}, 3000);
				}
			})
			.catch(() => toast.error(m.settings_load_failed()));
		return () => clearTimeout(refetch);
	});

	async function saveTmdb(e: SubmitEvent) {
		e.preventDefault();
		savingTmdb = true;
		try {
			await settingsApi.putSettings({ 'tmdb.api_key': tmdbKey });
			tmdbForm.reset();
			toast.success(m.settings_tmdb_saved());
		} catch {
			toast.error(m.settings_save_failed());
		} finally {
			savingTmdb = false;
		}
	}

	function toggleRendition(name: string, on: boolean) {
		ladder = on ? [...new Set([...ladder, name])] : ladder.filter((r) => r !== name);
	}

	async function saveTranscode(e: SubmitEvent) {
		e.preventDefault();
		savingTranscode = true;
		try {
			await settingsApi.putSettings({
				transcode: {
					hwAccel,
					ladder,
					preset,
					maxConcurrent: Math.max(1, Number(maxConcurrent) || 1),
					jitEnabled: jit === 'auto' ? null : jit === 'on',
					autoPrepare,
					deleteSourceAfterTranscode: deleteSource
				}
			});
			transcodeForm.reset();
			toast.success(m.settings_transcoding_saved());
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.settings_save_failed());
		} finally {
			savingTranscode = false;
		}
	}

	async function saveHome(e: SubmitEvent) {
		e.preventDefault();
		savingHome = true;
		try {
			const count = Math.min(10, Math.max(1, Number(featuredCount) || 3));
			await settingsApi.putSettings({ home: { featuredCount: count } });
			featuredCount = String(count);
			homeForm.reset();
			toast.success(m.settings_home_saved());
		} catch {
			toast.error(m.settings_save_failed());
		} finally {
			savingHome = false;
		}
	}

	async function saveFeatures(e: SubmitEvent) {
		e.preventDefault();
		savingFeatures = true;
		try {
			await settingsApi.putSettings({ features: { musicEnabled, couchEnabled } });
			features.musicEnabled = musicEnabled;
			features.couchEnabled = couchEnabled;
			featuresForm.reset();
			toast.success(m.settings_features_saved());
		} catch {
			toast.error(m.settings_save_failed());
		} finally {
			savingFeatures = false;
		}
	}

	// preview the accent live as the admin edits it
	$effect(() => applyAccent(accent));

	async function saveAccent(e: SubmitEvent) {
		e.preventDefault();
		savingAccent = true;
		try {
			await settingsApi.putSettings({ appearance: { accent } });
			accentForm.reset();
			toast.success(m.settings_accent_saved());
		} catch {
			toast.error(m.settings_save_failed());
		} finally {
			savingAccent = false;
		}
	}
</script>

<svelte:head>
	<title>{m.settings_page_title()}</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">{m.settings_heading()}</h1>

<div class="mb-6">
	<Tabs
		bind:value={tab}
		items={[
			{ value: 'general', label: m.settings_tab_general() },
			{ value: 'appearance', label: m.settings_tab_appearance() },
			{ value: 'transcoding', label: m.settings_tab_transcoding() },
			{ value: 'features', label: m.settings_tab_features() }
		]}
	/>
</div>

{#key tab}
	<div in:fly|global={{ y: 10, duration: 250 }}>
		{#if tab === 'general'}
			<div class="max-w-4xl space-y-6">
				<form
					onsubmit={saveTmdb}
					class="max-w-xl space-y-4 rounded-card border border-edge bg-surface/40 p-6"
				>
					<h2 class="text-sm font-semibold text-muted">{m.settings_metadata_heading()}</h2>
					<Input
						label={m.settings_tmdb_api_key_label()}
						bind:value={tmdbKey}
						placeholder={m.settings_tmdb_api_key_placeholder()}
						autocomplete="off"
					/>
					<p class="text-xs leading-relaxed text-faint">
						{m.settings_tmdb_api_key_hint()}
					</p>
					<div class="flex justify-end">
						<Button type="submit" loading={savingTmdb} disabled={!tmdbForm.dirty}
							>{m.common_save()}</Button
						>
					</div>
				</form>

				<form
					onsubmit={saveHome}
					class="max-w-xl space-y-4 rounded-card border border-edge bg-surface/40 p-6"
				>
					<h2 class="text-sm font-semibold text-muted">{m.settings_home_heading()}</h2>
					<Input
						label={m.settings_home_featured_count_label()}
						type="number"
						min="1"
						max="10"
						bind:value={featuredCount}
						class="w-28"
					/>
					<p class="text-xs leading-relaxed text-faint">
						{m.settings_home_featured_count_hint()}
					</p>
					<div class="flex justify-end">
						<Button type="submit" loading={savingHome} disabled={!homeForm.dirty}
							>{m.common_save()}</Button
						>
					</div>
				</form>

				<HomeRowsEditor />
			</div>
		{:else if tab === 'appearance'}
			<form
				onsubmit={saveAccent}
				class="max-w-xl space-y-4 rounded-card border border-edge bg-surface/40 p-6"
			>
				<h2 class="text-sm font-semibold text-muted">{m.settings_accent_heading()}</h2>
				<p class="text-xs leading-relaxed text-faint">
					{m.settings_accent_hint()}
				</p>

				<div class="flex items-center gap-3">
					<input
						type="color"
						bind:value={accent}
						class="size-11 cursor-pointer rounded-input border border-edge bg-transparent"
						aria-label={m.settings_accent_heading()}
					/>
					<Input
						bind:value={accent}
						class="w-32 font-mono"
						aria-label={m.settings_accent_hex_label()}
					/>
				</div>

				<div class="flex flex-wrap gap-2">
					{#each accentPresets as preset (preset)}
						<button
							type="button"
							onclick={() => (accent = preset)}
							class="size-7 rounded-full border-2 transition-transform hover:scale-110
						{accent.toLowerCase() === preset ? 'border-text' : 'border-transparent'}"
							style="background: {preset}"
							aria-label={preset}
						></button>
					{/each}
				</div>

				<div class="flex justify-end pt-1">
					<Button type="submit" loading={savingAccent} disabled={!accentForm.dirty}
						>{m.common_save()}</Button
					>
				</div>
			</form>
		{:else if tab === 'transcoding'}
			<form
				onsubmit={saveTranscode}
				class="max-w-xl space-y-4 rounded-card border border-edge bg-surface/40 p-6"
			>
				<h2 class="text-sm font-semibold text-muted">{m.settings_tab_transcoding()}</h2>

				<div>
					<p class="mb-1.5 text-xs font-medium text-muted">{m.settings_hwaccel_label()}</p>
					<Select
						bind:value={hwAccel}
						items={[
							{ value: 'auto', label: m.settings_hwaccel_auto() },
							{ value: 'none', label: m.settings_hwaccel_software() },
							...detectedEncoders.map((e) => ({ value: e, label: e }))
						]}
					/>
					<p class="mt-1.5 text-[11px] text-faint">
						{m.settings_detected_label()}{' '}{detecting
							? m.settings_detecting_encoders()
							: detectedEncoders.length
								? detectedEncoders.join(', ')
								: m.settings_detected_none()}
					</p>
				</div>

				<div>
					<p class="mb-1.5 text-xs font-medium text-muted">{m.settings_quality_ladder_label()}</p>
					<div class="flex gap-4">
						{#each allRenditions as rendition (rendition)}
							<label class="flex items-center gap-2 text-sm">
								<Checkbox
									checked={ladder.includes(rendition)}
									onCheckedChange={(on) => toggleRendition(rendition, on)}
								/>
								{rendition}
							</label>
						{/each}
					</div>
				</div>

				<div class="grid grid-cols-2 gap-4">
					<div>
						<p class="mb-1.5 text-xs font-medium text-muted">{m.settings_x264_preset_label()}</p>
						<Select
							bind:value={preset}
							items={['ultrafast', 'veryfast', 'fast', 'medium'].map((p) => ({
								value: p,
								label: p
							}))}
						/>
					</div>
					<Input
						label={m.settings_max_concurrent_label()}
						type="number"
						min="1"
						max="4"
						bind:value={maxConcurrent}
					/>
				</div>

				<label
					class="flex items-center justify-between rounded-input border border-edge px-3.5 py-2.5"
				>
					<span>
						<span class="block text-sm">{m.settings_auto_prepare_label()}</span>
						<span class="block text-[11px] text-faint">
							{m.settings_auto_prepare_hint()}
						</span>
					</span>
					<Switch bind:checked={autoPrepare} />
				</label>

				<label
					class="flex items-center justify-between rounded-input border border-edge px-3.5 py-2.5"
				>
					<span>
						<span class="block text-sm">{m.settings_delete_original_label()}</span>
						<span class="block text-[11px] text-faint">
							{m.settings_delete_original_hint()}
						</span>
					</span>
					<Switch bind:checked={deleteSource} />
				</label>

				<div>
					<p class="mb-1.5 text-xs font-medium text-muted">{m.settings_instant_play_label()}</p>
					<Select
						bind:value={jit}
						items={[
							{ value: 'auto', label: m.settings_instant_play_auto() },
							{ value: 'on', label: m.settings_instant_play_on() },
							{ value: 'off', label: m.settings_instant_play_off() }
						]}
					/>
					<p class="mt-1.5 text-[11px] text-faint">
						{m.settings_instant_play_hint()}
					</p>
				</div>

				<div class="flex justify-end">
					<Button type="submit" loading={savingTranscode} disabled={!transcodeForm.dirty}
						>{m.common_save()}</Button
					>
				</div>
			</form>
		{:else if tab === 'features'}
			<form
				onsubmit={saveFeatures}
				class="max-w-xl space-y-4 rounded-card border border-edge bg-surface/40 p-6"
			>
				<h2 class="text-sm font-semibold text-muted">{m.settings_tab_features()}</h2>

				<label
					class="flex items-center justify-between rounded-input border border-edge px-3.5 py-2.5"
				>
					<span>
						<span class="block text-sm">{m.settings_music_library_label()}</span>
						<span class="block text-[11px] text-faint">
							{m.settings_music_library_hint()}
						</span>
					</span>
					<Switch bind:checked={musicEnabled} />
				</label>

				<label
					class="flex items-center justify-between rounded-input border border-edge px-3.5 py-2.5"
				>
					<span>
						<span class="block text-sm">{m.settings_couch_sessions_label()}</span>
						<span class="block text-[11px] text-faint">
							{m.settings_couch_sessions_hint()}
						</span>
					</span>
					<Switch bind:checked={couchEnabled} />
				</label>

				<div class="flex justify-end">
					<Button type="submit" loading={savingFeatures} disabled={!featuresForm.dirty}
						>{m.common_save()}</Button
					>
				</div>
			</form>
		{/if}
	</div>
{/key}
