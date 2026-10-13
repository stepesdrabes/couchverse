// The render page's API (dist/render.html), driven by the sidecar over CDP. Every frame and
// every sample is a function of t and the spec: seek, wait for the promise, capture.

import { Kit } from './audio/kit';
import type { LoudnessReport } from './audio/loudness';
import { encodeWav } from './audio/wav';
import type { Cue, KeyRect } from './core/timeline';
import { FPS } from './core/timeline';
import {
	buildBreak,
	buildPiece,
	renderItemAudio,
	renderPieceAudio,
	type AudioRender,
	type Break,
	type Piece
} from './piece';
import type { Spec, TemplateName } from './spec';
import { Stage, type Layer } from './stage';
import { TEMPLATE_NAMES, TEMPLATES } from './templates';

export interface LoadResult {
	/** Seconds, a whole number of frames. */
	duration: number;
	/**
	 * Seconds the clip's sound lasts as an item of a break (where the next item starts), a whole
	 * number of frames: its duration, but a promo body's runs on through its end card.
	 */
	soundDuration: number;
	cues: Cue[];
	fps: number;
	width: number;
	height: number;
	/** The template's version, for the clip cache key (0 for a break). */
	version: number;
	/** The clip loops seamlessly (standby). */
	loop: boolean;
	/** The clip has a key rectangle (`keyRect`, the `matte` layer). */
	keyed: boolean;
	/** URLs that failed to load (stand-ins were used). */
	missing: string[];
}

export interface CouchTvRender {
	/**
	 * Every template's version, so the server keys its clip cache without loading a spec
	 * (`load` reports the same number for the spec's template).
	 */
	versions(): Record<TemplateName, number>;
	/**
	 * Load a clip spec (picture and sound) or a break spec (sound only). `sound` builds a clip
	 * for its sound alone, never shown.
	 */
	load(spec: Spec, options?: { sound?: boolean }): Promise<LoadResult>;
	/** Show the clip's frame at t (seconds); resolves once it is painted. */
	seek(t: number): Promise<void>;
	/** Whether the clip's frames at t0 and t1 are identical (nothing animates in between). */
	isStatic(t0: number, t1: number): boolean;
	/**
	 * The renderer's step to the frame at t from the one at `from` (negative for the first):
	 * false when nothing changes in between (the last capture can be reused), else the frame is
	 * set up and its images decoded. Unlike `seek` it does not wait for a paint: a screenshot
	 * draws a fresh frame of its own.
	 */
	advance(t: number, from: number): Promise<boolean>;
	/** The squeeze's program rectangle at t (stage pixels), or null. */
	keyRect(t: number): KeyRect | null;
	/** `matte`: white where the program shows, black elsewhere (the composite's alpha). */
	setLayer(layer: Layer): void;
	/** The loaded clip's or break's sound: 48 kHz stereo float WAV at the spec's loudness. */
	renderAudio(): Promise<ArrayBuffer>;
	/**
	 * The loaded clip's sound as an item of a break (48 kHz stereo float WAV): from its start,
	 * faded out from `soundDuration` over the break's crossfade with its ring-out after that, at
	 * the engine's level, not normalized. A break is these renders mixed at their starts, then
	 * normalized.
	 */
	renderItemAudio(): Promise<ArrayBuffer>;
	/** `renderAudio` as base64, for callers that can only move strings (CDP). */
	renderAudioBase64(): Promise<string>;
	/** Loudness of the last audio render. */
	audioReport(): LoudnessReport | null;
}

declare global {
	interface Window {
		couchtvRender: CouchTvRender;
	}
}

const stage = new Stage(document.getElementById('stage') as HTMLElement, { fit: false });
let piece: Piece | null = null;
let brk: Break | null = null;
let report: LoudnessReport | null = null;

const painted = () =>
	new Promise<void>((resolve) =>
		requestAnimationFrame(() => requestAnimationFrame(() => resolve()))
	);

function clip(): Piece {
	if (!piece) throw new Error('no clip loaded (a break renders sound only)');
	return piece;
}

async function audio(): Promise<AudioRender> {
	const out = brk ? await brk.render() : await renderPieceAudio(clip());
	report = out.report;
	return out;
}

const base = { fps: FPS, width: 1920, height: 1080 };

window.couchtvRender = {
	versions: () =>
		Object.fromEntries(TEMPLATE_NAMES.map((n) => [n, TEMPLATES[n].version])) as Record<
			TemplateName,
			number
		>,
	async load(spec, { sound = false } = {}) {
		piece?.dispose();
		brk?.dispose();
		piece = brk = null;
		report = null;
		stage.setLayer('picture');
		if (spec.template === 'break') {
			brk = await buildBreak(stage, spec);
			return {
				...base,
				duration: brk.duration,
				soundDuration: brk.duration,
				cues: [],
				version: 0,
				loop: false,
				keyed: false,
				missing: []
			};
		}
		piece = await buildPiece(stage, spec, new Kit(), { hidden: sound });
		if (!sound) {
			await piece.seek(0);
			await painted();
		}
		return {
			...base,
			duration: piece.duration,
			soundDuration: piece.soundDuration,
			cues: piece.cues,
			version: piece.version,
			loop: piece.loop,
			keyed: !!piece.tl.keyRect,
			missing: [...piece.assets.missing]
		};
	},
	async seek(t) {
		await clip().seek(t);
		await painted();
	},
	isStatic: (t0, t1) => clip().isStatic(t0, t1),
	async advance(t, from) {
		if (from >= 0 && clip().isStatic(from, t)) return false;
		await clip().seek(t);
		return true;
	},
	keyRect: (t) => clip().keyRect(t),
	setLayer: (layer) => stage.setLayer(layer),
	async renderAudio() {
		const out = await audio();
		return encodeWav(out.channels, out.sampleRate);
	},
	async renderItemAudio() {
		const out = await renderItemAudio(clip());
		return encodeWav(out.channels, out.sampleRate);
	},
	async renderAudioBase64() {
		const out = await audio();
		const bytes = new Uint8Array(encodeWav(out.channels, out.sampleRate));
		let s = '';
		for (let i = 0; i < bytes.length; i += 0x8000)
			s += String.fromCharCode(...bytes.subarray(i, i + 0x8000));
		return btoa(s);
	},
	audioReport: () => report
};
