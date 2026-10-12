<script lang="ts" module>
	export interface AudioPick {
		mediaFileId: string;
		startMs: number;
		durationMs: number;
	}
</script>

<script lang="ts">
	import { ChevronLeft, ChevronRight, Pause, Play } from 'lucide-svelte';
	import { onDestroy } from 'svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { problemMessage } from '$lib/api/problem';
	import * as m from '$lib/paraglide/messages';
	import { adminCouchTvWaveform, type CouchTvWaveform } from '../api';
	import { fitExcerpt, type AiredFile } from '../files';
	import { attachStream, type Grants } from '../media';
	import { formatLength, parseLength } from '../time';
	import Waveform from './Waveform.svelte';

	// The promo soundtrack excerpt: a file, a stretch of its waveform (five minutes at a
	// time) and a selection of up to 40 s, heard through the file's media grant.
	let {
		files,
		grants,
		pick,
		onpick
	}: {
		files: AiredFile[];
		grants: Grants;
		/** the excerpt promos use now (the analysis' or the admin's) */
		pick: AudioPick | null;
		onpick: (pick: AudioPick) => void;
	} = $props();

	const MAX = 40_000;
	const WINDOW = 5 * 60_000;

	// the picker opens on the current excerpt, else a minute into the first file
	// svelte-ignore state_referenced_locally
	const opening = pick ?? {
		mediaFileId: files[0]?.fileId ?? '',
		...fitExcerpt(60_000, 30_000, files[0]?.durationMs ?? 0, MAX)
	};
	let fileId = $state(opening.mediaFileId);
	let start = $state(opening.startMs);
	let length = $state(opening.durationMs);
	let from = $state(Math.max(0, opening.startMs - 60_000));
	let wave = $state<CouchTvWaveform | null>(null);
	let waveError = $state('');
	let startText = $state(formatLength(start));
	let lengthText = $state(String(length / 1000));

	let audio = $state<HTMLAudioElement>();
	let attachedTo = '';
	let detach: (() => void) | null = null;
	let playing = $state(false);
	let playhead = $state<number | null>(null);
	let playError = $state('');

	const file = $derived(files.find((f) => f.fileId === fileId));
	const fileItems = $derived(files.map((f) => ({ value: f.fileId, label: f.label })));

	$effect(() => {
		const id = fileId;
		const lo = from;
		if (!id) return;
		let live = true;
		const hi = Math.min(lo + WINDOW, file?.durationMs ?? lo + WINDOW);
		adminCouchTvWaveform(id, { fromMs: lo, toMs: Math.max(hi, lo + 1000), points: 300 })
			.then((w) => {
				if (live) {
					wave = w;
					waveError = '';
				}
			})
			.catch((err) => {
				if (live) {
					wave = null;
					waveError = problemMessage(err, m.admin_couchtv_waveform_failed());
				}
			});
		return () => {
			live = false;
		};
	});

	function select(s: number, l: number) {
		({ startMs: start, durationMs: length } = fitExcerpt(s, l, file?.durationMs ?? Infinity, MAX));
		startText = formatLength(start);
		lengthText = String(Math.round(length / 100) / 10);
		onpick({ mediaFileId: fileId, startMs: start, durationMs: length });
	}

	function typed() {
		const s = parseLength(startText);
		const l = Number(lengthText) * 1000;
		if (Number.isNaN(s) || !(l > 0)) return;
		select(s, l);
		if (s < from || s >= from + WINDOW) from = Math.max(0, s - 60_000);
	}

	function chooseFile(id: string) {
		stop();
		fileId = id;
		from = 0;
		select(60_000, length);
	}

	function shift(by: number) {
		const end = file?.durationMs ?? Infinity;
		from = Math.min(Math.max(0, from + by), Math.max(0, end - WINDOW));
	}

	async function toggle() {
		if (!audio || !file) return;
		if (playing) return stop();
		playError = '';
		if (attachedTo !== fileId) {
			try {
				detach?.();
				detach = await attachStream(audio, await grants.get(file));
				attachedTo = fileId;
			} catch {
				playError = m.admin_couchtv_audio_unplayable();
				return;
			}
		}
		try {
			audio.currentTime = start / 1000;
			await audio.play();
			playing = true;
		} catch {
			playError = m.admin_couchtv_audio_failed();
			playing = false;
		}
	}

	function stop() {
		audio?.pause();
		playing = false;
		playhead = null;
	}

	function tick() {
		if (!audio || !playing) return;
		const t = audio.currentTime * 1000;
		playhead = t;
		if (t >= start + length) stop();
	}

	onDestroy(() => detach?.());
</script>

<div class="space-y-3">
	{#if fileItems.length > 1}
		<Select
			items={fileItems}
			value={fileId}
			onchange={chooseFile}
			label={m.admin_couchtv_file()}
			class="max-w-full justify-between"
		/>
	{/if}

	{#if wave}
		<Waveform
			data={wave}
			{start}
			{length}
			max={MAX}
			{playhead}
			label={m.admin_couchtv_waveform_label({
				start: formatLength(start),
				length: formatLength(length)
			})}
			onselect={select}
		/>
	{:else if waveError}
		<p class="rounded-lg bg-surface px-3 py-6 text-center text-xs text-muted">{waveError}</p>
	{:else}
		<div class="h-28 animate-pulse rounded-lg bg-surface"></div>
	{/if}

	<div class="flex flex-wrap items-end gap-3">
		<div class="flex items-center gap-1">
			<Button
				variant="ghost"
				size="sm"
				aria-label={m.admin_couchtv_waveform_earlier()}
				onclick={() => shift(-4 * 60_000)}
			>
				<ChevronLeft class="size-4" />
			</Button>
			<span class="text-[11px] text-faint tnum">
				{formatLength(from)}-{formatLength(
					Math.min(from + WINDOW, file?.durationMs ?? from + WINDOW)
				)}
			</span>
			<Button
				variant="ghost"
				size="sm"
				aria-label={m.admin_couchtv_waveform_later()}
				onclick={() => shift(4 * 60_000)}
			>
				<ChevronRight class="size-4" />
			</Button>
		</div>
		<Input
			label={m.admin_couchtv_excerpt_start()}
			bind:value={startText}
			onchange={typed}
			class="w-28"
			inputmode="decimal"
		/>
		<Input
			label={m.admin_couchtv_excerpt_length()}
			type="number"
			min="1"
			max="40"
			step="0.5"
			bind:value={lengthText}
			onchange={typed}
			class="w-24"
		/>
		<Button variant="secondary" size="sm" onclick={toggle} disabled={!file}>
			{#if playing}
				<Pause class="size-4" />{m.common_pause()}
			{:else}
				<Play class="size-4" />{m.admin_couchtv_excerpt_play()}
			{/if}
		</Button>
	</div>
	{#if playError}<p class="text-xs text-danger">{playError}</p>{/if}
	<audio bind:this={audio} ontimeupdate={tick} onended={stop} class="hidden"></audio>
</div>
