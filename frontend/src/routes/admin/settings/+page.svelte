<script lang="ts">
	import { toast } from 'svelte-sonner';
	import * as libraryApi from '$lib/features/library/api';
	import * as settingsApi from '$lib/features/settings/api';
	import HomeRowsEditor from '$lib/components/admin/HomeRowsEditor.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Checkbox from '$lib/components/ui/Checkbox.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';

	let tmdbKey = $state('');
	let savingTmdb = $state(false);

	let detectedEncoders = $state<string[]>([]);
	let hwAccel = $state('auto');
	let ladder = $state<string[]>(['720p']);
	let preset = $state('veryfast');
	let maxConcurrent = $state('1');
	let jit = $state('auto');
	let autoPrepare = $state(true);
	let savingTranscode = $state(false);

	const allRenditions = ['1080p', '720p', '480p'];

	$effect(() => {
		settingsApi.getSettings().then((s) => {
			tmdbKey = typeof s['tmdb.api_key'] === 'string' ? (s['tmdb.api_key'] as string) : '';
		});
		libraryApi.transcodeInfo().then((info) => {
			detectedEncoders = info.detectedEncoders;
			hwAccel = info.settings.hwAccel;
			ladder = info.settings.ladder;
			preset = info.settings.preset;
			maxConcurrent = String(info.settings.maxConcurrent);
			jit = info.settings.jitEnabled === null ? 'auto' : info.settings.jitEnabled ? 'on' : 'off';
			autoPrepare = info.settings.autoPrepare ?? true;
		});
	});

	async function saveTmdb(e: SubmitEvent) {
		e.preventDefault();
		savingTmdb = true;
		try {
			await settingsApi.putSettings({ 'tmdb.api_key': tmdbKey });
			toast.success('TMDB settings saved');
		} catch {
			toast.error('Failed to save settings');
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
					autoPrepare
				}
			});
			toast.success('Transcoding settings saved — concurrency applies after a restart');
		} catch {
			toast.error('Failed to save settings');
		} finally {
			savingTranscode = false;
		}
	}
</script>

<svelte:head>
	<title>Settings — Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Settings</h1>

<div class="grid max-w-4xl gap-6 lg:grid-cols-2">
	<form onsubmit={saveTmdb} class="space-y-4 rounded-card border border-edge bg-surface/40 p-6">
		<h2 class="text-sm font-semibold text-muted">Metadata</h2>
		<Input
			label="TMDB API key"
			bind:value={tmdbKey}
			placeholder="paste your themoviedb.org API key"
			autocomplete="off"
		/>
		<p class="text-xs leading-relaxed text-faint">
			With a key set, the title editor can search TMDB and fill in posters, overviews and genres
			automatically.
		</p>
		<div class="flex justify-end">
			<Button type="submit" loading={savingTmdb}>Save</Button>
		</div>
	</form>

	<form
		onsubmit={saveTranscode}
		class="space-y-4 rounded-card border border-edge bg-surface/40 p-6"
	>
		<h2 class="text-sm font-semibold text-muted">Transcoding</h2>

		<div>
			<p class="mb-1.5 text-xs font-medium text-muted">Hardware acceleration</p>
			<Select
				bind:value={hwAccel}
				items={[
					{ value: 'auto', label: 'Auto (best available)' },
					{ value: 'none', label: 'Software (libx264)' },
					...detectedEncoders.map((e) => ({ value: e, label: e }))
				]}
			/>
			<p class="mt-1.5 text-[11px] text-faint">
				Detected: {detectedEncoders.length ? detectedEncoders.join(', ') : 'none (software only)'}
			</p>
		</div>

		<div>
			<p class="mb-1.5 text-xs font-medium text-muted">Quality ladder (full transcodes)</p>
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
				<p class="mb-1.5 text-xs font-medium text-muted">x264 preset</p>
				<Select
					bind:value={preset}
					items={['ultrafast', 'veryfast', 'fast', 'medium'].map((p) => ({
						value: p,
						label: p
					}))}
				/>
			</div>
			<Input label="Max concurrent jobs" type="number" min="1" max="4" bind:value={maxConcurrent} />
		</div>

		<label class="flex items-center justify-between rounded-input border border-edge px-3.5 py-2.5">
			<span>
				<span class="block text-sm">Auto-prepare unplayable files</span>
				<span class="block text-[11px] text-faint">
					Queue background transcodes for files browsers can't play, right after scanning.
				</span>
			</span>
			<Switch bind:checked={autoPrepare} />
		</label>

		<div>
			<p class="mb-1.5 text-xs font-medium text-muted">Instant play (on-the-fly transcoding)</p>
			<Select
				bind:value={jit}
				items={[
					{ value: 'auto', label: 'Auto (on with a hardware encoder)' },
					{ value: 'on', label: 'Always on' },
					{ value: 'off', label: 'Off' }
				]}
			/>
			<p class="mt-1.5 text-[11px] text-faint">
				Plays unprepared files immediately by transcoding live while you watch. Heavy without
				hardware acceleration.
			</p>
		</div>

		<div class="flex justify-end">
			<Button type="submit" loading={savingTranscode}>Save</Button>
		</div>
	</form>

	<div class="lg:col-span-2">
		<HomeRowsEditor />
	</div>
</div>
